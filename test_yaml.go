package main

import (
"fmt"
"strings"
)

func yamlString(value string) string {
	value = strings.ReplaceAll(value, `'`, `''`)
	return "'" + value + "'"
}

func main() {
	opts := map[string]string{"username": "1234"}
	lines := []string{
		fmt.Sprintf("  - name: %s", yamlString("test")),
		fmt.Sprintf("    type: %s", yamlString("vmess")),
		fmt.Sprintf("    server: %s", yamlString("1.1.1.1")),
		fmt.Sprintf("    port: %d", 443),
	}
	lines = append(lines,
fmt.Sprintf("    uuid: %s", yamlString(opts["username"])),
"    alterId: 0",
"    cipher: auto",
)
	fmt.Println(strings.Join(lines, "\n"))
}
