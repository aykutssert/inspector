package execx

import (
	"os/exec"
	"strings"
	"testing"
)

func TestErrWithOutputFallsBackToStdout(t *testing.T) {
	_, err := exec.Command("sh", "-c", "printf 'structured error'; exit 7").Output()
	if err == nil {
		t.Fatal("expected command failure")
	}
	got := ErrWithOutput(err, []byte("structured error")).Error()
	if !strings.Contains(got, "structured error") {
		t.Fatalf("expected stdout in wrapped error, got %q", got)
	}
}
