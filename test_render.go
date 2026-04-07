package main

import (
"fmt"
"strings"
)

type proxyNode struct {
	Name      string
	SurgeType string
	Host      string
	Port      int
	Options   []string
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
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, `'`, `''`)
	return "'" + value + "'"
}

func firstOrDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func isTrue(value string) bool {
	return strings.EqualFold(value, "true")
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
	case "vmess":
		lines = append(lines,
fmt.Sprintf("    uuid: %s", yamlString(opts["username"])),
"    alterId: 0",
"    cipher: auto",
)
	case "vless":
		lines = append(lines, fmt.Sprintf("    uuid: %s", yamlString(opts["username"])))
		if flow := opts["flow"]; flow != "" {
			lines = append(lines, fmt.Sprintf("    flow: %s", yamlString(flow)))
		}
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
				lines = append(lines, "      headers:", fmt.Sprintf("        %s: %s", parts[0], yamlString(strings.TrimSpace(parts[1]))))
			}
		}
	} else if svcName := opts["grpc-service-name"]; svcName != "" {
		lines = append(lines, "    network: grpc", "    grpc-opts:")
		lines = append(lines, fmt.Sprintf("      grpc-service-name: %s", yamlString(svcName)))
	}
	return lines
}

func main() {
	node := proxyNode{
		Name:      "英国-优化-GPT",
		SurgeType: "vmess",
		Host:      "planb.mojcn.com",
		Port:      16645,
		Options: []string{
			"username=1a06eba3-2e82-4303-8e35-23a1f882eba4",
			"vmess-aead=true",
			"ws=true",
			"path=/",
			"tls=false", // the payload has tls=""
            "ws-headers=",
		},
	}
	
	for _, l := range renderClashProxy(node) {
		fmt.Println(l)
	}
}
