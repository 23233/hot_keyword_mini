// Package main main.go
package main

import (
	"bufio"
	"fmt"
	"hot_keyword/services"
	"io"
	"os"

	"github.com/23233/ggg/logger"
)

func main() {
	// Stdio 协议要求 stdout 只承载 JSON-RPC；项目日志统一保留到文件并改走 stderr。
	_ = os.Setenv("MCP_STDIO_MODE", "1")
	disableConsoleLogs()
	if err := runStdio(os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "标准输入读取异常: %v\n", err)
	}
}

// runStdio 按一行一个 JSON-RPC 消息处理标准输入输出。
func runStdio(input io.Reader, output, errorOutput io.Writer) error {
	_ = os.Setenv("MCP_STDIO_MODE", "1")
	mcpService := services.NewMCPService()
	scanner := bufio.NewScanner(input)
	// 复杂 SDUI 协议可能显著超过 Scanner 默认 64KB，允许单条消息最大 4MB。
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	// 从标准输入读取 AI 客户端发送的 JSON-RPC 消息
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		respBytes, err := mcpService.HandleJSONRPC(line)
		if err != nil {
			fmt.Fprintf(errorOutput, "MCP 处理错误: %v\n", err)
			continue
		}

		// 标准输出输出响应
		if len(respBytes) > 0 {
			if _, err := fmt.Fprintf(output, "%s\n", respBytes); err != nil {
				return fmt.Errorf("写入标准输出失败: %w", err)
			}
		}
	}
	return scanner.Err()
}

func disableConsoleLogs() {
	for _, current := range []*logger.Log{logger.J, logger.JH, logger.Js, logger.JM} {
		if current == nil || current.Op == nil {
			continue
		}
		current.Op.CloseConsoleDisplay()
	}
	if logger.J != nil && logger.J.Op != nil {
		logger.J = logger.J.Op.InitLogger()
	}
	if logger.JH != nil && logger.JH.Op != nil {
		logger.JH = logger.JH.Op.InitLogger()
	}
	if logger.Js != nil && logger.Js.Op != nil {
		logger.Js = logger.Js.Op.InitLogger()
	}
	if logger.JM != nil && logger.JM.Op != nil {
		logger.JM = logger.JM.Op.InitLogger()
	}
}
