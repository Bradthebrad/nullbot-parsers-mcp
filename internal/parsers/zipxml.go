package parsers

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func readZipFile(path, name string, limit int64) ([]byte, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, limit))
	}
	return nil, fmt.Errorf("%s not found", name)
}

func zipEntries(path string, max int) ([]map[string]any, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	entries := make([]map[string]any, 0, len(reader.File))
	for i, file := range reader.File {
		if max > 0 && i >= max {
			break
		}
		entries = append(entries, map[string]any{
			"name":         file.Name,
			"compressed":   file.CompressedSize64,
			"uncompressed": file.UncompressedSize64,
			"modified":     file.Modified.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return entries, nil
}

func xmlText(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var out strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch value := token.(type) {
		case xml.CharData:
			text := strings.TrimSpace(string(value))
			if text != "" {
				out.WriteString(text)
				out.WriteByte(' ')
			}
		case xml.EndElement:
			switch strings.ToLower(value.Name.Local) {
			case "p", "br", "tr", "row", "slide", "div":
				out.WriteByte('\n')
			}
		}
	}
	return normalizeSpace(out.String())
}

func parseDocx(path string, maxBytes int) (map[string]any, error) {
	data, err := readZipFile(path, "word/document.xml", int64(maxBytes)*4)
	if err != nil {
		return nil, err
	}
	core, _ := ooxmlCoreProperties(path)
	return map[string]any{
		"format":   "docx",
		"metadata": core,
		"text":     truncate(xmlText(data), maxBytes),
	}, nil
}

var slideName = regexp.MustCompile(`^ppt/slides/slide([0-9]+)\.xml$`)

func parsePptx(path string, maxBytes int) (map[string]any, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	type slide struct {
		Index int    `json:"index"`
		Name  string `json:"name"`
		Text  string `json:"text"`
	}
	var slides []slide
	for _, file := range reader.File {
		match := slideName.FindStringSubmatch(file.Name)
		if len(match) == 0 {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(rc, int64(maxBytes)))
		_ = rc.Close()
		var index int
		_, _ = fmt.Sscanf(match[1], "%d", &index)
		slides = append(slides, slide{Index: index, Name: file.Name, Text: xmlText(data)})
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].Index < slides[j].Index })
	core, _ := ooxmlCoreProperties(path)
	return map[string]any{"format": "pptx", "metadata": core, "slides": slides}, nil
}

func parseXlsx(path string, maxBytes int) (map[string]any, error) {
	shared, _ := xlsxSharedStrings(path)
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	type sheet struct {
		Name string     `json:"name"`
		Rows [][]string `json:"rows"`
	}
	var sheets []sheet
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "xl/worksheets/sheet") || !strings.HasSuffix(file.Name, ".xml") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(rc, int64(maxBytes)*4))
		_ = rc.Close()
		sheets = append(sheets, sheet{Name: filepath.Base(file.Name), Rows: parseSheetRows(data, shared, 100)})
	}
	core, _ := ooxmlCoreProperties(path)
	return map[string]any{"format": "xlsx", "metadata": core, "sheets": sheets}, nil
}

func xlsxSharedStrings(path string) ([]string, error) {
	data, err := readZipFile(path, "xl/sharedStrings.xml", 16*1024*1024)
	if err != nil {
		return nil, err
	}
	type text struct {
		Value string `xml:",chardata"`
	}
	type si struct {
		Texts []text `xml:"t"`
	}
	type sst struct {
		Items []si `xml:"si"`
	}
	var parsed sst
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		var b strings.Builder
		for _, text := range item.Texts {
			b.WriteString(text.Value)
		}
		out = append(out, b.String())
	}
	return out, nil
}

func parseSheetRows(data []byte, shared []string, maxRows int) [][]string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var rows [][]string
	var row []string
	inValue := false
	cellType := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "row":
				row = nil
			case "c":
				cellType = ""
				for _, attr := range value.Attr {
					if attr.Name.Local == "t" {
						cellType = attr.Value
					}
				}
			case "v", "t":
				inValue = true
			}
		case xml.CharData:
			if inValue {
				text := string(value)
				if cellType == "s" {
					var index int
					if _, err := fmt.Sscanf(text, "%d", &index); err == nil && index >= 0 && index < len(shared) {
						text = shared[index]
					}
				}
				row = append(row, text)
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "v", "t":
				inValue = false
			case "row":
				if len(row) > 0 {
					rows = append(rows, row)
				}
				if len(rows) >= maxRows {
					return rows
				}
			}
		}
	}
	return rows
}

func ooxmlCoreProperties(path string) (map[string]string, error) {
	data, err := readZipFile(path, "docProps/core.xml", 1024*1024)
	if err != nil {
		return nil, err
	}
	props := map[string]string{}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var current string
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch value := token.(type) {
		case xml.StartElement:
			current = value.Name.Local
		case xml.CharData:
			text := strings.TrimSpace(string(value))
			if current != "" && text != "" {
				props[current] = text
			}
		case xml.EndElement:
			current = ""
		}
	}
	return props, nil
}

func parseOpenDocument(path string, maxBytes int) (map[string]any, error) {
	data, err := readZipFile(path, "content.xml", int64(maxBytes)*4)
	if err != nil {
		return nil, err
	}
	return map[string]any{"format": "opendocument", "text": truncate(xmlText(data), maxBytes)}, nil
}
