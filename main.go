package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
)

var apiBase = "https://api.github.com"

type config struct {
	token     string
	interval  time.Duration
	globs     []string
	botLogins []string
	markMode  string // "done" deletes the thread, "read" marks it read
	dryRun    bool
}

type notification struct {
	ID      string `json:"id"`
	Subject struct {
		Title string `json:"title"`
		URL   string `json:"url"`
		Type  string `json:"type"`
	} `json:"subject"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func poll(client *http.Client, cfg config, lastModified *string) (time.Duration, error) {
	req, err := http.NewRequest(http.MethodGet, apiBase+"/notifications?per_page=50", nil)

	if err != nil {
		return cfg.interval, err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if lastModified != nil && *lastModified != "" {
		req.Header.Set("If-Modified-Since", *lastModified)
	}

	resp, err := client.Do(req)

	if err != nil {
		return cfg.interval, err
	}

	defer resp.Body.Close()

	delay := cfg.interval
	if seconds, err := strconv.Atoi(resp.Header.Get("X-Poll-Interval")); err == nil && seconds > 0 {
		githubDelay := time.Duration(seconds) * time.Second
		if githubDelay > delay {
			delay = githubDelay
		}
	}

	if resp.StatusCode == http.StatusNotModified {
		return delay, nil
	}

	if resp.StatusCode != http.StatusOK {
		return delay, statusError("list notifications", resp)
	}

	if lastModified != nil {
		if header := resp.Header.Get("Last-Modified"); header != "" {
			*lastModified = header
		}
	}

	var notifs []notification

	if err := json.NewDecoder(resp.Body).Decode(&notifs); err != nil {
		return delay, err
	}

	for _, n := range notifs {
		if n.Subject.Type != "PullRequest" || !matchRepo(cfg.globs, n.Repository.FullName) {
			continue
		}

		login, err := prAuthor(client, cfg, n.Subject.URL)

		if err != nil {
			slog.Error("fetch PR author", "url", n.Subject.URL, "error", err)
			continue
		}

		if !slices.ContainsFunc(cfg.botLogins, func(s string) bool { return strings.EqualFold(s, login) }) {
			continue
		}

		if cfg.dryRun {
			slog.Info("dry-run: would clear", "repo", n.Repository.FullName, "title", n.Subject.Title, "author", login)
			continue
		}

		if err := clear(client, cfg, n.ID); err != nil {
			slog.Error("clear thread", "id", n.ID, "error", err)
			continue
		}

		slog.Info("cleared", "repo", n.Repository.FullName, "title", n.Subject.Title, "author", login)
	}

	return delay, nil
}

func matchRepo(globs []string, fullName string) bool {
	for _, g := range globs {
		if ok, _ := path.Match(g, fullName); ok {
			return true
		}
	}
	return false
}

func prAuthor(client *http.Client, cfg config, url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", statusError("get PR", resp)
	}

	var pr struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return "", err
	}

	return pr.User.Login, nil
}

func clear(client *http.Client, cfg config, id string) error {
	method := http.MethodDelete
	if cfg.markMode == "read" {
		method = http.MethodPatch
	}
	req, err := http.NewRequest(method, apiBase+"/notifications/threads/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices && resp.StatusCode != http.StatusNotModified {
		return statusError("clear thread", resp)
	}
	return nil
}

func statusError(what string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("%s: %s: %s", what, resp.Status, body)
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func splitTrim(s string) []string {
	out := []string{}
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config{
		token:     os.Getenv("GITHUB_TOKEN"),
		interval:  time.Duration(envInt("POLL_INTERVAL_SECONDS", 60)) * time.Second,
		globs:     splitTrim(env("REPO_FILTER", "wajeht/*")),
		botLogins: splitTrim(env("RENOVATE_BOT_LOGINS", "wajeht-renovate,wajeht-renovate[bot]")),
		markMode:  env("MARK_MODE", "done"),
		dryRun:    env("DRY_RUN", "false") == "true",
	}

	if cfg.token == "" {
		slog.Error("GITHUB_TOKEN is required (classic PAT with the notifications scope)")
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	})

	go func() {
		if err := http.ListenAndServe(":"+env("APP_PORT", "80"), mux); err != nil {
			slog.Error("healthz server stopped", "error", err)
		}
	}()

	client := &http.Client{Timeout: 30 * time.Second}
	var lastModified string

	slog.Info("starting", "dry_run", cfg.dryRun, "mark_mode", cfg.markMode, "repos", cfg.globs, "bots", cfg.botLogins, "interval", cfg.interval)

	for {
		delay, err := poll(client, cfg, &lastModified)
		if err != nil {
			slog.Error("poll failed", "error", err)
		}

		time.Sleep(delay)
	}
}
