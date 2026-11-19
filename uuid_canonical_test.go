package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	thingscloud "github.com/arthursoares/things-cloud-sdk"
	memory "github.com/arthursoares/things-cloud-sdk/state/memory"
)

// generateUUID once used a hand-rolled Base58 encoder that dropped leading
// zero bytes. About 1 identifier in 255 then decoded to fewer than 16 bytes,
// which Things.app cannot read: it crashes in BSIdentifierFromBase58String
// and the history cannot be repaired. Issue 30 measured 1962 invalid IDs in
// 500000 with that encoder.
//
// A canonical identifier decodes to exactly 16 bytes. Length is 22 characters
// for about 97% of random UUIDs and legitimately 21 for the rest (58^21 is
// larger than 2^120), so length alone does not identify a bad ID. The crashing
// form is a short string that decodes to 15 bytes or fewer.
func TestGenerateUUIDIsAlwaysCanonical(t *testing.T) {
	const n = 500000
	var leadingZero, len21, len22 int
	for i := 0; i < n; i++ {
		id := generateUUID()
		decoded, err := thingscloud.DecodeUUID(id)
		if err != nil {
			t.Fatalf("generateUUID produced %q (len %d): %v", id, len(id), err)
		}
		if got := len(decoded); got != 16 {
			t.Fatalf("generateUUID %q decoded to %d bytes, want 16", id, got)
		}
		switch len(id) {
		case 21:
			len21++
		case 22:
			len22++
		default:
			t.Fatalf("generateUUID produced %q with length %d, want 21 or 22", id, len(id))
		}
		if decoded[0] == 0 {
			leadingZero++
			if id[0] != '1' {
				t.Fatalf("leading-zero UUID encoded as %q, want a leading '1'", id)
			}
		}
	}
	if leadingZero == 0 {
		t.Fatal("500000 samples included no leading-zero UUID; the original bug would not have been exercised")
	}
	if len22 == 0 || len21 == 0 {
		t.Fatalf("expected both 21- and 22-character canonical IDs, got 21=%d 22=%d", len21, len22)
	}
}

// History writes are refused before the commit POST when any identifier in
// the payload is the 21-character form that decodes to 15 bytes.
func TestWriteGuardRejects21CharIdentifier(t *testing.T) {
	const crashID = "5wgL7Ek24ZTFXK6zBRtTi"
	if _, err := thingscloud.DecodeUUID(crashID); err == nil {
		t.Fatalf("fixture %q unexpectedly decoded", crashID)
	}

	var posts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			posts++
			t.Errorf("commit POST with a non-canonical identifier")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{}, "current-item-index": 0, "schema": 301,
		})
	}))
	defer server.Close()

	client := thingscloud.New(server.URL, "test@example.com", "password", thingscloud.WithRequestInterval(0))
	newMCP := func() *ThingsMCP {
		return &ThingsMCP{
			client:  client,
			history: &thingscloud.History{Client: client, ID: "history", LatestSchemaVersion: 301},
			state:   memory.NewState(),
		}
	}

	itemErr := newMCP().writeAndSync(writeEnvelope{
		id: crashID, action: 0, kind: "Task6", payload: map[string]any{"tt": "poison"},
	})
	if itemErr == nil {
		t.Fatal("write of a 21-character item UUID succeeded")
	}
	if !strings.Contains(itemErr.Error(), crashID) {
		t.Fatalf("item error %q does not name %s", itemErr, crashID)
	}

	refErr := newMCP().writeAndSync(writeEnvelope{
		id: testTaskID, action: 0, kind: "Task6",
		payload: map[string]any{"tt": "poison", "pr": []string{crashID}},
	})
	if refErr == nil {
		t.Fatal("write referencing a 21-character project UUID succeeded")
	}
	if !strings.Contains(refErr.Error(), crashID) {
		t.Fatalf("reference error %q does not name %s", refErr, crashID)
	}
	if posts != 0 {
		t.Fatalf("Things Cloud received %d commits", posts)
	}
}
