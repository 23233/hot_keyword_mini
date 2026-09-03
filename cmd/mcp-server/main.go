// Package main main.go
package main

import (
	"bufio"
	"fmt"
	"hot_keyword/services"
	"os"
)

func main() {
	mcpService := services.NewMCPService()
	scanner := bufio.NewScanner(os.Stdin)

	// 从标准输入读取 AI 客户端发送的 JSON-RPC 消息
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		respBytes, err := mcpService.HandleJSONRPC(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "MCP 处理错误: %v\n", err)
			continue
		}

		// 标准输出输出响应
		fmt.Printf("%s\n", respBytes)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "标准输入读取异常: %v\n", err)
	}
}
