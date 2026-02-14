package mediautil

import (
	"context"
	"testing"
)

func TestSendProgressDeliversMessage(t *testing.T) {
	ctx, ch := ContextWithProgress(context.Background())

	want := SyncProgress{Phase: PhaseItems, Current: 5, Total: 100, Library: "1"}
	SendProgress(ctx, want)

	got := <-ch
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSendProgressNoOpWithoutChannel(t *testing.T) {
	// Should not panic or block
	SendProgress(context.Background(), SyncProgress{Phase: PhaseItems, Current: 1, Total: 10})
}

func TestSendProgressDoesNotBlockWhenFull(t *testing.T) {
	ctx, _ := ContextWithProgress(context.Background())

	// Fill the buffer (capacity 64)
	for i := 0; i < 64; i++ {
		SendProgress(ctx, SyncProgress{Phase: PhaseItems, Current: i, Total: 100})
	}

	// This should not block even though the channel is full
	SendProgress(ctx, SyncProgress{Phase: PhaseItems, Current: 65, Total: 100})
}

func TestContextWithProgressReturnsBufferedChannel(t *testing.T) {
	_, ch := ContextWithProgress(context.Background())

	if cap(ch) != 64 {
		t.Errorf("channel capacity = %d, want 64", cap(ch))
	}
}

func TestCloseProgressClosesChannel(t *testing.T) {
	ctx, ch := ContextWithProgress(context.Background())
	CloseProgress(ctx)

	// Reading from a closed channel should return zero value immediately
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed")
	}
}

func TestCloseProgressNoOpWithoutChannel(t *testing.T) {
	// Should not panic
	CloseProgress(context.Background())
}

func TestSendProgressAfterCloseDoesNotPanic(t *testing.T) {
	ctx, _ := ContextWithProgress(context.Background())
	CloseProgress(ctx)

	// Should not panic — the recover guard in SendProgress handles this
	SendProgress(ctx, SyncProgress{Phase: PhaseItems, Current: 1, Total: 10})
}
