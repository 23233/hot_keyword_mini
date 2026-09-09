// Package main main_test.go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestStdioProtocolOutput 验证真实子进程 stdout 只包含逐行 JSON-RPC 响应。
func TestStdioProtocolOutput(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=TestStdioProtocolHelper")
	command.Env = append(os.Environ(), "MCP_STDIO_TEST_HELPER=1", "MCP_STDIO_MODE=1")
	command.Stdin = strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"sdui://rules"}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"sdui.template.list","arguments":{"business_type":"game","padding":"` + strings.Repeat("x", 70000) + `"}}}`,
	}, "\n") + "\n")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("Stdio MCP 子进程失败: %v, stderr=%s", err, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("stdout 应严格包含 4 行 JSON-RPC 响应，实际 %d 行: %s", len(lines), stdout.String())
	}
	for index, line := range lines {
		var response struct {
			JSONRPC string      `json:"jsonrpc"`
			ID      interface{} `json:"id"`
		}
		if err := json.Unmarshal([]byte(line), &response); err != nil || response.JSONRPC != "2.0" {
			t.Fatalf("stdout 第 %d 行不是合法 JSON-RPC: %s", index+1, line)
		}
	}
	if !strings.Contains(stderr.String(), "【MCP审计】") {
		t.Fatalf("Stdio 工具调用审计日志应写入 stderr")
	}
}

// TestStdioProtocolHelper 为子进程执行与生产入口相同的 Stdio 流程。
func TestStdioProtocolHelper(t *testing.T) {
	if os.Getenv("MCP_STDIO_TEST_HELPER") != "1" {
		return
	}
	disableConsoleLogs()
	if err := runStdio(os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
