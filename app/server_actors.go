package app

import (
	"time"

	"git.smesh.lol/actor"
)

// -- connPerIP actor types --

type connIPCheckArgs struct {
	IP  string
	Max int
}
type connIPCheckResp struct {
	allowed bool
	current int
}

type connIPDecArgs struct {
	IP string
}
type connIPDecResp struct{}

func (s *Server) startConnPerIPActor() {
	s.connIPCheck = actor.NewFunc[connIPCheckArgs, connIPCheckResp]()
	s.connIPDec = actor.NewFunc[connIPDecArgs, connIPDecResp]()
	s.connIPLC = actor.NewLifecycle()
	actor.Go(s.connIPLC, func() {
		m := make(map[string]int)
		for {
			select {
			case msg := <-s.connIPCheck.Recv():
				current := m[msg.Req.IP]
				if current >= msg.Req.Max {
					msg.Reply(connIPCheckResp{allowed: false, current: current})
				} else {
					m[msg.Req.IP]++
					msg.Reply(connIPCheckResp{allowed: true, current: current + 1})
				}
			case msg := <-s.connIPDec.Recv():
				m[msg.Req.IP]--
				if m[msg.Req.IP] <= 0 {
					delete(m, msg.Req.IP)
				}
				msg.Reply(connIPDecResp{})
			case <-s.Ctx.Done():
				return
			}
		}
	})
}

func (s *Server) ConnIPCheckAndInc(ip string, max int) (allowed bool, current int) {
	r := s.connIPCheck.Call(connIPCheckArgs{IP: ip, Max: max})
	return r.allowed, r.current
}

func (s *Server) ConnIPDec(ip string) {
	s.connIPDec.Call(connIPDecArgs{IP: ip})
}

// -- challenge actor types --

type chalSetArgs struct {
	Key  string
	Data []byte
}
type chalGetResp struct {
	data   []byte
	exists bool
}

func (s *Server) startChallengeActor() {
	s.chalSet = actor.NewInbox[chalSetArgs](4)
	s.chalGet = actor.NewFunc[string, chalGetResp]()
	s.chalDelete = actor.NewInbox[string](4)
	s.chalLC = actor.NewLifecycle()
	actor.Go(s.chalLC, func() {
		m := make(map[string][]byte)
		for {
			select {
			case args := <-s.chalSet.Recv():
				m[args.Key] = args.Data
			case msg := <-s.chalGet.Recv():
				data, exists := m[msg.Req]
				msg.Reply(chalGetResp{data: data, exists: exists})
			case key := <-s.chalDelete.Recv():
				delete(m, key)
			case <-s.Ctx.Done():
				return
			}
		}
	})
}

func (s *Server) ChallengeSet(key string, data []byte) {
	s.chalSet.TrySend(chalSetArgs{Key: key, Data: data})
}

func (s *Server) ChallengeGet(key string) ([]byte, bool) {
	r := s.chalGet.Call(key)
	return r.data, r.exists
}

func (s *Server) ChallengeDelete(key string) {
	s.chalDelete.TrySend(key)
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
