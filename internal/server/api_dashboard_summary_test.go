package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"streammon/internal/media"
	"streammon/internal/models"
)

// fakePoller implements pollerIface for testing.
type fakePoller struct{ sessions []models.ActiveStream }

func (f *fakePoller) CurrentSessions() []models.ActiveStream          { return f.sessions }
func (f *fakePoller) Subscribe() chan []models.ActiveStream            { return nil }
func (f *fakePoller) Unsubscribe(_ chan []models.ActiveStream)         {}
func (f *fakePoller) AddServer(_ int64, _ media.MediaServer)          {}
func (f *fakePoller) RemoveServer(_ int64)                            {}
func (f *fakePoller) GetServer(_ int64) (media.MediaServer, bool)     { return nil, false }
func (f *fakePoller) RefreshIdleTimeout()                             {}

func TestDashboardSummary_NoPoller_ReturnsZeros(t *testing.T) {
	ts, _ := newTestServerWrapped(t)
	// newTestServer does not initialize a poller.
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/summary", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp dashboardSummaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.StreamCount != 0 || resp.TranscodeCount != 0 || resp.DirectPlayCount != 0 || resp.TotalBandwidthBps != 0 {
		t.Errorf("expected zeros, got %+v", resp)
	}
}

func TestDashboardSummary_AggregatesActiveStreams(t *testing.T) {
	ts, st := newTestServerWrapped(t)

	if err := st.CreateServer(&models.Server{Name: "Plex", Type: "plex", URL: "http://x"}); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	ts.Server.SetPollerForTest(&fakePoller{
		sessions: []models.ActiveStream{
			{UserName: "alice", Bandwidth: 10_000_000, VideoDecision: models.TranscodeDecisionDirectPlay},
			{UserName: "bob", Bandwidth: 20_000_000, VideoDecision: models.TranscodeDecisionTranscode},
			{UserName: "alice", Bandwidth: 5_000_000, VideoDecision: models.TranscodeDecisionDirectPlay},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/summary", nil)
	w := httptest.NewRecorder()
	ts.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp dashboardSummaryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.StreamCount != 3 {
		t.Errorf("StreamCount=%d want 3", resp.StreamCount)
	}
	if resp.TranscodeCount != 1 {
		t.Errorf("TranscodeCount=%d want 1", resp.TranscodeCount)
	}
	if resp.DirectPlayCount != 2 {
		t.Errorf("DirectPlayCount=%d want 2", resp.DirectPlayCount)
	}
	if resp.TotalBandwidthBps != 35_000_000 {
		t.Errorf("TotalBandwidthBps=%d want 35M", resp.TotalBandwidthBps)
	}
	if resp.ActiveUserCount != 2 {
		t.Errorf("ActiveUserCount=%d want 2", resp.ActiveUserCount)
	}
	if resp.ServerCount != 1 {
		t.Errorf("ServerCount=%d want 1", resp.ServerCount)
	}
}
