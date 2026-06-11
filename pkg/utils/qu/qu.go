// Package qu is a library for making handling signal (chan struct{}) channels
// simpler, as well as monitoring the state of the signal channels in an
// application.
package qu

import (
	"fmt"
	"strings"
	"time"

	"go.uber.org/atomic"
	"git.smesh.lol/orly/pkg/lol"
	"git.smesh.lol/orly/pkg/lol/log"
)

// C is your basic empty struct signal channel
type C chan struct{}

// --- Actor request/response types ---

type quCreateReq struct {
	msg    string
	bufN   int
	resp   chan C
}

type quGetLocReq struct {
	c    C
	resp chan string
}

type quPrintStateReq struct {
	resp chan struct{}
}

type quOpenCountReq struct {
	buffered bool // true = count buffered, false = count unbuffered
	resp     chan int
}

var (
	logEnabled = atomic.NewBool(false)
	actorCh    = make(chan interface{}, 64)
)

// SetLogging switches on and off the channel logging
func SetLogging(on bool) {
	logEnabled.Store(on)
}

func l(a ...interface{}) {
	if logEnabled.Load() {
		log.D.Ln(a...)
	}
}

func lc(cl func() string) {
	if logEnabled.Load() {
		log.D.Ln(cl())
	}
}

func init() {
	go quActorLoop()
}

func quActorLoop() {
	var createdList []string
	var createdChannels []C
	var createdChannelBufferCounts []int

	cleanupTicker := time.NewTicker(time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case msg := <-actorCh:
			switch req := msg.(type) {
			case quCreateReq:
				createdList = append(createdList, req.msg)
				var o C
				if req.bufN > 0 {
					o = make(C, req.bufN)
				} else {
					o = make(C)
				}
				createdChannels = append(createdChannels, o)
				createdChannelBufferCounts = append(createdChannelBufferCounts, req.bufN)
				req.resp <- o
			case quGetLocReq:
				s := "not found"
				for i := range createdList {
					if i >= len(createdChannels) {
						break
					}
					if createdChannels[i] == req.c {
						s = createdList[i]
					}
				}
				req.resp <- s
			case quPrintStateReq:
				for i := range createdChannels {
					if i >= len(createdList) {
						break
					}
					if testChanIsClosed(createdChannels[i]) {
						log.T.Ln(">>> closed", createdList[i])
					} else {
						log.T.Ln("<<< open", createdList[i])
					}
				}
				req.resp <- struct{}{}
			case quOpenCountReq:
				count := 0
				for i := range createdChannels {
					if i >= len(createdChannels) {
						break
					}
					if req.buffered && createdChannelBufferCounts[i] < 1 {
						continue
					}
					if !req.buffered && createdChannelBufferCounts[i] > 0 {
						continue
					}
					if !testChanIsClosed(createdChannels[i]) {
						count++
					}
				}
				req.resp <- count
			}
		case <-cleanupTicker.C:
			l("cleaning up closed channels")
			var c []C
			var ll []string
			var bc []int
			for i := range createdChannels {
				if i >= len(createdList) {
					break
				}
				if !testChanIsClosed(createdChannels[i]) {
					c = append(c, createdChannels[i])
					ll = append(ll, createdList[i])
					if i < len(createdChannelBufferCounts) {
						bc = append(bc, createdChannelBufferCounts[i])
					}
				}
			}
			createdChannels = c
			createdList = ll
			createdChannelBufferCounts = bc
		}
	}
}

// T creates an unbuffered chan struct{} for trigger and quit signalling
func T() C {
	msg := fmt.Sprintf("chan from %s", lol.GetLoc(1))
	l("created", msg)
	req := quCreateReq{msg: msg, bufN: 0, resp: make(chan C, 1)}
	actorCh <- req
	return <-req.resp
}

// Ts creates a buffered chan struct{}
func Ts(n int) C {
	msg := fmt.Sprintf("buffered chan (%d) from %s", n, lol.GetLoc(1))
	l("created", msg)
	req := quCreateReq{msg: msg, bufN: n, resp: make(chan C, 1)}
	actorCh <- req
	return <-req.resp
}

// Q closes the channel, which makes it emit a nil every time it is selected.
func (c C) Q() {
	open := !testChanIsClosed(c)
	lc(
		func() (o string) {
			lo := getLocForChan(c)
			if open {
				return "closing chan from " + lo + "\n" + strings.Repeat(
					" ",
					48,
				) + "from" + lol.GetLoc(1)
			} else {
				return "from" + lol.GetLoc(1) + "\n" + strings.Repeat(" ", 48) +
					"channel " + lo + " was already closed"
			}
		},
	)
	if open {
		close(c)
	}
}

// Signal sends struct{}{} on the channel which functions as a momentary switch
func (c C) Signal() {
	lc(func() (o string) { return "signalling " + getLocForChan(c) })
	if !testChanIsClosed(c) {
		c <- struct{}{}
	}
}

// Wait should be placed with a `<-` in a select case
func (c C) Wait() <-chan struct{} {
	lc(
		func() (o string) {
			return fmt.Sprint(
				"waiting on "+getLocForChan(c)+"at",
				lol.GetLoc(1),
			)
		},
	)
	return c
}

// IsClosed exposes a test to see if the channel is closed
func (c C) IsClosed() bool {
	return testChanIsClosed(c)
}

// testChanIsClosed allows you to see whether the channel has been closed
func testChanIsClosed(ch C) (o bool) {
	if ch == nil {
		return true
	}
	select {
	case <-ch:
		o = true
	default:
	}
	return
}

// getLocForChan finds which record connects to the channel in question
func getLocForChan(c C) string {
	req := quGetLocReq{c: c, resp: make(chan string, 1)}
	actorCh <- req
	return <-req.resp
}

// PrintChanState creates an output showing the current state of the channels
func PrintChanState() {
	req := quPrintStateReq{resp: make(chan struct{}, 1)}
	actorCh <- req
	<-req.resp
}

// GetOpenUnbufferedChanCount returns the number of qu channels that are still open
func GetOpenUnbufferedChanCount() int {
	req := quOpenCountReq{buffered: false, resp: make(chan int, 1)}
	actorCh <- req
	return <-req.resp
}

// GetOpenBufferedChanCount returns the number of qu channels that are still open
func GetOpenBufferedChanCount() int {
	req := quOpenCountReq{buffered: true, resp: make(chan int, 1)}
	actorCh <- req
	return <-req.resp
}
