package ratelimit

import (
	"testing"
	"time"
)

func TestPIDController_BasicOperation(t *testing.T) {
	pid := DefaultPIDControllerForWrites()

	// First call should return 0 (initialization)
	delay := pid.Update(0.5)
	if delay != 0 {
		t.Errorf("expected 0 delay on first call, got %v", delay)
	}

	// Sleep a bit to ensure dt > 0
	time.Sleep(10 * time.Millisecond)

	// Process variable below setpoint (0.5 < 0.85) should return 0 delay
	delay = pid.Update(0.5)
	if delay != 0 {
		t.Errorf("expected 0 delay when below setpoint, got %v", delay)
	}

	// Process variable above setpoint should return positive delay
	time.Sleep(10 * time.Millisecond)
	delay = pid.Update(0.95) // 0.95 > 0.85 setpoint
	if delay <= 0 {
		t.Errorf("expected positive delay when above setpoint, got %v", delay)
	}
}

func TestPIDController_IntegralAccumulation(t *testing.T) {
	pid := NewPIDController(
		0.5, 0.5, 0.0, // High Ki, no Kd
		0.5,  // setpoint
		0.2,  // filter alpha
		-10, 10, // integral bounds
		0, 1.0, // output bounds
	)

	// Initialize
	pid.Update(0.5)
	time.Sleep(10 * time.Millisecond)

	// Continuously above setpoint should accumulate integral
	for i := 0; i < 10; i++ {
		time.Sleep(10 * time.Millisecond)
		pid.Update(0.8) // 0.3 above setpoint
	}

	integral, _, _ := pid.GetState()
	if integral <= 0 {
		t.Errorf("expected positive integral after sustained error, got %v", integral)
	}
}

func TestPIDController_FilteredDerivative(t *testing.T) {
	pid := NewPIDController(
		0.0, 0.0, 1.0, // Only Kd
		0.5,  // setpoint
		0.5,  // 50% filtering
		-10, 10,
		0, 1.0,
	)

	// Initialize with low value
	pid.Update(0.5)
	time.Sleep(10 * time.Millisecond)

	// Second call with same value - derivative should be near zero
	pid.Update(0.5)
	_, _, prevFiltered := pid.GetState()

	time.Sleep(10 * time.Millisecond)

	// Big jump - filtered derivative should be dampened
	delay := pid.Update(1.0)

	// The filtered derivative should cause some response, but dampened
	// Since we only have Kd=1.0 and alpha=0.5, the response should be modest
	if delay < 0 {
		t.Errorf("expected non-negative delay, got %v", delay)
	}

	_, _, newFiltered := pid.GetState()
	// Filtered error should have moved toward the new error but not fully
	if newFiltered <= prevFiltered {
		t.Errorf("filtered error should increase with rising process variable")
	}
}

func TestPIDController_AntiWindup(t *testing.T) {
	pid := NewPIDController(
		0.0, 1.0, 0.0, // Only Ki
		0.5,     // setpoint
		0.2,     // filter alpha
		-1.0, 1.0, // tight integral bounds
		0, 10.0, // wide output bounds
	)

	// Initialize
	pid.Update(0.5)

	// Drive the integral to its limit
	for i := 0; i < 100; i++ {
		time.Sleep(1 * time.Millisecond)
		pid.Update(1.0) // Large positive error
	}

	integral, _, _ := pid.GetState()
	if integral > 1.0 {
		t.Errorf("integral should be clamped at 1.0, got %v", integral)
	}
}

func TestPIDController_Reset(t *testing.T) {
	pid := DefaultPIDControllerForWrites()

	// Build up some state
	pid.Update(0.5)
	time.Sleep(10 * time.Millisecond)
	pid.Update(0.9)
	time.Sleep(10 * time.Millisecond)
	pid.Update(0.95)

	// Reset
	pid.Reset()

	integral, prevErr, prevFiltered := pid.GetState()
	if integral != 0 || prevErr != 0 || prevFiltered != 0 {
		t.Errorf("expected all state to be zero after reset")
	}

	// Next call should behave like first call
	delay := pid.Update(0.9)
	if delay != 0 {
		t.Errorf("expected 0 delay on first call after reset, got %v", delay)
	}
}

func TestPIDController_SetGains(t *testing.T) {
	pid := DefaultPIDControllerForWrites()

	// Change gains
	pid.SetGains(1.0, 0.5, 0.1)

	if pid.Kp != 1.0 || pid.Ki != 0.5 || pid.Kd != 0.1 {
		t.Errorf("gains not updated correctly")
	}
}

func TestPIDController_SetSetpoint(t *testing.T) {
	pid := DefaultPIDControllerForWrites()

	pid.SetSetpoint(0.7)

	if pid.Setpoint != 0.7 {
		t.Errorf("setpoint not updated, got %v", pid.Setpoint)
	}
}

func TestDefaultControllers(t *testing.T) {
	writePID := DefaultPIDControllerForWrites()
	readPID := DefaultPIDControllerForReads()

	// Write controller should have higher gains and lower setpoint
	if writePID.Kp <= readPID.Kp {
		t.Errorf("write Kp should be higher than read Kp")
	}

	if writePID.Setpoint >= readPID.Setpoint {
		t.Errorf("write setpoint should be lower than read setpoint")
	}
}
