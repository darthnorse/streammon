package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestRunBoundedCleanupCompletesWhenFnFinishesFast verifies the happy path:
// fn finishes well within the budget, and runBoundedCleanup returns as soon
// as it does rather than waiting out the full deadline.
func TestRunBoundedCleanupCompletesWhenFnFinishesFast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var ran bool
	var mu sync.Mutex

	start := time.Now()
	runBoundedCleanup(ctx, func() {
		mu.Lock()
		ran = true
		mu.Unlock()
	})
	elapsed := time.Since(start)

	mu.Lock()
	defer mu.Unlock()
	if !ran {
		t.Fatal("expected fn to run")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("runBoundedCleanup took %v for an instantly-completing fn; expected it to return promptly", elapsed)
	}
}

// TestRunBoundedCleanupReturnsOnDeadline verifies the shutdown sequence
// itself is bounded: if a cleanup step hangs (e.g. a stuck DB write with no
// deadline of its own), runBoundedCleanup must still return once ctx's
// deadline passes rather than blocking main() forever and risking a
// container SIGKILL.
func TestRunBoundedCleanupReturnsOnDeadline(t *testing.T) {
	const budget = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()

	blockForever := make(chan struct{})
	defer close(blockForever) // let the leaked goroutine exit once the test is done

	start := time.Now()
	runBoundedCleanup(ctx, func() {
		<-blockForever
	})
	elapsed := time.Since(start)

	if elapsed < budget {
		t.Fatalf("runBoundedCleanup returned after %v, before its %v budget elapsed", elapsed, budget)
	}
	if elapsed > budget+2*time.Second {
		t.Fatalf("runBoundedCleanup took %v to return after a %v budget; expected it to give up promptly once the deadline passed", elapsed, budget)
	}
}

// TestReadSecretEnvPlainVar verifies the plain "<name>" env var is returned
// as-is when set, without consulting "<name>_FILE" at all.
func TestReadSecretEnvPlainVar(t *testing.T) {
	t.Setenv("TEST_SECRET", "plain-value")

	got, err := readSecretEnv("TEST_SECRET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "plain-value" {
		t.Fatalf("got %q, want %q", got, "plain-value")
	}
}

// TestReadSecretEnvFileFallback verifies the value is read from the file
// named by "<name>_FILE" when the plain env var is unset, and that a
// trailing newline (as written by `echo` or most secret-mounting tooling)
// is trimmed.
func TestReadSecretEnvFileFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("file-value\n"), 0600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}
	t.Setenv("TEST_SECRET_FILE", path)

	got, err := readSecretEnv("TEST_SECRET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "file-value" {
		t.Fatalf("got %q, want %q", got, "file-value")
	}
}

// TestReadSecretEnvPlainVarWinsOverFile verifies the plain env var takes
// precedence when both "<name>" and "<name>_FILE" are set.
func TestReadSecretEnvPlainVarWinsOverFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("file-value"), 0600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}
	t.Setenv("TEST_SECRET_FILE", path)
	t.Setenv("TEST_SECRET", "plain-value")

	got, err := readSecretEnv("TEST_SECRET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "plain-value" {
		t.Fatalf("got %q, want %q; plain env var should win over _FILE", got, "plain-value")
	}
}

// TestReadSecretEnvNeitherSet verifies an empty string and no error is
// returned when neither the plain var nor the _FILE var is set, so callers
// can treat it the same as "not configured".
func TestReadSecretEnvNeitherSet(t *testing.T) {
	got, err := readSecretEnv("TEST_SECRET_UNSET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}

// TestReadSecretEnvFileMissing verifies a missing file referenced by
// "<name>_FILE" surfaces as an error rather than silently falling back to
// an empty secret.
func TestReadSecretEnvFileMissing(t *testing.T) {
	t.Setenv("TEST_SECRET_FILE", filepath.Join(t.TempDir(), "does-not-exist"))

	_, err := readSecretEnv("TEST_SECRET")
	if err == nil {
		t.Fatal("expected an error for a missing secret file, got nil")
	}
}
