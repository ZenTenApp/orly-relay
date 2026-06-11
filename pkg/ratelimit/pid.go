// Package ratelimit provides adaptive rate limiting using PID control.
// The PID controller uses proportional, integral, and derivative terms
// with a low-pass filter on the derivative to suppress high-frequency noise.
package ratelimit

import (
	"math"
	"time"
)

// PID actor request types
type (
	pidUpdateReq struct {
		pv   float64
		resp chan float64
	}
	pidResetReq struct {
		done chan struct{}
	}
	pidSetSetpointReq struct {
		setpoint float64
		done     chan struct{}
	}
	pidSetGainsReq struct {
		kp, ki, kd float64
		done       chan struct{}
	}
	pidGetStateReq struct {
		resp chan pidStateSnapshot
	}
)

type pidStateSnapshot struct {
	integral          float64
	prevError         float64
	prevFilteredError float64
}

// PIDController implements a PID controller with filtered derivative.
// It is designed for rate limiting database operations based on load metrics.
//
// The controller computes a delay recommendation based on:
//   - Proportional (P): Immediate response to current error
//   - Integral (I): Accumulated error to eliminate steady-state offset
//   - Derivative (D): Rate of change prediction (filtered to reduce noise)
//
// The filtered derivative uses a low-pass filter to attenuate high-frequency
// noise that would otherwise cause erratic control behavior.
type PIDController struct {
	// Gains (read by tests directly, but mutations go through actor)
	Kp float64 // Proportional gain
	Ki float64 // Integral gain
	Kd float64 // Derivative gain

	// Setpoint is the target process variable value (e.g., 0.85 for 85% of target memory).
	// The controller drives the process variable toward this setpoint.
	Setpoint float64

	// DerivativeFilterAlpha is the low-pass filter coefficient for the derivative term.
	// Range: 0.0-1.0, where lower values provide stronger filtering.
	// Recommended: 0.2 for strong filtering, 0.5 for moderate filtering.
	DerivativeFilterAlpha float64

	// Integral limits for anti-windup
	IntegralMax float64
	IntegralMin float64

	// Output limits
	OutputMin float64 // Minimum output (typically 0 = no delay)
	OutputMax float64 // Maximum output (max delay in seconds)

	// Actor channels
	updateCh      chan pidUpdateReq
	resetCh       chan pidResetReq
	setSetpointCh chan pidSetSetpointReq
	setGainsCh    chan pidSetGainsReq
	getStateCh    chan pidGetStateReq
	stopCh        chan struct{}
}

// DefaultPIDControllerForWrites creates a PID controller tuned for write operations.
// Writes benefit from aggressive integral and moderate proportional response.
func DefaultPIDControllerForWrites() *PIDController {
	p := &PIDController{
		Kp:                    0.5,    // Moderate proportional response
		Ki:                    0.1,    // Steady integral to eliminate offset
		Kd:                    0.05,   // Small derivative for prediction
		Setpoint:              0.85,   // Target 85% of memory limit
		DerivativeFilterAlpha: 0.2,    // Strong filtering (20% new, 80% old)
		IntegralMax:           10.0,   // Anti-windup: max 10 seconds accumulated
		IntegralMin:           -2.0,   // Allow small negative for faster recovery
		OutputMin:             0.0,    // No delay minimum
		OutputMax:             1.0,    // Max 1 second delay per write
	}
	p.startActor()
	return p
}

// DefaultPIDControllerForReads creates a PID controller tuned for read operations.
// Reads should be more responsive but with less aggressive throttling.
func DefaultPIDControllerForReads() *PIDController {
	p := &PIDController{
		Kp:                    0.3,    // Lower proportional (reads are more important)
		Ki:                    0.05,   // Lower integral (don't accumulate as aggressively)
		Kd:                    0.02,   // Very small derivative
		Setpoint:              0.90,   // Target 90% (more tolerant of memory use)
		DerivativeFilterAlpha: 0.15,   // Very strong filtering
		IntegralMax:           5.0,    // Lower anti-windup limit
		IntegralMin:           -1.0,   // Allow small negative
		OutputMin:             0.0,    // No delay minimum
		OutputMax:             0.5,    // Max 500ms delay per read
	}
	p.startActor()
	return p
}

// NewPIDController creates a new PID controller with custom parameters.
func NewPIDController(
	kp, ki, kd float64,
	setpoint float64,
	derivativeFilterAlpha float64,
	integralMin, integralMax float64,
	outputMin, outputMax float64,
) *PIDController {
	p := &PIDController{
		Kp:                    kp,
		Ki:                    ki,
		Kd:                    kd,
		Setpoint:              setpoint,
		DerivativeFilterAlpha: derivativeFilterAlpha,
		IntegralMin:           integralMin,
		IntegralMax:           integralMax,
		OutputMin:             outputMin,
		OutputMax:             outputMax,
	}
	p.startActor()
	return p
}

func (p *PIDController) startActor() {
	p.updateCh = make(chan pidUpdateReq, 1)
	p.resetCh = make(chan pidResetReq, 1)
	p.setSetpointCh = make(chan pidSetSetpointReq, 1)
	p.setGainsCh = make(chan pidSetGainsReq, 1)
	p.getStateCh = make(chan pidGetStateReq, 1)
	p.stopCh = make(chan struct{})

	go p.actorLoop()
}

func (p *PIDController) actorLoop() {
	var (
		integral          float64
		prevError         float64
		prevFilteredError float64
		lastUpdate        time.Time
		initialized       bool
	)

	// Local copies of config that the actor owns
	kp := p.Kp
	ki := p.Ki
	kd := p.Kd
	setpoint := p.Setpoint
	filterAlpha := p.DerivativeFilterAlpha
	integralMin := p.IntegralMin
	integralMax := p.IntegralMax
	outputMin := p.OutputMin
	outputMax := p.OutputMax

	for {
		select {
		case req := <-p.updateCh:
			now := time.Now()
			processVariable := req.pv

			if !initialized {
				lastUpdate = now
				prevError = processVariable - setpoint
				prevFilteredError = prevError
				initialized = true
				req.resp <- 0
				continue
			}

			dt := now.Sub(lastUpdate).Seconds()
			if dt <= 0 {
				dt = 0.001
			}
			lastUpdate = now

			err := processVariable - setpoint

			pTerm := kp * err

			integral += err * dt
			integral = clamp(integral, integralMin, integralMax)
			iTerm := ki * integral

			filteredError := filterAlpha*err + (1-filterAlpha)*prevFilteredError
			var dTerm float64
			if dt > 0 {
				dTerm = kd * (filteredError - prevFilteredError) / dt
			}

			prevError = err
			prevFilteredError = filteredError

			output := pTerm + iTerm + dTerm
			output = clamp(output, outputMin, outputMax)

			if output < 0 {
				output = 0
			}
			req.resp <- output

		case req := <-p.resetCh:
			integral = 0
			prevError = 0
			prevFilteredError = 0
			initialized = false
			close(req.done)

		case req := <-p.setSetpointCh:
			setpoint = req.setpoint
			// Update exported field for test compatibility
			p.Setpoint = req.setpoint
			close(req.done)

		case req := <-p.setGainsCh:
			kp = req.kp
			ki = req.ki
			kd = req.kd
			// Update exported fields for test compatibility
			p.Kp = req.kp
			p.Ki = req.ki
			p.Kd = req.kd
			close(req.done)

		case req := <-p.getStateCh:
			req.resp <- pidStateSnapshot{
				integral:          integral,
				prevError:         prevError,
				prevFilteredError: prevFilteredError,
			}

		case <-p.stopCh:
			return
		}
	}
}

// Update computes the PID output based on the current process variable.
// The process variable should be in the range [0.0, 1.0+] representing load level.
//
// Returns the recommended delay in seconds. A value of 0 means no delay needed.
func (p *PIDController) Update(processVariable float64) float64 {
	resp := make(chan float64, 1)
	p.updateCh <- pidUpdateReq{pv: processVariable, resp: resp}
	return <-resp
}

// Reset clears the controller state, useful when conditions change significantly.
func (p *PIDController) Reset() {
	done := make(chan struct{})
	p.resetCh <- pidResetReq{done: done}
	<-done
}

// SetSetpoint updates the target setpoint.
func (p *PIDController) SetSetpoint(setpoint float64) {
	done := make(chan struct{})
	p.setSetpointCh <- pidSetSetpointReq{setpoint: setpoint, done: done}
	<-done
}

// SetGains updates the PID gains.
func (p *PIDController) SetGains(kp, ki, kd float64) {
	done := make(chan struct{})
	p.setGainsCh <- pidSetGainsReq{kp: kp, ki: ki, kd: kd, done: done}
	<-done
}

// GetState returns the current internal state for monitoring/debugging.
func (p *PIDController) GetState() (integral, prevError, prevFilteredError float64) {
	resp := make(chan pidStateSnapshot, 1)
	p.getStateCh <- pidGetStateReq{resp: resp}
	s := <-resp
	return s.integral, s.prevError, s.prevFilteredError
}

// clamp restricts a value to the range [min, max].
func clamp(value, min, max float64) float64 {
	if math.IsNaN(value) {
		return 0
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
