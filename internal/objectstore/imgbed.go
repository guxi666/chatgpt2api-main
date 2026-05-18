package objectstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type ImgBedConfig struct {
	Enabled       bool
	UploadURL     string
	AuthCode      string
	UploadChannel string
}

func (c ImgBedConfig) Normalize() ImgBedConfig {
	c.UploadURL = strings.TrimSpace(c.UploadURL)
	c.AuthCode = strings.TrimSpace(c.AuthCode)
	c.UploadChannel = strings.TrimSpace(c.UploadChannel)
	return c
}

func (c ImgBedConfig) Ready() bool {
	return c.Enabled && strings.TrimSpace(c.UploadURL) != ""
}

func UploadToImgBed(ctx context.Context, cfg ImgBedConfig, objectKey string, data []byte, contentType string) (string, error) {
	cfg = cfg.Normalize()
	if !cfg.Ready() {
		return "", fmt.Errorf("external image bed is not configured")
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	filename := path.Base(strings.TrimSpace(objectKey))
	if filename == "." || filename == "/" || filename == "" {
		filename = "image.bin"
	}
	requestURL, err := normalizeImgBedUploadURL(cfg)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		_ = writer.Close()
		return "", err
	}
	if _, err := fileWriter.Write(data); err != nil {
		_ = writer.Close()
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("external image bed upload failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	publicURL := extractImgBedURL(respBody)
	if publicURL == "" {
		return "", fmt.Errorf("external image bed upload succeeded but no URL was found in response")
	}
	if strings.HasPrefix(publicURL, "http://") || strings.HasPrefix(publicURL, "https://") {
		return publicURL, nil
	}
	if strings.HasPrefix(publicURL, "//") {
		return "https:" + publicURL, nil
	}
	base := strings.TrimRight(cfg.UploadURL, "/")
	if idx := strings.Index(base, "/api/"); idx > 0 {
		base = base[:idx]
	}
	if strings.HasSuffix(base, "/upload") {
		base = strings.TrimSuffix(base, "/upload")
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(publicURL, "/"), nil
}

func normalizeImgBedUploadURL(cfg ImgBedConfig) (string, error) {
	parsed, err := url.Parse(cfg.UploadURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	if cfg.AuthCode != "" {
		query.Set("authCode", cfg.AuthCode)
	}
	if cfg.UploadChannel != "" {
		query.Set("uploadChannel", cfg.UploadChannel)
	}
	// Request absolute URLs to avoid parsing ambiguity across different ImgBed deployments.
	query.Set("returnFormat", "full")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func extractImgBedURL(data []byte) string {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	if direct := lookupString(payload, "url", "src", "data.url", "data.src", "result.url", "result.src", "images.0.url", "images.0.src"); direct != "" {
		return strings.TrimSpace(direct)
	}
	if list, ok := payload.([]any); ok {
		for _, item := range list {
			if direct := lookupString(item, "url", "src", "data.url", "data.src"); direct != "" {
				return strings.TrimSpace(direct)
			}
		}
	}
	return ""
}

func lookupString(payload any, paths ...string) string {
	for _, p := range paths {
		if v := lookupByPath(payload, p); v != "" {
			return v
		}
	}
	return ""
}

func lookupByPath(payload any, path string) string {
	current := payload
	segments := strings.Split(path, ".")
	for _, segment := range segments {
		switch node := current.(type) {
		case map[string]any:
			current = node[segment]
		case []any:
			if segment == "0" {
				if len(node) == 0 {
					return ""
				}
				current = node[0]
			} else {
				return ""
			}
		default:
			return ""
		}
	}
	value, _ := current.(string)
	return strings.TrimSpace(value)
}
