package integration_test

import (
	"os"
	"os/exec"
	"testing"
)

func dockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func TestStarterKitSandbox(t *testing.T) {
	if os.Getenv("VANISYNC_INTEGRATION") != "1" {
		t.Skip("set VANISYNC_INTEGRATION=1 to run integration tests")
	}
	if !dockerAvailable() {
		t.Skip("docker not available")
	}

	// Scaffold: bring up docker/compose.starter-kit.yml and assert gateway health.
	// See docker/compose.starter-kit.yml for beckn/starter-kit wiring.
	t.Skip("starter-kit end-to-end test not yet implemented")
}
