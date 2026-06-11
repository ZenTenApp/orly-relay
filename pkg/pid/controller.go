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

	"git.smesh.lol/actor"
	pidif "git.smesh.lol/orly/pkg/interfaces/pid"
)

type gainsArgs struct {
	Kp, Ki, Kd float64
}

type limitsArgs struct {
	Min, Max float64
}

type pidState struct {
	Integral          float64
	PrevError         float64
	PrevFilteredError float64
	Initialized       bool
}

// Controller implements a PID controller with filtered derivative.
// All mutable state is owned by the actor goroutine.
type Controller struct {
	update          actor.Func[pidif.ProcessVariable, pidif.Output]
	updateValue     actor.Func[float64, pidif.Output]
	reset           actor.Signal
	setSetpoint     actor.Proc[float64]
	getSetpoint     actor.Query[float64]
	setGains        actor.Proc[gainsArgs]
	getGains        actor.Query[[3]float64]
	setOutputLimits actor.Proc[limitsArgs]
	setIntegralLim  actor.Proc[limitsArgs]
	setDerivFilter  actor.Proc[float64]
	getTuning       actor.Query[pidif.Tuning]
	setTuning       actor.Proc[pidif.Tuning]
	state           actor.Query[pidState]
	actor.Lifecycle
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
	actor.Go(c.Lifecycle, func() { c.actorLoop(tuning) })
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
	actor.Go(c.Lifecycle, func() { c.actorLoop(tuning) })
	return c
}

// NewDefault creates a new PID controller with default tuning.
func NewDefault() *Controller {
	c := newController()
	actor.Go(c.Lifecycle, func() { c.actorLoop(pidif.DefaultTuning()) })
	return c
}

func newController() *Controller {
	return &Controller{
		update:          actor.NewFunc[pidif.ProcessVariable, pidif.Output](),
		updateValue:     actor.NewFunc[float64, pidif.Output](),
		reset:           actor.NewSignal(),
		setSetpoint:     actor.NewProc[float64](),
		getSetpoint:     actor.NewQuery[float64](),
		setGains:        actor.NewProc[gainsArgs](),
		getGains:        actor.NewQuery[[3]float64](),
		setOutputLimits: actor.NewProc[limitsArgs](),
		setIntegralLim:  actor.NewProc[limitsArgs](),
		setDerivFilter:  actor.NewProc[float64](),
		getTuning:       actor.NewQuery[pidif.Tuning](),
		setTuning:       actor.NewProc[pidif.Tuning](),
		state:           actor.NewQuery[pidState](),
		Lifecycle:       actor.NewLifecycle(),
	}
}

func (c *Controller) actorLoop(tuning pidif.Tuning) {
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
		case <-c.Stopping():
			return
		case msg := <-c.update.Recv():
			msg.Reply(doUpdate(msg.Req))
		case msg := <-c.updateValue.Recv():
			msg.Reply(doUpdate(pidif.NewProcessVariable(msg.Req)))
		case msg := <-c.reset.Recv():
			integral = 0
			prevError = 0
			prevFilteredError = 0
			initialized = false
			msg.Done()
		case msg := <-c.setSetpoint.Recv():
			tuning.Setpoint = msg.Req
			msg.Done()
		case msg := <-c.getSetpoint.Recv():
			msg.Reply(tuning.Setpoint)
		case msg := <-c.setGains.Recv():
			tuning.Kp = msg.Req.Kp
			tuning.Ki = msg.Req.Ki
			tuning.Kd = msg.Req.Kd
			msg.Done()
		case msg := <-c.getGains.Recv():
			msg.Reply([3]float64{tuning.Kp, tuning.Ki, tuning.Kd})
		case msg := <-c.setOutputLimits.Recv():
			tuning.OutputMin = msg.Req.Min
			tuning.OutputMax = msg.Req.Max
			msg.Done()
		case msg := <-c.setIntegralLim.Recv():
			tuning.IntegralMin = msg.Req.Min
			tuning.IntegralMax = msg.Req.Max
			msg.Done()
		case msg := <-c.setDerivFilter.Recv():
			tuning.DerivativeFilterAlpha = msg.Req
			msg.Done()
		case msg := <-c.getTuning.Recv():
			msg.Reply(tuning)
		case msg := <-c.setTuning.Recv():
			tuning = msg.Req
			msg.Done()
		case msg := <-c.state.Recv():
			msg.Reply(pidState{
				Integral:          integral,
				PrevError:         prevError,
				PrevFilteredError: prevFilteredError,
				Initialized:       initialized,
			})
		}
	}
}

// Shutdown stops the controller actor.
func (c *Controller) Shutdown() {
	c.Stop()
}

// Update computes the controller output based on the current process variable.
func (c *Controller) Update(pv pidif.ProcessVariable) pidif.Output {
	return c.update.Call(pv)
}

// UpdateValue is a convenience method that takes a raw float64 value.
func (c *Controller) UpdateValue(value float64) pidif.Output {
	return c.updateValue.Call(value)
}

// Reset clears all internal state.
func (c *Controller) Reset() {
	c.reset.Call()
}

// SetSetpoint updates the target value.
func (c *Controller) SetSetpoint(setpoint float64) {
	c.setSetpoint.Call(setpoint)
}

// Setpoint returns the current setpoint.
func (c *Controller) Setpoint() float64 {
	return c.getSetpoint.Call()
}

// SetGains updates the PID gains.
func (c *Controller) SetGains(kp, ki, kd float64) {
	c.setGains.Call(gainsArgs{Kp: kp, Ki: ki, Kd: kd})
}

// Gains returns the current PID gains.
func (c *Controller) Gains() (kp, ki, kd float64) {
	r := c.getGains.Call()
	return r[0], r[1], r[2]
}

// SetOutputLimits updates the output clamping limits.
func (c *Controller) SetOutputLimits(min, max float64) {
	c.setOutputLimits.Call(limitsArgs{Min: min, Max: max})
}

// SetIntegralLimits updates the anti-windup limits.
func (c *Controller) SetIntegralLimits(min, max float64) {
	c.setIntegralLim.Call(limitsArgs{Min: min, Max: max})
}

// SetDerivativeFilter updates the derivative filter coefficient.
// Lower values provide stronger filtering (0.1-0.3 recommended).
func (c *Controller) SetDerivativeFilter(alpha float64) {
	c.setDerivFilter.Call(alpha)
}

// Tuning returns a copy of the current tuning parameters.
func (c *Controller) Tuning() pidif.Tuning {
	return c.getTuning.Call()
}

// SetTuning updates all tuning parameters at once.
func (c *Controller) SetTuning(tuning pidif.Tuning) {
	c.setTuning.Call(tuning)
}

// State returns the current internal state for monitoring/debugging.
func (c *Controller) State() (integral, prevError, prevFilteredError float64, initialized bool) {
	r := c.state.Call()
	return r.Integral, r.PrevError, r.PrevFilteredError, r.Initialized
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
