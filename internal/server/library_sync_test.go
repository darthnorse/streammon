package server

import (
	"errors"
	"sync"
	"testing"
	"time"

	"streammon/internal/mediautil"
)

func newTestSyncManager() *librarySyncManager {
	return &librarySyncManager{active: make(map[string]*librarySyncJob)}
}

func TestLibrarySyncTryStart(t *testing.T) {
	m := newTestSyncManager()

	if !m.tryStart("1-lib1", "lib1") {
		t.Fatal("first tryStart should succeed")
	}
	if m.tryStart("1-lib1", "lib1") {
		t.Fatal("second tryStart for same key should fail while running")
	}
	// Different key should succeed
	if !m.tryStart("2-lib2", "lib2") {
		t.Fatal("tryStart for different key should succeed")
	}

	m.finish("1-lib1", 10, 2, nil)
	m.finish("2-lib2", 5, 1, nil)
}

func TestLibrarySyncTryStartAfterFinish(t *testing.T) {
	m := newTestSyncManager()

	if !m.tryStart("1-lib1", "lib1") {
		t.Fatal("first tryStart should succeed")
	}
	m.finish("1-lib1", 10, 2, nil)

	if !m.tryStart("1-lib1", "lib1") {
		t.Fatal("tryStart should succeed after finish")
	}
	m.finish("1-lib1", 0, 0, nil)
}

func TestLibrarySyncUpdateProgress(t *testing.T) {
	m := newTestSyncManager()
	m.tryStart("1-lib1", "lib1")

	m.updateProgress("1-lib1", mediautil.SyncProgress{
		Phase:   mediautil.PhaseHistory,
		Current: 50,
		Total:   100,
		Library: "lib1",
	})

	status := m.status()
	got, ok := status["1-lib1"]
	if !ok {
		t.Fatal("expected status for 1-lib1")
	}
	if got.Phase != mediautil.PhaseHistory {
		t.Errorf("phase = %q, want %q", got.Phase, mediautil.PhaseHistory)
	}
	if got.Current != 50 || got.Total != 100 {
		t.Errorf("progress = %d/%d, want 50/100", got.Current, got.Total)
	}

	m.finish("1-lib1", 0, 0, nil)
}

func TestLibrarySyncUpdateProgressUnknownKey(t *testing.T) {
	m := newTestSyncManager()
	// Should not panic
	m.updateProgress("nonexistent", mediautil.SyncProgress{Phase: mediautil.PhaseItems})
}

func TestLibrarySyncFinishSuccess(t *testing.T) {
	m := newTestSyncManager()
	m.tryStart("1-lib1", "lib1")
	m.finish("1-lib1", 42, 5, nil)

	status := m.status()
	got, ok := status["1-lib1"]
	if !ok {
		t.Fatal("expected completed job in status")
	}
	if got.Phase != mediautil.PhaseDone {
		t.Errorf("phase = %q, want %q", got.Phase, mediautil.PhaseDone)
	}
	if got.Synced != 42 {
		t.Errorf("synced = %d, want 42", got.Synced)
	}
	if got.Deleted != 5 {
		t.Errorf("deleted = %d, want 5", got.Deleted)
	}
}

func TestLibrarySyncFinishError(t *testing.T) {
	m := newTestSyncManager()
	m.tryStart("1-lib1", "lib1")
	m.finish("1-lib1", 0, 0, &syncError{message: "server not found"})

	status := m.status()
	got, ok := status["1-lib1"]
	if !ok {
		t.Fatal("expected errored job in status")
	}
	if got.Phase != mediautil.PhaseError {
		t.Errorf("phase = %q, want %q", got.Phase, mediautil.PhaseError)
	}
	if got.Error != "server not found" {
		t.Errorf("error = %q, want %q", got.Error, "server not found")
	}
}

func TestLibrarySyncFinishGenericError(t *testing.T) {
	m := newTestSyncManager()
	m.tryStart("1-lib1", "lib1")
	m.finish("1-lib1", 0, 0, errors.New("something broke"))

	status := m.status()
	got := status["1-lib1"]
	if got.Phase != mediautil.PhaseError {
		t.Errorf("phase = %q, want %q", got.Phase, mediautil.PhaseError)
	}
	if got.Error != "something broke" {
		t.Errorf("error = %q, want %q", got.Error, "something broke")
	}
}

func TestLibrarySyncStatusCleansExpired(t *testing.T) {
	m := newTestSyncManager()
	m.tryStart("1-lib1", "lib1")

	// Manually set doneAt in the past to simulate expiry.
	m.mu.Lock()
	job := m.active["1-lib1"]
	job.doneAt = time.Now().UTC().Add(-completedJobTTL - time.Second)
	job.synced = 10
	m.mu.Unlock()
	m.wg.Done() // balance the Add(1) from tryStart

	status := m.status()
	if _, ok := status["1-lib1"]; ok {
		t.Error("expected expired job to be cleaned from status")
	}

	// Map should also be cleaned
	m.mu.Lock()
	_, inMap := m.active["1-lib1"]
	m.mu.Unlock()
	if inMap {
		t.Error("expected expired job to be removed from active map")
	}
}

func TestLibrarySyncStatusEmptyWhenNoJobs(t *testing.T) {
	m := newTestSyncManager()
	status := m.status()
	if len(status) != 0 {
		t.Errorf("expected empty status, got %d entries", len(status))
	}
}

func TestLibrarySyncWait(t *testing.T) {
	m := newTestSyncManager()
	m.tryStart("1-lib1", "lib1")

	done := make(chan struct{})
	go func() {
		m.Wait()
		close(done)
	}()

	// Wait should block until finish
	select {
	case <-done:
		t.Fatal("Wait returned before finish")
	case <-time.After(50 * time.Millisecond):
	}

	m.finish("1-lib1", 0, 0, nil)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after finish")
	}
}

func TestLibrarySyncConcurrentAccess(t *testing.T) {
	m := newTestSyncManager()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if m.tryStart("1-lib1", "lib1") {
				for j := 0; j < 100; j++ {
					m.updateProgress("1-lib1", mediautil.SyncProgress{
						Phase:   mediautil.PhaseItems,
						Current: j,
						Total:   100,
						Library: "lib1",
					})
				}
				m.status()
				m.finish("1-lib1", 100, 0, nil)
			}
		}()
	}

	wg.Wait()
}

func TestSyncErrorFallback(t *testing.T) {
	se := &syncError{message: "user message", logMessage: "detailed log"}
	if se.Error() != "detailed log" {
		t.Errorf("Error() = %q, want %q", se.Error(), "detailed log")
	}

	se2 := &syncError{message: "user message"}
	if se2.Error() != "user message" {
		t.Errorf("Error() = %q, want %q", se2.Error(), "user message")
	}
}
