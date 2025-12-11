// Package pid defines interfaces for PID controller process variable sources.
// This abstraction allows the PID controller to be used for any dynamic
// adjustment scenario - rate limiting, PoW difficulty adjustment, etc.
package pid

import "time"

// ProcessVariable represents a measurable quantity that the PID controller
// regulates. Implementations provide the current value and optional metadata.
type ProcessVariable interface {
	// Value returns the current process variable value.
	// The value should typically be normalized to a range where the setpoint
	// makes sense (e.g., 0.0-1.0 for percentage-based control, or absolute
	// values for things like hash rate or block time).
	Value() float64

	// Timestamp returns when this measurement was taken.
	// This is used for derivative calculations and staleness detection.
	Timestamp() time.Time
}

// Source provides process variable measurements to the PID controller.
// Implementations are domain-specific (e.g., memory monitor, hash rate tracker).
type Source interface {
	// Sample returns the current process variable measurement.
	// This should be efficient as it may be called frequently.
	Sample() ProcessVariable

	// Name returns a human-readable name for this source (for logging/debugging).
	Name() string
}

// Output represents the result of a PID controller update.
type Output interface {
	// Value returns the computed output value.
	// The interpretation depends on the application:
	// - For rate limiting: delay in seconds
	// - For PoW difficulty: difficulty adjustment factor
	// - For temperature control: heater power level
	Value() float64

	// Clamped returns true if the output was clamped to limits.
	Clamped() bool

	// Components returns the individual P, I, D contributions for debugging.
	Components() (p, i, d float64)
}

// Controller defines the interface for a PID controller.
// This allows for different controller implementations (standard PID,
// PID with filtered derivative, adaptive PID, etc.).
type Controller interface {
	// Update computes the controller output based on the current process variable.
	// Returns the computed output.
	Update(pv ProcessVariable) Output

	// UpdateValue is a convenience method that takes a raw float64 value.
	// Uses the current time as the timestamp.
	UpdateValue(value float64) Output

	// Reset clears all internal state (integral accumulator, previous values).
	Reset()

	// SetSetpoint updates the target value.
	SetSetpoint(setpoint float64)

	// Setpoint returns the current setpoint.
	Setpoint() float64

	// SetGains updates the PID gains.
	SetGains(kp, ki, kd float64)

	// Gains returns the current PID gains.
	Gains() (kp, ki, kd float64)
}

// Tuning holds PID tuning parameters.
// This can be used for configuration or auto-tuning.
type Tuning struct {
	Kp float64 // Proportional gain
	Ki float64 // Integral gain
	Kd float64 // Derivative gain

	Setpoint float64 // Target value

	// Derivative filtering (0.0-1.0, lower = more filtering)
	DerivativeFilterAlpha float64

	// Anti-windup limits for integral term
	IntegralMin float64
	IntegralMax float64

	// Output limits
	OutputMin float64
	OutputMax float64
}

// DefaultTuning returns sensible defaults for a normalized (0-1) process variable.
func DefaultTuning() Tuning {
	return Tuning{
		Kp:                    0.5,
		Ki:                    0.1,
		Kd:                    0.05,
		Setpoint:              0.5,
		DerivativeFilterAlpha: 0.2,
		IntegralMin:           -10.0,
		IntegralMax:           10.0,
		OutputMin:             0.0,
		OutputMax:             1.0,
	}
}

// SimpleProcessVariable is a basic implementation of ProcessVariable.
type SimpleProcessVariable struct {
	V float64
	T time.Time
}

// Value returns the process variable value.
func (p SimpleProcessVariable) Value() float64 { return p.V }

// Timestamp returns when this measurement was taken.
func (p SimpleProcessVariable) Timestamp() time.Time { return p.T }

// NewProcessVariable creates a SimpleProcessVariable with the current time.
func NewProcessVariable(value float64) SimpleProcessVariable {
	return SimpleProcessVariable{V: value, T: time.Now()}
}

// NewProcessVariableAt creates a SimpleProcessVariable with a specific time.
func NewProcessVariableAt(value float64, t time.Time) SimpleProcessVariable {
	return SimpleProcessVariable{V: value, T: t}
}
