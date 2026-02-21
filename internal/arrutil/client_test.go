package arrutil

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoPutSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/v3/series/10" {
			t.Errorf("expected path /api/v3/series/10, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("expected X-Api-Key test-key, got %s", r.Header.Get("X-Api-Key"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading body: %v", err)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshaling body: %v", err)
		}
		if payload["title"] != "Updated" {
			t.Errorf("expected title Updated, got %v", payload["title"])
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	c, err := New("Test", ts.URL, "test-key")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"title": "Updated"})
	resp, err := c.DoPut(context.Background(), "/series/10", body)
	if err != nil {
		t.Fatalf("DoPut: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %s", result["status"])
	}
}

func TestDoPutErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"validation failed"}`))
	}))
	defer ts.Close()

	c, err := New("Test", ts.URL, "test-key")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"title": ""})
	_, err = c.DoPut(context.Background(), "/series/10", body)
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestDoPutWithRawJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshaling body: %v", err)
		}
		if payload["id"] != float64(5) {
			t.Errorf("expected id 5, got %v", payload["id"])
		}
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer ts.Close()

	c, err := New("Test", ts.URL, "test-key")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	raw := json.RawMessage(`{"id":5,"name":"test"}`)
	resp, err := c.DoPut(context.Background(), "/resource/5", raw)
	if err != nil {
		t.Fatalf("DoPut: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result["id"] != float64(5) {
		t.Errorf("expected id 5, got %v", result["id"])
	}
}
