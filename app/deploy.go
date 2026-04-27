package app

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

const (
	deployMaxSize  = 50 << 20 // 50 MB total
	deployMaxChunk = 1 << 20  // 1 MB per chunk
)

// deploySession holds chunks being uploaded.
type deploySession struct {
	parts map[int][]byte
	total int
}

var (
	deploySessions   = map[string]*deploySession{}
	deploySessionsMu sync.Mutex
)

func (s *Smesh3Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if s.dir == "" {
		http.Error(w, "deploy requires disk mode (ORLY_SMESH3_DIR)", http.StatusBadRequest)
		return
	}

	action := r.URL.Query().Get("action")

	switch action {
	case "begin":
		s.deployBegin(w, r)
	case "part":
		s.deployPart(w, r)
	case "apply":
		s.deployApply(w, r)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
	}
}

func (s *Smesh3Server) deployBegin(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	totalStr := r.URL.Query().Get("parts")
	total, err := strconv.Atoi(totalStr)
	if err != nil || total <= 0 || total > 1000 || len(hash) != 64 {
		http.Error(w, "bad params", http.StatusBadRequest)
		return
	}

	deploySessionsMu.Lock()
	deploySessions[hash] = &deploySession{
		parts: make(map[int][]byte),
		total: total,
	}
	deploySessionsMu.Unlock()

	log.I.F("smesh deploy: begin hash=%s parts=%d", hash[:16], total)
	fmt.Fprint(w, "ok")
}

func (s *Smesh3Server) deployPart(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	indexStr := r.URL.Query().Get("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil || len(hash) != 64 {
		http.Error(w, "bad params", http.StatusBadRequest)
		return
	}

	deploySessionsMu.Lock()
	sess, ok := deploySessions[hash]
	deploySessionsMu.Unlock()
	if !ok {
		http.Error(w, "no session", http.StatusBadRequest)
		return
	}

	if index < 0 || index >= sess.total {
		http.Error(w, "bad index", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, deployMaxChunk+1))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	if len(body) > deployMaxChunk {
		http.Error(w, "chunk too large", http.StatusRequestEntityTooLarge)
		return
	}

	deploySessionsMu.Lock()
	sess.parts[index] = body
	got := len(sess.parts)
	deploySessionsMu.Unlock()

	fmt.Fprintf(w, "ok %d/%d", got, sess.total)
}

func (s *Smesh3Server) deployApply(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	sigHex := r.Header.Get("X-Sig")
	if len(hash) != 64 || sigHex == "" {
		http.Error(w, "bad params", http.StatusBadRequest)
		return
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != 64 {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	deploySessionsMu.Lock()
	sess, ok := deploySessions[hash]
	if ok {
		delete(deploySessions, hash)
	}
	deploySessionsMu.Unlock()
	if !ok {
		http.Error(w, "no session", http.StatusBadRequest)
		return
	}

	if len(sess.parts) != sess.total {
		http.Error(w, fmt.Sprintf("incomplete: %d/%d parts", len(sess.parts), sess.total), http.StatusBadRequest)
		return
	}

	// Reassemble.
	var bundle []byte
	for i := 0; i < sess.total; i++ {
		p, ok := sess.parts[i]
		if !ok {
			http.Error(w, fmt.Sprintf("missing part %d", i), http.StatusBadRequest)
			return
		}
		bundle = append(bundle, p...)
	}

	if len(bundle) > deployMaxSize {
		http.Error(w, "bundle too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Verify hash matches.
	computed := sha256.Sum256(bundle)
	if hex.EncodeToString(computed[:]) != hash {
		http.Error(w, "hash mismatch", http.StatusBadRequest)
		return
	}

	// Verify BIP-340 signature.
	verifier, err := p8k.New()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := verifier.InitPub(s.deployPub); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	ok2, err := verifier.Verify(computed[:], sig)
	if err != nil || !ok2 {
		log.W.F("smesh deploy: signature verification failed")
		http.Error(w, "signature verification failed", http.StatusForbidden)
		return
	}

	// Extract and swap via symlink pivot.
	s.extractAndSwap(w, bundle, computed)
}

func (s *Smesh3Server) extractAndSwap(w http.ResponseWriter, bundle []byte, hash [32]byte) {
	hashHex := hex.EncodeToString(hash[:8])
	newDir := s.dir + "-" + hashHex

	os.RemoveAll(newDir)
	if err := extractTarXz(bundle, newDir); err != nil {
		os.RemoveAll(newDir)
		log.E.F("smesh deploy: extract failed: %v", err)
		http.Error(w, "extract failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Resolve s.dir: could be a symlink (subsequent deploy) or real dir (first deploy).
	tmpLink := s.dir + ".new"
	os.Remove(tmpLink)

	if err := os.Symlink(newDir, tmpLink); err != nil {
		os.RemoveAll(newDir)
		log.E.F("smesh deploy: symlink failed: %v", err)
		http.Error(w, "deploy failed", http.StatusInternalServerError)
		return
	}

	// Read old target before swap (if s.dir is already a symlink).
	oldTarget, _ := os.Readlink(s.dir)

	// Try atomic rename (works when s.dir is a symlink).
	err := os.Rename(tmpLink, s.dir)
	if err != nil {
		// First deploy: s.dir is a real directory. Migrate to symlink.
		os.Remove(tmpLink)
		oldDir := s.dir + ".old"
		os.RemoveAll(oldDir)
		if err := os.Rename(s.dir, oldDir); err != nil {
			os.RemoveAll(newDir)
			log.E.F("smesh deploy: rename live→old failed: %v", err)
			http.Error(w, "deploy failed", http.StatusInternalServerError)
			return
		}
		if err := os.Symlink(newDir, s.dir); err != nil {
			os.Rename(oldDir, s.dir) // rollback
			os.RemoveAll(newDir)
			log.E.F("smesh deploy: symlink failed: %v", err)
			http.Error(w, "deploy failed", http.StatusInternalServerError)
			return
		}
		os.RemoveAll(oldDir)
		oldTarget = "" // nothing more to clean
	}

	// Clean up previous version.
	if oldTarget != "" {
		os.RemoveAll(oldTarget)
	}

	s.notifyFullRefresh()

	log.I.F("smesh deploy: applied %s (%d bytes xz)", hashHex, len(bundle))
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "ok version=%d\n", s.version)
}

// extractTarXz decompresses xz data and extracts the tar archive to dest.
func extractTarXz(data []byte, dest string) error {
	cmd := exec.Command("xz", "-d", "--stdout")
	cmd.Stdin = bytes.NewReader(data)

	tarData, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("xz decompress: %w", err)
	}

	tr := tar.NewReader(bytes.NewReader(tarData))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		clean := filepath.Clean(hdr.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("bad path in tar: %s", hdr.Name)
		}
		target := filepath.Join(dest, clean)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}
