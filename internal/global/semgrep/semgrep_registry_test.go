package semgrep

import (
	"reflect"
	"testing"

	"github.com/aykutssert/scout/internal/core"
)

func TestSplitRegistrySeparatesRemoteFromLocal(t *testing.T) {
	registry, local := splitRegistry([]string{
		"p/javascript",
		"r/typescript.lang.security",
		"https://semgrep.dev/p/react",
		"rules/javascript",
		"/abs/path/to/rules",
		"./local.yaml",
	})
	wantRegistry := []string{"p/javascript", "r/typescript.lang.security", "https://semgrep.dev/p/react"}
	wantLocal := []string{"rules/javascript", "/abs/path/to/rules", "./local.yaml"}
	if !reflect.DeepEqual(registry, wantRegistry) {
		t.Fatalf("registry mismatch\nwant %#v\n got %#v", wantRegistry, registry)
	}
	if !reflect.DeepEqual(local, wantLocal) {
		t.Fatalf("local mismatch\nwant %#v\n got %#v", wantLocal, local)
	}
}

func TestIsRegistryFailureMatchesNetworkErrors(t *testing.T) {
	networkStderr := []string{
		"Failed to download config from https://semgrep.dev/p/javascript",
		"Could not reach the Semgrep Registry",
		"requests.exceptions.ConnectionError: Max retries exceeded with url",
		"socket.gaierror: [Errno -3] Temporary failure in name resolution",
		"OSError: [Errno 101] Network is unreachable",
		"requests.exceptions.SSLError: certificate verify failed",
		"503 Server Error: Service Unavailable for url",
		"502 Bad Gateway",
	}
	for _, s := range networkStderr {
		if !isRegistryFailure(s) {
			t.Errorf("expected registry failure for stderr: %q", s)
		}
	}
}

func TestIsRegistryFailureIgnoresRealConfigErrors(t *testing.T) {
	configStderr := []string{
		"Invalid rule schema: 'patterns' is a required property",
		"Rule 'foo' has no valid pattern",
		"YAML parse error at line 4",
		"Cannot parse target file app.ts",
	}
	for _, s := range configStderr {
		if isRegistryFailure(s) {
			t.Errorf("config error misclassified as registry failure: %q", s)
		}
	}
}

func TestRegistryNoticeIsWarningNotError(t *testing.T) {
	for _, ranLocal := range []bool{true, false} {
		n := registryNotice("semgrep", ranLocal)
		if n.RuleID != "semgrep-registry-unavailable" {
			t.Fatalf("ruleID: %q", n.RuleID)
		}
		if n.Severity != core.SeverityWarning {
			t.Fatalf("registry-unavailable must be a warning, not an error: %v", n.Severity)
		}
		if n.Confidence != core.ConfidenceHint {
			t.Fatalf("confidence: %q", n.Confidence)
		}
	}
}
