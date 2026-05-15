package objectstore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Enabled         bool
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
	Prefix          string
}

func (c Config) Ready() bool {
	return c.Enabled &&
		strings.TrimSpace(c.Endpoint) != "" &&
		strings.TrimSpace(c.Bucket) != "" &&
		strings.TrimSpace(c.AccessKeyID) != "" &&
		strings.TrimSpace(c.SecretAccessKey) != ""
}

func (c Config) Normalize() Config {
	c.Endpoint = strings.TrimRight(strings.TrimSpace(c.Endpoint), "/")
	c.Region = strings.TrimSpace(c.Region)
	if c.Region == "" {
		c.Region = "auto"
	}
	c.Bucket = strings.Trim(strings.TrimSpace(c.Bucket), "/")
	c.AccessKeyID = strings.TrimSpace(c.AccessKeyID)
	c.SecretAccessKey = strings.TrimSpace(c.SecretAccessKey)
	c.PublicBaseURL = strings.TrimRight(strings.TrimSpace(c.PublicBaseURL), "/")
	c.Prefix = strings.Trim(strings.TrimSpace(filepath.ToSlash(c.Prefix)), "/")
	return c
}

func (c Config) ObjectKey(rel string) string {
	rel = strings.Trim(strings.TrimSpace(filepath.ToSlash(rel)), "/")
	if c.Prefix == "" {
		return rel
	}
	return c.Prefix + "/" + rel
}

func (c Config) PublicURL(key string) string {
	if strings.TrimSpace(c.PublicBaseURL) == "" {
		return ""
	}
	return strings.TrimRight(c.PublicBaseURL, "/") + "/" + escapePath(key)
}

func Upload(ctx context.Context, cfg Config, key string, data []byte, contentType string) (string, error) {
	cfg = cfg.Normalize()
	if !cfg.Ready() {
		return "", errors.New("R2 storage is not configured")
	}
	if strings.TrimSpace(key) == "" {
		return "", errors.New("object key is required")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	objectURL := cfg.Endpoint + "/" + escapePath(cfg.Bucket) + "/" + escapePath(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
	req.Header.Set("X-Amz-Content-Sha256", payloadHash(data))
	signV4(req, cfg, data)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("R2 upload failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return cfg.PublicURL(key), nil
}

func ContentTypeForPath(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		if contentType := mime.TypeByExtension(ext); contentType != "" {
			return contentType
		}
		return "application/octet-stream"
	}
}

func signV4(req *http.Request, cfg Config, data []byte) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	payload := payloadHash(data)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payload)
	req.Host = req.URL.Host

	canonicalHeaders := "cache-control:" + strings.TrimSpace(req.Header.Get("Cache-Control")) + "\n" +
		"content-type:" + strings.TrimSpace(req.Header.Get("Content-Type")) + "\n" +
		"host:" + req.URL.Host + "\n" +
		"x-amz-content-sha256:" + payload + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "cache-control;content-type;host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := strings.Join([]string{
		req.Method,
		emptyPathAsSlash(req.URL.EscapedPath()),
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payload,
	}, "\n")

	scope := dateStamp + "/" + cfg.Region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))
	signingKey := v4SigningKey(cfg.SecretAccessKey, dateStamp, cfg.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+cfg.AccessKeyID+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func payloadHash(data []byte) string {
	return sha256Hex(data)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func v4SigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func escapePath(value string) string {
	parts := strings.Split(filepath.ToSlash(strings.Trim(value, "/")), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func emptyPathAsSlash(value string) string {
	if value == "" {
		return "/"
	}
	return path.Clean("/" + strings.TrimPrefix(value, "/"))
}
