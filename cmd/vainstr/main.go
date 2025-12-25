// Package main is a simple nostr key miner that uses the fast bitcoin secp256k1
// C library to derive npubs with specified prefix/infix/suffix strings present.
package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/crypto/ec/bech32"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"

	"github.com/alexflint/go-arg"
)

var prefix = append(bech32encoding.PubHRP, '1')

const (
	PositionBeginning = iota
	PositionContains
	PositionEnding
)

type Result struct {
	sec  []byte
	npub []byte
	pub  []byte
}

var args struct {
	String   string `arg:"positional" help:"the string you want to appear in the npub"`
	Position string `arg:"positional" default:"end" help:"[begin|contain|end] default: end"`
	Threads  int    `help:"number of threads to mine with - defaults to using all CPU threads available"`
}

func main() {
	arg.MustParse(&args)
	if args.String == "" {
		_, _ = fmt.Fprintln(
			os.Stderr,
			`Usage: vainstr [--threads THREADS] [STRING [POSITION]]

Positional arguments:
  STRING                 the string you want to appear in the npub
  POSITION               [begin|contain|end] default: end

Options:
  --threads THREADS      number of threads to mine with - defaults to using all CPU threads available
  --help, -h             display this help and exit`,
		)
		os.Exit(0)
	}
	var where int
	canonical := strings.ToLower(args.Position)
	switch {
	case strings.HasPrefix(canonical, "begin"):
		where = PositionBeginning
	case strings.Contains(canonical, "contain"):
		where = PositionContains
	case strings.HasSuffix(canonical, "end"):
		where = PositionEnding
	}
	if args.Threads == 0 {
		args.Threads = runtime.NumCPU()
	}
	if err := Vanity(args.String, where, args.Threads); chk.E(err) {
		log.F.F("error: %s", err)
	}
}

func Vanity(str string, where int, threads int) (err error) {
	// check the string has valid bech32 ciphers
	for i := range str {
		wrong := true
		for j := range bech32.Charset {
			if str[i] == bech32.Charset[j] {
				wrong = false
				break
			}
		}
		if wrong {
			return fmt.Errorf(
				"found invalid character '%c' only ones from '%s' allowed\n",
				str[i], bech32.Charset,
			)
		}
	}
	started := time.Now()
	quit := make(chan struct{})
	resC := make(chan Result)

	// Handle interrupt
	go func() {
		c := make(chan os.Signal, 1)
		<-c
		close(quit)
		log.I.Ln("\rinterrupt signal received")
		os.Exit(0)
	}()

	var wg sync.WaitGroup
	var counter int64
	for i := 0; i < threads; i++ {
		log.D.F("starting up worker %d", i)
		go mine(str, where, quit, resC, &wg, &counter)
	}
	tick := time.NewTicker(time.Second * 5)
	var res Result
out:
	for {
		select {
		case <-tick.C:
			workingFor := time.Now().Sub(started)
			wm := workingFor % time.Second
			workingFor -= wm
			fmt.Printf(
				" working for %v, attempts %d",
				workingFor, atomic.LoadInt64(&counter),
			)
		case r := <-resC:
			// one of the workers found the solution
			res = r
			// tell the others to stop
			close(quit)
			break out
		}
	}

	// wait for all workers to stop
	wg.Wait()

	fmt.Printf(
		"\r# generated in %d attempts using %d threads, taking %v                                                 ",
		atomic.LoadInt64(&counter), args.Threads, time.Now().Sub(started),
	)
	fmt.Printf(
		"\nHSEC = %s\nHPUB = %s\n",
		hex.EncodeToString(res.sec),
		hex.EncodeToString(res.pub),
	)
	nsec, _ := bech32encoding.BinToNsec(res.sec)
	fmt.Printf("NSEC = %s\nNPUB = %s\n", nsec, res.npub)
	return
}

func mine(
	str string, where int, quit <-chan struct{}, resC chan Result, wg *sync.WaitGroup,
	counter *int64,
) {
	wg.Add(1)
	defer wg.Done()

	var r Result
	var e error
	found := false
out:
	for {
		select {
		case <-quit:
			if found {
				// send back the result
				log.D.Ln("sending back result")
				resC <- r
				log.D.Ln("sent")
			} else {
				log.D.Ln("other thread found it")
			}
			break out
		default:
		}
		atomic.AddInt64(counter, 1)
		r.sec, r.pub, e = Gen()
		if e != nil {
			log.E.Ln("error generating key: '%v' worker stopping", e)
			break out
		}
		if r.npub, e = bech32encoding.BinToNpub(r.pub); e != nil {
			log.E.Ln("fatal error generating npub: %s", e)
			break out
		}
		fmt.Printf("\rgenerating key: %s", r.npub)
		switch where {
		case PositionBeginning:
			if bytes.HasPrefix(r.npub, append(prefix, []byte(str)...)) {
				found = true
				// Signal quit by sending result
				select {
				case resC <- r:
				default:
				}
				return
			}
		case PositionEnding:
			if bytes.HasSuffix(r.npub, []byte(str)) {
				found = true
				select {
				case resC <- r:
				default:
				}
				return
			}
		case PositionContains:
			if bytes.Contains(r.npub, []byte(str)) {
				found = true
				select {
				case resC <- r:
				default:
				}
				return
			}
		}
	}
}

func Gen() (skb, pkb []byte, err error) {
	sign, err := p8k.New()
	if err != nil {
		return nil, nil, err
	}
	if err = sign.Generate(); chk.E(err) {
		return
	}
	skb, pkb = sign.Sec(), sign.Pub()
	return
}
