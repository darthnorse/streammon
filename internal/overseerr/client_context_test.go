package overseerr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func slowHandler(w http.ResponseWriter, r *http.Request) {
	select {
	case <-time.After(10 * time.Second):
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	case <-r.Context().Done():
	}
}

func TestClientRespectsContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(slowHandler))
	defer ts.Close()

	c, err := NewClient(ts.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		call func(ctx context.Context) error
	}{
		{"Search", func(ctx context.Context) error {
			_, err := c.Search(ctx, "test", 1)
			return err
		}},
		{"CreateRequest", func(ctx context.Context) error {
			_, err := c.CreateRequest(ctx, []byte(`{"mediaType":"movie","mediaId":1}`))
			return err
		}},
		{"DeleteRequest", func(ctx context.Context) error {
			return c.DeleteRequest(ctx, 1)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()

			if err := tt.call(ctx); err == nil {
				t.Fatal("expected error due to context cancellation")
			}
		})
	}
}
