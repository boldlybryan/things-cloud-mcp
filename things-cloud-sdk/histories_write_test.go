package thingscloud

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// crashID is the 21-character identifier from issue 30. It decodes to 15
// bytes, which Things.app cannot read.
const crashID = "5wgL7Ek24ZTFXK6zBRtTi"

// canonical21 is a legitimate 21-character identifier that still decodes to
// 16 bytes. The write guard must accept it.
const canonical21 = "8AQGAut7N92awznwCnjvy"

const canonical22 = "HLLMFXihVwDfiGakYYdv42"

type payloadItem struct {
	ID      string         `json:"-"`
	Action  int            `json:"t"`
	Kind    string         `json:"e"`
	Payload map[string]any `json:"p,omitempty"`
}

func (p payloadItem) UUID() string { return p.ID }

func TestHistoryWriteRejectsNonCanonicalUUIDBeforePost(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"server-head-index":1}`)
	}))
	defer server.Close()

	c := New(server.URL, "test@example.com", "password", WithRequestInterval(0))
	h := &History{Client: c, ID: "history", LatestSchemaVersion: 301}

	badItems := []payloadItem{
		{ID: crashID, Action: 0, Kind: "Task6", Payload: map[string]any{"tt": "poison"}},
		{ID: canonical22, Action: 0, Kind: "Task6", Payload: map[string]any{"pr": []string{crashID}}},
		{ID: canonical22, Action: 0, Kind: "Task6", Payload: map[string]any{"ar": []string{crashID}}},
		{ID: canonical22, Action: 0, Kind: "Task6", Payload: map[string]any{"tg": []string{crashID}}},
		{ID: canonical22, Action: 0, Kind: "Task6", Payload: map[string]any{"agr": []string{crashID}}},
		{ID: canonical22, Action: 0, Kind: "Task6", Payload: map[string]any{"rt": []string{crashID}}},
		{ID: canonical22, Action: 0, Kind: "Task6", Payload: map[string]any{"dl": []string{crashID}}},
		{ID: canonical22, Action: 0, Kind: "ChecklistItem3", Payload: map[string]any{"ts": []string{crashID}}},
		{ID: canonical22, Action: 0, Kind: "Tag4", Payload: map[string]any{"pn": []string{crashID}}},
		{ID: canonical22, Action: 0, Kind: "Tombstone2", Payload: map[string]any{"dloid": crashID}},
	}
	for _, item := range badItems {
		if err := h.Write(item); err == nil {
			t.Errorf("Write(%q) succeeded, want rejection", item.Payload)
			continue
		} else if !strings.Contains(err.Error(), crashID) {
			t.Errorf("Write error %q does not name %s", err, crashID)
		}
	}
	if requests != 0 {
		t.Fatalf("commit endpoint called %d times", requests)
	}
}

func TestHistoryWriteAcceptsCanonical21CharUUID(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"server-head-index":1}`)
	}))
	defer server.Close()

	c := New(server.URL, "test@example.com", "password", WithRequestInterval(0))
	h := &History{Client: c, ID: "history", LatestSchemaVersion: 301}
	item := payloadItem{
		ID: canonical21, Action: 0, Kind: "Task6",
		Payload: map[string]any{
			"tt":  "ok",
			"pr":  []string{canonical22},
			"ar":  []string{canonical22},
			"tg":  []string{canonical22},
			"agr": []string{canonical22},
		},
	}
	if err := h.Write(item); err != nil {
		t.Fatalf("Write of canonical 21-character UUID: %v", err)
	}
	if requests != 1 {
		t.Fatalf("commit endpoint called %d times, want 1", requests)
	}
}
