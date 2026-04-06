package main

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultListen         = "0.0.0.0"
	defaultPort           = "8090"
	defaultTestURL        = "http://www.gstatic.com/generate_204"
	defaultUserAgent      = "surge-sub-converter/0.4.9"
	defaultFetchTimeout   = 15 * time.Second
	defaultCacheTTL       = 60 * time.Second
	defaultProxyGroupName = "Proxy"
	defaultTarget         = "surge"
	version               = "v0.5.1"
)

type config struct {
	Listen       string
	Port         string
	TestURL      string
	UserAgent    string
	FetchTimeout time.Duration
	CacheTTL     time.Duration
	HTTPClient   *http.Client
	CertFile     string
	KeyFile      string
	LinksFile    string
}

type proxyNode struct {
	Name      string
	SurgeType string
	Host      string
	Port      int
	Options   []string
}

type pageData struct {
	DefaultTarget string
	Version       string
}

type vmessPayload struct {
	Add  string `json:"add"`
	Port any    `json:"port"`
	ID   string `json:"id"`
	PS   string `json:"ps"`
	Net  string `json:"net"`
	Path string `json:"path"`
	Host string `json:"host"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni"`
	ALPN string `json:"alpn"`
}

type cacheEntry struct {
	Result    string
	Ignored   []string
	Count     int
	ExpiresAt time.Time
}

type converterCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

var resultCache = converterCache{items: map[string]cacheEntry{}}

func main() {
	cfg := loadConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex(cfg))
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/convert", handleConvert(cfg, false))
	mux.HandleFunc("/api/convert", handleConvert(cfg, true))
	mux.HandleFunc("/api/shorten", handleShortenAPI(cfg))
	mux.HandleFunc("/s/", handleShortLink(cfg))

	addr := cfg.Listen + ":" + cfg.Port
	server := &http.Server{
		Addr:              addr,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("surge-sub-converter listening on %s", addr)
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		log.Fatal(server.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile))
	}
	log.Fatal(server.ListenAndServe())
}

func loadConfig() config {
	cfg := config{
		Listen:       getenv("SSC_LISTEN", defaultListen),
		Port:         getenv("SSC_PORT", defaultPort),
		TestURL:      getenv("SSC_TEST_URL", defaultTestURL),
		UserAgent:    getenv("SSC_USER_AGENT", defaultUserAgent),
		FetchTimeout: defaultFetchTimeout,
		CacheTTL:     defaultCacheTTL,
		CertFile:     getenv("SSC_CERT_FILE", ""),
		KeyFile:      getenv("SSC_KEY_FILE", ""),
		LinksFile:    getenv("SSC_LINKS_FILE", "/opt/surge-sub-converter/data/subscriptions.txt"),
	}
	if raw := os.Getenv("SSC_FETCH_TIMEOUT"); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			cfg.FetchTimeout = time.Duration(seconds) * time.Second
		}
	}
	if raw := os.Getenv("SSC_CACHE_TTL"); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds >= 0 {
			cfg.CacheTTL = time.Duration(seconds) * time.Second
		}
	}
	cfg.HTTPClient = &http.Client{Timeout: cfg.FetchTimeout}
	return cfg
}

func handleIndex(cfg config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if err := indexTemplate.Execute(w, pageData{
			DefaultTarget: defaultTarget,
			Version:       version,
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func handleConvert(cfg config, apiMode bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		subURL := strings.TrimSpace(r.URL.Query().Get("url"))
		if subURL == "" {
			writeError(w, apiMode, http.StatusBadRequest, "missing url query parameter")
			return
		}

		opts := parseRequestOptions(r, cfg)
		result, ignored, count, err := convertSubscription(r.Context(), cfg, subURL, opts)
		if err != nil {
			writeError(w, apiMode, http.StatusBadRequest, err.Error())
			return
		}

		if apiMode {
			writeJSON(w, http.StatusOK, map[string]any{
				"success":        true,
				"nodeCount":      count,
				"ignoredTypes":   ignored,
				"config":         result,
				"subscription":   subURL,
				"target":         opts.Target,
				"policy":         opts.PolicyName,
				"skipCertVerify": opts.SkipCertVerify,
			})
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s.txt"`, opts.Target))
		if cfg.CacheTTL > 0 {
			w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(cfg.CacheTTL.Seconds())))
		}
		w.Header().Set("X-Node-Count", strconv.Itoa(count))
		if len(ignored) > 0 {
			w.Header().Set("X-Ignored-Types", strings.Join(ignored, ","))
		}
		_, _ = w.Write([]byte(result))
	}
}

type requestOptions struct {
	Target         string
	PolicyName     string
	TestURL        string
	AllowUDP       bool
	SkipCertVerify bool
	IncludeDirect  bool
	ProxyListOnly  bool // list=true 时仅输出纯代理行，不含章节头，供 surge 外部策略组使用
}

func parseRequestOptions(r *http.Request, cfg config) requestOptions {
	target := strings.ToLower(firstOrDefault(r.URL.Query().Get("target"), defaultTarget))
	policy := sanitizeName(firstOrDefault(r.URL.Query().Get("policy"), defaultProxyGroupName))
	if policy == "" {
		policy = defaultProxyGroupName
	}
	return requestOptions{
		Target:         target,
		PolicyName:     policy,
		TestURL:        firstOrDefault(r.URL.Query().Get("test_url"), cfg.TestURL),
		AllowUDP:       parseBoolDefault(r.URL.Query().Get("udp"), true),
		SkipCertVerify: parseBoolDefault(r.URL.Query().Get("skip_cert_verify"), true),
		IncludeDirect:  parseBoolDefault(r.URL.Query().Get("direct"), true),
		ProxyListOnly:  parseBoolDefault(r.URL.Query().Get("list"), false),
	}
}

func convertSubscription(ctx context.Context, cfg config, subURL string, opts requestOptions) (string, []string, int, error) {
	cacheKey := buildCacheKey(subURL, opts)
	if entry, ok := resultCache.Get(cacheKey); ok {
		return entry.Result, entry.Ignored, entry.Count, nil
	}

	linksText, err := fetchSubscription(cfg, subURL)
	if err != nil {
		return "", nil, 0, err
	}
	_ = ctx
	links := splitLinks(linksText)
	if len(links) == 0 {
		return "", nil, 0, errors.New("no nodes found in subscription")
	}

	nodes := make([]proxyNode, 0, len(links))
	ignoredSet := map[string]struct{}{}
	for _, link := range links {
		node, err := parseProxy(link, opts)
		if err != nil {
			scheme := "unknown"
			if parts := strings.SplitN(link, "://", 2); len(parts) > 1 && parts[0] != "" {
				scheme = parts[0]
			}
			ignoredSet[scheme] = struct{}{}
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return "", nil, 0, errors.New("no supported nodes found in subscription")
	}

	nodes = withUniqueNames(nodes)
	configText, err := renderByTarget(nodes, opts)
	if err != nil {
		return "", nil, 0, err
	}
	ignored := make([]string, 0, len(ignoredSet))
	for item := range ignoredSet {
		ignored = append(ignored, item)
	}
	sort.Strings(ignored)
	if cfg.CacheTTL > 0 {
		resultCache.Set(cacheKey, cacheEntry{
			Result:    configText,
			Ignored:   append([]string(nil), ignored...),
			Count:     len(nodes),
			ExpiresAt: time.Now().Add(cfg.CacheTTL),
		})
	}
	return configText, ignored, len(nodes), nil
}

func fetchSubscription(cfg config, subURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, subURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid subscription url: %w", err)
	}
	req.Header.Set("User-Agent", cfg.UserAgent)

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read subscription: %w", err)
	}
	return decodeSubscriptionBody(body), nil
}

func decodeSubscriptionBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ""
	}

	lines := splitLines(text)
	for _, line := range lines {
		if strings.Contains(line, "://") {
			return strings.Join(lines, "\n")
		}
	}

	compacted := strings.Join(strings.Fields(text), "")
	if decoded, err := decodeBase64(compacted); err == nil {
		decodedText := strings.TrimSpace(string(decoded))
		if strings.Contains(decodedText, "://") {
			return decodedText
		}
	}
	return text
}

func buildCacheKey(subURL string, opts requestOptions) string {
	raw := strings.Join([]string{
		subURL,
		opts.Target,
		opts.PolicyName,
		opts.TestURL,
		strconv.FormatBool(opts.AllowUDP),
		strconv.FormatBool(opts.SkipCertVerify),
		strconv.FormatBool(opts.IncludeDirect),
	}, "|")
	sum := sha1.Sum([]byte(raw))
	return fmt.Sprintf("%x", sum[:])
}

func splitLines(text string) []string {
	parts := strings.Split(text, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(part)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitLinks(text string) []string {
	lines := splitLines(text)
	links := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "://") {
			links = append(links, line)
		}
	}
	return links
}

func parseProxy(link string, opts requestOptions) (proxyNode, error) {
	switch {
	case strings.HasPrefix(link, "vmess://"):
		return parseVMess(link, opts)
	case strings.HasPrefix(link, "vless://"):
		return parseVLess(link, opts)
	case strings.HasPrefix(link, "trojan://"):
		return parseTrojan(link, opts)
	case strings.HasPrefix(link, "ss://"):
		return parseShadowsocks(link, opts)
	default:
		return proxyNode{}, errors.New("unsupported link type")
	}
}

func parseVMess(link string, opts requestOptions) (proxyNode, error) {
	raw := strings.TrimPrefix(link, "vmess://")
	decoded, err := decodeBase64(raw)
	if err != nil {
		return proxyNode{}, fmt.Errorf("invalid vmess payload: %w", err)
	}

	var payload vmessPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return proxyNode{}, fmt.Errorf("invalid vmess json: %w", err)
	}

	port, err := parseFlexiblePort(payload.Port)
	if err != nil {
		return proxyNode{}, fmt.Errorf("invalid vmess port: %w", err)
	}

	params := map[string]string{
		"security": payload.TLS,
		"sni":      payload.SNI,
		"alpn":     payload.ALPN,
		"type":     payload.Net,
		"path":     payload.Path,
		"host":     payload.Host,
	}

	options := []string{
		"username=" + payload.ID,
		"vmess-aead=true",
	}
	options = append(options, buildCommonOptions(params, opts)...)

	name := payload.PS
	if name == "" {
		name = payload.Add
	}

	return proxyNode{
		Name:      sanitizeName(name),
		SurgeType: "vmess",
		Host:      payload.Add,
		Port:      port,
		Options:   options,
	}, nil
}

func parseVLess(link string, opts requestOptions) (proxyNode, error) {
	parsed, err := url.Parse(link)
	if err != nil {
		return proxyNode{}, err
	}

	port, err := normalizePort(parsed.Port(), 443)
	if err != nil {
		return proxyNode{}, err
	}

	params := map[string]string{}
	for key, values := range parsed.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}

	username := ""
	if parsed.User != nil {
		username = parsed.User.Username()
	}

	options := []string{"username=" + username}
	options = append(options, buildCommonOptions(params, opts)...)
	if flow := params["flow"]; flow != "" {
		options = append(options, "flow="+flow)
	}

	name := fragmentOrHost(parsed)
	return proxyNode{
		Name:      sanitizeName(name),
		SurgeType: "vless",
		Host:      parsed.Hostname(),
		Port:      port,
		Options:   options,
	}, nil
}

func parseTrojan(link string, opts requestOptions) (proxyNode, error) {
	parsed, err := url.Parse(link)
	if err != nil {
		return proxyNode{}, err
	}

	port, err := normalizePort(parsed.Port(), 443)
	if err != nil {
		return proxyNode{}, err
	}

	params := map[string]string{}
	for key, values := range parsed.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}

	password := ""
	if parsed.User != nil {
		password = parsed.User.Username()
	}

	options := []string{"password=" + password}
	options = append(options, buildCommonOptions(params, opts)...)

	name := fragmentOrHost(parsed)
	return proxyNode{
		Name:      sanitizeName(name),
		SurgeType: "trojan",
		Host:      parsed.Hostname(),
		Port:      port,
		Options:   options,
	}, nil
}

func parseShadowsocks(link string, opts requestOptions) (proxyNode, error) {
	raw := strings.TrimPrefix(link, "ss://")
	name := "ss-node"
	if hashIndex := strings.Index(raw, "#"); hashIndex >= 0 {
		name = raw[hashIndex+1:]
		raw = raw[:hashIndex]
		if decoded, err := url.QueryUnescape(name); err == nil {
			name = decoded
		}
	}

	plugin := ""
	if queryIndex := strings.Index(raw, "?"); queryIndex >= 0 {
		query, err := url.ParseQuery(raw[queryIndex+1:])
		if err == nil {
			plugin = query.Get("plugin")
		}
		raw = raw[:queryIndex]
	}

	var credentials string
	var server string
	if atIndex := strings.LastIndex(raw, "@"); atIndex >= 0 {
		credentials = raw[:atIndex]
		server = raw[atIndex+1:]
		if !strings.Contains(credentials, ":") {
			decoded, err := decodeBase64(credentials)
			if err != nil {
				return proxyNode{}, fmt.Errorf("invalid ss auth payload: %w", err)
			}
			credentials = string(decoded)
		}
	} else {
		decoded, err := decodeBase64(raw)
		if err != nil {
			return proxyNode{}, fmt.Errorf("invalid ss payload: %w", err)
		}
		parts := strings.SplitN(string(decoded), "@", 2)
		if len(parts) != 2 {
			return proxyNode{}, errors.New("invalid ss decoded payload")
		}
		credentials = parts[0]
		server = parts[1]
	}

	credParts := strings.SplitN(credentials, ":", 2)
	if len(credParts) != 2 {
		return proxyNode{}, errors.New("invalid ss credentials")
	}

	host, portText, ok := strings.Cut(server, ":")
	if !ok {
		return proxyNode{}, errors.New("invalid ss server")
	}
	port, err := normalizePort(portText, 8388)
	if err != nil {
		return proxyNode{}, err
	}

	options := []string{
		"encrypt-method=" + credParts[0],
		"password=" + credParts[1],
	}
	if opts.AllowUDP {
		options = append(options, "udp-relay=true")
	}
	if plugin != "" {
		options = append(options, "plugin="+plugin)
	}

	return proxyNode{
		Name:      sanitizeName(name),
		SurgeType: "ss",
		Host:      host,
		Port:      port,
		Options:   options,
	}, nil
}

func buildCommonOptions(params map[string]string, opts requestOptions) []string {
	options := make([]string, 0, 8)

	security := strings.ToLower(params["security"])
	sni := firstNonEmpty(params["sni"], params["peer"], params["servername"], params["serverName"])
	transport := strings.ToLower(params["type"])

	if security == "tls" || security == "reality" {
		options = append(options, "tls=true")
		if sni != "" {
			options = append(options, "sni="+sni)
		}
		if opts.SkipCertVerify {
			options = append(options, "skip-cert-verify=true")
		}
	}
	// 移除 ALPN：因为 3x-ui 经常返回 "h2,http/1.1" 含有逗号，会导致 Surge 的 proxyline 解析混乱崩溃

	switch transport {
	case "ws":
		options = append(options, "ws=true")
		path := firstNonEmpty(params["path"], "/")
		options = append(options, "ws-path="+path)
        // 移除了 ws-headers=Host 避免在部分 Surge 版本引发语法错误，SNI 已经足够
	case "grpc":
		if serviceName := firstNonEmpty(params["serviceName"], params["service_name"]); serviceName != "" {
			options = append(options, "grpc-service-name="+serviceName)
		}
	}

	if opts.AllowUDP {
		options = append(options, "udp-relay=true")
	}
	return options
}

func renderByTarget(nodes []proxyNode, opts requestOptions) (string, error) {
	switch opts.Target {
	case "surge":
		return renderSurge(nodes, opts), nil
	case "clash", "stash":
		return renderClashLike(nodes, opts), nil
	case "quantumultx", "quantumult-x", "quanx", "quantumult":
		return renderQuantumultX(nodes, opts), nil
	default:
		return "", fmt.Errorf("unsupported target: %s", opts.Target)
	}
}

func withUniqueNames(nodes []proxyNode) []proxyNode {
	usedNames := map[string]int{}
	for i := range nodes {
		nodes[i].Name = makeUniqueName(nodes[i].Name, usedNames)
	}
	return nodes
}

func renderSurge(nodes []proxyNode, opts requestOptions) string {
	if opts.ProxyListOnly {
		lines := make([]string, 0, len(nodes))
		for _, node := range nodes {
			lines = append(lines, renderNode(node))
		}
		// 根据用户的 Gemini 指导方案，直接返回纯文本行，不再做 Base64 编码
		return strings.Join(lines, "\n")
	}

	groupMembers := make([]string, 0, len(nodes)+1)
	lines := []string{
		"[General]",
		"loglevel = notify",
		"skip-proxy = 127.0.0.1, localhost, *.local",
		"",
		"[Proxy]",
	}

	for _, node := range nodes {
		groupMembers = append(groupMembers, node.Name)
		lines = append(lines, renderNode(node))
	}
	if opts.IncludeDirect {
		groupMembers = append(groupMembers, "DIRECT")
	}

	lines = append(lines,
		"",
		"[Proxy Group]",
		fmt.Sprintf("%s = select, %s", opts.PolicyName, strings.Join(groupMembers, ", ")),
		fmt.Sprintf("Auto = url-test, %s, url=%s, interval=600, tolerance=150", strings.Join(groupMembers, ", "), opts.TestURL),
		"",
		"[Rule]",
		fmt.Sprintf("FINAL,%s", opts.PolicyName),
		"",
	)

	return strings.Join(lines, "\n")
}

func renderClashLike(nodes []proxyNode, opts requestOptions) string {
	lines := []string{
		"mixed-port: 7890",
		"allow-lan: true",
		"mode: rule",
		"log-level: info",
		"",
		"proxies:",
	}
	for _, node := range nodes {
		lines = append(lines, renderClashProxy(node)...)
	}
	groupMembers := make([]string, 0, len(nodes)+1)
	for _, node := range nodes {
		groupMembers = append(groupMembers, yamlString(node.Name))
	}
	if opts.IncludeDirect {
		groupMembers = append(groupMembers, yamlString("DIRECT"))
	}
	lines = append(lines,
		"",
		"proxy-groups:",
		fmt.Sprintf("  - { name: %s, type: select, proxies: [%s] }", yamlString(opts.PolicyName), strings.Join(groupMembers, ", ")),
		fmt.Sprintf("  - { name: %s, type: url-test, url: %s, interval: 600, proxies: [%s] }", yamlString("Auto"), yamlString(opts.TestURL), strings.Join(groupMembers, ", ")),
		"",
		"rules:",
		fmt.Sprintf("  - MATCH,%s", opts.PolicyName),
	)
	return strings.Join(lines, "\n")
}

func renderQuantumultX(nodes []proxyNode, opts requestOptions) string {
	lines := []string{"[server_local]"}
	for _, node := range nodes {
		if line := renderQuantumultXNode(node); line != "" {
			lines = append(lines, line)
		}
	}
	groupMembers := make([]string, 0, len(nodes)+1)
	for _, node := range nodes {
		groupMembers = append(groupMembers, node.Name)
	}
	if opts.IncludeDirect {
		groupMembers = append(groupMembers, "direct")
	}
	lines = append(lines,
		"",
		"[policy]",
		fmt.Sprintf("%s = select, %s", opts.PolicyName, strings.Join(groupMembers, ", ")),
		fmt.Sprintf("Auto = available, %s", strings.Join(groupMembers, ", ")),
		"",
		"[filter_local]",
		fmt.Sprintf("final, %s", opts.PolicyName),
	)
	return strings.Join(lines, "\n")
}

func renderNode(node proxyNode) string {
	values := []string{node.SurgeType, node.Host, strconv.Itoa(node.Port)}
	values = append(values, node.Options...)
	return fmt.Sprintf("%s = %s", node.Name, strings.Join(values, ", "))
}

func renderClashProxy(node proxyNode) []string {
	opts := parseOptionPairs(node.Options)
	lines := []string{
		fmt.Sprintf("  - name: %s", yamlString(node.Name)),
		fmt.Sprintf("    type: %s", yamlString(node.SurgeType)),
		fmt.Sprintf("    server: %s", yamlString(node.Host)),
		fmt.Sprintf("    port: %d", node.Port),
	}
	switch node.SurgeType {
	case "vmess", "vless":
		lines = append(lines, fmt.Sprintf("    uuid: %s", yamlString(opts["username"])))
	case "trojan":
		lines = append(lines, fmt.Sprintf("    password: %s", yamlString(opts["password"])))
	case "ss":
		lines = append(lines,
			fmt.Sprintf("    cipher: %s", yamlString(opts["encrypt-method"])),
			fmt.Sprintf("    password: %s", yamlString(opts["password"])),
		)
	}
	if isTrue(opts["tls"]) {
		lines = append(lines, "    tls: true")
	}
	if sni := opts["sni"]; sni != "" {
		lines = append(lines, fmt.Sprintf("    servername: %s", yamlString(sni)))
	}
	if isTrue(opts["skip-cert-verify"]) {
		lines = append(lines, "    skip-cert-verify: true")
	}
	if isTrue(opts["udp-relay"]) {
		lines = append(lines, "    udp: true")
	}
	if isTrue(opts["ws"]) {
		lines = append(lines, "    network: ws", "    ws-opts:")
		lines = append(lines, fmt.Sprintf("      path: %s", yamlString(firstOrDefault(opts["ws-path"], "/"))))
		if header := opts["ws-headers"]; header != "" {
			parts := strings.SplitN(header, ":", 2)
			if len(parts) == 2 {
				lines = append(lines, "      headers:", fmt.Sprintf("        %s: %s", parts[0], yamlString(parts[1])))
			}
		}
	}
	return lines
}

func renderQuantumultXNode(node proxyNode) string {
	opts := parseOptionPairs(node.Options)
	tag := quantumultTag(node.SurgeType)
	if tag == "" {
		return ""
	}
	parts := []string{tag, node.Host, strconv.Itoa(node.Port)}
	switch node.SurgeType {
	case "vmess", "vless":
		parts = append(parts, "username="+opts["username"])
	case "trojan":
		parts = append(parts, "password="+opts["password"])
	case "ss":
		parts = append(parts, "method="+opts["encrypt-method"], "password="+opts["password"])
	}
	if isTrue(opts["tls"]) {
		parts = append(parts, "over-tls=true")
	}
	if sni := opts["sni"]; sni != "" {
		parts = append(parts, "tls-host="+sni)
	}
	if isTrue(opts["ws"]) {
		parts = append(parts, "obfs=wss", "obfs-uri="+firstOrDefault(opts["ws-path"], "/"))
		if header := opts["ws-headers"]; header != "" {
			if host := strings.TrimPrefix(header, "Host:"); host != "" {
				parts = append(parts, "obfs-host="+host)
			}
		}
	}
	parts = append(parts, "tag="+node.Name)
	return strings.Join(parts, ", ")
}

func quantumultTag(kind string) string {
	switch kind {
	case "vmess":
		return "vmess"
	case "vless":
		return "vless"
	case "trojan":
		return "trojan"
	case "ss":
		return "shadowsocks"
	default:
		return ""
	}
}

func parseOptionPairs(items []string) map[string]string {
	out := map[string]string{}
	for _, item := range items {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			out[key] = value
		}
	}
	return out
}

func yamlString(value string) string {
	value = strings.ReplaceAll(value, `'`, `''`)
	return "'" + value + "'"
}

func isTrue(value string) bool {
	return strings.EqualFold(value, "true")
}

func makeUniqueName(name string, used map[string]int) string {
	name = sanitizeName(name)
	if name == "" {
		name = "node"
	}
	used[name]++
	if used[name] == 1 {
		return name
	}
	return fmt.Sprintf("%s %d", name, used[name])
}

func fragmentOrHost(parsed *url.URL) string {
	name := parsed.Fragment
	if name == "" {
		name = parsed.Hostname()
	}
	if decoded, err := url.QueryUnescape(name); err == nil {
		return decoded
	}
	return name
}

func decodeBase64(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	raw += strings.Repeat("=", (4-len(raw)%4)%4)
	decoded, err := base64.URLEncoding.DecodeString(raw)
	if err == nil {
		return decoded, nil
	}
	return base64.StdEncoding.DecodeString(raw)
}

func normalizePort(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 || port > 65535 {
		return 0, errors.New("invalid port")
	}
	return port, nil
}

func parseFlexiblePort(value any) (int, error) {
	switch v := value.(type) {
	case string:
		return normalizePort(v, 0)
	case float64:
		port := int(v)
		if float64(port) != v || port <= 0 || port > 65535 {
			return 0, errors.New("invalid numeric port")
		}
		return port, nil
	case int:
		if v <= 0 || v > 65535 {
			return 0, errors.New("invalid numeric port")
		}
		return v, nil
	default:
		return 0, errors.New("unsupported port type")
	}
}

func sanitizeName(name string) string {
	replacer := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ", ",", " ", "=", " ")
	return strings.TrimSpace(replacer.Replace(name))
}

func parseBoolDefault(raw string, fallback bool) bool {
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func writeError(w http.ResponseWriter, apiMode bool, status int, message string) {
	if apiMode {
		writeJSON(w, status, map[string]any{
			"success": false,
			"error":   message,
		})
		return
	}
	http.Error(w, message, status)
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func firstOrDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (c *converterCache) Get(key string) (cacheEntry, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return cacheEntry{}, false
	}
	if time.Now().After(entry.ExpiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return cacheEntry{}, false
	}
	return entry, true
}

func (c *converterCache) Set(key string, entry cacheEntry) {
	c.mu.Lock()
	c.items[key] = entry
	c.mu.Unlock()
}

func handleShortLink(cfg config) http.HandlerFunc {
	converterFunc := handleConvert(cfg, false)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		idStr := strings.TrimPrefix(r.URL.Path, "/s/")
		if idStr == "" {
			http.NotFound(w, r)
			return
		}

		data, err := os.ReadFile(cfg.LinksFile)
		if err != nil {
			log.Printf("failed to read links file %s: %v", cfg.LinksFile, err)
			http.NotFound(w, r)
			return
		}

		lines := strings.Split(string(data), "\n")
		var target, urlStr string
		found := false

		// 改进的查找逻辑：支持 Token 匹配，并向后兼容数字 ID 索引
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			
			// 格式 A: Token|Title|Target|URL (4列)
			if len(parts) >= 4 && parts[0] == idStr {
				target = strings.TrimSpace(parts[2])
				urlStr = strings.TrimSpace(parts[3])
				found = true
				break
			}
			
			// 兼容格式 B: Title|Target|URL (3列) -> 仍然支持旧的数字 ID 访问
			if len(parts) == 3 {
				numericID, err := strconv.Atoi(idStr)
				if err == nil && numericID == (i+1) {
					target = strings.TrimSpace(parts[1])
					urlStr = strings.TrimSpace(parts[2])
					found = true
					break
				}
			}
		}

		if !found {
			http.NotFound(w, r)
			return
		}

		// 检测是否为代理客户端（Surge/Clash/Stash 等）
		ua := strings.ToLower(r.Header.Get("User-Agent"))
		isProxyClient := strings.Contains(ua, "surge") ||
			strings.Contains(ua, "clash") ||
			strings.Contains(ua, "stash") ||
			strings.Contains(ua, "quantumult") ||
			strings.Contains(ua, "shadowrocket") ||
			strings.Contains(ua, "loon") ||
			strings.Contains(ua, "sing-box")

		if isProxyClient {
			// 如果是代理客户端(Surge等)访问，自动追加 list=true 以触发纯代理行输出
			q := r.URL.Query()
			q.Set("list", "true")
			r.URL.RawQuery = q.Encode()
		}

		// 走正常转换流程
		q := r.URL.Query()
		q.Set("target", target)
		q.Set("url", urlStr)
		r.URL.RawQuery = q.Encode()
		converterFunc(w, r)
	}
}

var linksFileMu sync.Mutex

type shortenRequest struct {
	Target string `json:"target"`
	URL    string `json:"url"`
}

func handleShortenAPI(cfg config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Printf("[DEBUG] Shorten API called by %s", r.RemoteAddr)
		log.Printf("[DEBUG] Using LinksFile: %s", cfg.LinksFile)

		var req shortenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		target := strings.TrimSpace(req.Target)
		urlStr := strings.TrimSpace(req.URL)
		if target == "" || urlStr == "" {
			http.Error(w, "target and url are required", http.StatusBadRequest)
			return
		}

		linksFileMu.Lock()
		defer linksFileMu.Unlock()

		data, err := os.ReadFile(cfg.LinksFile)
		if err != nil && !os.IsNotExist(err) {
			log.Printf("failed to read links file %s: %v", cfg.LinksFile, err)
		}
		lines := strings.Split(string(data), "\n")
		
		token := ""
		needMigration := false
		var newLines []string

		// 检查是否已有该链接，并顺便执行旧数据格式检查（迁移）
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			
			if len(parts) == 3 {
				// 旧格式检测：自动分配 Token
				oldTitle, oldTarget, oldUrl := parts[0], parts[1], parts[2]
				newToken := generateRandomToken(32)
				newEntry := fmt.Sprintf("%s|%s|%s|%s", newToken, oldTitle, oldTarget, oldUrl)
				newLines = append(newLines, newEntry)
				needMigration = true
				
				if oldTarget == target && oldUrl == urlStr {
					token = newToken
				}
			} else if len(parts) >= 4 {
				// 新格式
				newLines = append(newLines, line)
				if parts[2] == target && parts[3] == urlStr {
					token = parts[0]
				}
			}
		}

		// 如果发现旧格式链接，立即保存迁移后的完整文件
		if needMigration {
			_ = os.WriteFile(cfg.LinksFile, []byte(strings.Join(newLines, "\n")+"\n"), 0644)
		}

		// 如果是全新的链接
		if token == "" {
			token = generateRandomToken(32)
			newLine := fmt.Sprintf("%s|网页生成|%s|%s\n", token, target, urlStr)
			f, err := os.OpenFile(cfg.LinksFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Printf("failed to open links file %s: %v", cfg.LinksFile, err)
				http.Error(w, "server error", http.StatusInternalServerError)
				return
			}
			defer f.Close()
			if _, err := f.WriteString(newLine); err != nil {
				log.Printf("failed to write links file: %v", err)
				http.Error(w, "server error", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		shortPath := fmt.Sprintf("/s/%s", token)
		log.Printf("[DEBUG] Secure short link generated: %s", shortPath)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"shortUrl": shortPath,
		})
	}
}

// generateRandomToken 生成指定长度的随机 Token
func generateRandomToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}
