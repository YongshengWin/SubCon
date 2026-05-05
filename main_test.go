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

func TestParseVLessXHTTPRendersClashTransport(t *testing.T) {
	t.Parallel()

	link := "vless://123e4567-e89b-12d3-a456-426614174000@rfc.example.com:53985?type=xhttp&encryption=none&path=%2Fdsiouofehsf&host=&mode=auto&security=reality&pbk=test-public-key&fp=chrome&sni=www.apple.com&sid=1f#demo"
	node, err := parseVLess(link, requestOptions{AllowUDP: true})
	if err != nil {
		t.Fatalf("parse vless xhttp link: %v", err)
	}

	lines := renderClashProxy(node)
	if len(lines) != 1 {
		t.Fatalf("unexpected clash proxy line count: %d", len(lines))
	}
	got := lines[0]

	for _, want := range []string{
		"type: vless",
		"uuid: 123e4567-e89b-12d3-a456-426614174000",
		"udp: true",
		"tls: true",
		"servername: www.apple.com",
		"client-fingerprint: 'chrome'",
		"reality-opts: { public-key: 'test-public-key', short-id: '1f' }",
		"encryption: ''",
		"network: xhttp",
		"xhttp-opts: { path: '/dsiouofehsf', mode: 'auto' }",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("clash vless xhttp line missing %q:\n%s", want, got)
		}
	}
}

func TestParseHysteria2Basic(t *testing.T) {
	t.Parallel()

	link := "hysteria2://mypassword@hy2.example.com:8443/?sni=real.example.com#测试节点"
	node, err := parseHysteria2(link, requestOptions{AllowUDP: true, SkipCertVerify: false})
	if err != nil {
		t.Fatalf("parse hysteria2 link: %v", err)
	}

	if node.SurgeType != "hysteria2" {
		t.Fatalf("expected SurgeType=hysteria2, got %s", node.SurgeType)
	}
	if node.Host != "hy2.example.com" {
		t.Fatalf("expected host=hy2.example.com, got %s", node.Host)
	}
	if node.Port != 8443 {
		t.Fatalf("expected port=8443, got %d", node.Port)
	}
	if node.Name != "测试节点" {
		t.Fatalf("expected name=测试节点, got %s", node.Name)
	}

	opts := parseOptionPairs(node.Options)
	if opts["password"] != "mypassword" {
		t.Fatalf("expected password=mypassword, got %s", opts["password"])
	}
	if opts["sni"] != "real.example.com" {
		t.Fatalf("expected sni=real.example.com, got %s", opts["sni"])
	}
	if opts["download-bandwidth"] != "10000" {
		t.Fatalf("expected download-bandwidth=10000, got %s", opts["download-bandwidth"])
	}
	if opts["udp-relay"] != "true" {
		t.Fatalf("expected udp-relay=true, got %s", opts["udp-relay"])
	}
}

func TestParseHysteria2WithHy2Prefix(t *testing.T) {
	t.Parallel()

	link := "hy2://pwd123@node.example.com:443#HY2节点"
	node, err := parseHysteria2(link, requestOptions{AllowUDP: true, SkipCertVerify: true})
	if err != nil {
		t.Fatalf("parse hy2 link: %v", err)
	}

	if node.SurgeType != "hysteria2" {
		t.Fatalf("expected SurgeType=hysteria2, got %s", node.SurgeType)
	}
	if node.Host != "node.example.com" {
		t.Fatalf("expected host=node.example.com, got %s", node.Host)
	}

	opts := parseOptionPairs(node.Options)
	if opts["password"] != "pwd123" {
		t.Fatalf("expected password=pwd123, got %s", opts["password"])
	}
	if opts["sni"] != "node.example.com" {
		t.Fatalf("expected sni=node.example.com, got %s", opts["sni"])
	}
	if opts["skip-cert-verify"] != "true" {
		t.Fatalf("expected skip-cert-verify=true, got %s", opts["skip-cert-verify"])
	}
}

func TestParseHysteria2WithObfs(t *testing.T) {
	t.Parallel()

	link := "hysteria2://letmein@example.com:443/?obfs=salamander&obfs-password=gawrgura&insecure=1&pinSHA256=deadbeef&sni=real.example.com#混淆节点"
	node, err := parseHysteria2(link, requestOptions{AllowUDP: false, SkipCertVerify: false})
	if err != nil {
		t.Fatalf("parse hysteria2 obfs link: %v", err)
	}

	opts := parseOptionPairs(node.Options)
	if opts["obfs"] != "salamander" {
		t.Fatalf("expected obfs=salamander, got %s", opts["obfs"])
	}
	if opts["obfs-password"] != "gawrgura" {
		t.Fatalf("expected obfs-password=gawrgura, got %s", opts["obfs-password"])
	}
	if opts["server-cert-fingerprint-sha256"] != "deadbeef" {
		t.Fatalf("expected pinSHA256=deadbeef, got %s", opts["server-cert-fingerprint-sha256"])
	}
	if opts["skip-cert-verify"] != "true" {
		t.Fatalf("expected skip-cert-verify=true (from insecure=1), got %s", opts["skip-cert-verify"])
	}
	if _, hasUDP := opts["udp-relay"]; hasUDP {
		t.Fatalf("expected no udp-relay when AllowUDP=false")
	}
}

func TestRenderClashProxyHysteria2(t *testing.T) {
	t.Parallel()

	node := proxyNode{
		Name:      "hy2-clash-node",
		SurgeType: "hysteria2",
		Host:      "hy2.example.com",
		Port:      443,
		Options: []string{
			"password=testpwd",
			"download-bandwidth=10000",
			"sni=cdn.example.com",
			"skip-cert-verify=true",
			"obfs=salamander",
			"obfs-password=secret",
			"udp-relay=true",
		},
	}

	lines := renderClashProxy(node)
	if len(lines) != 1 {
		t.Fatalf("unexpected clash proxy line count: %d", len(lines))
	}
	got := lines[0]

	for _, want := range []string{
		"type: hysteria2",
		"password: testpwd",
		`down: "10000 Mbps"`,
		"obfs: salamander",
		"obfs-password: secret",
		"servername: cdn.example.com",
		"skip-cert-verify: true",
		"udp: true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("clash hysteria2 line missing %q:\n%s", want, got)
		}
	}
}

func TestRenderShadowrocketHysteria2(t *testing.T) {
	t.Parallel()

	node := proxyNode{
		Name:      "hy2-sr-node",
		SurgeType: "hysteria2",
		Host:      "sr.example.com",
		Port:      8443,
		Options: []string{
			"password=srpwd",
			"download-bandwidth=10000",
			"sni=sr.example.com",
			"skip-cert-verify=true",
			"obfs=salamander",
			"obfs-password=obfspwd",
			"server-cert-fingerprint-sha256=aabbccdd",
		},
	}

	raw, err := base64.StdEncoding.DecodeString(renderShadowrocket([]proxyNode{node}, requestOptions{}))
	if err != nil {
		t.Fatalf("decode shadowrocket payload: %v", err)
	}
	got := string(raw)

	for _, want := range []string{
		"hysteria2://srpwd@sr.example.com:8443?",
		"sni=sr.example.com",
		"insecure=1",
		"obfs=salamander",
		"obfs-password=obfspwd",
		"pinSHA256=aabbccdd",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("shadowrocket hysteria2 output missing %q:\n%s", want, got)
		}
	}
}
