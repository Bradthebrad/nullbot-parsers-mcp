package parsers

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tinychain/mcp"
)

func TestCSVAndHTMLParsing(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "table.csv"), "name,role\nAda,Dev\nLinus,Ops\n")
	mustWrite(t, filepath.Join(dir, "page.html"), "<html><body><h1>Hello</h1><p>World</p></body></html>")

	server := testServer(t, dir)
	csvText := resultText(callMCPTool(t, server, "extract_tables", map[string]any{"path": "table.csv"}))
	if !strings.Contains(csvText, "Ada") {
		t.Fatalf("csv result = %s", csvText)
	}
	htmlText := resultText(callMCPTool(t, server, "extract_text", map[string]any{"path": "page.html"}))
	if !strings.Contains(htmlText, "Hello") || !strings.Contains(htmlText, "World") {
		t.Fatalf("html result = %s", htmlText)
	}
}

func TestDocxAndParts(t *testing.T) {
	dir := t.TempDir()
	docx := filepath.Join(dir, "sample.docx")
	makeZip(t, docx, map[string]string{
		"word/document.xml": "<w:document><w:body><w:p><w:r><w:t>Hello Docx</w:t></w:r></w:p></w:body></w:document>",
		"docProps/core.xml": "<cp:coreProperties><dc:title>Test Title</dc:title></cp:coreProperties>",
	})
	server := testServer(t, dir)
	text := resultText(callMCPTool(t, server, "extract_text", map[string]any{"path": "sample.docx"}))
	if !strings.Contains(text, "Hello Docx") {
		t.Fatalf("docx text = %s", text)
	}
	parts := resultText(callMCPTool(t, server, "list_document_parts", map[string]any{"path": "sample.docx"}))
	if !strings.Contains(parts, "word/document.xml") {
		t.Fatalf("parts = %s", parts)
	}
}

func TestToolsList(t *testing.T) {
	server := testServer(t, t.TempDir())
	resp := callTool(t, server, "tools/list", nil)
	data, _ := json.Marshal(resp.Result)
	var tools mcp.ListToolsResult
	if err := json.Unmarshal(data, &tools); err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) < 8 {
		t.Fatalf("expected parser tools, got %d", len(tools.Tools))
	}
}

func testServer(t *testing.T, dir string) *mcp.Server {
	t.Helper()
	parserTools, err := New(Config{Workspace: dir})
	if err != nil {
		t.Fatal(err)
	}
	server := mcp.NewServer("test")
	for _, tool := range parserTools.Tools() {
		server.AddTool(tool)
	}
	return server
}

func callMCPTool(t *testing.T, server *mcp.Server, name string, args map[string]any) mcp.ToolResult {
	t.Helper()
	params, _ := json.Marshal(mcp.CallToolParams{Name: name, Arguments: args})
	resp := callTool(t, server, "tools/call", params)
	if resp.Error != nil {
		t.Fatalf("%s error: %s", name, resp.Error.Message)
	}
	data, _ := json.Marshal(resp.Result)
	var result mcp.ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func callTool(t *testing.T, server *mcp.Server, method string, params json.RawMessage) mcp.Response {
	t.Helper()
	resp := server.Handle(context.Background(), mcp.Request{JSONRPC: mcp.JSONRPCVersion, ID: 1, Method: method, Params: params})
	if resp.Error != nil {
		t.Fatalf("%s error: %s", method, resp.Error.Message)
	}
	return resp
}

func resultText(result mcp.ToolResult) string {
	var out strings.Builder
	for _, content := range result.Content {
		out.WriteString(content.Text)
	}
	return out.String()
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	for name, content := range files {
		writer, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}
