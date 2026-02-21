package mediautil

import "context"

const (
	PhaseItems      = "items"
	PhaseHistory    = "history"
	PhaseEnriching  = "enriching"
	PhaseEvaluating = "evaluating"
	PhaseDone       = "done"
	PhaseError      = "error"
)

type SyncProgress struct {
	Phase   string `json:"phase"`
	Current int    `json:"current,omitempty"`
	Total   int    `json:"total,omitempty"`
	Library string `json:"library"`
	Error   string `json:"error,omitempty"`
	Synced  int    `json:"synced,omitempty"`
	Deleted int    `json:"deleted,omitempty"`
}

type progressKeyType struct{}

var progressKey progressKeyType

// ContextWithProgress returns a new context with a progress channel attached,
// along with the receive-only end of that channel.
func ContextWithProgress(ctx context.Context) (context.Context, <-chan SyncProgress) {
	ch := make(chan SyncProgress, 64)
	return context.WithValue(ctx, progressKey, ch), ch
}

// SendProgress sends a progress update if a progress channel is present in the context.
// It never blocks — if the channel is full, absent, or closed, the update is silently dropped.
func SendProgress(ctx context.Context, p SyncProgress) {
	ch, ok := ctx.Value(progressKey).(chan SyncProgress)
	if !ok || ch == nil {
		return
	}
	defer func() { recover() }()
	select {
	case ch <- p:
	default:
	}
}

// CloseProgress closes the progress channel in the context, if present.
// After closing, any subsequent SendProgress calls on this context are safely ignored.
func CloseProgress(ctx context.Context) {
	ch, ok := ctx.Value(progressKey).(chan SyncProgress)
	if !ok || ch == nil {
		return
	}
	defer func() { recover() }()
	close(ch)
}
