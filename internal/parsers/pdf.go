package parsers

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	pdfStringPattern = regexp.MustCompile(`\((?:\\.|[^\\)])*\)`)
	pdfMetaPattern   = regexp.MustCompile(`/([A-Za-z]+)\s*\(([^)]*)\)`)
	pdfStreamPattern = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
	pdfNodePattern   = regexp.MustCompile(`\bnode\d{5,}\b`)
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
	return cleanPDFText(strings.Join(chunks, "\n"))
}

func pdfTextFromBytes(data []byte) string {
	matches := pdfStringPattern.FindAll(data, -1)
	var out strings.Builder
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		text := pdfUnescape(string(match[1 : len(match)-1]))
		if isReadablePDFText(text) {
			out.WriteString(text)
			out.WriteByte(' ')
		}
	}
	return out.String()
}

func isReadablePDFText(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 2 || !utf8.ValidString(text) {
		return false
	}
	if strings.EqualFold(text, "Adobe UCS") || isPDFLigatureToken(text) {
		return false
	}
	printable := 0
	bad := 0
	for _, r := range text {
		switch {
		case r == utf8.RuneError:
			bad++
		case r == '\n' || r == '\r' || r == '\t':
			printable++
		case unicode.IsPrint(r):
			printable++
		default:
			bad++
		}
	}
	return printable >= 2 && bad*4 <= printable
}

func isPDFLigatureToken(text string) bool {
	switch text {
	case "fi", "fl", "ff", "ffi", "ffl":
		return true
	default:
		return false
	}
}

func cleanPDFText(text string) string {
	lines := strings.Split(normalizeSpace(text), "\n")
	var kept []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if isContentPDFLine(line) {
			kept = append(kept, line)
		}
	}
	cleaned := strings.Join(kept, "\n")
	if contentWordCount(cleaned) < 5 {
		return ""
	}
	return cleaned
}

func isContentPDFLine(line string) bool {
	if len(line) < 4 {
		return false
	}
	if strings.EqualFold(line, "Adobe UCS") {
		return false
	}
	if pdfNodePattern.MatchString(line) {
		return false
	}
	if strings.Contains(line, "http://") || strings.Contains(line, "https://") || strings.Contains(line, "mailto:") || strings.Contains(line, "tel:") || strings.Contains(line, "@") {
		return true
	}
	words := strings.FieldsFunc(line, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	longWords := 0
	vowelWords := 0
	alnum := 0
	for _, r := range line {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			alnum++
		}
	}
	for _, word := range words {
		if len([]rune(word)) >= 3 {
			longWords++
			if hasVowel(word) {
				vowelWords++
			}
		}
	}
	return longWords >= 3 && vowelWords*2 >= longWords && alnum*3 >= len([]rune(line))
}

func hasVowel(text string) bool {
	for _, r := range strings.ToLower(text) {
		switch r {
		case 'a', 'e', 'i', 'o', 'u', 'y':
			return true
		}
	}
	return false
}

func contentWordCount(text string) int {
	count := 0
	for _, word := range strings.FieldsFunc(text, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	}) {
		if len([]rune(word)) >= 3 && !strings.EqualFold(word, "Adobe") && !strings.EqualFold(word, "UCS") {
			count++
		}
	}
	return count
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
