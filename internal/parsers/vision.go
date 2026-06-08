package parsers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxVisionPayloadBytes = 45 * 1024 * 1024

type visionOptions struct {
	Provider  string
	Model     string
	Prompt    string
	MaxImages int
}

type visionImage struct {
	Name string
	MIME string
	Data []byte
}

func (p *ParserTools) visionExtract(ctx context.Context, path string, opts visionOptions) (string, error) {
	provider, model, key := visionProvider(opts)
	if key == "" {
		return "", fmt.Errorf("no OpenAI or OpenRouter API key available in environment")
	}
	prompt := strings.TrimSpace(opts.Prompt)
	if prompt == "" {
		prompt = "Read this document/image carefully. Extract visible text with OCR where needed, summarize the important content, and call out forms, tables, contact details, defects, dates, addresses, and action items."
	}
	if opts.MaxImages <= 0 {
		opts.MaxImages = 6
	}
	switch ext(path) {
	case ".pdf":
		if provider != "openai" {
			return "", fmt.Errorf("PDF file vision currently requires OpenAI Responses; OpenRouter OCR is available for images embedded in DOCX or direct image files")
		}
		return openAIFileVision(ctx, key, model, path, prompt)
	case ".docx":
		images, err := docxImages(path, opts.MaxImages)
		if err != nil {
			return "", err
		}
		if len(images) == 0 {
			return "", fmt.Errorf("no embedded DOCX images found for OCR pass")
		}
		return visionImages(ctx, provider, key, model, prompt, images)
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		image, err := fileVisionImage(path)
		if err != nil {
			return "", err
		}
		return visionImages(ctx, provider, key, model, prompt, []visionImage{image})
	default:
		return "", fmt.Errorf("vision_extract supports PDF, DOCX embedded images, and image files; got %s", ext(path))
	}
}

func visionProvider(opts visionOptions) (provider, model, key string) {
	provider = strings.ToLower(strings.TrimSpace(opts.Provider))
	if provider == "" {
		provider = strings.ToLower(strings.TrimSpace(os.Getenv("NULLBOT_VISION_PROVIDER")))
	}
	openAIKey := os.Getenv("OPENAI_API_KEY")
	openRouterKey := os.Getenv("OPENROUTER_API_KEY")
	if provider == "openrouter" && openRouterKey != "" {
		return "openrouter", defaultVisionModel(opts.Model, "openrouter"), openRouterKey
	}
	if openAIKey != "" {
		return "openai", defaultVisionModel(opts.Model, "openai"), openAIKey
	}
	if openRouterKey != "" {
		return "openrouter", defaultVisionModel(opts.Model, "openrouter"), openRouterKey
	}
	return provider, defaultVisionModel(opts.Model, provider), ""
}

func defaultVisionModel(model, provider string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		model = strings.TrimSpace(os.Getenv("NULLBOT_VISION_MODEL"))
	}
	if model != "" {
		return model
	}
	if provider == "openrouter" {
		return "openai/gpt-4o-mini"
	}
	return "gpt-4.1-mini"
}

func openAIFileVision(ctx context.Context, key, model, path, prompt string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) > maxVisionPayloadBytes {
		return "", fmt.Errorf("file is too large for inline vision payload: %d bytes", len(data))
	}
	body := map[string]any{
		"model": model,
		"input": []map[string]any{{
			"role": "user",
			"content": []map[string]any{
				{"type": "input_text", "text": prompt},
				{"type": "input_file", "filename": filepath.Base(path), "file_data": base64.StdEncoding.EncodeToString(data)},
			},
		}},
	}
	return postVision(ctx, "https://api.openai.com/v1/responses", key, body, extractOpenAIResponsesText)
}

func visionImages(ctx context.Context, provider, key, model, prompt string, images []visionImage) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("no images to inspect")
	}
	if provider == "openrouter" {
		return openRouterImageVision(ctx, key, model, prompt, images)
	}
	return openAIImageVision(ctx, key, model, prompt, images)
}

func openAIImageVision(ctx context.Context, key, model, prompt string, images []visionImage) (string, error) {
	content := []map[string]any{{"type": "input_text", "text": prompt}}
	for _, image := range images {
		content = append(content, map[string]any{
			"type":      "input_image",
			"image_url": dataURL(image.MIME, image.Data),
			"detail":    "high",
		})
	}
	body := map[string]any{
		"model": model,
		"input": []map[string]any{{"role": "user", "content": content}},
	}
	return postVision(ctx, "https://api.openai.com/v1/responses", key, body, extractOpenAIResponsesText)
}

func openRouterImageVision(ctx context.Context, key, model, prompt string, images []visionImage) (string, error) {
	content := []map[string]any{{"type": "text", "text": prompt}}
	for _, image := range images {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": dataURL(image.MIME, image.Data)},
		})
	}
	body := map[string]any{
		"model": model,
		"messages": []map[string]any{{
			"role":    "user",
			"content": content,
		}},
	}
	return postVision(ctx, "https://openrouter.ai/api/v1/chat/completions", key, body, extractOpenRouterText)
}

func postVision(ctx context.Context, url, key string, body any, extract func([]byte) (string, error)) (string, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("vision provider status %d: %s", resp.StatusCode, truncate(string(out), 800))
	}
	return extract(out)
}

func extractOpenAIResponsesText(data []byte) (string, error) {
	var resp struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	var parts []string
	for _, item := range resp.Output {
		for _, content := range item.Content {
			if content.Text != "" {
				parts = append(parts, content.Text)
			}
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("vision response contained no text")
	}
	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func extractOpenRouterText(data []byte) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("vision response contained no text")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func docxImages(path string, maxImages int) ([]visionImage, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var images []visionImage
	for _, file := range reader.File {
		if len(images) >= maxImages {
			break
		}
		if !strings.HasPrefix(file.Name, "word/media/") {
			continue
		}
		mimeType := mimeFromName(file.Name)
		if !strings.HasPrefix(mimeType, "image/") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(rc, maxVisionPayloadBytes))
		_ = rc.Close()
		if len(data) > 0 {
			images = append(images, visionImage{Name: file.Name, MIME: mimeType, Data: data})
		}
	}
	return images, nil
}

func fileVisionImage(path string) (visionImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return visionImage{}, err
	}
	if len(data) > maxVisionPayloadBytes {
		return visionImage{}, fmt.Errorf("image is too large for inline vision payload: %d bytes", len(data))
	}
	return visionImage{Name: filepath.Base(path), MIME: mimeFromName(path), Data: data}, nil
}

func dataURL(mimeType string, data []byte) string {
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func mimeFromName(name string) string {
	if typ := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); typ != "" {
		return typ
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func visionStatus() map[string]any {
	return map[string]any{
		"openai_key":     os.Getenv("OPENAI_API_KEY") != "",
		"openrouter_key": os.Getenv("OPENROUTER_API_KEY") != "",
		"provider":       os.Getenv("NULLBOT_VISION_PROVIDER"),
		"model":          os.Getenv("NULLBOT_VISION_MODEL"),
	}
}

func formatVisionResult(path, localText, vision string) string {
	if strings.TrimSpace(localText) == "" {
		localText = "(no local text extracted)"
	}
	if strings.TrimSpace(vision) == "" {
		vision = "(no vision/OCR output)"
	}
	return "File: `" + path + "`\n\n## Local text extraction\n\n" + localText + "\n\n## Vision/OCR extraction\n\n" + vision
}
