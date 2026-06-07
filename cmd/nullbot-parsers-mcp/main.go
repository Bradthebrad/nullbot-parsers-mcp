package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Bradthebrad/nullbot-parsers-mcp/internal/parsers"
	"tinychain/mcp"
)

const version = "0.1.0"

func main() {
	transport := flag.String("transport", "stdio", "Transport: stdio, streamable-http, http, or sse.")
	addr := flag.String("addr", "127.0.0.1:8770", "HTTP/SSE listen address.")
	path := flag.String("path", "/mcp", "Streamable HTTP endpoint path.")
	ssePath := flag.String("sse-path", "/sse", "Legacy SSE endpoint path.")
	messagePath := flag.String("message-path", "/message", "Legacy SSE message endpoint path.")
	workspace := flag.String("workspace", ".", "Workspace root. Parser tools cannot escape this directory.")
	maxBytes := flag.Int64("max-bytes", 1024*1024, "Maximum bytes returned by extraction tools.")
	showVersion := flag.Bool("version", false, "Print version and exit.")
	flag.Parse()

	if *showVersion {
		fmt.Println("nullbot-parsers-mcp", version)
		return
	}

	parserTools, err := parsers.New(parsers.Config{Workspace: *workspace, MaxBytes: *maxBytes})
	if err != nil {
		fmt.Fprintln(os.Stderr, "nullbot-parsers-mcp:", err)
		os.Exit(2)
	}

	server := mcp.NewServer("nullbot-parsers-mcp")
	server.Version = version
	for _, tool := range parserTools.Tools() {
		server.AddTool(tool)
	}
	if *transport != "stdio" {
		fmt.Fprintf(os.Stderr, "nullbot-parsers-mcp serving %s on %s\n", *transport, *addr)
	}
	err = server.Run(
		context.Background(),
		mcp.WithTransport(*transport),
		mcp.WithAddr(*addr),
		mcp.WithPath(*path),
		mcp.WithSSEPaths(*ssePath, *messagePath),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nullbot-parsers-mcp:", err)
		os.Exit(1)
	}
}
