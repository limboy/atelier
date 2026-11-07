package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// fakeTransport answers every request with status and body, counting them.
type fakeTransport struct {
	status int
	body   string
	calls  int
}

func (f *fakeTransport) RoundTrip(*http.Request) (*http.Response, error) {
	f.calls++
	if f.status == 0 {
		return nil, errors.New("offline")
	}
	return &http.Response{StatusCode: f.status, Status: http.StatusText(f.status), Body: io.NopCloser(strings.NewReader(f.body))}, nil
}

func newTestMet(t *testing.T, ft *fakeTransport) *Met {
	m := NewMet(t.TempDir())
	m.http = &http.Client{Transport: ft}
	m.tick = time.NewTicker(time.Millisecond)
	return m
}

// cache saves body as the copy of u, fetched age ago.
func cache(t *testing.T, m *Met, u, body string, age time.Duration) string {
	t.Helper()
	path := m.cachePath(u)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-age)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	return path
}

const apiURL = metBase + "/objects/1"

func TestStaleCopyWhenFetchFails(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{"offline", 0},
		{"server error", http.StatusInternalServerError},
		{"firewall", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ft := &fakeTransport{status: 200, body: `{"objectID":1}`}
			m := newTestMet(t, ft)
			cache(t, m, apiURL, `{"old":true}`, 48*time.Hour)
			ft.status = tc.status

			b, err := m.get(context.Background(), apiURL, 24*time.Hour)
			if err != nil || string(b) != `{"old":true}` {
				t.Fatalf("got %q, %v; want the stale copy", b, err)
			}
			if ft.calls != 1 {
				t.Errorf("%d requests, want 1: with a copy to fall back on, no retries", ft.calls)
			}
		})
	}
}

func TestStaleCopyAtOnceWhilePaused(t *testing.T) {
	ft := &fakeTransport{status: 200, body: `{"objectID":1}`}
	m := newTestMet(t, ft)
	cache(t, m, apiURL, `{"old":true}`, 48*time.Hour)
	m.pauseUntil = time.Now().Add(time.Hour)

	b, err := m.get(context.Background(), apiURL, 24*time.Hour)
	if err != nil || string(b) != `{"old":true}` {
		t.Fatalf("got %q, %v; want the stale copy", b, err)
	}
	if ft.calls != 0 {
		t.Errorf("%d requests while paused, want 0", ft.calls)
	}
}

func TestNotFoundIsNotHiddenByStaleCopy(t *testing.T) {
	ft := &fakeTransport{status: 200, body: `{"objectID":1}`}
	m := newTestMet(t, ft)
	cache(t, m, apiURL, `{"old":true}`, 48*time.Hour)
	ft.status = http.StatusNotFound

	if _, err := m.get(context.Background(), apiURL, 24*time.Hour); !errors.Is(err, errNotFound) {
		t.Fatalf("err = %v, want errNotFound", err)
	}
}

func TestStaleCopyIsRefreshed(t *testing.T) {
	ft := &fakeTransport{status: 200, body: `{"objectID":1}`}
	m := newTestMet(t, ft)
	path := cache(t, m, apiURL, `{"old":true}`, 48*time.Hour)
	ft.body = `{"new":true}`

	b, err := m.get(context.Background(), apiURL, 24*time.Hour)
	if err != nil || string(b) != `{"new":true}` {
		t.Fatalf("got %q, %v; want the new copy", b, err)
	}
	if saved, _ := os.ReadFile(path); string(saved) != `{"new":true}` {
		t.Errorf("saved %q, want the new copy", saved)
	}
}

func TestFreshCopyMakesNoRequest(t *testing.T) {
	ft := &fakeTransport{status: 200, body: `{"objectID":1}`}
	m := newTestMet(t, ft)
	cache(t, m, apiURL, `{"old":true}`, time.Hour)

	if b, _ := m.get(context.Background(), apiURL, 24*time.Hour); string(b) != `{"old":true}` {
		t.Fatalf("got %q, want the cached copy", b)
	}
	if ft.calls != 0 {
		t.Errorf("%d requests, want 0", ft.calls)
	}
}

func TestNoCopyStillReportsTheError(t *testing.T) {
	ft := &fakeTransport{status: 0}
	m := newTestMet(t, ft)
	if _, err := m.get(context.Background(), apiURL, 24*time.Hour); err == nil {
		t.Fatal("no error offline with nothing cached")
	}
	if ft.calls != 3 {
		t.Errorf("%d requests, want 3: without a copy, it retries", ft.calls)
	}
}
