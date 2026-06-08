package parsers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"tinychain/mcp"
)

func (p *ParserTools) Tools() []mcp.Tool {
	return []mcp.Tool{
		p.parserWorkspaceInfoTool(),
		p.detectFileTypeTool(),
		p.parseFileTool(),
		p.extractTextTool(),
		p.extractMetadataTool(),
		p.listDocumentPartsTool(),
		p.extractPartTool(),
		p.extractTablesTool(),
		p.parseArchiveTool(),
		p.visionExtractTool(),
	}
}

func (p *ParserTools) parserWorkspaceInfoTool() mcp.Tool {
	return mcp.Tool{
		Name:        "parser_workspace_info",
		Description: "Describe parser workspace, path policy, output limits, and supported parser families.",
		InputSchema: schema(map[string]any{}),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			return mcp.Text(pretty(map[string]any{
				"workspace":   p.root,
				"path_policy": "all file paths must be relative to workspace and cannot escape it",
				"max_bytes":   p.maxBytes,
				"supported": []string{
					"pdf-best-effort", "docx", "pptx", "xlsx/xlsm", "csv/tsv", "epub",
					"zip/tar/gzip/tgz", "eml", "ipynb", "html", "xml/rss/atom", "json",
					"png/jpeg/gif image metadata", "opendocument text",
					"vision_extract with OpenAI/OpenRouter keys from environment",
				},
				"vision": visionStatus(),
			})), nil
		},
	}
}

func (p *ParserTools) detectFileTypeTool() mcp.Tool {
	return mcp.Tool{
		Name:        "detect_file_type",
		Description: "Detect file type, MIME hint, parser family, size, and supported document parts for a workspace-relative file.",
		InputSchema: schema(map[string]any{"path": stringProp("Workspace-relative file path.")}, "path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			path, err := p.resolve(textArg(args, "path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			info, err := os.Stat(path)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			kind := detect(path)
			return mcp.Text(pretty(map[string]any{
				"path":      p.rel(path),
				"size":      info.Size(),
				"kind":      kind.Kind,
				"mime":      kind.MIME,
				"parser":    kind.Parser,
				"parts":     kind.Parts,
				"summary":   kind.Description,
				"extension": ext(path),
			})), nil
		},
	}
}

func (p *ParserTools) parseFileTool() mcp.Tool {
	return mcp.Tool{
		Name:        "parse_file",
		Description: "Parse a supported binary/document file into structured JSON containing metadata, extracted text, rows, slides, chapters, or entries. For scanned PDFs, images, or DOCX embedded-image OCR, use vision_extract.",
		InputSchema: schema(map[string]any{
			"path":      stringProp("Workspace-relative file path."),
			"max_bytes": numberProp("Maximum extracted text bytes. Defaults to server max_bytes."),
		}, "path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			path, err := p.resolve(textArg(args, "path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			maxBytes := intArg(args, "max_bytes", int(p.maxBytes))
			if maxBytes <= 0 || int64(maxBytes) > p.maxBytes {
				maxBytes = int(p.maxBytes)
			}
			result, err := p.parse(path, maxBytes)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			result["path"] = p.rel(path)
			return mcp.Text(pretty(result)), nil
		},
	}
}

func (p *ParserTools) extractTextTool() mcp.Tool {
	return mcp.Tool{
		Name:        "extract_text",
		Description: "Extract plain text from a supported file, preserving rough page/slide/sheet/chapter boundaries where possible. If little/no text is returned from a PDF/DOCX/image and vision keys are available, call vision_extract.",
		InputSchema: schema(map[string]any{
			"path":      stringProp("Workspace-relative file path."),
			"max_bytes": numberProp("Maximum text bytes. Defaults to server max_bytes."),
		}, "path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			path, err := p.resolve(textArg(args, "path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			maxBytes := intArg(args, "max_bytes", int(p.maxBytes))
			result, err := p.parse(path, maxBytes)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(extractText(result, maxBytes)), nil
		},
	}
}

func (p *ParserTools) extractMetadataTool() mcp.Tool {
	return mcp.Tool{
		Name:        "extract_metadata",
		Description: "Return metadata and structural hints for a supported file without returning full content.",
		InputSchema: schema(map[string]any{"path": stringProp("Workspace-relative file path.")}, "path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			path, err := p.resolve(textArg(args, "path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			info, err := os.Stat(path)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			kind := detect(path)
			metadata := map[string]any{"path": p.rel(path), "size": info.Size(), "kind": kind}
			if result, err := p.parse(path, 4096); err == nil {
				if raw, ok := result["metadata"]; ok {
					metadata["metadata"] = raw
				}
			}
			return mcp.Text(pretty(metadata)), nil
		},
	}
}

func (p *ParserTools) listDocumentPartsTool() mcp.Tool {
	return mcp.Tool{
		Name:        "list_document_parts",
		Description: "List pages/slides/sheets/chapters/archive entries when a file exposes addressable parts.",
		InputSchema: schema(map[string]any{
			"path":      stringProp("Workspace-relative file path."),
			"max_items": numberProp("Maximum parts to return. Defaults to 200."),
		}, "path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			path, err := p.resolve(textArg(args, "path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			limit := intArg(args, "max_items", 200)
			parts, err := p.parts(path, limit)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(map[string]any{"path": p.rel(path), "parts": parts})), nil
		},
	}
}

func (p *ParserTools) extractPartTool() mcp.Tool {
	return mcp.Tool{
		Name:        "extract_part",
		Description: "Extract one named part from a multi-part document. For archives/EPUB/OOXML, part_name is the internal entry path.",
		InputSchema: schema(map[string]any{
			"path":      stringProp("Workspace-relative file path."),
			"part_name": stringProp("Part name from list_document_parts."),
			"max_bytes": numberProp("Maximum bytes to return."),
		}, "path", "part_name"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			path, err := p.resolve(textArg(args, "path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			maxBytes := int64(intArg(args, "max_bytes", int(p.maxBytes)))
			if maxBytes <= 0 || maxBytes > p.maxBytes {
				maxBytes = p.maxBytes
			}
			data, err := readZipFile(path, textArg(args, "part_name"), maxBytes)
			if err != nil {
				return mcp.ToolResult{}, err
			}
			text := string(data)
			if ext(textArg(args, "part_name")) == ".xml" || ext(textArg(args, "part_name")) == ".rels" {
				text = xmlText(data)
			} else if ext(textArg(args, "part_name")) == ".html" || ext(textArg(args, "part_name")) == ".xhtml" {
				text = stripTags(text)
			}
			return mcp.Text(pretty(map[string]any{"part_name": textArg(args, "part_name"), "content": text})), nil
		},
	}
}

func (p *ParserTools) extractTablesTool() mcp.Tool {
	return mcp.Tool{
		Name:        "extract_tables",
		Description: "Extract table-like data from CSV/TSV/XLSX files. Returns sheet or row samples rather than full huge datasets.",
		InputSchema: schema(map[string]any{
			"path":     stringProp("Workspace-relative file path."),
			"max_rows": numberProp("Maximum rows per table/sheet. Defaults to 100."),
		}, "path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			path, err := p.resolve(textArg(args, "path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			maxRows := intArg(args, "max_rows", 100)
			switch ext(path) {
			case ".csv":
				result, err := parseCSVFile(path, ',', maxRows)
				return mcp.Text(pretty(result)), err
			case ".tsv":
				result, err := parseCSVFile(path, '\t', maxRows)
				return mcp.Text(pretty(result)), err
			case ".xlsx", ".xlsm":
				result, err := parseXlsx(path, int(p.maxBytes))
				return mcp.Text(pretty(result)), err
			default:
				return mcp.ToolResult{}, fmt.Errorf("tables not supported for %s", ext(path))
			}
		},
	}
}

func (p *ParserTools) parseArchiveTool() mcp.Tool {
	return mcp.Tool{
		Name:        "parse_archive",
		Description: "List archive entries for ZIP/TAR/GZIP/TGZ and ZIP-container document formats like DOCX/PPTX/XLSX/EPUB.",
		InputSchema: schema(map[string]any{
			"path":        stringProp("Workspace-relative archive path."),
			"max_entries": numberProp("Maximum entries to return. Defaults to 500."),
		}, "path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			path, err := p.resolve(textArg(args, "path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			result, err := parseArchive(path, intArg(args, "max_entries", 500))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			return mcp.Text(pretty(result)), nil
		},
	}
}

func (p *ParserTools) visionExtractTool() mcp.Tool {
	return mcp.Tool{
		Name:        "vision_extract",
		Description: "Run a local text extraction pass, then use configured OpenAI/OpenRouter vision when available to OCR/read PDF pages, images, or embedded DOCX images.",
		InputSchema: schema(map[string]any{
			"path":       stringProp("Workspace-relative PDF, DOCX, or image path."),
			"prompt":     stringProp("Optional focused instruction for the vision/OCR pass."),
			"max_bytes":  numberProp("Maximum local text bytes. Defaults to server max_bytes."),
			"max_images": numberProp("Maximum embedded images to send for DOCX/image-based vision. Defaults to 6."),
			"provider":   stringProp("Optional override: openai or openrouter."),
			"model":      stringProp("Optional model override. Defaults to NULLBOT_VISION_MODEL or a provider default."),
		}, "path"),
		Handler: func(ctx context.Context, args map[string]any) (mcp.ToolResult, error) {
			path, err := p.resolve(textArg(args, "path"))
			if err != nil {
				return mcp.ToolResult{}, err
			}
			maxBytes := intArg(args, "max_bytes", int(p.maxBytes))
			if maxBytes <= 0 || int64(maxBytes) > p.maxBytes {
				maxBytes = int(p.maxBytes)
			}
			result, err := p.parse(path, maxBytes)
			localText := ""
			if err == nil {
				localText = extractText(result, maxBytes)
			}
			vision, err := p.visionExtract(ctx, path, visionOptions{
				Provider:  textArg(args, "provider"),
				Model:     textArg(args, "model"),
				Prompt:    textArg(args, "prompt"),
				MaxImages: intArg(args, "max_images", 6),
			})
			if err != nil {
				vision = "Vision/OCR pass unavailable: " + err.Error()
			}
			return mcp.Text(formatVisionResult(p.rel(path), localText, vision)), nil
		},
	}
}

func (p *ParserTools) parse(path string, maxBytes int) (map[string]any, error) {
	switch ext(path) {
	case ".pdf":
		return parsePDF(path, maxBytes)
	case ".docx":
		return parseDocx(path, maxBytes)
	case ".pptx":
		return parsePptx(path, maxBytes)
	case ".xlsx", ".xlsm":
		return parseXlsx(path, maxBytes)
	case ".csv":
		return parseCSVFile(path, ',', 100)
	case ".tsv":
		return parseCSVFile(path, '\t', 100)
	case ".epub":
		return parseEPUB(path, maxBytes)
	case ".zip", ".tar", ".gz", ".tgz":
		return parseArchive(path, 500)
	case ".eml":
		return parseEmailFile(path, maxBytes)
	case ".ipynb":
		return parseNotebook(path, maxBytes)
	case ".html", ".htm":
		return parseHTMLFile(path, maxBytes)
	case ".xml", ".rss", ".atom":
		return parseXMLFile(path, maxBytes)
	case ".json":
		return parseJSONFile(path, maxBytes)
	case ".png", ".jpg", ".jpeg", ".gif":
		return parseImageFile(path)
	case ".odt", ".ods", ".odp":
		return parseOpenDocument(path, maxBytes)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", ext(path))
	}
}

func (p *ParserTools) parts(path string, maxItems int) ([]map[string]any, error) {
	switch ext(path) {
	case ".zip", ".epub", ".docx", ".xlsx", ".xlsm", ".pptx", ".odt", ".ods", ".odp":
		return zipEntries(path, maxItems)
	case ".tar", ".gz", ".tgz":
		result, err := parseArchive(path, maxItems)
		if err != nil {
			return nil, err
		}
		entries, _ := result["entries"].([]map[string]any)
		return entries, nil
	default:
		kind := detect(path)
		parts := make([]map[string]any, 0, len(kind.Parts))
		for _, part := range kind.Parts {
			parts = append(parts, map[string]any{"name": part})
		}
		return parts, nil
	}
}

func extractText(result map[string]any, maxBytes int) string {
	if text, ok := result["text"].(string); ok {
		if strings.TrimSpace(text) == "" {
			if note, ok := result["parser_note"].(string); ok && note != "" {
				return note
			}
		}
		return truncate(text, maxBytes)
	}
	return truncate(pretty(result), maxBytes)
}
