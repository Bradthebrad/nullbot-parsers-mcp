package parsers

import (
	"encoding/json"
	"html"
	"path/filepath"
	"regexp"
	"strings"
)

func pretty(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func textArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func intArg(args map[string]any, key string, fallback int) int {
	value, ok := args[key].(float64)
	if !ok {
		return fallback
	}
	return int(value)
}

func boolArg(args map[string]any, key string) bool {
	value, _ := args[key].(bool)
	return value
}

func stringProp(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func numberProp(description string) map[string]any {
	return map[string]any{"type": "number", "description": description}
}

func boolProp(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func schema(properties map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
		"required":             required,
	}
}

func truncate(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	suffix := "\n...[truncated]"
	if limit <= len(suffix) {
		return text[:limit]
	}
	return strings.TrimRight(text[:limit-len(suffix)], "\r\n") + suffix
}

func normalizeSpace(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

var tagPattern = regexp.MustCompile(`(?s)<[^>]+>`)

func stripTags(text string) string {
	text = tagPattern.ReplaceAllString(text, " ")
	return normalizeSpace(html.UnescapeString(text))
}

func ext(path string) string {
	return strings.ToLower(filepath.Ext(path))
}
