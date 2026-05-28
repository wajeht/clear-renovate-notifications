package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testConfig() config {
	return config{
		token:     "test-token",
		globs:     []string{"wajeht/*"},
		botLogins: []string{"wajeht-renovate", "wajeht-renovate[bot]"},
		markMode:  "done",
	}
}

func TestMatchRepo(t *testing.T) {
	globs := []string{"wajeht/*", "acme/api"}
	cases := map[string]bool{
		"wajeht/commit": true,
		"acme/api":      true,
		"acme/web":      false,
		"other/thing":   false,
	}
	for repo, want := range cases {
		if got := matchRepo(globs, repo); got != want {
			t.Errorf("matchRepo(%q) = %v, want %v", repo, got, want)
		}
	}
}

// 111 = PR by the renovate bot; 222 = PR by a human. subject.url points back at
// the same stub so prAuthor can fetch the author.
const notificationsJSON = `[
  {"id":"111","subject":{"title":"chore(deps): update postgres docker tag to v18","type":"PullRequest","url":"%s/repos/wajeht/home-ops/pulls/1"},"repository":{"full_name":"wajeht/home-ops"}},
  {"id":"222","subject":{"title":"Fix login redirect bug","type":"PullRequest","url":"%s/repos/wajeht/home-ops/pulls/2"},"repository":{"full_name":"wajeht/home-ops"}}
]`

func newStub(t *testing.T, deletes, patches *[]string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/notifications":
			fmt.Fprintf(w, notificationsJSON, srv.URL, srv.URL)
		case r.URL.Path == "/repos/wajeht/home-ops/pulls/1":
			io.WriteString(w, `{"user":{"login":"wajeht-renovate[bot]"}}`)
		case r.URL.Path == "/repos/wajeht/home-ops/pulls/2":
			io.WriteString(w, `{"user":{"login":"jaw"}}`)
		case r.Method == http.MethodDelete:
			*deletes = append(*deletes, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch:
			*patches = append(*patches, r.URL.Path)
			w.WriteHeader(http.StatusResetContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = "https://api.github.com"; srv.Close() })
	return srv
}

func TestPollClearsOnlyBotPRs(t *testing.T) {
	var deletes, patches []string
	newStub(t, &deletes, &patches)

	if _, err := poll(http.DefaultClient, testConfig(), nil); err != nil {
		t.Fatalf("poll: %v", err)
	}

	want := "/notifications/threads/111"
	if len(deletes) != 1 || deletes[0] != want {
		t.Errorf("deletes = %v, want [%s]", deletes, want)
	}
	if len(patches) != 0 {
		t.Errorf("patches = %v, want none", patches)
	}
}

func TestPollDryRunMakesNoMutations(t *testing.T) {
	var deletes, patches []string
	newStub(t, &deletes, &patches)

	cfg := testConfig()
	cfg.dryRun = true
	if _, err := poll(http.DefaultClient, cfg, nil); err != nil {
		t.Fatalf("poll: %v", err)
	}

	if len(deletes) != 0 || len(patches) != 0 {
		t.Errorf("dry-run mutated: deletes=%v patches=%v", deletes, patches)
	}
}

func TestPollReadModeUsesPatch(t *testing.T) {
	var deletes, patches []string
	newStub(t, &deletes, &patches)

	cfg := testConfig()
	cfg.markMode = "read"
	if _, err := poll(http.DefaultClient, cfg, nil); err != nil {
		t.Fatalf("poll: %v", err)
	}

	want := "/notifications/threads/111"
	if len(patches) != 1 || patches[0] != want {
		t.Errorf("patches = %v, want [%s]", patches, want)
	}
	if len(deletes) != 0 {
		t.Errorf("deletes = %v, want none", deletes)
	}
}

func TestPollUsesLastModifiedAndPollInterval(t *testing.T) {
	const lastModified = "Wed, 01 Jan 2025 00:00:00 GMT"
	var seenIfModifiedSince []string
	var calls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/notifications" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}

		calls++
		seenIfModifiedSince = append(seenIfModifiedSince, r.Header.Get("If-Modified-Since"))

		w.Header().Set("X-Poll-Interval", "120")
		if calls == 1 {
			w.Header().Set("Last-Modified", lastModified)
			io.WriteString(w, `[]`)
			return
		}

		w.WriteHeader(http.StatusNotModified)
	}))
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = "https://api.github.com"; srv.Close() })

	cfg := testConfig()
	cfg.interval = 60 * time.Second
	var storedLastModified string

	delay, err := poll(http.DefaultClient, cfg, &storedLastModified)
	if err != nil {
		t.Fatalf("first poll: %v", err)
	}
	if delay != 120*time.Second {
		t.Errorf("delay = %s, want 2m0s", delay)
	}
	if storedLastModified != lastModified {
		t.Errorf("storedLastModified = %q, want %q", storedLastModified, lastModified)
	}

	if _, err := poll(http.DefaultClient, cfg, &storedLastModified); err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if len(seenIfModifiedSince) != 2 || seenIfModifiedSince[0] != "" || seenIfModifiedSince[1] != lastModified {
		t.Errorf("If-Modified-Since values = %v", seenIfModifiedSince)
	}
}
