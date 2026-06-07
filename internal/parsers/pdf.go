package parsers

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	pdfStringPattern = regexp.MustCompile(`\((?:\\.|[^\\)])*\)`)
	pdfMetaPattern   = regexp.MustCompile(`/([A-Za-z]+)\s*\(([^)]*)\)`)
	pdfStreamPattern = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
)

func parsePDF(path string, maxBytes int) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := pdfText(data)
	metadata := map[string]string{}
	for _, match := range pdfMetaPattern.FindAllSubmatch(data, -1) {
		key := string(match[1])
		if key == "Title" || key == "Author" || key == "Subject" || key == "Creator" || key == "Producer" || key == "CreationDate" || key == "ModDate" {
			metadata[key] = pdfUnescape(string(match[2]))
		}
	}
	return map[string]any{
		"format":      "pdf",
		"parser_note": "best-effort text-layer extraction; scanned PDFs need OCR/Tika fallback in a future version",
		"metadata":    metadata,
		"text":        truncate(text, maxBytes),
	}, nil
}

func pdfText(data []byte) string {
	chunks := []string{pdfTextFromBytes(data)}
	for _, match := range pdfStreamPattern.FindAllSubmatch(data, -1) {
		stream := match[1]
		if inflated, err := inflate(stream); err == nil {
			chunks = append(chunks, pdfTextFromBytes(inflated))
		}
	}
	return normalizeSpace(strings.Join(chunks, "\n"))
}

func pdfTextFromBytes(data []byte) string {
	matches := pdfStringPattern.FindAll(data, -1)
	var out strings.Builder
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		text := pdfUnescape(string(match[1 : len(match)-1]))
		if strings.TrimSpace(text) != "" {
			out.WriteString(text)
			out.WriteByte(' ')
		}
	}
	return out.String()
}

func inflate(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(bytes.TrimSpace(data)))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func pdfUnescape(text string) string {
	replacer := strings.NewReplacer(
		`\(`, `(`,
		`\)`, `)`,
		`\\`, `\`,
		`\n`, "\n",
		`\r`, "\n",
		`\t`, "\t",
		`\b`, "",
		`\f`, "",
	)
	return replacer.Replace(text)
}
