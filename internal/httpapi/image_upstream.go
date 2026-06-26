package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"chatgpt2api/internal/protocol"
	"chatgpt2api/internal/util"
)

func (a *App) usesRemoteImageProvider() bool {
	return a != nil && a.config != nil && a.config.ImageProvider() == "chatgpt2api"
}

func (a *App) runConfiguredImageGeneration(ctx context.Context, payload map[string]any) (map[string]any, error) {
	if !a.usesRemoteImageProvider() {
		result, _, err := a.engine.HandleImageGenerations(ctx, payload)
		return result, err
	}
	if util.ToBool(payload["stream"]) {
		return nil, protocol.HTTPError{Status: http.StatusBadRequest, Message: "stream is not supported by the configured ChatGPT2API image provider"}
	}
	return a.postRemoteImageJSON(ctx, "/v1/images/generations", a.remoteImageJSONPayload(payload))
}

func (a *App) runConfiguredImageEdit(ctx context.Context, payload map[string]any, images []protocol.UploadedImage) (map[string]any, error) {
	if !a.usesRemoteImageProvider() {
		result, _, err := a.engine.HandleImageEdits(ctx, payload, images)
		return result, err
	}
	if util.ToBool(payload["stream"]) {
		return nil, protocol.HTTPError{Status: http.StatusBadRequest, Message: "stream is not supported by the configured ChatGPT2API image provider"}
	}
	if len(images) == 0 {
		return nil, &protocol.ImageGenerationError{Message: "image file is required", StatusCode: http.StatusBadRequest, Type: "invalid_request_error", Code: "missing_image"}
	}
	return a.postRemoteImageMultipart(ctx, "/v1/images/edits", a.remoteImageJSONPayload(payload), images)
}

func (a *App) postRemoteImageJSON(ctx context.Context, endpoint string, payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := a.newRemoteImageRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(body), "application/json")
	if err != nil {
		return nil, err
	}
	return a.doRemoteImageRequest(request)
}

func (a *App) postRemoteImageMultipart(ctx context.Context, endpoint string, payload map[string]any, images []protocol.UploadedImage) (map[string]any, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, image := range images {
		filename := firstNonEmpty(strings.TrimSpace(image.Filename), "image.png")
		part, err := writer.CreateFormFile("image", filename)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(image.Data); err != nil {
			return nil, err
		}
	}
	for key, value := range payload {
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
			_ = writer.WriteField(key, typed)
		case []map[string]any, []any, map[string]any:
			data, err := json.Marshal(typed)
			if err != nil {
				return nil, err
			}
			_ = writer.WriteField(key, string(data))
		default:
			_ = writer.WriteField(key, fmt.Sprint(value))
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	request, err := a.newRemoteImageRequest(ctx, http.MethodPost, endpoint, &body, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	return a.doRemoteImageRequest(request)
}

func (a *App) newRemoteImageRequest(ctx context.Context, method, endpoint string, body io.Reader, contentType string) (*http.Request, error) {
	baseURL := strings.TrimRight(a.config.ImageChatGPT2APIBaseURL(), "/")
	if baseURL == "" {
		return nil, protocol.HTTPError{Status: http.StatusBadRequest, Message: "ChatGPT2API image base URL is not configured"}
	}
	request, err := http.NewRequestWithContext(ctx, method, baseURL+endpoint, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	apiKey := strings.TrimSpace(a.config.ImageChatGPT2APIAPIKey())
	if apiKey == "" {
		return nil, protocol.HTTPError{Status: http.StatusBadRequest, Message: "ChatGPT2API image API key is not configured"}
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	return request, nil
}

func (a *App) doRemoteImageRequest(request *http.Request) (map[string]any, error) {
	client := a.proxy.HTTPClient(5 * time.Minute)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, remoteImageProviderError(response.StatusCode, data)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, protocol.HTTPError{Status: http.StatusBadGateway, Message: "invalid JSON response from ChatGPT2API image provider"}
	}
	return result, nil
}

func (a *App) remoteImageJSONPayload(payload map[string]any) map[string]any {
	out := map[string]any{
		"prompt":          util.Clean(payload["prompt"]),
		"model":           firstNonEmpty(util.Clean(payload["model"]), util.ImageModelAuto),
		"n":               util.ToInt(payload["n"], 1),
		"size":            util.Clean(payload["size"]),
		"quality":         util.Clean(payload["quality"]),
		"response_format": firstNonEmpty(util.Clean(payload["response_format"]), "url"),
		"base_url":        strings.TrimRight(a.config.ImageChatGPT2APIBaseURL(), "/"),
		"visibility":      util.Clean(payload["visibility"]),
	}
	if out["size"] == "" {
		delete(out, "size")
	}
	if out["quality"] == "" {
		delete(out, "quality")
	}
	if out["visibility"] == "" {
		delete(out, "visibility")
	}
	if messages := util.AsMapSlice(payload["messages"]); len(messages) > 0 {
		out["messages"] = messages
	}
	if images := util.AsStringSlice(payload["images"]); len(images) > 0 {
		out["images"] = images
	}
	if resolution := firstNonEmpty(util.Clean(payload["image_resolution"]), util.Clean(payload["resolution"])); resolution != "" {
		out["resolution"] = resolution
	}
	if outputFormat := util.Clean(payload["output_format"]); outputFormat != "" {
		out["output_format"] = outputFormat
	}
	if compression, ok := imageOutputCompressionFromBody(payload["output_compression"]); ok {
		out["output_compression"] = compression
	}
	return out
}

func remoteImageProviderError(status int, body []byte) error {
	var openAIError struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &openAIError) == nil && strings.TrimSpace(openAIError.Error.Message) != "" {
		return &protocol.ImageGenerationError{
			Message:    strings.TrimSpace(openAIError.Error.Message),
			StatusCode: status,
			Type:       firstNonEmpty(strings.TrimSpace(openAIError.Error.Type), "server_error"),
			Code:       strings.TrimSpace(openAIError.Error.Code),
		}
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		if detail := util.StringMap(payload["detail"]); detail != nil {
			if message := firstNonEmpty(util.Clean(detail["error"]), util.Clean(detail["message"])); message != "" {
				return protocol.HTTPError{Status: status, Message: message}
			}
		}
		if message := firstNonEmpty(util.Clean(payload["message"]), util.Clean(payload["error"])); message != "" {
			return protocol.HTTPError{Status: status, Message: message}
		}
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(status)
	}
	return protocol.HTTPError{Status: status, Message: message}
}
