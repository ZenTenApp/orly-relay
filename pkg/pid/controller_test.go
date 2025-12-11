package pid

import (
	"testing"
	"time"

	pidif "next.orly.dev/pkg/interfaces/pid"
)

func TestController_BasicOperation(t *testing.T) {
	ctrl := New(RateLimitWriteTuning())

	// First call should return 0 (initialization)
	out := ctrl.UpdateValue(0.5)
	if out.Value() != 0 {
		t.Errorf("expected 0 on first call, got %v", out.Value())
	}

	// Sleep a bit to ensure dt > 0
	time.Sleep(10 * time.Millisecond)

	// Process variable below setpoint (0.5 < 0.85) should return 0 or negative (clamped to 0)
	out = ctrl.UpdateValue(0.5)
	if out.Value() != 0 {
		t.Errorf("expected 0 when below setpoint, got %v", out.Value())
	}

	// Process variable above setpoint should return positive output
	time.Sleep(10 * time.Millisecond)
	out = ctrl.UpdateValue(0.95) // 0.95 > 0.85 setpoint
	if out.Value() <= 0 {
		t.Errorf("expected positive output when above setpoint, got %v", out.Value())
	}
}

func TestController_IntegralAccumulation(t *testing.T) {
	tuning := pidif.Tuning{
		Kp:                    0.5,
		Ki:                    0.5, // High Ki
		Kd:                    0.0, // No Kd
		Setpoint:              0.5,
		DerivativeFilterAlpha: 0.2,
		IntegralMin:           -10,
		IntegralMax:           10,
		OutputMin:             0,
		OutputMax:             1.0,
	}
	ctrl := New(tuning)

	// Initialize
	ctrl.UpdateValue(0.5)
	time.Sleep(10 * time.Millisecond)

	// Continuously above setpoint should accumulate integral
	for i := 0; i < 10; i++ {
		time.Sleep(10 * time.Millisecond)
		ctrl.UpdateValue(0.8) // 0.3 above setpoint
	}

	integral, _, _, _ := ctrl.State()
	if integral <= 0 {
		t.Errorf("expected positive integral after sustained error, got %v", integral)
	}
}

func TestController_FilteredDerivative(t *testing.T) {
	tuning := pidif.Tuning{
		Kp:                    0.0,
		Ki:                    0.0,
		Kd:                    1.0, // Only Kd
		Setpoint:              0.5,
		DerivativeFilterAlpha: 0.5, // 50% filtering
		IntegralMin:           -10,
		IntegralMax:           10,
		OutputMin:             0,
		OutputMax:             1.0,
	}
	ctrl := New(tuning)

	// Initialize with low value
	ctrl.UpdateValue(0.5)
	time.Sleep(10 * time.Millisecond)

	// Second call with same value - derivative should be near zero
	ctrl.UpdateValue(0.5)
	_, _, prevFiltered, _ := ctrl.State()

	time.Sleep(10 * time.Millisecond)

	// Big jump - filtered derivative should be dampened
	out := ctrl.UpdateValue(1.0)

	// The filtered derivative should cause some response, but dampened
	if out.Value() < 0 {
		t.Errorf("expected non-negative output, got %v", out.Value())
	}

	_, _, newFiltered, _ := ctrl.State()
	// Filtered error should have moved toward the new error but not fully
	if newFiltered <= prevFiltered {
		t.Errorf("filtered error should increase with rising process variable")
	}
}

func TestController_AntiWindup(t *testing.T) {
	tuning := pidif.Tuning{
		Kp:                    0.0,
		Ki:                    1.0, // Only Ki
		Kd:                    0.0,
		Setpoint:              0.5,
		DerivativeFilterAlpha: 0.2,
		IntegralMin:           -1.0, // Tight integral bounds
		IntegralMax:           1.0,
		OutputMin:             0,
		OutputMax:             10.0, // Wide output bounds
	}
	ctrl := New(tuning)

	// Initialize
	ctrl.UpdateValue(0.5)

	// Drive the integral to its limit
	for i := 0; i < 100; i++ {
		time.Sleep(1 * time.Millisecond)
		ctrl.UpdateValue(1.0) // Large positive error
	}

	integral, _, _, _ := ctrl.State()
	if integral > 1.0 {
		t.Errorf("integral should be clamped at 1.0, got %v", integral)
	}
}

func TestController_Reset(t *testing.T) {
	ctrl := New(RateLimitWriteTuning())

	// Build up some state
	ctrl.UpdateValue(0.5)
	time.Sleep(10 * time.Millisecond)
	ctrl.UpdateValue(0.9)
	time.Sleep(10 * time.Millisecond)
	ctrl.UpdateValue(0.95)

	// Reset
	ctrl.Reset()

	integral, prevErr, prevFiltered, initialized := ctrl.State()
	if integral != 0 || prevErr != 0 || prevFiltered != 0 || initialized {
		t.Errorf("expected all state to be zero after reset, got integral=%v, prevErr=%v, prevFiltered=%v, initialized=%v",
			integral, prevErr, prevFiltered, initialized)
	}

	// Next call should behave like first call
	out := ctrl.UpdateValue(0.9)
	if out.Value() != 0 {
		t.Errorf("expected 0 on first call after reset, got %v", out.Value())
	}
}

func TestController_SetGains(t *testing.T) {
	ctrl := New(RateLimitWriteTuning())

	// Change gains
	ctrl.SetGains(1.0, 0.5, 0.1)

	kp, ki, kd := ctrl.Gains()
	if kp != 1.0 || ki != 0.5 || kd != 0.1 {
		t.Errorf("gains not updated correctly: kp=%v, ki=%v, kd=%v", kp, ki, kd)
	}
}

func TestController_SetSetpoint(t *testing.T) {
	ctrl := New(RateLimitWriteTuning())

	ctrl.SetSetpoint(0.7)

	if ctrl.Setpoint() != 0.7 {
		t.Errorf("setpoint not updated, got %v", ctrl.Setpoint())
	}
}

func TestController_OutputClamping(t *testing.T) {
	tuning := pidif.Tuning{
		Kp:                    10.0, // Very high Kp
		Ki:                    0.0,
		Kd:                    0.0,
		Setpoint:              0.5,
		DerivativeFilterAlpha: 0.2,
		IntegralMin:           -10,
		IntegralMax:           10,
		OutputMin:             0,
		OutputMax:             1.0, // Strict output max
	}
	ctrl := New(tuning)

	// Initialize
	ctrl.UpdateValue(0.5)
	time.Sleep(10 * time.Millisecond)

	// Very high error should be clamped
	out := ctrl.UpdateValue(2.0) // 1.5 error * 10 Kp = 15, should clamp to 1.0
	if out.Value() > 1.0 {
		t.Errorf("output should be clamped to 1.0, got %v", out.Value())
	}
	if !out.Clamped() {
		t.Errorf("expected output to be flagged as clamped")
	}
}

func TestController_Components(t *testing.T) {
	tuning := pidif.Tuning{
		Kp:                    1.0,
		Ki:                    0.5,
		Kd:                    0.1,
		Setpoint:              0.5,
		DerivativeFilterAlpha: 0.2,
		IntegralMin:           -10,
		IntegralMax:           10,
		OutputMin:             -100,
		OutputMax:             100,
	}
	ctrl := New(tuning)

	// Initialize
	ctrl.UpdateValue(0.5)
	time.Sleep(10 * time.Millisecond)

	// Get components
	out := ctrl.UpdateValue(0.8)
	p, i, d := out.Components()

	// Proportional should be positive (0.3 * 1.0 = 0.3)
	expectedP := 0.3
	if p < expectedP*0.9 || p > expectedP*1.1 {
		t.Errorf("expected P term ~%v, got %v", expectedP, p)
	}

	// Integral should be small but positive (accumulated over ~10ms)
	if i <= 0 {
		t.Errorf("expected positive I term, got %v", i)
	}

	// Derivative should be non-zero (error changed)
	// The sign depends on filtering and timing
	_ = d // Just verify it's accessible
}

func TestPresets(t *testing.T) {
	// Test that all presets create valid controllers
	tests := []struct {
		name   string
		tuning pidif.Tuning
	}{
		{"RateLimitWrite", RateLimitWriteTuning()},
		{"RateLimitRead", RateLimitReadTuning()},
		{"DifficultyAdjustment", DifficultyAdjustmentTuning()},
		{"TemperatureControl", TemperatureControlTuning(25.0)},
		{"MotorSpeed", MotorSpeedTuning()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := New(tt.tuning)
			if ctrl == nil {
				t.Error("expected non-nil controller")
				return
			}

			// Basic sanity check
			out := ctrl.UpdateValue(tt.tuning.Setpoint)
			if out == nil {
				t.Error("expected non-nil output")
			}
		})
	}
}

func TestFactoryFunctions(t *testing.T) {
	// Test convenience factory functions
	writeCtrl := NewRateLimitWriteController()
	if writeCtrl == nil {
		t.Error("NewRateLimitWriteController returned nil")
	}

	readCtrl := NewRateLimitReadController()
	if readCtrl == nil {
		t.Error("NewRateLimitReadController returned nil")
	}

	diffCtrl := NewDifficultyAdjustmentController()
	if diffCtrl == nil {
		t.Error("NewDifficultyAdjustmentController returned nil")
	}

	tempCtrl := NewTemperatureController(72.0)
	if tempCtrl == nil {
		t.Error("NewTemperatureController returned nil")
	}

	motorCtrl := NewMotorSpeedController()
	if motorCtrl == nil {
		t.Error("NewMotorSpeedController returned nil")
	}
}

func TestController_ProcessVariableInterface(t *testing.T) {
	ctrl := New(RateLimitWriteTuning())

	// Test using the full ProcessVariable interface
	pv := pidif.NewProcessVariableAt(0.9, time.Now())
	out := ctrl.Update(pv)

	// First call returns 0
	if out.Value() != 0 {
		t.Errorf("expected 0 on first call, got %v", out.Value())
	}

	time.Sleep(10 * time.Millisecond)

	pv2 := pidif.NewProcessVariableAt(0.95, time.Now())
	out2 := ctrl.Update(pv2)

	// Above setpoint should produce positive output
	if out2.Value() <= 0 {
		t.Errorf("expected positive output above setpoint, got %v", out2.Value())
	}
}

func TestController_NewWithGains(t *testing.T) {
	ctrl := NewWithGains(1.0, 0.5, 0.1, 0.7)

	kp, ki, kd := ctrl.Gains()
	if kp != 1.0 || ki != 0.5 || kd != 0.1 {
		t.Errorf("gains not set correctly: kp=%v, ki=%v, kd=%v", kp, ki, kd)
	}

	if ctrl.Setpoint() != 0.7 {
		t.Errorf("setpoint not set correctly, got %v", ctrl.Setpoint())
	}
}

func TestController_SetTuning(t *testing.T) {
	ctrl := NewDefault()

	newTuning := RateLimitWriteTuning()
	ctrl.SetTuning(newTuning)

	tuning := ctrl.Tuning()
	if tuning.Kp != newTuning.Kp || tuning.Ki != newTuning.Ki || tuning.Setpoint != newTuning.Setpoint {
		t.Errorf("tuning not updated correctly")
	}
}

func TestController_SetOutputLimits(t *testing.T) {
	ctrl := NewDefault()
	ctrl.SetOutputLimits(-5.0, 5.0)

	tuning := ctrl.Tuning()
	if tuning.OutputMin != -5.0 || tuning.OutputMax != 5.0 {
		t.Errorf("output limits not updated: min=%v, max=%v", tuning.OutputMin, tuning.OutputMax)
	}
}

func TestController_SetIntegralLimits(t *testing.T) {
	ctrl := NewDefault()
	ctrl.SetIntegralLimits(-2.0, 2.0)

	tuning := ctrl.Tuning()
	if tuning.IntegralMin != -2.0 || tuning.IntegralMax != 2.0 {
		t.Errorf("integral limits not updated: min=%v, max=%v", tuning.IntegralMin, tuning.IntegralMax)
	}
}

func TestController_SetDerivativeFilter(t *testing.T) {
	ctrl := NewDefault()
	ctrl.SetDerivativeFilter(0.5)

	tuning := ctrl.Tuning()
	if tuning.DerivativeFilterAlpha != 0.5 {
		t.Errorf("derivative filter alpha not updated: %v", tuning.DerivativeFilterAlpha)
	}
}

func TestDefaultTuning(t *testing.T) {
	tuning := pidif.DefaultTuning()

	if tuning.Kp <= 0 || tuning.Ki <= 0 || tuning.Kd <= 0 {
		t.Error("default tuning should have positive gains")
	}

	if tuning.DerivativeFilterAlpha <= 0 || tuning.DerivativeFilterAlpha > 1.0 {
		t.Errorf("default derivative filter alpha should be in (0, 1], got %v", tuning.DerivativeFilterAlpha)
	}

	if tuning.OutputMin >= tuning.OutputMax {
		t.Error("default output min should be less than max")
	}

	if tuning.IntegralMin >= tuning.IntegralMax {
		t.Error("default integral min should be less than max")
	}
}
