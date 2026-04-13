package main

import (
	"bytes"
	"encoding/base64"
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

func TestRenderShadowrocketVLessUsesUUIDAndRealityParams(t *testing.T) {
	t.Parallel()

	node := proxyNode{
		Name:      "demo-vless",
		SurgeType: "vless",
		Host:      "edge.example.com",
		Port:      443,
		Options: []string{
			"username=123e4567-e89b-12d3-a456-426614174000",
			"tls=true",
			"tls-security=reality",
			"sni=cdn.example.com",
			"grpc-service-name=subcon",
			"reality-public-key=test-public-key",
			"reality-short-id=abcd1234",
			"client-fingerprint=chrome",
		},
	}

	raw, err := base64.StdEncoding.DecodeString(renderShadowrocket([]proxyNode{node}, requestOptions{}))
	if err != nil {
		t.Fatalf("decode shadowrocket payload: %v", err)
	}
	got := string(raw)

	for _, want := range []string{
		"vless://123e4567-e89b-12d3-a456-426614174000@edge.example.com:443?",
		"type=grpc",
		"serviceName=subcon",
		"security=reality",
		"sni=cdn.example.com",
		"pbk=test-public-key",
		"sid=abcd1234",
		"fp=chrome",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("shadowrocket vless output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderShadowrocketVMessUsesUUID(t *testing.T) {
	t.Parallel()

	node := proxyNode{
		Name:      "demo-vmess",
		SurgeType: "vmess",
		Host:      "vmess.example.com",
		Port:      443,
		Options: []string{
			"username=123e4567-e89b-12d3-a456-426614174999",
			"alterId=0",
			"tls=true",
			"sni=vmess.example.com",
		},
	}

	raw, err := base64.StdEncoding.DecodeString(renderShadowrocket([]proxyNode{node}, requestOptions{}))
	if err != nil {
		t.Fatalf("decode shadowrocket payload: %v", err)
	}

	link := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(link, "vmess://") {
		t.Fatalf("unexpected shadowrocket vmess link: %s", link)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(link, "vmess://"))
	if err != nil {
		t.Fatalf("decode vmess config: %v", err)
	}
	if !strings.Contains(string(decoded), `"id":"123e4567-e89b-12d3-a456-426614174999"`) {
		t.Fatalf("shadowrocket vmess config missing UUID:\n%s", string(decoded))
	}
}

func TestRenderClashProxyVLessIncludesRealityOptions(t *testing.T) {
	t.Parallel()

	node := proxyNode{
		Name:      "demo-clash-vless",
		SurgeType: "vless",
		Host:      "edge.example.com",
		Port:      443,
		Options: []string{
			"username=123e4567-e89b-12d3-a456-426614174000",
			"tls=true",
			"sni=www.apple.com",
			"skip-cert-verify=true",
			"client-fingerprint=chrome",
			"reality-public-key=test-public-key",
			"reality-short-id=abcd1234",
			"flow=xtls-rprx-vision",
		},
	}

	lines := renderClashProxy(node)
	if len(lines) != 1 {
		t.Fatalf("unexpected clash proxy line count: %d", len(lines))
	}
	got := lines[0]

	for _, want := range []string{
		"type: vless",
		"uuid: 123e4567-e89b-12d3-a456-426614174000",
		"flow: xtls-rprx-vision",
		"tls: true",
		"servername: www.apple.com",
		"client-fingerprint: 'chrome'",
		"reality-opts: { public-key: 'test-public-key', short-id: 'abcd1234' }",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("clash vless line missing %q:\n%s", want, got)
		}
	}
}
