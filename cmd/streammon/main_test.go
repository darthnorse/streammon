package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
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

// TestReadSecretEnvFileFallbackTrimsCRLF verifies a Windows-style CRLF
// line ending on the secret file is trimmed too, not just a bare "\n".
func TestReadSecretEnvFileFallbackTrimsCRLF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("file-value\r\n"), 0600); err != nil {
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

// TestReadSecretEnvWarnsWhenBothSet verifies that when both "<name>" and
// "<name>_FILE" are set, readSecretEnv still returns the env var (existing
// precedence) but also logs a warning identifying the shadowed file, so an
// operator who mounts a secret file but forgot to remove a leftover plain
// env var gets a signal instead of a silent wrong-value bug.
func TestReadSecretEnvWarnsWhenBothSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("file-value"), 0600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}
	t.Setenv("TEST_SECRET_FILE", path)
	t.Setenv("TEST_SECRET", "plain-value")

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	got, err := readSecretEnv("TEST_SECRET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "plain-value" {
		t.Fatalf("got %q, want %q; plain env var should still win over _FILE", got, "plain-value")
	}
	if !strings.Contains(buf.String(), "TEST_SECRET") || !strings.Contains(buf.String(), "TEST_SECRET_FILE") {
		t.Fatalf("expected a warning naming both TEST_SECRET and TEST_SECRET_FILE, got log output: %q", buf.String())
	}
}

// TestReadSecretEnvNoWarningWhenOnlyFileSet verifies the shadowing warning
// is not logged when only "<name>_FILE" is set (the normal Docker-secrets
// case, with nothing being silently ignored).
func TestReadSecretEnvNoWarningWhenOnlyFileSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("file-value"), 0600); err != nil {
		t.Fatalf("writing secret file: %v", err)
	}
	t.Setenv("TEST_SECRET_FILE", path)

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	if _, err := readSecretEnv("TEST_SECRET"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no log output when only _FILE is set, got %q", buf.String())
	}
}

// TestOptionalSecretReturnsValueWhenSet verifies optionalSecret behaves
// like readSecretEnv for the success path.
func TestOptionalSecretReturnsValueWhenSet(t *testing.T) {
	t.Setenv("TEST_OPT_SECRET", "plain-value")

	if got := optionalSecret("TEST_OPT_SECRET"); got != "plain-value" {
		t.Fatalf("got %q, want %q", got, "plain-value")
	}
}

// TestOptionalSecretWarnsAndReturnsEmptyOnFileReadError verifies that for
// an optional secret (e.g. TMDB_API_KEY), a "<name>_FILE" read error
// degrades gracefully: it's logged as a warning and an empty string is
// returned instead of propagating the error, so a mis-typed _FILE path or
// a secrets-mount race can't take down the whole app for an optional
// integration.
func TestOptionalSecretWarnsAndReturnsEmptyOnFileReadError(t *testing.T) {
	t.Setenv("TEST_OPT_SECRET_FILE", filepath.Join(t.TempDir(), "does-not-exist"))

	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	got := optionalSecret("TEST_OPT_SECRET")
	if got != "" {
		t.Fatalf("got %q, want empty string on read error", got)
	}
	if !strings.Contains(buf.String(), "WARNING") {
		t.Fatalf("expected a WARNING logged on read error, got %q", buf.String())
	}
}

// TestOptionalSecretNeitherSet verifies the no-configuration case still
// returns an empty string with no warning logged.
func TestOptionalSecretNeitherSet(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	if got := optionalSecret("TEST_OPT_SECRET_UNSET"); got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no log output when secret is simply unconfigured, got %q", buf.String())
	}
}
