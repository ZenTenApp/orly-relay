package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// DgraphDocker manages a dgraph instance via Docker Compose
type DgraphDocker struct {
	composeFile string
	projectName string
	running     bool
}

// NewDgraphDocker creates a new dgraph Docker manager
func NewDgraphDocker() *DgraphDocker {
	// Try to find the docker-compose file in the current directory first
	composeFile := "docker-compose-dgraph.yml"

	// If not found, try the cmd/benchmark directory (for running from project root)
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		composeFile = filepath.Join("cmd", "benchmark", "docker-compose-dgraph.yml")
	}

	return &DgraphDocker{
		composeFile: composeFile,
		projectName: "orly-benchmark-dgraph",
		running:     false,
	}
}

// Start starts the dgraph Docker containers
func (d *DgraphDocker) Start(ctx context.Context) error {
	fmt.Println("Starting dgraph Docker containers...")

	// Stop any existing containers first
	d.Stop()

	// Start containers
	cmd := exec.CommandContext(
		ctx,
		"docker-compose",
		"-f", d.composeFile,
		"-p", d.projectName,
		"up", "-d",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start dgraph containers: %w", err)
	}

	fmt.Println("Waiting for dgraph to be healthy...")

	// Wait for health checks to pass
	if err := d.waitForHealthy(ctx, 60*time.Second); err != nil {
		d.Stop() // Clean up on failure
		return err
	}

	d.running = true
	fmt.Println("Dgraph is ready!")
	return nil
}

// waitForHealthy waits for dgraph to become healthy
func (d *DgraphDocker) waitForHealthy(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check if alpha is healthy by checking docker health status
		cmd := exec.CommandContext(
			ctx,
			"docker",
			"inspect",
			"--format={{.State.Health.Status}}",
			"orly-benchmark-dgraph-alpha",
		)

		output, err := cmd.Output()
		if err == nil && string(output) == "healthy\n" {
			// Additional short wait to ensure full readiness
			time.Sleep(2 * time.Second)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			// Continue waiting
		}
	}

	return fmt.Errorf("dgraph failed to become healthy within %v", timeout)
}

// Stop stops and removes the dgraph Docker containers
func (d *DgraphDocker) Stop() error {
	if !d.running {
		// Try to stop anyway in case of untracked state
		cmd := exec.Command(
			"docker-compose",
			"-f", d.composeFile,
			"-p", d.projectName,
			"down", "-v",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // Ignore errors
		return nil
	}

	fmt.Println("Stopping dgraph Docker containers...")

	cmd := exec.Command(
		"docker-compose",
		"-f", d.composeFile,
		"-p", d.projectName,
		"down", "-v",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop dgraph containers: %w", err)
	}

	d.running = false
	fmt.Println("Dgraph containers stopped")
	return nil
}

// GetGRPCEndpoint returns the dgraph gRPC endpoint
func (d *DgraphDocker) GetGRPCEndpoint() string {
	return "localhost:9080"
}

// IsRunning returns whether dgraph is running
func (d *DgraphDocker) IsRunning() bool {
	return d.running
}

// Logs returns the logs from dgraph containers
func (d *DgraphDocker) Logs() error {
	cmd := exec.Command(
		"docker-compose",
		"-f", d.composeFile,
		"-p", d.projectName,
		"logs",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
