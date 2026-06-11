// Package pid provides a generic PID controller implementation with filtered derivative.
//
// This package implements a Proportional-Integral-Derivative controller suitable
// for various dynamic adjustment scenarios:
//   - Rate limiting (memory/load-based throttling)
//   - PoW difficulty adjustment (block time targeting)
//   - Temperature control
//   - Motor speed control
//   - Any system requiring feedback-based regulation
//
// The controller features:
//   - Low-pass filtered derivative to suppress high-frequency noise
//   - Anti-windup on the integral term to prevent saturation
//   - Configurable output clamping
//   - Thread-safe operation via actor goroutine
//
// # Control Theory Background
//
// The PID controller computes an output based on the error between the current
// process variable and a target setpoint:
//
//	output = Kp*error + Ki*integral(error*dt) + Kd*d(filtered_error)/dt
//
// Where:
//   - Proportional (P): Immediate response proportional to current error
//   - Integral (I): Accumulated error to eliminate steady-state offset
//   - Derivative (D): Rate of change to anticipate future error (filtered)
//
// # Filtered Derivative
//
// Raw derivative amplifies high-frequency noise. This implementation applies
// an exponential moving average (low-pass filter) before computing the derivative:
//
//	filtered_error = alpha*current_error + (1-alpha)*previous_filtered_error
//	derivative = (filtered_error - previous_filtered_error) / dt
//
// Lower alpha values provide stronger filtering (recommended: 0.1-0.3).
package pid

import (
	"math"
	"time"

	pidif "git.smesh.lol/orly/pkg/interfaces/pid"
)

// Controller implements a PID controller with filtered derivative.
// It is safe for concurrent use via an internal actor goroutine.
type Controller struct {
	// Actor channels
	updateCh            chan updateReq
	updateValueCh       chan updateValueReq
	resetCh             chan resetReq
	setSetpointCh       chan setSetpointReq
	getSetpointCh       chan getSetpointReq
	setGainsCh          chan setGainsReq
	getGainsCh          chan getGainsReq
	setOutputLimitsCh   chan setOutputLimitsReq
	setIntegralLimitsCh chan setIntegralLimitsReq
	setDerivFilterCh    chan setDerivFilterReq
	getTuningCh         chan getTuningReq
	setTuningCh         chan setTuningReq
	stateCh             chan stateReq
	stop                chan struct{}
	done                chan struct{}
}

type updateReq struct {
	pv   pidif.ProcessVariable
	resp chan pidif.Output
}

type updateValueReq struct {
	value float64
	resp  chan pidif.Output
}

type resetReq struct {
	resp chan struct{}
}

type setSetpointReq struct {
	setpoint float64
}

type getSetpointReq struct {
	resp chan float64
}

type setGainsReq struct {
	kp, ki, kd float64
}

type getGainsReq struct {
	resp chan [3]float64
}

type setOutputLimitsReq struct {
	min, max float64
}

type setIntegralLimitsReq struct {
	min, max float64
}

type setDerivFilterReq struct {
	alpha float64
}

type getTuningReq struct {
	resp chan pidif.Tuning
}

type setTuningReq struct {
	tuning pidif.Tuning
}

type stateReq struct {
	resp chan stateResp
}

type stateResp struct {
	integral          float64
	prevError         float64
	prevFilteredError float64
	initialized       bool
}

// Compile-time check that Controller implements pidif.Controller
var _ pidif.Controller = (*Controller)(nil)

// output implements pidif.Output
type output struct {
	value   float64
	clamped bool
	pTerm   float64
	iTerm   float64
	dTerm   float64
}

func (o output) Value() float64                { return o.value }
func (o output) Clamped() bool                 { return o.clamped }
func (o output) Components() (p, i, d float64) { return o.pTerm, o.iTerm, o.dTerm }

// New creates a new PID controller with the given tuning parameters.
func New(tuning pidif.Tuning) *Controller {
	c := newController()
	go c.actor(tuning)
	return c
}

// NewWithGains creates a new PID controller with specified gains and defaults for other parameters.
func NewWithGains(kp, ki, kd, setpoint float64) *Controller {
	tuning := pidif.DefaultTuning()
	tuning.Kp = kp
	tuning.Ki = ki
	tuning.Kd = kd
	tuning.Setpoint = setpoint
	c := newController()
	go c.actor(tuning)
	return c
}

// NewDefault creates a new PID controller with default tuning.
func NewDefault() *Controller {
	c := newController()
	go c.actor(pidif.DefaultTuning())
	return c
}

func newController() *Controller {
	return &Controller{
		updateCh:            make(chan updateReq),
		updateValueCh:       make(chan updateValueReq),
		resetCh:             make(chan resetReq),
		setSetpointCh:       make(chan setSetpointReq, 16),
		getSetpointCh:       make(chan getSetpointReq),
		setGainsCh:          make(chan setGainsReq, 16),
		getGainsCh:          make(chan getGainsReq),
		setOutputLimitsCh:   make(chan setOutputLimitsReq, 16),
		setIntegralLimitsCh: make(chan setIntegralLimitsReq, 16),
		setDerivFilterCh:    make(chan setDerivFilterReq, 16),
		getTuningCh:         make(chan getTuningReq),
		setTuningCh:         make(chan setTuningReq, 16),
		stateCh:             make(chan stateReq),
		stop:                make(chan struct{}),
		done:                make(chan struct{}),
	}
}

func (c *Controller) actor(tuning pidif.Tuning) {
	defer close(c.done)

	var (
		integral          float64
		prevError         float64
		prevFilteredError float64
		lastUpdate        time.Time
		initialized       bool
	)

	doUpdate := func(pv pidif.ProcessVariable) pidif.Output {
		now := pv.Timestamp()
		value := pv.Value()

		if !initialized {
			lastUpdate = now
			prevError = value - tuning.Setpoint
			prevFilteredError = prevError
			initialized = true
			return output{value: 0, clamped: false}
		}

		dt := now.Sub(lastUpdate).Seconds()
		if dt <= 0 {
			dt = 0.001
		}
		lastUpdate = now

		err := value - tuning.Setpoint

		pTerm := tuning.Kp * err

		integral += err * dt
		integral = clamp(integral, tuning.IntegralMin, tuning.IntegralMax)
		iTerm := tuning.Ki * integral

		alpha := tuning.DerivativeFilterAlpha
		if alpha <= 0 {
			alpha = 0.2
		}
		filteredError := alpha*err + (1-alpha)*prevFilteredError

		var dTerm float64
		if dt > 0 {
			dTerm = tuning.Kd * (filteredError - prevFilteredError) / dt
		}

		prevError = err
		prevFilteredError = filteredError

		rawOutput := pTerm + iTerm + dTerm
		clampedOutput := clamp(rawOutput, tuning.OutputMin, tuning.OutputMax)

		return output{
			value:   clampedOutput,
			clamped: rawOutput != clampedOutput,
			pTerm:   pTerm,
			iTerm:   iTerm,
			dTerm:   dTerm,
		}
	}

	for {
		select {
		case <-c.stop:
			return
		case req := <-c.updateCh:
			req.resp <- doUpdate(req.pv)
		case req := <-c.updateValueCh:
			req.resp <- doUpdate(pidif.NewProcessVariable(req.value))
		case req := <-c.resetCh:
			integral = 0
			prevError = 0
			prevFilteredError = 0
			initialized = false
			close(req.resp)
		case req := <-c.setSetpointCh:
			tuning.Setpoint = req.setpoint
		case req := <-c.getSetpointCh:
			req.resp <- tuning.Setpoint
		case req := <-c.setGainsCh:
			tuning.Kp = req.kp
			tuning.Ki = req.ki
			tuning.Kd = req.kd
		case req := <-c.getGainsCh:
			req.resp <- [3]float64{tuning.Kp, tuning.Ki, tuning.Kd}
		case req := <-c.setOutputLimitsCh:
			tuning.OutputMin = req.min
			tuning.OutputMax = req.max
		case req := <-c.setIntegralLimitsCh:
			tuning.IntegralMin = req.min
			tuning.IntegralMax = req.max
		case req := <-c.setDerivFilterCh:
			tuning.DerivativeFilterAlpha = req.alpha
		case req := <-c.getTuningCh:
			req.resp <- tuning
		case req := <-c.setTuningCh:
			tuning = req.tuning
		case req := <-c.stateCh:
			req.resp <- stateResp{
				integral:          integral,
				prevError:         prevError,
				prevFilteredError: prevFilteredError,
				initialized:       initialized,
			}
		}
	}
}

// Stop shuts down the controller actor.
func (c *Controller) Stop() {
	close(c.stop)
	<-c.done
}

// Update computes the controller output based on the current process variable.
func (c *Controller) Update(pv pidif.ProcessVariable) pidif.Output {
	resp := make(chan pidif.Output, 1)
	c.updateCh <- updateReq{pv: pv, resp: resp}
	return <-resp
}

// UpdateValue is a convenience method that takes a raw float64 value.
func (c *Controller) UpdateValue(value float64) pidif.Output {
	resp := make(chan pidif.Output, 1)
	c.updateValueCh <- updateValueReq{value: value, resp: resp}
	return <-resp
}

// Reset clears all internal state.
func (c *Controller) Reset() {
	resp := make(chan struct{})
	c.resetCh <- resetReq{resp: resp}
	<-resp
}

// SetSetpoint updates the target value.
func (c *Controller) SetSetpoint(setpoint float64) {
	c.setSetpointCh <- setSetpointReq{setpoint: setpoint}
}

// Setpoint returns the current setpoint.
func (c *Controller) Setpoint() float64 {
	resp := make(chan float64, 1)
	c.getSetpointCh <- getSetpointReq{resp: resp}
	return <-resp
}

// SetGains updates the PID gains.
func (c *Controller) SetGains(kp, ki, kd float64) {
	c.setGainsCh <- setGainsReq{kp: kp, ki: ki, kd: kd}
}

// Gains returns the current PID gains.
func (c *Controller) Gains() (kp, ki, kd float64) {
	resp := make(chan [3]float64, 1)
	c.getGainsCh <- getGainsReq{resp: resp}
	r := <-resp
	return r[0], r[1], r[2]
}

// SetOutputLimits updates the output clamping limits.
func (c *Controller) SetOutputLimits(min, max float64) {
	c.setOutputLimitsCh <- setOutputLimitsReq{min: min, max: max}
}

// SetIntegralLimits updates the anti-windup limits.
func (c *Controller) SetIntegralLimits(min, max float64) {
	c.setIntegralLimitsCh <- setIntegralLimitsReq{min: min, max: max}
}

// SetDerivativeFilter updates the derivative filter coefficient.
// Lower values provide stronger filtering (0.1-0.3 recommended).
func (c *Controller) SetDerivativeFilter(alpha float64) {
	c.setDerivFilterCh <- setDerivFilterReq{alpha: alpha}
}

// Tuning returns a copy of the current tuning parameters.
func (c *Controller) Tuning() pidif.Tuning {
	resp := make(chan pidif.Tuning, 1)
	c.getTuningCh <- getTuningReq{resp: resp}
	return <-resp
}

// SetTuning updates all tuning parameters at once.
func (c *Controller) SetTuning(tuning pidif.Tuning) {
	c.setTuningCh <- setTuningReq{tuning: tuning}
}

// State returns the current internal state for monitoring/debugging.
func (c *Controller) State() (integral, prevError, prevFilteredError float64, initialized bool) {
	resp := make(chan stateResp, 1)
	c.stateCh <- stateReq{resp: resp}
	r := <-resp
	return r.integral, r.prevError, r.prevFilteredError, r.initialized
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
