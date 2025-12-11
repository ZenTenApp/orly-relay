package pid

import (
	pidif "next.orly.dev/pkg/interfaces/pid"
)

// Presets for common PID controller use cases.
// These provide good starting points that can be fine-tuned for specific applications.

// RateLimitWriteTuning returns tuning optimized for write rate limiting.
// - Aggressive response to prevent memory exhaustion
// - Moderate integral for sustained load handling
// - Small derivative with strong filtering
func RateLimitWriteTuning() pidif.Tuning {
	return pidif.Tuning{
		Kp:                    0.5,
		Ki:                    0.1,
		Kd:                    0.05,
		Setpoint:              0.85, // Target 85% of limit
		DerivativeFilterAlpha: 0.2,  // Strong filtering
		IntegralMin:           -2.0,
		IntegralMax:           10.0,
		OutputMin:             0.0,
		OutputMax:             1.0, // Max 1 second delay
	}
}

// RateLimitReadTuning returns tuning optimized for read rate limiting.
// - Less aggressive than writes (reads are more latency-sensitive)
// - Lower gains to avoid over-throttling queries
func RateLimitReadTuning() pidif.Tuning {
	return pidif.Tuning{
		Kp:                    0.3,
		Ki:                    0.05,
		Kd:                    0.02,
		Setpoint:              0.90, // Target 90% of limit
		DerivativeFilterAlpha: 0.15, // Very strong filtering
		IntegralMin:           -1.0,
		IntegralMax:           5.0,
		OutputMin:             0.0,
		OutputMax:             0.5, // Max 500ms delay
	}
}

// DifficultyAdjustmentTuning returns tuning for PoW difficulty adjustment.
// Designed for block time targeting where:
// - Process variable: actual_block_time / target_block_time (1.0 = on target)
// - Output: difficulty multiplier (1.0 = no change, >1 = harder, <1 = easier)
//
// This uses:
// - Low Kp to avoid overreacting to individual blocks
// - Moderate Ki to converge on target over time
// - Small Kd with strong filtering to anticipate trends
func DifficultyAdjustmentTuning() pidif.Tuning {
	return pidif.Tuning{
		Kp:                    0.1,  // Low proportional (blocks are noisy)
		Ki:                    0.05, // Moderate integral for convergence
		Kd:                    0.02, // Small derivative
		Setpoint:              1.0,  // Target: actual == expected block time
		DerivativeFilterAlpha: 0.1,  // Very strong filtering (blocks are noisy)
		IntegralMin:           -0.5, // Limit integral windup
		IntegralMax:           0.5,
		OutputMin:             0.5,  // Min 50% difficulty change
		OutputMax:             2.0,  // Max 200% difficulty change
	}
}

// TemperatureControlTuning returns tuning for temperature regulation.
// Suitable for heating/cooling systems where:
// - Process variable: current temperature
// - Setpoint: target temperature
// - Output: heater/cooler power level (0-1)
func TemperatureControlTuning(targetTemp float64) pidif.Tuning {
	return pidif.Tuning{
		Kp:                    0.1,        // Moderate response
		Ki:                    0.01,       // Slow integral (thermal inertia)
		Kd:                    0.05,       // Some anticipation
		Setpoint:              targetTemp,
		DerivativeFilterAlpha: 0.3,        // Moderate filtering
		IntegralMin:           -100.0,
		IntegralMax:           100.0,
		OutputMin:             0.0,
		OutputMax:             1.0,
	}
}

// MotorSpeedTuning returns tuning for motor speed control.
// - Process variable: actual RPM / target RPM
// - Output: motor power level
func MotorSpeedTuning() pidif.Tuning {
	return pidif.Tuning{
		Kp:                    0.5,  // Quick response
		Ki:                    0.2,  // Eliminate steady-state error
		Kd:                    0.1,  // Dampen oscillations
		Setpoint:              1.0,  // Target: actual == desired speed
		DerivativeFilterAlpha: 0.4,  // Moderate filtering
		IntegralMin:           -1.0,
		IntegralMax:           1.0,
		OutputMin:             0.0,
		OutputMax:             1.0,
	}
}

// NewRateLimitWriteController creates a controller for write rate limiting.
func NewRateLimitWriteController() *Controller {
	return New(RateLimitWriteTuning())
}

// NewRateLimitReadController creates a controller for read rate limiting.
func NewRateLimitReadController() *Controller {
	return New(RateLimitReadTuning())
}

// NewDifficultyAdjustmentController creates a controller for PoW difficulty.
func NewDifficultyAdjustmentController() *Controller {
	return New(DifficultyAdjustmentTuning())
}

// NewTemperatureController creates a controller for temperature regulation.
func NewTemperatureController(targetTemp float64) *Controller {
	return New(TemperatureControlTuning(targetTemp))
}

// NewMotorSpeedController creates a controller for motor speed control.
func NewMotorSpeedController() *Controller {
	return New(MotorSpeedTuning())
}
