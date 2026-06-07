package parsers

import (
	"archive/tar"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
)

func parseCSVFile(path string, delimiter rune, maxRows int) (map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	var rows [][]string
	for len(rows) < maxRows {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return map[string]any{"format": "delimited", "rows": rows, "returned_rows": len(rows)}, nil
}

func parseJSONFile(path string, maxBytes int) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return map[string]any{"format": "json", "preview": truncate(pretty(value), maxBytes)}, nil
}

func parseHTMLFile(path string, maxBytes int) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"format": "html", "text": truncate(stripTags(string(data)), maxBytes)}, nil
}

func parseXMLFile(path string, maxBytes int) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return map[string]any{"format": "xml", "text": truncate(xmlText(data), maxBytes)}, nil
}

func parseImageFile(path string) (map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return nil, err
	}
	info, _ := os.Stat(path)
	return map[string]any{
		"format": format,
		"width":  config.Width,
		"height": config.Height,
		"size":   info.Size(),
	}, nil
}

func parseEmailFile(path string, maxBytes int) (map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	msg, err := mail.ReadMessage(file)
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(io.LimitReader(msg.Body, int64(maxBytes)))
	headers := map[string]string{}
	for key, values := range msg.Header {
		headers[key] = strings.Join(values, ", ")
	}
	contentType := msg.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	text := string(body)
	if strings.Contains(mediaType, "html") {
		text = stripTags(text)
	}
	return map[string]any{"format": "eml", "headers": headers, "body": truncate(text, maxBytes)}, nil
}

type notebook struct {
	Cells []struct {
		CellType string   `json:"cell_type"`
		Source   []string `json:"source"`
		Outputs  []any    `json:"outputs"`
	} `json:"cells"`
	Metadata map[string]any `json:"metadata"`
}

func parseNotebook(path string, maxBytes int) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var nb notebook
	if err := json.Unmarshal(data, &nb); err != nil {
		return nil, err
	}
	cells := make([]map[string]any, 0, len(nb.Cells))
	for i, cell := range nb.Cells {
		cells = append(cells, map[string]any{
			"index":      i + 1,
			"type":       cell.CellType,
			"source":     truncate(strings.Join(cell.Source, ""), maxBytes/4),
			"outputs":    len(cell.Outputs),
			"has_output": len(cell.Outputs) > 0,
		})
	}
	return map[string]any{"format": "ipynb", "metadata": nb.Metadata, "cells": cells}, nil
}

func parseArchive(path string, maxEntries int) (map[string]any, error) {
	switch ext(path) {
	case ".zip", ".epub", ".docx", ".xlsx", ".xlsm", ".pptx", ".odt", ".ods", ".odp":
		entries, err := zipEntries(path, maxEntries)
		return map[string]any{"format": "zip", "entries": entries}, err
	case ".tar":
		return parseTar(path, maxEntries)
	case ".gz", ".tgz":
		return parseGzip(path, maxEntries)
	default:
		return nil, os.ErrInvalid
	}
}

func parseTar(path string, maxEntries int) (map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := tar.NewReader(file)
	var entries []map[string]any
	for len(entries) < maxEntries {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, map[string]any{"name": header.Name, "size": header.Size, "type": header.Typeflag})
	}
	return map[string]any{"format": "tar", "entries": entries}, nil
}

func parseGzip(path string, maxEntries int) (map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	if strings.HasSuffix(strings.ToLower(path), ".tgz") || strings.HasSuffix(strings.ToLower(path), ".tar.gz") {
		reader := tar.NewReader(gz)
		var entries []map[string]any
		for len(entries) < maxEntries {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			entries = append(entries, map[string]any{"name": header.Name, "size": header.Size, "type": header.Typeflag})
		}
		return map[string]any{"format": "tgz", "entries": entries}, nil
	}
	return map[string]any{"format": "gzip", "name": gz.Name, "comment": gz.Comment, "modified": gz.ModTime}, nil
}

func parseEPUB(path string, maxBytes int) (map[string]any, error) {
	entries, err := zipEntries(path, 500)
	if err != nil {
		return nil, err
	}
	var chapters []map[string]any
	for _, entry := range entries {
		name, _ := entry["name"].(string)
		if strings.HasSuffix(name, ".xhtml") || strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".htm") {
			data, err := readZipFile(path, name, int64(maxBytes))
			if err != nil {
				continue
			}
			chapters = append(chapters, map[string]any{"name": name, "text": truncate(stripTags(string(data)), maxBytes/4)})
			if len(chapters) >= 25 {
				break
			}
		}
	}
	return map[string]any{"format": "epub", "entries": len(entries), "chapters": chapters}, nil
}

func partName(path string) string {
	return filepath.Base(path)
}
