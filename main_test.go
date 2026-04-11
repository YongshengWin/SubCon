package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleShortenAPIUpdatesExistingToken(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	linksFile := filepath.Join(tmpDir, "subscriptions.txt")
	if err := os.WriteFile(linksFile, []byte("keep-token|测试订阅|surge|https://old.example/sub\n"), 0644); err != nil {
		t.Fatalf("write links file: %v", err)
	}

	body, err := json.Marshal(map[string]string{
		"target":        "clash",
		"url":           "https://new.example/sub",
		"existingShort": "/s/keep-token",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handleShortenAPI(config{LinksFile: linksFile}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["shortUrl"]; got != "/s/keep-token" {
		t.Fatalf("unexpected shortUrl: %v", got)
	}
	if got := resp["updated"]; got != true {
		t.Fatalf("expected updated=true, got %v", got)
	}

	gotData, err := os.ReadFile(linksFile)
	if err != nil {
		t.Fatalf("read links file: %v", err)
	}
	wantLine := "keep-token|测试订阅|clash|https://new.example/sub\n"
	if string(gotData) != wantLine {
		t.Fatalf("unexpected links file:\nwant: %q\ngot:  %q", wantLine, string(gotData))
	}
}

func TestHandleShortenAPIMigratesLegacyNumericLink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	linksFile := filepath.Join(tmpDir, "subscriptions.txt")
	if err := os.WriteFile(linksFile, []byte("旧订阅|surge|https://legacy.example/sub\n"), 0644); err != nil {
		t.Fatalf("write links file: %v", err)
	}

	body, err := json.Marshal(map[string]string{
		"target": "surge",
		"url":    "https://legacy.example/sub",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handleShortenAPI(config{LinksFile: linksFile}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["shortUrl"]; got != "/s/1" {
		t.Fatalf("unexpected shortUrl: %v", got)
	}

	gotData, err := os.ReadFile(linksFile)
	if err != nil {
		t.Fatalf("read links file: %v", err)
	}
	if !strings.Contains(string(gotData), "1|旧订阅|surge|https://legacy.example/sub") {
		t.Fatalf("legacy entry was not migrated with preserved numeric token: %q", string(gotData))
	}
}
