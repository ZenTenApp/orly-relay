package app

import (
	"time"
)

// -- connPerIP actor types --

type connIPCheckAndIncReq struct {
	ip   string
	max  int
	resp chan connIPCheckResp // buffered 1
}
type connIPCheckResp struct {
	allowed bool
	current int
}
type connIPDecReq struct {
	ip string
}

func (s *Server) startConnPerIPActor() {
	s.connIPCheckCh = make(chan connIPCheckAndIncReq)
	s.connIPDecCh = make(chan connIPDecReq, 16)
	s.connIPDone = make(chan struct{})
	go func() {
		defer close(s.connIPDone)
		m := make(map[string]int)
		for {
			select {
			case req := <-s.connIPCheckCh:
				current := m[req.ip]
				if current >= req.max {
					req.resp <- connIPCheckResp{allowed: false, current: current}
				} else {
					m[req.ip]++
					req.resp <- connIPCheckResp{allowed: true, current: current + 1}
				}
			case req := <-s.connIPDecCh:
				m[req.ip]--
				if m[req.ip] <= 0 {
					delete(m, req.ip)
				}
			case <-s.Ctx.Done():
				return
			}
		}
	}()
}

func (s *Server) ConnIPCheckAndInc(ip string, max int) (allowed bool, current int) {
	resp := make(chan connIPCheckResp, 1)
	s.connIPCheckCh <- connIPCheckAndIncReq{ip: ip, max: max, resp: resp}
	r := <-resp
	return r.allowed, r.current
}

func (s *Server) ConnIPDec(ip string) {
	s.connIPDecCh <- connIPDecReq{ip: ip}
}

// -- challenge actor types --

type chalSetReq struct {
	key  string
	data []byte
}
type chalGetReq struct {
	key  string
	resp chan chalGetResp // buffered 1
}
type chalGetResp struct {
	data   []byte
	exists bool
}
type chalDeleteReq struct {
	key string
}

func (s *Server) startChallengeActor() {
	s.chalSetCh = make(chan chalSetReq, 4)
	s.chalGetCh = make(chan chalGetReq)
	s.chalDeleteCh = make(chan chalDeleteReq, 4)
	s.chalDone = make(chan struct{})
	go func() {
		defer close(s.chalDone)
		m := make(map[string][]byte)
		for {
			select {
			case req := <-s.chalSetCh:
				m[req.key] = req.data
			case req := <-s.chalGetCh:
				data, exists := m[req.key]
				req.resp <- chalGetResp{data: data, exists: exists}
			case req := <-s.chalDeleteCh:
				delete(m, req.key)
			case <-s.Ctx.Done():
				return
			}
		}
	}()
}

func (s *Server) ChallengeSet(key string, data []byte) {
	s.chalSetCh <- chalSetReq{key: key, data: data}
}

func (s *Server) ChallengeGet(key string) ([]byte, bool) {
	resp := make(chan chalGetResp, 1)
	s.chalGetCh <- chalGetReq{key: key, resp: resp}
	r := <-resp
	return r.data, r.exists
}

func (s *Server) ChallengeDelete(key string) {
	s.chalDeleteCh <- chalDeleteReq{key: key}
}

func (s *Server) ChallengeSetWithExpiry(key string, data []byte, ttl time.Duration) {
	s.ChallengeSet(key, data)
	go func() {
		time.Sleep(ttl)
		s.ChallengeDelete(key)
	}()
}

// -- message gate (drain-then-pause) --
//
// The gate tracks in-flight message handlers via a counting channel.
// enterCh is buffered to the max concurrent handler count.
// PauseMessageProcessing blocks new entries and drains existing ones.

type msgGateEnterReq struct{}
type msgGateExitReq struct{}
type msgGatePauseReq struct {
	resp chan struct{} // buffered 1: ack when drain complete
}
type msgGateResumeReq struct{}

func (s *Server) startMessageGate() {
	s.msgGateEnterCh = make(chan msgGateEnterReq, 128) // buffered: absorb bursts
	s.msgGateExitCh = make(chan msgGateExitReq, 128)
	s.msgGatePauseCh = make(chan msgGatePauseReq)
	s.msgGateResumeCh = make(chan msgGateResumeReq)
	s.msgGateDone = make(chan struct{})
	go func() {
		defer close(s.msgGateDone)
		inFlight := 0
		paused := false
		var pauseAck chan struct{} // set when paused and waiting for drain

		for {
			if paused {
				// While paused, only accept exits and resume
				select {
				case <-s.msgGateExitCh:
					inFlight--
					if inFlight == 0 && pauseAck != nil {
						close(pauseAck)
						pauseAck = nil
					}
				case <-s.msgGateResumeCh:
					paused = false
				case <-s.Ctx.Done():
					return
				}
			} else {
				select {
				case <-s.msgGateEnterCh:
					inFlight++
				case <-s.msgGateExitCh:
					inFlight--
				case req := <-s.msgGatePauseCh:
					paused = true
					if inFlight == 0 {
						close(req.resp)
					} else {
						pauseAck = req.resp
					}
				case <-s.Ctx.Done():
					return
				}
			}
		}
	}()
}

func (s *Server) PauseMessageProcessing() {
	resp := make(chan struct{}, 1)
	s.msgGatePauseCh <- msgGatePauseReq{resp: resp}
	<-resp // blocks until all in-flight drained
}

func (s *Server) ResumeMessageProcessing() {
	s.msgGateResumeCh <- msgGateResumeReq{}
}

func (s *Server) AcquireMessageProcessingLock() {
	s.msgGateEnterCh <- msgGateEnterReq{}
}

func (s *Server) ReleaseMessageProcessingLock() {
	s.msgGateExitCh <- msgGateExitReq{}
}
