package parsers

import (
	"net/http"
	"os"
)

type fileKind struct {
	Kind        string   `json:"kind"`
	MIME        string   `json:"mime"`
	Parser      string   `json:"parser"`
	Description string   `json:"description"`
	Parts       []string `json:"parts,omitempty"`
}

func detect(path string) fileKind {
	switch ext(path) {
	case ".pdf":
		return fileKind{"pdf", "application/pdf", "pdf_best_effort", "PDF text-layer and metadata extraction.", []string{"pages"}}
	case ".docx":
		return fileKind{"docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "ooxml_docx", "Word OOXML text and metadata extraction.", []string{"document"}}
	case ".pptx":
		return fileKind{"pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", "ooxml_pptx", "PowerPoint OOXML slide extraction.", []string{"slides"}}
	case ".xlsx", ".xlsm":
		return fileKind{"xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "ooxml_xlsx", "Excel OOXML workbook extraction.", []string{"sheets"}}
	case ".csv":
		return fileKind{"csv", "text/csv", "csv", "CSV rows, columns, and samples.", []string{"rows"}}
	case ".tsv":
		return fileKind{"tsv", "text/tab-separated-values", "csv", "TSV rows, columns, and samples.", []string{"rows"}}
	case ".epub":
		return fileKind{"epub", "application/epub+zip", "epub", "EPUB metadata and chapter text extraction.", []string{"chapters"}}
	case ".zip":
		return fileKind{"archive", "application/zip", "zip_archive", "ZIP archive listing and nested text preview.", []string{"entries"}}
	case ".tar":
		return fileKind{"archive", "application/x-tar", "tar_archive", "TAR archive listing.", []string{"entries"}}
	case ".gz", ".tgz":
		return fileKind{"archive", "application/gzip", "gzip_archive", "Gzip/TGZ archive listing.", []string{"entries"}}
	case ".eml":
		return fileKind{"email", "message/rfc822", "eml", "Email header, body, and attachment metadata.", []string{"headers", "body"}}
	case ".ipynb":
		return fileKind{"notebook", "application/x-ipynb+json", "ipynb", "Jupyter notebook cells and outputs.", []string{"cells"}}
	case ".html", ".htm":
		return fileKind{"html", "text/html", "html", "HTML text and link extraction.", []string{"document"}}
	case ".xml", ".rss", ".atom":
		return fileKind{"xml", "application/xml", "xml", "XML text extraction.", []string{"document"}}
	case ".json":
		return fileKind{"json", "application/json", "json", "JSON structure preview.", []string{"document"}}
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		return fileKind{"image", "image/*", "image_metadata", "Image dimensions and format metadata.", []string{"metadata"}}
	case ".odt", ".ods", ".odp":
		return fileKind{"opendocument", "application/vnd.oasis.opendocument", "opendocument", "OpenDocument ZIP/XML text extraction.", []string{"document"}}
	default:
		return fileKind{"unknown", sniffMIME(path), "unsupported", "No dedicated parser yet.", nil}
	}
}

func sniffMIME(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer file.Close()
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	return http.DetectContentType(buf[:n])
}
