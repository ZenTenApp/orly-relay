// Package ratelimit provides adaptive rate limiting using PID control.
// The PID controller uses proportional, integral, and derivative terms
// with a low-pass filter on the derivative to suppress high-frequency noise.
package ratelimit

import (
	"math"
	"sync"
	"time"
)

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
	// Gains
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

	// Internal state (protected by mutex)
	mu                sync.Mutex
	integral          float64
	prevError         float64
	prevFilteredError float64
	lastUpdate        time.Time
	initialized       bool
}

// DefaultPIDControllerForWrites creates a PID controller tuned for write operations.
// Writes benefit from aggressive integral and moderate proportional response.
func DefaultPIDControllerForWrites() *PIDController {
	return &PIDController{
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
}

// DefaultPIDControllerForReads creates a PID controller tuned for read operations.
// Reads should be more responsive but with less aggressive throttling.
func DefaultPIDControllerForReads() *PIDController {
	return &PIDController{
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
}

// NewPIDController creates a new PID controller with custom parameters.
func NewPIDController(
	kp, ki, kd float64,
	setpoint float64,
	derivativeFilterAlpha float64,
	integralMin, integralMax float64,
	outputMin, outputMax float64,
) *PIDController {
	return &PIDController{
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
}

// Update computes the PID output based on the current process variable.
// The process variable should be in the range [0.0, 1.0+] representing load level.
//
// Returns the recommended delay in seconds. A value of 0 means no delay needed.
func (p *PIDController) Update(processVariable float64) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	// Initialize on first call
	if !p.initialized {
		p.lastUpdate = now
		p.prevError = processVariable - p.Setpoint
		p.prevFilteredError = p.prevError
		p.initialized = true
		return 0 // No delay on first call
	}

	// Calculate time delta
	dt := now.Sub(p.lastUpdate).Seconds()
	if dt <= 0 {
		dt = 0.001 // Minimum 1ms to avoid division by zero
	}
	p.lastUpdate = now

	// Calculate current error (positive when above setpoint = need to throttle)
	error := processVariable - p.Setpoint

	// Proportional term: immediate response to current error
	pTerm := p.Kp * error

	// Integral term: accumulate error over time
	// Apply anti-windup by clamping the integral
	p.integral += error * dt
	p.integral = clamp(p.integral, p.IntegralMin, p.IntegralMax)
	iTerm := p.Ki * p.integral

	// Derivative term with low-pass filter
	// Apply exponential moving average to filter high-frequency noise:
	//   filtered = alpha * new + (1 - alpha) * old
	// This is equivalent to a first-order low-pass filter
	filteredError := p.DerivativeFilterAlpha*error + (1-p.DerivativeFilterAlpha)*p.prevFilteredError

	// Derivative of the filtered error
	var dTerm float64
	if dt > 0 {
		dTerm = p.Kd * (filteredError - p.prevFilteredError) / dt
	}

	// Update previous values for next iteration
	p.prevError = error
	p.prevFilteredError = filteredError

	// Compute total output and clamp to limits
	output := pTerm + iTerm + dTerm
	output = clamp(output, p.OutputMin, p.OutputMax)

	// Only return positive delays (throttle when above setpoint)
	if output < 0 {
		return 0
	}
	return output
}

// Reset clears the controller state, useful when conditions change significantly.
func (p *PIDController) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.integral = 0
	p.prevError = 0
	p.prevFilteredError = 0
	p.initialized = false
}

// SetSetpoint updates the target setpoint.
func (p *PIDController) SetSetpoint(setpoint float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Setpoint = setpoint
}

// SetGains updates the PID gains.
func (p *PIDController) SetGains(kp, ki, kd float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Kp = kp
	p.Ki = ki
	p.Kd = kd
}

// GetState returns the current internal state for monitoring/debugging.
func (p *PIDController) GetState() (integral, prevError, prevFilteredError float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.integral, p.prevError, p.prevFilteredError
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
