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
	opts := map[string]string{"username": "1234\r\n"}
	lines := []string{
		fmt.Sprintf("    uuid: %s", yamlString(opts["username"])),
		"    alterId: 0",
		"    cipher: auto",
	}
	fmt.Println(strings.Join(lines, "\n"))
}
