package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Neo4jDocker manages a Neo4j instance via Docker Compose
type Neo4jDocker struct {
	composeFile string
	projectName string
}

// NewNeo4jDocker creates a new Neo4j Docker manager
func NewNeo4jDocker() (*Neo4jDocker, error) {
	// Look for docker-compose-neo4j.yml in current directory or cmd/benchmark
	composeFile := "docker-compose-neo4j.yml"
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		// Try in cmd/benchmark directory
		composeFile = filepath.Join("cmd", "benchmark", "docker-compose-neo4j.yml")
	}

	return &Neo4jDocker{
		composeFile: composeFile,
		projectName: "orly-benchmark-neo4j",
	}, nil
}

// Start starts the Neo4j Docker container
func (d *Neo4jDocker) Start() error {
	fmt.Println("Starting Neo4j Docker container...")

	// Pull image first
	pullCmd := exec.Command("docker-compose",
		"-f", d.composeFile,
		"-p", d.projectName,
		"pull",
	)
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr
	if err := pullCmd.Run(); err != nil {
		return fmt.Errorf("failed to pull Neo4j image: %w", err)
	}

	// Start containers
	upCmd := exec.Command("docker-compose",
		"-f", d.composeFile,
		"-p", d.projectName,
		"up", "-d",
	)
	upCmd.Stdout = os.Stdout
	upCmd.Stderr = os.Stderr
	if err := upCmd.Run(); err != nil {
		return fmt.Errorf("failed to start Neo4j container: %w", err)
	}

	fmt.Println("Waiting for Neo4j to be healthy...")
	if err := d.waitForHealthy(); err != nil {
		return err
	}

	fmt.Println("Neo4j is ready!")
	return nil
}

// waitForHealthy waits for Neo4j to become healthy
func (d *Neo4jDocker) waitForHealthy() error {
	timeout := 120 * time.Second
	deadline := time.Now().Add(timeout)

	containerName := "orly-benchmark-neo4j"

	for time.Now().Before(deadline) {
		// Check container health status
		checkCmd := exec.Command("docker", "inspect",
			"--format={{.State.Health.Status}}",
			containerName,
		)
		output, err := checkCmd.Output()
		if err == nil && string(output) == "healthy\n" {
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("Neo4j failed to become healthy within %v", timeout)
}

// Stop stops and removes the Neo4j Docker container
func (d *Neo4jDocker) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get logs before stopping (useful for debugging)
	logsCmd := exec.CommandContext(ctx, "docker-compose",
		"-f", d.composeFile,
		"-p", d.projectName,
		"logs", "--tail=50",
	)
	logsCmd.Stdout = os.Stdout
	logsCmd.Stderr = os.Stderr
	_ = logsCmd.Run() // Ignore errors

	fmt.Println("Stopping Neo4j Docker container...")

	// Stop and remove containers
	downCmd := exec.Command("docker-compose",
		"-f", d.composeFile,
		"-p", d.projectName,
		"down", "-v",
	)
	downCmd.Stdout = os.Stdout
	downCmd.Stderr = os.Stderr
	if err := downCmd.Run(); err != nil {
		return fmt.Errorf("failed to stop Neo4j container: %w", err)
	}

	return nil
}

// GetBoltEndpoint returns the Neo4j Bolt endpoint
func (d *Neo4jDocker) GetBoltEndpoint() string {
	return "bolt://localhost:7687"
}

// IsRunning returns whether Neo4j is running
func (d *Neo4jDocker) IsRunning() bool {
	checkCmd := exec.Command("docker", "ps", "--filter", "name=orly-benchmark-neo4j", "--format", "{{.Names}}")
	output, err := checkCmd.Output()
	return err == nil && len(output) > 0
}

// Logs returns the logs from Neo4j container
func (d *Neo4jDocker) Logs(tail int) (string, error) {
	logsCmd := exec.Command("docker-compose",
		"-f", d.composeFile,
		"-p", d.projectName,
		"logs", "--tail", fmt.Sprintf("%d", tail),
	)
	output, err := logsCmd.CombinedOutput()
	return string(output), err
}
