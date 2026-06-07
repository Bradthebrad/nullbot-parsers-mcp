# nullbot-parsers-mcp

`nullbot-parsers-mcp` is a standalone Go MCP server for turning non-code files into agent-readable text, metadata, tables, and structured previews.

It is designed for NullBot, but it is not NullBot-specific. Any MCP-capable client can launch it as a stdio server or connect to it over local HTTP/SSE transports.

## Tool Suite

| Tool | Purpose |
| --- | --- |
| `parser_workspace_info` | Describe workspace, limits, and supported parser families. |
| `detect_file_type` | Identify type, MIME hint, parser family, size, and available parts. |
| `parse_file` | Parse a file into structured JSON. |
| `extract_text` | Return plain extracted text or a bounded structured fallback. |
| `extract_metadata` | Return metadata and structural hints without full content. |
| `list_document_parts` | List pages/slides/sheets/chapters/archive entries where available. |
| `extract_part` | Extract one ZIP-container part by internal path. |
| `extract_tables` | Extract table-like samples from CSV/TSV/XLSX. |
| `parse_archive` | List ZIP/TAR/GZIP/TGZ and ZIP-container entries. |

## Supported Formats

| Family | Extensions | Notes |
| --- | --- | --- |
| PDF | `.pdf` | Best-effort text-layer and metadata extraction. Scanned PDFs need future OCR/Tika fallback. |
| Word OOXML | `.docx` | Extracts `word/document.xml` text and core properties. |
| PowerPoint OOXML | `.pptx` | Extracts slide XML text and core properties. |
| Excel OOXML | `.xlsx`, `.xlsm` | Extracts shared strings and worksheet row samples. |
| Delimited tables | `.csv`, `.tsv` | Returns row samples. |
| OpenDocument | `.odt`, `.ods`, `.odp` | Extracts `content.xml` text. |
| EPUB | `.epub` | Extracts chapter-like XHTML/HTML entries. |
| Archives | `.zip`, `.tar`, `.gz`, `.tgz` | Lists entries and metadata. |
| Email | `.eml` | Extracts headers and body text. |
| Notebooks | `.ipynb` | Extracts cell type/source/output counts. |
| Web/XML/Data | `.html`, `.htm`, `.xml`, `.rss`, `.atom`, `.json` | Extracts text or pretty JSON preview. |
| Images | `.png`, `.jpg`, `.jpeg`, `.gif` | Extracts dimensions and format metadata. |

## Safety Model

All paths are workspace-relative. Absolute paths are rejected, and tools verify that resolved paths stay inside the configured workspace.

This server does not mutate files. It reads files, extracts bounded previews, and returns structured output intended for agent context.

## Build

```powershell
go build -trimpath -ldflags "-s -w" -o nullbot-parsers-mcp.exe ./cmd/nullbot-parsers-mcp
```

## Run

Default stdio mode:

```powershell
.\nullbot-parsers-mcp.exe --workspace C:\path\to\files
```

Streamable HTTP-style endpoint:

```powershell
.\nullbot-parsers-mcp.exe --transport streamable-http --addr 127.0.0.1:8770 --path /mcp --workspace C:\path\to\files
```

Legacy SSE-compatible endpoints:

```powershell
.\nullbot-parsers-mcp.exe --transport sse --addr 127.0.0.1:8770 --sse-path /sse --message-path /message --workspace C:\path\to\files
```

## Future Heavy Backends

The first release is pure Go. Future versions can add optional integrations for:

- Apache Tika for legacy Office, obscure document formats, and broader metadata extraction.
- OCR for scanned PDFs and images.
- Media transcription for audio/video.
- Deeper PDF table extraction.
- SQLite/Parquet/Avro scientific/data parsers.

## Development

```powershell
go test ./...
```
