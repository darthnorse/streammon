package server

import (
	"fmt"
	"sync"
	"testing"
)

func TestSSEConnLimiter_AllowsUpToCap(t *testing.T) {
	var l sseConnLimiter
	for i := 0; i < maxSSEConnsPerPrincipal; i++ {
		if !l.tryAcquire("user:1") {
			t.Fatalf("acquire %d: expected success within cap", i)
		}
	}
}

func TestSSEConnLimiter_RejectsOverCap(t *testing.T) {
	var l sseConnLimiter
	for i := 0; i < maxSSEConnsPerPrincipal; i++ {
		if !l.tryAcquire("user:1") {
			t.Fatalf("acquire %d: expected success within cap", i)
		}
	}
	if l.tryAcquire("user:1") {
		t.Fatal("expected (cap+1)th acquire to be rejected")
	}
}

func TestSSEConnLimiter_ReleaseFreesSlot(t *testing.T) {
	var l sseConnLimiter
	for i := 0; i < maxSSEConnsPerPrincipal; i++ {
		if !l.tryAcquire("user:1") {
			t.Fatalf("acquire %d: expected success within cap", i)
		}
	}
	if l.tryAcquire("user:1") {
		t.Fatal("expected over-cap acquire to be rejected before release")
	}
	l.release("user:1")
	if !l.tryAcquire("user:1") {
		t.Fatal("expected acquire to succeed after release frees a slot")
	}
}

func TestSSEConnLimiter_DifferentPrincipalsAreIndependent(t *testing.T) {
	var l sseConnLimiter
	for i := 0; i < maxSSEConnsPerPrincipal; i++ {
		if !l.tryAcquire("user:1") {
			t.Fatalf("user:1 acquire %d: expected success within cap", i)
		}
	}
	if !l.tryAcquire("user:2") {
		t.Fatal("expected a different principal to be unaffected by another principal's cap")
	}
}

func TestSSEConnLimiter_ReleaseDoesNotUnderflow(t *testing.T) {
	var l sseConnLimiter
	// Releasing a principal with no acquired slots must not panic or go negative.
	l.release("user:1")
	if !l.tryAcquire("user:1") {
		t.Fatal("expected acquire to succeed after a no-op release")
	}
	l.mu.Lock()
	count := l.counts["user:1"]
	l.mu.Unlock()
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestSSEConnLimiter_ConcurrentAcquireRelease(t *testing.T) {
	var l sseConnLimiter
	const goroutines = 50
	const iterations = 200

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			principal := fmt.Sprintf("user:%d", g%5)
			for i := 0; i < iterations; i++ {
				if l.tryAcquire(principal) {
					l.release(principal)
				}
			}
		}(g)
	}
	wg.Wait()

	l.mu.Lock()
	defer l.mu.Unlock()
	for principal, count := range l.counts {
		if count != 0 {
			t.Errorf("principal %q left with count %d, want 0 (leak or underflow)", principal, count)
		}
	}
}
