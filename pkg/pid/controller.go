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
//   - Thread-safe operation
//
// # Control Theory Background
//
// The PID controller computes an output based on the error between the current
// process variable and a target setpoint:
//
//	output = Kp*error + Ki*∫error*dt + Kd*d(filtered_error)/dt
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
//	filtered_error = α*current_error + (1-α)*previous_filtered_error
//	derivative = (filtered_error - previous_filtered_error) / dt
//
// Lower α values provide stronger filtering (recommended: 0.1-0.3).
package pid

import (
	"math"
	"sync"
	"time"

	pidif "next.orly.dev/pkg/interfaces/pid"
)

// Controller implements a PID controller with filtered derivative.
// It is safe for concurrent use.
type Controller struct {
	// Configuration (protected by mutex for dynamic updates)
	mu      sync.Mutex
	tuning  pidif.Tuning

	// Internal state
	integral          float64
	prevError         float64
	prevFilteredError float64
	lastUpdate        time.Time
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

func (o output) Value() float64                      { return o.value }
func (o output) Clamped() bool                       { return o.clamped }
func (o output) Components() (p, i, d float64)       { return o.pTerm, o.iTerm, o.dTerm }

// New creates a new PID controller with the given tuning parameters.
func New(tuning pidif.Tuning) *Controller {
	return &Controller{tuning: tuning}
}

// NewWithGains creates a new PID controller with specified gains and defaults for other parameters.
func NewWithGains(kp, ki, kd, setpoint float64) *Controller {
	tuning := pidif.DefaultTuning()
	tuning.Kp = kp
	tuning.Ki = ki
	tuning.Kd = kd
	tuning.Setpoint = setpoint
	return &Controller{tuning: tuning}
}

// NewDefault creates a new PID controller with default tuning.
func NewDefault() *Controller {
	return &Controller{tuning: pidif.DefaultTuning()}
}

// Update computes the controller output based on the current process variable.
func (c *Controller) Update(pv pidif.ProcessVariable) pidif.Output {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := pv.Timestamp()
	value := pv.Value()

	// Initialize on first call
	if !c.initialized {
		c.lastUpdate = now
		c.prevError = value - c.tuning.Setpoint
		c.prevFilteredError = c.prevError
		c.initialized = true
		return output{value: 0, clamped: false}
	}

	// Calculate time delta
	dt := now.Sub(c.lastUpdate).Seconds()
	if dt <= 0 {
		dt = 0.001 // Minimum 1ms to avoid division by zero
	}
	c.lastUpdate = now

	// Calculate current error (positive when above setpoint)
	err := value - c.tuning.Setpoint

	// Proportional term
	pTerm := c.tuning.Kp * err

	// Integral term with anti-windup
	c.integral += err * dt
	c.integral = clamp(c.integral, c.tuning.IntegralMin, c.tuning.IntegralMax)
	iTerm := c.tuning.Ki * c.integral

	// Derivative term with low-pass filter
	alpha := c.tuning.DerivativeFilterAlpha
	if alpha <= 0 {
		alpha = 0.2 // Default if not set
	}
	filteredError := alpha*err + (1-alpha)*c.prevFilteredError

	var dTerm float64
	if dt > 0 {
		dTerm = c.tuning.Kd * (filteredError - c.prevFilteredError) / dt
	}

	// Update previous values
	c.prevError = err
	c.prevFilteredError = filteredError

	// Compute total output
	rawOutput := pTerm + iTerm + dTerm
	clampedOutput := clamp(rawOutput, c.tuning.OutputMin, c.tuning.OutputMax)

	return output{
		value:   clampedOutput,
		clamped: rawOutput != clampedOutput,
		pTerm:   pTerm,
		iTerm:   iTerm,
		dTerm:   dTerm,
	}
}

// UpdateValue is a convenience method that takes a raw float64 value.
func (c *Controller) UpdateValue(value float64) pidif.Output {
	return c.Update(pidif.NewProcessVariable(value))
}

// Reset clears all internal state.
func (c *Controller) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.integral = 0
	c.prevError = 0
	c.prevFilteredError = 0
	c.initialized = false
}

// SetSetpoint updates the target value.
func (c *Controller) SetSetpoint(setpoint float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tuning.Setpoint = setpoint
}

// Setpoint returns the current setpoint.
func (c *Controller) Setpoint() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tuning.Setpoint
}

// SetGains updates the PID gains.
func (c *Controller) SetGains(kp, ki, kd float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tuning.Kp = kp
	c.tuning.Ki = ki
	c.tuning.Kd = kd
}

// Gains returns the current PID gains.
func (c *Controller) Gains() (kp, ki, kd float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tuning.Kp, c.tuning.Ki, c.tuning.Kd
}

// SetOutputLimits updates the output clamping limits.
func (c *Controller) SetOutputLimits(min, max float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tuning.OutputMin = min
	c.tuning.OutputMax = max
}

// SetIntegralLimits updates the anti-windup limits.
func (c *Controller) SetIntegralLimits(min, max float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tuning.IntegralMin = min
	c.tuning.IntegralMax = max
}

// SetDerivativeFilter updates the derivative filter coefficient.
// Lower values provide stronger filtering (0.1-0.3 recommended).
func (c *Controller) SetDerivativeFilter(alpha float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tuning.DerivativeFilterAlpha = alpha
}

// Tuning returns a copy of the current tuning parameters.
func (c *Controller) Tuning() pidif.Tuning {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tuning
}

// SetTuning updates all tuning parameters at once.
func (c *Controller) SetTuning(tuning pidif.Tuning) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tuning = tuning
}

// State returns the current internal state for monitoring/debugging.
func (c *Controller) State() (integral, prevError, prevFilteredError float64, initialized bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.integral, c.prevError, c.prevFilteredError, c.initialized
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
