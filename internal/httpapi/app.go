package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"chatgpt2api/internal/config"
	"chatgpt2api/internal/protocol"
	"chatgpt2api/internal/service"
	"chatgpt2api/internal/storage"
	"chatgpt2api/internal/util"
	"chatgpt2api/internal/version"
	frontend "chatgpt2api/internal/web"

	_ "github.com/HugoSmits86/nativewebp"
)

const (
	maxLoginPageImageSize      = 10 << 20
	imageThumbnailCacheControl = "public, max-age=31536000, immutable"
	authSessionCookieName      = "chatgpt2api_session"
)

type App struct {
	config      *config.Store
	auth        *service.AuthService
	accounts    *service.AccountService
	logs        *service.LogService
	logger      *service.Logger
	proxy       *service.ProxyService
	engine      *protocol.Engine
	images      *service.ImageService
	tasks       *service.ImageTaskService
	announce    *service.AnnouncementService
	prompts     *service.PromptFavoriteService
	cpa         *service.CPAConfig
	cpaImport   *service.CPAImportService
	sub2        *service.Sub2APIConfig
	sub2Import  *service.Sub2APIService
	register    *service.RegisterService
	billing     *service.EmailBillingService
	emailVerify *service.EmailVerificationService
	update      *service.UpdateService
	cancel      context.CancelFunc
}

func NewApp() (*App, error) {
	cfg, err := config.NewStore()
	if err != nil {
		return nil, err
	}
	storageBackend, err := cfg.StorageBackend()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	logs := service.NewLogService(cfg.DataDir, storageBackend)
	logger, err := service.NewLogger(cfg.DataDir, cfg.LogLevels)
	if err != nil {
		cancel()
		return nil, err
	}
	proxy := service.NewProxyService(cfg)
	accounts := service.NewAccountService(storageBackend, cfg, proxy, logs)
	auth := service.NewAuthService(storageBackend)
	bootstrap, err := auth.EnsureBootstrapAdmin(cfg.AdminUsername(), cfg.AdminPassword())
	if err != nil {
		cancel()
		return nil, err
	}
	if bootstrap.Created && bootstrap.Generated {
		fmt.Fprintf(os.Stderr, "bootstrap admin password generated: username=%s password=%s\n", bootstrap.Username, bootstrap.Password)
		logger.Warning("bootstrap admin password generated", "username", bootstrap.Username)
	}
	documentStore, _ := storageBackend.(storage.JSONDocumentBackend)
	engine := &protocol.Engine{Accounts: accounts, Config: cfg, Storage: documentStore, Proxy: proxy, Logger: logger}
	app := &App{config: cfg, auth: auth, accounts: accounts, logs: logs, logger: logger, proxy: proxy, engine: engine, images: service.NewImageService(cfg, storageBackend), announce: service.NewAnnouncementService(cfg.DataDir, storageBackend), prompts: service.NewPromptFavoriteService(cfg.DataDir, storageBackend), cpa: service.NewCPAConfig(cfg.DataDir, storageBackend), sub2: service.NewSub2APIConfig(cfg.DataDir, storageBackend), emailVerify: service.NewEmailVerificationService(cfg.EmailSMTP()), update: newUpdateService(cfg), cancel: cancel}
	app.billing = service.NewEmailBillingService(cfg.DataDir, storageBackend, auth)
	app.cpaImport = service.NewCPAImportService(app.cpa, accounts, proxy)
	app.sub2Import = service.NewSub2APIService(app.sub2, accounts)
	app.register = service.NewRegisterService(cfg.DataDir, accounts, storageBackend)
	app.tasks = service.NewStoredImageTaskService(filepath.Join(cfg.DataDir, "image_tasks.json"), storageBackend,
		func(ctx context.Context, identity service.Identity, payload map[string]any) (map[string]any, error) {
			return app.runLoggedImageTask(ctx, identity, payload, "/api/creation-tasks/image-generations", "文生图", func(ctx context.Context, payload map[string]any) (map[string]any, error) {
				result, _, err := engine.HandleImageGenerations(ctx, payload)
				return result, err
			})
		},
		func(ctx context.Context, identity service.Identity, payload map[string]any) (map[string]any, error) {
			return app.runLoggedImageTask(ctx, identity, payload, "/api/creation-tasks/image-edits", "图生图", func(ctx context.Context, payload map[string]any) (map[string]any, error) {
				images, _ := payload["images"].([]protocol.UploadedImage)
				result, _, err := engine.HandleImageEdits(ctx, payload, images)
				return result, err
			})
		},
		func(ctx context.Context, identity service.Identity, payload map[string]any) (map[string]any, error) {
			return app.runLoggedChatTask(ctx, identity, payload)
		},
		cfg.ImageRetentionDays,
		cfg.ImageConcurrentLimit,
		cfg.UserDefaultConcurrentLimit,
		cfg.UserDefaultRPMLimit,
	)
	app.tasks.SetTaskTimeoutGetter(func() time.Duration {
		return time.Duration(app.config.ImageTaskTimeoutSeconds()) * time.Second
	})
	app.tasks.SetResponseImageHandler(func(ctx context.Context, identity service.Identity, payload map[string]any) (map[string]any, error) {
		return app.runLoggedImageTask(ctx, identity, payload, "/api/creation-tasks/response-image-generations", "Responses 浣滅敾", func(ctx context.Context, payload map[string]any) (map[string]any, error) {
			return app.runResponsesImageGenerationTask(ctx, payload)
		})
	})
	accounts.StartLimitedWatcher(ctx, time.Duration(cfg.RefreshAccountIntervalMinute())*time.Minute)
	cfg.CleanupOldImages()
	return app, nil
}

func newUpdateService(cfg *config.Store) *service.UpdateService {
	return service.NewUpdateService(service.UpdateOptions{
		CurrentVersion: version.Get(),
		BuildType:      version.GetBuildType(),
		Repo:           cfg.UpdateRepo(),
		ProxyURL:       cfg.UpdateProxyURL(),
		GitHubToken:    cfg.UpdateGitHubToken(),
	})
}

func (a *App) Close() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.logger != nil {
		_ = a.logger.Close()
	}
}

func (a *App) Logger() *service.Logger {
	return a.logger
}

func (a *App) handleModels(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	result, err := a.engine.ListModels(r.Context())
	a.writeProtocol(w, r, result, nil, err, "openai", "/v1/models", "models", identity, "妯″瀷鍒楄〃", service.ImageVisibilityPrivate)
}

func (a *App) handleImageGenerations(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	normalizeImageRequestPayload(body)
	if !a.validateImageSingleCount(w, util.ToInt(body["n"], 1)) {
		return
	}
	if !a.ensureImageBillingCredit(w, identity, body) {
		return
	}
	body["owner_id"] = identityScope(identity)
	body["owner_name"] = identityDisplayName(identity)
	body["base_url"] = a.resolveImageBaseURL(r)
	visibility, err := service.NormalizeImageVisibility(util.Clean(body["visibility"]))
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if util.ToBool(body["async"]) {
		taskID := firstNonEmpty(util.Clean(body["client_task_id"]), "img-"+util.NewHex(24))
		task, submitErr := a.tasks.SubmitGenerationWithOptions(
			r.Context(),
			identity,
			taskID,
			util.Clean(body["prompt"]),
			firstNonEmpty(util.Clean(body["model"]), util.ImageModelAuto),
			util.Clean(body["size"]),
			util.Clean(body["quality"]),
			a.resolveImageBaseURL(r),
			util.ToInt(body["n"], 1),
			body["messages"],
			imageTaskRequestMetadata(body),
			imageOutputOptionsFromBody(body),
			visibility,
		)
		if submitErr != nil {
			writeCreationTaskSubmitError(w, submitErr)
			return
		}
		util.WriteJSON(w, http.StatusAccepted, asyncImageTaskResponse(task))
		return
	}
	model := firstNonEmpty(util.Clean(body["model"]), util.ImageModelAuto)
	result, stream, err := a.engine.HandleImageGenerations(r.Context(), body)
	if err == nil && stream == nil && hasImageResult(result) {
		if chargeErr := a.chargeImageUsage(identity, "/v1/images/generations", body); chargeErr != nil {
			util.WriteError(w, http.StatusPaymentRequired, chargeErr.Error())
			return
		}
	}
	a.writeProtocol(w, r, result, stream, err, "openai", "/v1/images/generations", model, identity, "文生图", visibility)
}

func (a *App) handleImageEdits(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	body, images, err := readMultipartImageBody(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	normalizeImageRequestPayload(body)
	if !a.validateImageSingleCount(w, util.ToInt(body["n"], 1)) {
		return
	}
	if !a.ensureImageBillingCredit(w, identity, body) {
		return
	}
	if len(images) == 0 {
		util.WriteError(w, http.StatusBadRequest, "image file is required")
		return
	}
	body["owner_id"] = identityScope(identity)
	body["owner_name"] = identityDisplayName(identity)
	body["base_url"] = a.resolveImageBaseURL(r)
	visibility, err := service.NormalizeImageVisibility(util.Clean(body["visibility"]))
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if util.ToBool(body["async"]) {
		taskID := firstNonEmpty(util.Clean(body["client_task_id"]), "img-edit-"+util.NewHex(24))
		task, submitErr := a.tasks.SubmitEditWithOptions(
			r.Context(),
			identity,
			taskID,
			util.Clean(body["prompt"]),
			firstNonEmpty(util.Clean(body["model"]), util.ImageModelAuto),
			util.Clean(body["size"]),
			util.Clean(body["quality"]),
			a.resolveImageBaseURL(r),
			images,
			util.ToInt(body["n"], 1),
			body["messages"],
			imageTaskRequestMetadata(body),
			imageOutputOptionsFromBody(body),
			visibility,
		)
		if submitErr != nil {
			writeCreationTaskSubmitError(w, submitErr)
			return
		}
		util.WriteJSON(w, http.StatusAccepted, asyncImageTaskResponse(task))
		return
	}
	model := firstNonEmpty(util.Clean(body["model"]), util.ImageModelAuto)
	result, stream, err := a.engine.HandleImageEdits(r.Context(), body, images)
	if err == nil && stream == nil && hasImageResult(result) {
		if chargeErr := a.chargeImageUsage(identity, "/v1/images/edits", body); chargeErr != nil {
			util.WriteError(w, http.StatusPaymentRequired, chargeErr.Error())
			return
		}
	}
	a.writeProtocol(w, r, result, stream, err, "openai", "/v1/images/edits", model, identity, "图生图", visibility)
}

func (a *App) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	body["owner_id"] = identityScope(identity)
	body["owner_name"] = identityDisplayName(identity)
	model := firstNonEmpty(util.Clean(body["model"]), "auto")
	result, stream, err := a.engine.HandleChatCompletions(r.Context(), body)
	a.writeProtocol(w, r, result, stream, err, "openai", "/v1/chat/completions", model, identity, "鏂囨湰鐢熸垚", service.ImageVisibilityPrivate)
}

func (a *App) handleResponses(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	body["owner_id"] = identityScope(identity)
	body["owner_name"] = identityDisplayName(identity)
	model := firstNonEmpty(util.Clean(body["model"]), "auto")
	result, stream, err := a.engine.HandleResponsesScoped(r.Context(), body, identityScope(identity))
	a.writeProtocol(w, r, result, stream, err, "openai", "/v1/responses", model, identity, "Responses", service.ImageVisibilityPrivate)
}

func (a *App) handleMessages(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" && r.Header.Get("x-api-key") != "" {
		authHeader = "Bearer " + r.Header.Get("x-api-key")
	}
	identity, ok := a.requireIdentity(w, r, authHeader)
	if !ok {
		return
	}
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	model := firstNonEmpty(util.Clean(body["model"]), "auto")
	result, stream, err := a.engine.HandleMessages(r.Context(), body)
	a.writeProtocol(w, r, result, stream, err, "anthropic", "/v1/messages", model, identity, "Messages", service.ImageVisibilityPrivate)
}

func (a *App) writeProtocol(w http.ResponseWriter, r *http.Request, result map[string]any, stream *protocol.StreamResult, err error, sseKind, endpoint, model string, identity service.Identity, summary, visibility string) {
	start := time.Now()
	if err != nil {
		a.logCall(identity, summary, r.Method, endpoint, model, start, "failed", protocolErrorHTTPStatus(err), err.Error(), nil)
		a.writeProtocolError(w, err)
		return
	}
	if stream == nil {
		urls := collectURLs(result)
		a.recordGeneratedImages(identity, urls, visibility)
		a.logCall(identity, summary, r.Method, endpoint, model, start, "success", http.StatusOK, "", urls)
		util.WriteJSON(w, http.StatusOK, result)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	if stream.Kind == "anthropic" || sseKind == "anthropic" {
		var urls []string
		for item := range stream.Items {
			urls = append(urls, collectURLs(item)...)
			event := firstNonEmpty(util.Clean(item["type"]), "message_delta")
			fmt.Fprintf(w, "event: %s\n", event)
			fmt.Fprintf(w, "data: %s\n\n", jsonString(item))
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err := <-stream.Err; err != nil {
			a.recordGeneratedImages(identity, urls, visibility)
			a.logCall(identity, summary, r.Method, endpoint, model, start, "failed", protocolErrorHTTPStatus(err), err.Error(), urls)
			fmt.Fprintf(w, "event: error\n")
			fmt.Fprintf(w, "data: %s\n\n", jsonString(map[string]any{"type": "error", "error": map[string]any{"type": fmt.Sprintf("%T", err), "message": err.Error()}}))
			return
		}
		a.recordGeneratedImages(identity, urls, visibility)
		a.logCall(identity, summary, r.Method, endpoint, model, start, "success", http.StatusOK, "", urls)
		return
	}
	fmt.Fprint(w, ": stream-open\n\n")
	if flusher != nil {
		flusher.Flush()
	}
	var urls []string
	for item := range stream.Items {
		urls = append(urls, collectURLs(item)...)
		fmt.Fprintf(w, "data: %s\n\n", jsonString(item))
		if flusher != nil {
			flusher.Flush()
		}
	}
	if err := <-stream.Err; err != nil {
		a.recordGeneratedImages(identity, urls, visibility)
		a.logCall(identity, summary, r.Method, endpoint, model, start, "failed", protocolErrorHTTPStatus(err), err.Error(), urls)
		fmt.Fprintf(w, "data: %s\n\n", jsonString(openAIErrorForStream(err)))
	} else {
		a.recordGeneratedImages(identity, urls, visibility)
		a.logCall(identity, summary, r.Method, endpoint, model, start, "success", http.StatusOK, "", urls)
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func protocolErrorHTTPStatus(err error) int {
	var httpErr protocol.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Status
	}
	var imageErr *protocol.ImageGenerationError
	if errors.As(err, &imageErr) {
		return imageErr.StatusCode
	}
	message := err.Error()
	if strings.Contains(strings.ToLower(message), "no available image quota") {
		return http.StatusTooManyRequests
	}
	return http.StatusBadGateway
}

func (a *App) writeProtocolError(w http.ResponseWriter, err error) {
	var httpErr protocol.HTTPError
	if errors.As(err, &httpErr) {
		util.WriteError(w, httpErr.Status, httpErr.Message)
		return
	}
	var imageErr *protocol.ImageGenerationError
	if errors.As(err, &imageErr) {
		util.WriteJSON(w, imageErr.StatusCode, imageErr.OpenAIError())
		return
	}
	message := err.Error()
	if strings.Contains(strings.ToLower(message), "no available image quota") {
		util.WriteJSON(w, http.StatusTooManyRequests, map[string]any{"error": map[string]any{"message": "no available image quota", "type": "insufficient_quota", "param": nil, "code": "insufficient_quota"}})
		return
	}
	util.WriteJSON(w, http.StatusBadGateway, map[string]any{"detail": map[string]any{"error": message}})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := a.requireCFTurnstile(r, loginBodyTurnstileToken(body)); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if key := util.Clean(body["key"]); key != "" {
		identity := a.auth.Authenticate(key)
		if identity == nil {
			util.WriteError(w, http.StatusBadRequest, "invalid key")
			return
		}
		setAuthSessionCookie(w, r, key)
		a.writeLoginResponse(w, *identity, key)
		return
	}
	identityInput := firstNonEmpty(util.Clean(body["username"]), util.Clean(body["email"]))
	password := util.Clean(body["password"])
	if identityInput == "" || password == "" {
		util.WriteError(w, http.StatusBadRequest, "username/email and password are required")
		return
	}
	identity, token, err := a.auth.LoginPassword(identityInput, password)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	setAuthSessionCookie(w, r, token)
	a.writeLoginResponse(w, *identity, token)
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	if token := requestBearerToken(r); token != "" {
		setAuthSessionCookie(w, r, token)
	}
	a.writeLoginResponse(w, identity, "")
}

func (a *App) handleAccountRegister(w http.ResponseWriter, r *http.Request) {
	if !a.config.RegistrationEnabled() {
		util.WriteError(w, http.StatusForbidden, "registration is disabled")
		return
	}
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := a.requireCFTurnstile(r, loginBodyTurnstileToken(body)); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := normalizeRegisterEmail(util.Clean(body["email"]))
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	inviteCode := firstNonEmpty(
		util.Clean(body["invite_code"]),
		util.Clean(body["invitation_code"]),
		util.Clean(body["referral_code"]),
	)
	registerIP := clientIP(r)
	registerDeviceID := firstNonEmpty(
		util.Clean(body["device_id"]),
		strings.TrimSpace(r.Header.Get("X-Device-ID")),
		strings.TrimSpace(r.Header.Get("X-Client-Device-ID")),
	)
	if err := a.billing.ValidateRegisterFingerprint(registerIP, registerDeviceID, 1, 1); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.billing.ValidateInviteCode(inviteCode); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !a.isRegistrationEmailAllowed(email) {
		util.WriteError(w, http.StatusBadRequest, "email domain is not allowed")
		return
	}
	if a.emailVerify == nil || !a.emailVerify.Enabled() {
		util.WriteError(w, http.StatusBadRequest, "email verification is not configured")
		return
	}
	if err := a.emailVerify.VerifyCode(email, util.Clean(body["code"])); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	identity, token, err := a.auth.RegisterPasswordEmailUser(email, util.Clean(body["password"]), util.Clean(body["name"]))
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	userID := strings.TrimSpace(identity.OwnerID)
	if userID == "" {
		userID = strings.TrimSpace(identity.ID)
	}
	a.billing.EnsureWalletUserWithEmail(userID, email, identity.Name, service.AuthProviderLocal)
	if err := a.billing.ApplyRegisterBonusForUser(userID, a.config.ImagePriceCents(), a.config.RegistrationBonusImageTimes()); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.billing.BindRegisterFingerprint(userID, registerIP, registerDeviceID); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.billing.ApplyInviteCodeForUser(userID, inviteCode, a.config.ImagePriceCents()); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	setAuthSessionCookie(w, r, token)
	a.writeLoginResponse(w, *identity, token)
}

func (a *App) handleRegisterSendCode(w http.ResponseWriter, r *http.Request) {
	if !a.config.RegistrationEnabled() {
		util.WriteError(w, http.StatusForbidden, "registration is disabled")
		return
	}
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := a.requireCFTurnstile(r, loginBodyTurnstileToken(body)); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := normalizeRegisterEmail(util.Clean(body["email"]))
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !a.isRegistrationEmailAllowed(email) {
		util.WriteError(w, http.StatusBadRequest, "email domain is not allowed")
		return
	}
	if a.emailVerify == nil {
		util.WriteError(w, http.StatusBadRequest, "email verification is not configured")
		return
	}
	if err := a.emailVerify.SendCode(email); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"expires_in": 600,
	})
}

func (a *App) handlePasswordResetSendCode(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := a.requireCFTurnstile(r, loginBodyTurnstileToken(body)); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := normalizeRegisterEmail(util.Clean(body["email"]))
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if a.emailVerify == nil || !a.emailVerify.Enabled() {
		util.WriteError(w, http.StatusBadRequest, "email verification is not configured")
		return
	}
	if !a.auth.HasPasswordEmailAccount(email) {
		util.WriteError(w, http.StatusBadRequest, "该邮箱未注册")
		return
	}
	if err := a.emailVerify.SendCode(email); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"expires_in": 600,
	})
}

func (a *App) handlePasswordReset(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := a.requireCFTurnstile(r, loginBodyTurnstileToken(body)); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	email, err := normalizeRegisterEmail(util.Clean(body["email"]))
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if a.emailVerify == nil || !a.emailVerify.Enabled() {
		util.WriteError(w, http.StatusBadRequest, "email verification is not configured")
		return
	}
	code := firstNonEmpty(util.Clean(body["code"]), util.Clean(body["verification_code"]))
	if err := a.emailVerify.VerifyCode(email, code); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.auth.ResetPasswordByEmail(email, util.Clean(body["password"])); err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	clearAuthSessionCookie(w, r)
	util.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) writeLoginResponse(w http.ResponseWriter, identity service.Identity, token string) {
	permissions := a.identityPermissions(identity)
	payload := map[string]any{
		"ok":              true,
		"version":         version.Get(),
		"token":           token,
		"role":            identity.Role,
		"role_id":         identity.RoleID,
		"role_name":       identity.RoleName,
		"subject_id":      identity.ID,
		"name":            identity.Name,
		"provider":        identity.Provider,
		"credential_id":   identity.CredentialID,
		"credential_name": identity.CredentialName,
		"menu_paths":      permissions.MenuPaths,
		"api_permissions": permissions.APIPermissions,
		"menus":           service.FilterMenuPermissions(permissions.MenuPaths),
	}
	if token == "" {
		delete(payload, "token")
	}
	util.WriteJSON(w, http.StatusOK, payload)
}

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		util.WriteJSON(w, http.StatusOK, map[string]any{"config": a.config.Get()})
	case http.MethodPost:
		body, err := readJSONMap(r)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		updated, err := a.config.Update(body)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.emailVerify = service.NewEmailVerificationService(a.config.EmailSMTP())
		a.update = newUpdateService(a.config)
		util.WriteJSON(w, http.StatusOK, map[string]any{"config": updated})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) handleAppMeta(w http.ResponseWriter, r *http.Request) {
	presetsJSON := strings.TrimSpace(fmt.Sprint(util.ValueOr(a.config.Get()["image_prompt_presets_json"], "")))
	util.WriteJSON(w, http.StatusOK, map[string]any{
		"app_title":                   a.config.BrandTopLeftName(),
		"project_name":                a.config.BrandSiteName(),
		"top_left_logo_url":           a.config.BrandTopLeftLogoURL(),
		"site_logo_url":               a.config.BrandSiteLogoURL(),
		"image_single_count_limit":    a.config.ImageSingleCountLimit(),
		"image_prompt_presets_json":   presetsJSON,
		"login_page_image_url":        a.config.LoginPageImageURL(),
		"login_page_image_mode":       a.config.LoginPageImageMode(),
		"login_page_image_zoom":       a.config.LoginPageImageZoom(),
		"login_page_image_position_x": a.config.LoginPageImagePositionX(),
		"login_page_image_position_y": a.config.LoginPageImagePositionY(),
	})
}

func (a *App) validateImageSingleCount(w http.ResponseWriter, count int) bool {
	limit := a.config.ImageSingleCountLimit()
	if count < 1 || count > limit {
		util.WriteError(w, http.StatusBadRequest, fmt.Sprintf("超出单次出图数量限制（最多 %d 张）", limit))
		return false
	}
	return true
}

func (a *App) handlePermissionCatalog(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	util.WriteJSON(w, http.StatusOK, a.auth.PermissionCatalog())
}

func (a *App) handleLoginPageImageSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(maxLoginPageImageSize + (1 << 20)); err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	currentImageURL := a.config.LoginPageImageURL()
	nextImageURL := strings.TrimSpace(r.FormValue("login_page_image_url"))
	uploadedImageURL := ""
	switch strings.ToLower(strings.TrimSpace(r.FormValue("login_page_image_action"))) {
	case "remove":
		nextImageURL = ""
	case "replace":
		fileHeader := firstMultipartFile(r.MultipartForm, "login_page_image_file")
		if fileHeader == nil {
			util.WriteError(w, http.StatusBadRequest, "login page image file is required")
			return
		}
		storedURL, err := a.storeLoginPageImage(fileHeader)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		nextImageURL = storedURL
		uploadedImageURL = storedURL
	}

	updated, err := a.config.Update(map[string]any{
		"login_page_image_url":        nextImageURL,
		"login_page_image_mode":       strings.TrimSpace(r.FormValue("login_page_image_mode")),
		"login_page_image_zoom":       strings.TrimSpace(r.FormValue("login_page_image_zoom")),
		"login_page_image_position_x": strings.TrimSpace(r.FormValue("login_page_image_position_x")),
		"login_page_image_position_y": strings.TrimSpace(r.FormValue("login_page_image_position_y")),
	})
	if err != nil {
		if uploadedImageURL != "" {
			a.deleteLocalLoginPageImage(uploadedImageURL)
		}
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if currentImageURL != "" && currentImageURL != nextImageURL {
		a.deleteLocalLoginPageImage(currentImageURL)
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"config": updated})
}

func (a *App) storeLoginPageImage(header *multipart.FileHeader) (string, error) {
	data, ext, err := readLoginPageImageFile(header)
	if err != nil {
		return "", err
	}
	stem := safeUploadStem(header.Filename)
	if stem == "" {
		stem = "login-page"
	}
	filename := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), stem, ext)
	target := filepath.Join(a.config.LoginPageImagesDir(), filename)
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", err
	}
	return "/login-page-images/" + filename, nil
}

func readLoginPageImageFile(header *multipart.FileHeader) ([]byte, string, error) {
	if header == nil {
		return nil, "", fmt.Errorf("image file is required")
	}
	if header.Size > maxLoginPageImageSize {
		return nil, "", fmt.Errorf("login page image cannot exceed 10MB")
	}
	file, err := header.Open()
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxLoginPageImageSize+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("image file is empty")
	}
	if len(data) > maxLoginPageImageSize {
		return nil, "", fmt.Errorf("login page image cannot exceed 10MB")
	}
	if ext := strings.ToLower(filepath.Ext(header.Filename)); ext == ".svg" && bytes.Contains(bytes.ToLower(data[:min(len(data), 512)]), []byte("<svg")) {
		return data, ".svg", nil
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		return nil, "", fmt.Errorf("unsupported image file")
	}
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return data, ".jpg", nil
	case "image/gif":
		return data, ".gif", nil
	case "image/webp":
		return data, ".webp", nil
	default:
		return data, ".png", nil
	}
}

func (a *App) deleteLocalLoginPageImage(imageURL string) {
	imagePath, ok := a.localLoginPageImagePath(imageURL)
	if ok {
		_ = os.Remove(imagePath)
	}
}

func (a *App) localLoginPageImagePath(imageURL string) (string, bool) {
	cleanURL := strings.TrimSpace(imageURL)
	if !strings.HasPrefix(cleanURL, "/login-page-images/") {
		return "", false
	}
	rel := strings.TrimPrefix(path.Clean(cleanURL), "/login-page-images/")
	if rel == "." || rel == "" || strings.Contains(rel, "..") {
		return "", false
	}
	root, err := filepath.Abs(a.config.LoginPageImagesDir())
	if err != nil {
		return "", false
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", false
	}
	return target, true
}

func firstMultipartFile(form *multipart.Form, key string) *multipart.FileHeader {
	if form == nil || len(form.File[key]) == 0 {
		return nil
	}
	return form.File[key][0]
}

func safeUploadStem(filename string) string {
	name := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	name = strings.ToLower(strings.TrimSpace(name))
	var builder strings.Builder
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '-' || char == '_':
			builder.WriteRune(char)
		case char == ' ' || char == '.':
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-_")
}

func (a *App) handleImages(w http.ResponseWriter, r *http.Request) {
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		scope, status, message := imageListAccessScope(identity, r.URL.Query().Get("scope"))
		if status != 0 {
			util.WriteError(w, status, message)
			return
		}
		query := r.URL.Query()
		startDate := strings.TrimSpace(query.Get("start_date"))
		endDate := strings.TrimSpace(query.Get("end_date"))
		ownerQuery := strings.TrimSpace(firstNonEmpty(query.Get("owner_query"), query.Get("owner"), query.Get("user")))
		page := util.ToInt(query.Get("page"), 0)
		pageSize := util.ToInt(query.Get("page_size"), 0)
		var payload map[string]any
		// owner_query 需要先整体过滤再分页，否则会出现“本页匹配不到但下一页有”的错位结果。
		if ownerQuery != "" {
			payload = a.images.ListImages(a.resolveImageBaseURL(r), startDate, endDate, scope)
		} else if page > 0 || pageSize > 0 {
			payload = a.images.ListImagesPage(a.resolveImageBaseURL(r), startDate, endDate, scope, page, pageSize)
		} else {
			payload = a.images.ListImages(a.resolveImageBaseURL(r), startDate, endDate, scope)
		}
		a.decorateImageList(payload)
		if ownerQuery != "" {
			filtered := filterImageItemsByOwnerQuery(util.AsMapSlice(payload["items"]), ownerQuery, a.imageOwnerSearchIndex())
			payload = paginateImageItemsPayload(filtered, page, pageSize)
		}
		util.WriteJSON(w, http.StatusOK, payload)
	case http.MethodDelete:
		body, err := readJSONMap(r)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		deleteScope := service.ImageAccessScope{OwnerID: identityScope(identity)}
		// 管理员与被授予自定义角色（非默认用户角色）的成员，允许执行全局删除。
		if identity.Role == service.AuthRoleAdmin || (identity.RoleID != "" && identity.RoleID != service.DefaultManagedRoleID) {
			deleteScope = service.ImageAccessScope{All: true}
		}
		result, err := a.images.DeleteImages(util.AsStringSlice(body["paths"]), deleteScope)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, result)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) handleImageVisibility(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	path := util.Clean(body["path"])
	if path == "" {
		util.WriteError(w, http.StatusBadRequest, "path is required")
		return
	}
	visibility := util.Clean(body["visibility"])
	scope := service.ImageAccessScope{OwnerID: identityScope(identity)}
	if identity.Role == service.AuthRoleAdmin {
		scope = service.ImageAccessScope{All: true}
	}
	item, err := a.images.UpdateImageVisibility(path, visibility, scope)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "image not found" {
			status = http.StatusNotFound
		}
		util.WriteError(w, status, err.Error())
		return
	}
	a.decorateImageItem(item, a.imageOwnerDisplayNames())
	util.WriteJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (a *App) handleImageR2Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	identity, ok := a.requireIdentity(w, r, "")
	if !ok {
		return
	}
	if identity.Role != service.AuthRoleAdmin {
		util.WriteError(w, http.StatusForbidden, "admin permission required")
		return
	}
	body, err := readJSONMap(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	result, err := a.images.UploadImagesToObjectStorage(r.Context(), util.AsStringSlice(body["paths"]), service.ImageAccessScope{All: true})
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, result)
}

func (a *App) handleImageFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rel, err := imageFileRequestPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ref, ok := a.authorizeImageFileRequest(w, r, rel)
	if !ok {
		return
	}
	http.ServeFile(w, r, ref.Path)
}

func (a *App) handleImageFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}

	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		util.WriteError(w, http.StatusBadRequest, "url is required")
		return
	}
	targetURL, err := url.Parse(rawURL)
	if err != nil || !targetURL.IsAbs() {
		util.WriteError(w, http.StatusBadRequest, "invalid url")
		return
	}
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		util.WriteError(w, http.StatusBadRequest, "unsupported url scheme")
		return
	}
	if !a.isAllowedImageFetchHost(targetURL, r.Host) {
		util.WriteError(w, http.StatusForbidden, "image host is not allowed")
		return
	}

	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, targetURL.String(), nil)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, "invalid url")
		return
	}
	request.Header.Set("User-Agent", "chatgpt2api-image-fetch/1.0")
	response, err := a.proxy.HTTPClient(30 * time.Second).Do(request)
	if err != nil {
		util.WriteError(w, http.StatusBadGateway, "failed to fetch image")
		return
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		util.WriteError(w, http.StatusBadGateway, fmt.Sprintf("upstream status %d", response.StatusCode))
		return
	}

	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := strings.TrimSpace(r.URL.Query().Get("name"))
	if filename == "" {
		filename = path.Base(strings.TrimSpace(targetURL.Path))
	}
	if filename == "." || filename == "/" || filename == "" {
		filename = "image"
	}
	filename = strings.ReplaceAll(filename, "\"", "")

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	if _, copyErr := io.Copy(w, io.LimitReader(response.Body, 64<<20)); copyErr != nil {
		return
	}
}

func (a *App) isAllowedImageFetchHost(targetURL *url.URL, requestHost string) bool {
	targetHost := normalizedHostname(targetURL.Host)
	if targetHost == "" {
		return false
	}

	allowedHosts := map[string]struct{}{}
	imageExternal := a.config.ImageExternalStorage()
	for _, candidate := range []string{
		requestHost,
		a.config.BaseURL(),
		a.config.ImageObjectStorage().PublicBaseURL,
		a.config.ImageSecondaryObjectStorage().PublicBaseURL,
		imageExternal.UploadURL,
	} {
		host := normalizedHostname(candidate)
		if host != "" {
			allowedHosts[host] = struct{}{}
		}
	}
	_, ok := allowedHosts[targetHost]
	return ok
}

func normalizedHostname(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		raw = parsed.Host
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		return strings.ToLower(strings.TrimSpace(host))
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func (a *App) authorizeImageFileRequest(w http.ResponseWriter, r *http.Request, rel string) (service.ImageFileAccess, bool) {
	ref, err := a.images.ImageFileAccess(rel, service.ImageAccessScope{All: true})
	if err != nil {
		http.NotFound(w, r)
		return service.ImageFileAccess{}, false
	}
	if ref.Visibility == service.ImageVisibilityPublic {
		return ref, true
	}
	identity, ok := a.imageRequestIdentity(w, r)
	if !ok {
		return service.ImageFileAccess{}, false
	}
	if identity.Role == service.AuthRoleAdmin || (ref.OwnerID != "" && ref.OwnerID == identityScope(identity)) {
		return ref, true
	}
	http.NotFound(w, r)
	return service.ImageFileAccess{}, false
}

func (a *App) handleImageThumbnail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	thumbnailRel, err := imageThumbnailRequestPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sourceRel, sourceErr := a.images.SourceImageRelativePathFromThumbnail(thumbnailRel)
	if sourceErr != nil {
		http.NotFound(w, r)
		return
	}
	if _, ok := a.authorizeImageFileRequest(w, r, sourceRel); !ok {
		return
	}
	_ = a.images.EnsureThumbnail(thumbnailRel)
	thumbPath := filepath.Join(a.config.ImageThumbnailsDir(), filepath.FromSlash(thumbnailRel))
	if info, err := os.Stat(thumbPath); err == nil && !info.IsDir() {
		w.Header().Set("Cache-Control", imageThumbnailCacheControl)
		http.ServeFile(w, r, thumbPath)
		return
	}
	sourcePath := filepath.Join(a.config.ImagesDir(), filepath.FromSlash(sourceRel))
	if info, err := os.Stat(sourcePath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, sourcePath)
		return
	}
	http.NotFound(w, r)
}

func imageFileRequestPath(r *http.Request) (string, error) {
	raw := strings.TrimPrefix(r.URL.EscapedPath(), "/images/")
	if raw == "" || raw == r.URL.EscapedPath() {
		return "", errors.New("invalid image path")
	}
	rel, err := url.PathUnescape(raw)
	if err != nil {
		return "", err
	}
	return rel, nil
}

func imageThumbnailRequestPath(r *http.Request) (string, error) {
	raw := strings.TrimPrefix(r.URL.EscapedPath(), "/image-thumbnails/")
	if raw == "" || raw == r.URL.EscapedPath() {
		return "", errors.New("invalid thumbnail path")
	}
	rel, err := url.PathUnescape(raw)
	if err != nil {
		return "", err
	}
	return rel, nil
}

func (a *App) handleLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	query, err := parseLogQuery(r)
	if err != nil {
		util.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	items := a.logs.Search(query)
	util.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items), "page_size": normalizedHTTPLogPageSize(query.Limit)})
}

func (a *App) handleLogGovernance(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		util.WriteJSON(w, http.StatusOK, map[string]any{"governance": a.logs.GovernanceSummary()})
	case http.MethodPost:
		body, err := readJSONMap(r)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		retentionDays := util.ToInt(body["retention_days"], a.config.LogRetentionDays())
		result, err := a.logs.CleanupOlderThan(retentionDays)
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{
			"cleanup":    result,
			"governance": a.logs.GovernanceSummary(),
		})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) handleStorageInfo(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	backend, err := a.config.StorageBackend()
	if err != nil {
		util.WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	util.WriteJSON(w, http.StatusOK, map[string]any{"backend": backend.Info(), "health": backend.HealthCheck()})
}

func (a *App) handleProxy(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireIdentity(w, r, ""); !ok {
		return
	}
	if r.URL.Path == "/api/proxy/test" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, _ := readJSONMap(r)
		candidate := strings.TrimSpace(util.Clean(body["url"]))
		if candidate == "" {
			candidate = a.config.Proxy()
		}
		if candidate == "" {
			util.WriteError(w, http.StatusBadRequest, "proxy url is required")
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"result": a.proxy.Test(candidate, 15*time.Second)})
		return
	}
	switch r.Method {
	case http.MethodGet:
		util.WriteJSON(w, http.StatusOK, map[string]any{"proxy": map[string]any{"url": a.config.Proxy()}})
	case http.MethodPost:
		body, _ := readJSONMap(r)
		url := util.Clean(body["url"])
		updated, err := a.config.Update(map[string]any{"proxy": url})
		if err != nil {
			util.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		util.WriteJSON(w, http.StatusOK, map[string]any{"proxy": map[string]any{"url": updated["proxy"]}})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *App) requireIdentity(w http.ResponseWriter, r *http.Request, overrideAuth string) (service.Identity, bool) {
	token := overrideAuthToken(overrideAuth, r)
	if identity := a.auth.Authenticate(token); identity != nil {
		if !a.identityCanAccessRequest(*identity, r) {
			util.WriteError(w, http.StatusForbidden, "permission denied")
			return service.Identity{}, false
		}
		*r = *r.WithContext(withRequestIdentity(r.Context(), *identity))
		return *identity, true
	}
	util.WriteError(w, http.StatusUnauthorized, "authorization is invalid")
	return service.Identity{}, false
}

func overrideAuthToken(overrideAuth string, r *http.Request) string {
	if overrideAuth != "" {
		return extractBearerToken(overrideAuth)
	}
	return requestAuthToken(r)
}

func requestAuthToken(r *http.Request) string {
	if token := requestBearerToken(r); token != "" {
		return token
	}
	return requestAuthCookieToken(r)
}

func requestBearerToken(r *http.Request) string {
	return extractBearerToken(r.Header.Get("Authorization"))
}

func requestAuthCookieToken(r *http.Request) string {
	cookie, err := r.Cookie(authSessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func (a *App) imageRequestIdentity(w http.ResponseWriter, r *http.Request) (service.Identity, bool) {
	token := requestAuthToken(r)
	if token == "" {
		util.WriteError(w, http.StatusUnauthorized, "authorization is invalid")
		return service.Identity{}, false
	}
	if identity := a.auth.Authenticate(token); identity != nil {
		return *identity, true
	}
	util.WriteError(w, http.StatusUnauthorized, "authorization is invalid")
	return service.Identity{}, false
}

func (a *App) identityPermissions(identity service.Identity) service.PermissionSet {
	if identity.Role == service.AuthRoleAdmin {
		return service.DefaultPermissionSetForRole(service.AuthRoleAdmin)
	}
	return service.PermissionSet{
		MenuPaths:      service.NormalizeMenuPermissions(identity.MenuPaths),
		APIPermissions: service.NormalizeAPIPermissions(identity.APIPermissions),
	}
}

func (a *App) identityCanAccessRequest(identity service.Identity, r *http.Request) bool {
	if identity.Role == service.AuthRoleAdmin || isPermissionCheckSkipped(r.URL.Path) {
		return true
	}
	return a.identityCanAccessAPI(identity, r.Method, r.URL.Path)
}

func (a *App) identityCanAccessAPI(identity service.Identity, method, path string) bool {
	if identity.Role == service.AuthRoleAdmin {
		return true
	}
	return service.HasAPIPermission(a.identityPermissions(identity), method, path)
}

func isPermissionCheckSkipped(path string) bool {
	switch path {
	case "/auth/login":
		return true
	case "/auth/logout":
		return true
	case "/auth/register":
		return true
	case "/auth/password/send-code":
		return true
	case "/auth/password/reset":
		return true
	case "/auth/session":
		return true
	case "/api/profile":
		return true
	case "/api/profile/password":
		return true
	case "/api/profile/api-key":
		return true
	case "/api/profile/prompt-favorites":
		return true
	case "/api/agency/withdrawals":
		return true
	case "/api/agency/withdraw-profile":
		return true
	case "/api/agency/withdraw-profile/upload":
		return true
	default:
		return strings.HasPrefix(path, "/api/profile/api-key/") || strings.HasPrefix(path, "/api/profile/prompt-favorites/")
	}
}

func extractBearerToken(auth string) string {
	scheme, value, ok := strings.Cut(strings.TrimSpace(auth), " ")
	if !ok || strings.ToLower(scheme) != "bearer" {
		return ""
	}
	return strings.TrimSpace(value)
}

func setAuthSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authSessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   isHTTPSRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     authSessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPSRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) resolveImageBaseURL(r *http.Request) string {
	if base := a.config.BaseURL(); base != "" {
		return base
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("x-forwarded-proto"); forwarded != "" {
		scheme = strings.Split(forwarded, ",")[0]
	}
	host := r.Host
	if value := r.Header.Get("host"); value != "" {
		host = value
	}
	return scheme + "://" + host
}

func readJSONMap(r *http.Request) (map[string]any, error) {
	var body map[string]any
	err := util.DecodeJSON(r.Body, &body)
	if body == nil {
		body = map[string]any{}
	}
	return body, err
}

func readMultipartImageBody(r *http.Request) (map[string]any, []protocol.UploadedImage, error) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		return nil, nil, err
	}
	body := map[string]any{
		"client_task_id":     firstForm(r.MultipartForm, "client_task_id"),
		"prompt":             firstForm(r.MultipartForm, "prompt"),
		"model":              firstNonEmpty(firstForm(r.MultipartForm, "model"), util.ImageModelAuto),
		"n":                  util.ToInt(firstForm(r.MultipartForm, "n"), 1),
		"size":               firstForm(r.MultipartForm, "size"),
		"quality":            firstForm(r.MultipartForm, "quality"),
		"output_format":      firstForm(r.MultipartForm, "output_format"),
		"output_compression": firstForm(r.MultipartForm, "output_compression"),
		"visibility":         firstForm(r.MultipartForm, "visibility"),
		"response_format":    firstNonEmpty(firstForm(r.MultipartForm, "response_format"), "b64_json"),
		"stream":             util.ToBool(firstForm(r.MultipartForm, "stream")),
	}
	if rawMessages := strings.TrimSpace(firstForm(r.MultipartForm, "messages")); rawMessages != "" {
		var messages any
		if err := json.Unmarshal([]byte(rawMessages), &messages); err != nil {
			return nil, nil, fmt.Errorf("invalid messages")
		}
		body["messages"] = messages
	}
	var images []protocol.UploadedImage
	for _, field := range []string{"image", "image[]"} {
		for _, header := range r.MultipartForm.File[field] {
			image, err := readUpload(header)
			if err != nil {
				return nil, nil, err
			}
			if len(image.Data) == 0 {
				return nil, nil, fmt.Errorf("image file is empty")
			}
			images = append(images, image)
		}
	}
	return body, images, nil
}

func firstForm(form *multipart.Form, key string) string {
	if form == nil || len(form.Value[key]) == 0 {
		return ""
	}
	return form.Value[key][0]
}

func readUpload(header *multipart.FileHeader) (protocol.UploadedImage, error) {
	file, err := header.Open()
	if err != nil {
		return protocol.UploadedImage{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return protocol.UploadedImage{}, err
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/png"
	}
	filename := header.Filename
	if filename == "" {
		filename = "image.png"
	}
	return protocol.UploadedImage{Data: data, Filename: filename, ContentType: contentType}, nil
}

func jsonString(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func openAIErrorForStream(err error) map[string]any {
	var imageErr *protocol.ImageGenerationError
	if errors.As(err, &imageErr) {
		return imageErr.OpenAIError()
	}
	return map[string]any{"error": map[string]any{"message": err.Error(), "type": fmt.Sprintf("%T", err)}}
}

func (a *App) logCall(identity service.Identity, summary, method, endpoint, model string, started time.Time, outcome string, status int, errText string, urls []string) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if status <= 0 {
		status = http.StatusOK
		if outcome == "failed" {
			status = http.StatusInternalServerError
		}
	}
	ended := time.Now()
	detail := map[string]any{
		"method":         method,
		"path":           endpoint,
		"endpoint":       endpoint,
		"module":         inferAuditModule(endpoint),
		"model":          model,
		"started_at":     started.Format("2006-01-02 15:04:05"),
		"ended_at":       ended.Format("2006-01-02 15:04:05"),
		"duration_ms":    ended.Sub(started).Milliseconds(),
		"status":         status,
		"outcome":        outcome,
		"operation_type": operationTypeForMethod(method),
		"log_level":      logLevelForStatus(status),
	}
	addIdentityLogDetail(detail, identity)
	if name := identityDisplayName(identity); name != "" {
		detail["username"] = name
	}
	if errText != "" {
		detail["error"] = errText
	}
	if len(urls) > 0 {
		detail["urls"] = dedupe(urls)
	}
	suffix := "璋冪敤瀹屾垚"
	if outcome == "failed" {
		suffix = "璋冪敤澶辫触"
	}
	a.logs.Add(summary+suffix, detail)
}

func addIdentityLogDetail(detail map[string]any, identity service.Identity) {
	if name := util.Clean(firstNonEmpty(identity.CredentialName, identity.Name)); name != "" {
		detail["key_name"] = name
	}
	if role := util.Clean(identity.Role); role != "" {
		detail["key_role"] = role
	}
	if id := util.Clean(firstNonEmpty(identity.CredentialID, identity.ID)); id != "" {
		detail["key_id"] = id
	}
	if id := util.Clean(identity.ID); id != "" && id != util.Clean(identity.CredentialID) {
		detail["subject_id"] = id
	}
	if provider := util.Clean(identity.Provider); provider != "" {
		detail["provider"] = provider
	}
}

func identityScope(identity service.Identity) string {
	if owner := util.Clean(identity.OwnerID); owner != "" {
		return owner
	}
	if id := util.Clean(identity.ID); id != "" {
		return id
	}
	return "anonymous"
}

func identityDisplayName(identity service.Identity) string {
	return firstNonEmpty(util.Clean(identity.Name), util.Clean(identity.CredentialName))
}

func imageAccessScope(identity service.Identity) service.ImageAccessScope {
	if identity.Role == service.AuthRoleAdmin {
		return service.ImageAccessScope{All: true}
	}
	return service.ImageAccessScope{OwnerID: identityScope(identity)}
}

func imageListAccessScope(identity service.Identity, value string) (service.ImageAccessScope, int, string) {
	switch strings.TrimSpace(value) {
	case "":
		return imageAccessScope(identity), 0, ""
	case "mine":
		return service.ImageAccessScope{OwnerID: identityScope(identity)}, 0, ""
	case "public":
		if identity.Role == service.AuthRoleAdmin {
			return service.ImageAccessScope{All: true}, 0, ""
		}
		return service.ImageAccessScope{Public: true}, 0, ""
	case "all":
		if identity.Role != service.AuthRoleAdmin {
			return service.ImageAccessScope{}, http.StatusForbidden, "admin permission required"
		}
		return service.ImageAccessScope{All: true}, 0, ""
	default:
		return service.ImageAccessScope{}, http.StatusBadRequest, "scope must be mine, public, or all"
	}
}

func (a *App) recordGeneratedImages(identity service.Identity, urls []string, visibility string) {
	if len(urls) == 0 || a.images == nil {
		return
	}
	ownerID := identityScope(identity)
	a.images.RecordGeneratedImages(urls, ownerID, identityDisplayName(identity), visibility)
}

func (a *App) recordGeneratedImagesForPayload(identity service.Identity, urls []string, visibility string, payload map[string]any) {
	if len(urls) == 0 || a.images == nil {
		return
	}
	ownerID := identityScope(identity)
	a.images.RecordGeneratedImages(urls, ownerID, identityDisplayName(identity), visibility, service.GeneratedImageMetadata{
		ResolutionPreset: util.Clean(payload["image_resolution"]),
		RequestedSize:    util.Clean(payload["size"]),
		OutputFormat:     service.NormalizeImageOutputFormat(util.Clean(payload["output_format"])),
	})
}

func (a *App) decorateImageList(payload map[string]any) {
	ownerNames := a.imageOwnerDisplayNames()
	ownerProfiles := a.imageOwnerProfiles()
	for _, item := range util.AsMapSlice(payload["items"]) {
		a.decorateImageItem(item, ownerNames)
		a.decorateImageOwnerIdentity(item, ownerProfiles)
	}
}

func filterImageItemsByOwnerQuery(items []map[string]any, rawQuery string, ownerSearch map[string]string) []map[string]any {
	query := strings.ToLower(strings.TrimSpace(rawQuery))
	if query == "" {
		return items
	}
	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		ownerID := util.Clean(item["owner_id"])
		ownerName := util.Clean(item["owner_name"])
		searchText := strings.ToLower(strings.Join([]string{
			ownerID,
			ownerName,
			ownerSearch[ownerID],
		}, " "))
		if strings.Contains(searchText, query) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func paginateImageItemsPayload(items []map[string]any, page, pageSize int) map[string]any {
	if page <= 0 && pageSize <= 0 {
		groupMap := map[string][]map[string]any{}
		order := make([]string, 0)
		for _, item := range items {
			day := util.Clean(item["date"])
			if _, ok := groupMap[day]; !ok {
				order = append(order, day)
			}
			groupMap[day] = append(groupMap[day], item)
		}
		groups := make([]map[string]any, 0, len(order))
		for _, day := range order {
			groups = append(groups, map[string]any{"date": day, "items": groupMap[day]})
		}
		return map[string]any{"items": items, "groups": groups}
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}
	if page <= 0 {
		page = 1
	}
	total := len(items)
	totalPages := 1
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	pagedItems := items[start:end]
	groupMap := map[string][]map[string]any{}
	order := make([]string, 0)
	for _, item := range pagedItems {
		day := util.Clean(item["date"])
		if _, ok := groupMap[day]; !ok {
			order = append(order, day)
		}
		groupMap[day] = append(groupMap[day], item)
	}
	groups := make([]map[string]any, 0, len(order))
	for _, day := range order {
		groups = append(groups, map[string]any{"date": day, "items": groupMap[day]})
	}
	return map[string]any{
		"items":      pagedItems,
		"groups":     groups,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
		"total_page": totalPages,
	}
}

func (a *App) decorateImageItem(item map[string]any, ownerNames map[string]string) {
	if item == nil || util.Clean(item["owner_name"]) != "" {
		return
	}
	ownerID := util.Clean(item["owner_id"])
	if ownerID == "" {
		item["owner_name"] = "鏈煡鐢ㄦ埛"
		return
	}
	if name := ownerNames[ownerID]; name != "" {
		item["owner_name"] = name
		return
	}
	item["owner_name"] = "鏈煡鐢ㄦ埛"
}

func (a *App) imageOwnerDisplayNames() map[string]string {
	names := map[string]string{"admin": "管理员"}
	for _, item := range a.auth.ListUsers() {
		name := util.Clean(item["name"])
		if name == "" {
			continue
		}
		if id := util.Clean(item["id"]); id != "" {
			names[id] = name
		}
		if ownerID := util.Clean(item["owner_id"]); ownerID != "" {
			names[ownerID] = name
		}
	}
	return names
}

type imageOwnerProfile struct {
	Name     string
	Username string
	Email    string
}

func (a *App) decorateImageOwnerIdentity(item map[string]any, ownerProfiles map[string]imageOwnerProfile) {
	if item == nil {
		return
	}
	ownerID := util.Clean(item["owner_id"])
	profile := ownerProfiles[ownerID]
	ownerName := util.Clean(item["owner_name"])
	if ownerName == "" {
		ownerName = profile.Name
	}
	if profile.Email != "" {
		item["owner_email"] = profile.Email
	}
	if profile.Username != "" {
		item["owner_username"] = profile.Username
	}
	if display := imageOwnerDisplayLabel(ownerName, profile.Email, profile.Username, ownerID); display != "" {
		item["owner_display"] = display
	}
}

func imageOwnerDisplayLabel(name, email, username, ownerID string) string {
	name = strings.TrimSpace(name)
	email = managedUserEmailCandidate(email)
	username = strings.TrimSpace(username)
	ownerID = strings.TrimSpace(ownerID)
	if name != "" && email != "" && !strings.EqualFold(name, email) {
		return name + "（" + email + "）"
	}
	if name != "" && username != "" && !strings.EqualFold(name, username) {
		return name + "（" + username + "）"
	}
	if name != "" {
		return name
	}
	if email != "" {
		return email
	}
	if username != "" {
		return username
	}
	return ownerID
}

func (a *App) imageOwnerProfiles() map[string]imageOwnerProfile {
	profiles := map[string]imageOwnerProfile{"admin": {Name: "管理员", Username: "admin"}}
	for _, item := range a.auth.ListUsers() {
		name := util.Clean(item["name"])
		username := util.Clean(item["username"])
		email := managedUserEmailCandidate(util.Clean(item["email"]), username)
		profile := imageOwnerProfile{Name: firstNonEmpty(name, username, email), Username: username, Email: email}
		if id := util.Clean(item["id"]); id != "" {
			profiles[id] = profile
		}
		if ownerID := util.Clean(item["owner_id"]); ownerID != "" {
			profiles[ownerID] = profile
		}
	}
	return profiles
}

func (a *App) imageOwnerSearchIndex() map[string]string {
	index := map[string]string{"admin": "admin 管理员"}
	for _, item := range a.auth.ListUsers() {
		userID := util.Clean(item["id"])
		ownerID := util.Clean(item["owner_id"])
		name := util.Clean(item["name"])
		username := util.Clean(item["username"])
		email := util.Clean(item["email"])
		joined := strings.ToLower(strings.TrimSpace(strings.Join([]string{name, username, email}, " ")))
		if userID != "" && joined != "" {
			index[userID] = firstNonEmpty(index[userID], joined)
		}
		if ownerID != "" && joined != "" {
			index[ownerID] = firstNonEmpty(index[ownerID], joined)
		}
	}
	return index
}

func (a *App) runLoggedImageTask(ctx context.Context, identity service.Identity, payload map[string]any, endpoint, summary string, run func(context.Context, map[string]any) (map[string]any, error)) (map[string]any, error) {
	start := time.Now()
	payload["owner_id"] = identityScope(identity)
	payload["owner_name"] = identityDisplayName(identity)
	model := firstNonEmpty(util.Clean(payload["model"]), util.ImageModelAuto)
	result, err := run(ctx, payload)
	urls := collectURLs(result)
	a.recordGeneratedImagesForPayload(identity, urls, util.Clean(payload["visibility"]), payload)
	if err != nil {
		a.logCall(identity, summary, http.MethodPost, endpoint, model, start, "failed", protocolErrorHTTPStatus(err), err.Error(), urls)
		return result, err
	}
	if len(util.AsMapSlice(result["data"])) == 0 {
		message := firstNonEmpty(util.Clean(result["message"]), "image task returned no image data")
		a.logCall(identity, summary, http.MethodPost, endpoint, model, start, "failed", http.StatusBadGateway, message, urls)
		return result, nil
	}
	if chargeErr := a.chargeImageUsage(identity, endpoint, payload); chargeErr != nil {
		a.logCall(identity, summary, http.MethodPost, endpoint, model, start, "failed", http.StatusPaymentRequired, chargeErr.Error(), urls)
		return result, chargeErr
	}
	a.logCall(identity, summary, http.MethodPost, endpoint, model, start, "success", http.StatusOK, "", urls)
	return result, nil
}

func (a *App) runResponsesImageGenerationTask(ctx context.Context, payload map[string]any) (map[string]any, error) {
	body := responseImageTaskBody(payload)
	completed, _, err := a.engine.HandleResponsesScoped(ctx, body, util.Clean(payload["owner_id"]))
	if err != nil {
		if completed == nil {
			completed = map[string]any{}
		}
		return completed, err
	}
	result := responsesImageTaskResult(a.engine, completed, payload)
	if err := responsesImageTaskTextOutputError(result, completed); err != nil {
		return result, err
	}
	return result, nil
}

func responsesImageTaskTextOutputError(result map[string]any, completed map[string]any) error {
	if len(util.AsMapSlice(result["data"])) > 0 {
		return nil
	}
	text := responseOutputText(completed["output"])
	if text == "" {
		return nil
	}
	result["message"] = text
	result["output_type"] = "text"
	return &protocol.ImageGenerationError{
		Message:    firstNonEmpty(text, "Responses image_generation returned text instead of image data."),
		StatusCode: http.StatusBadGateway,
		Type:       "server_error",
		Code:       "image_generation_text_response",
	}
}

func responseImageTaskBody(payload map[string]any) map[string]any {
	prompt := util.Clean(payload["prompt"])
	images := responseImageTaskDataURLs(payload["images"])
	input := any(prompt)
	if len(images) > 0 {
		content := []map[string]any{{"type": "input_text", "text": prompt}}
		for _, imageURL := range images {
			content = append(content, map[string]any{"type": "input_image", "image_url": imageURL})
		}
		input = []map[string]any{{"role": "user", "content": content}}
	}
	tool := map[string]any{
		"type":          "image_generation",
		"action":        responseImageTaskAction(images),
		"size":          firstNonEmpty(util.Clean(payload["size"]), "auto"),
		"output_format": service.NormalizeImageOutputFormat(util.Clean(payload["output_format"])),
	}
	if resolution := firstNonEmpty(util.Clean(payload["resolution"]), util.Clean(payload["image_resolution"])); resolution != "" {
		tool["resolution"] = resolution
	}
	if util.Clean(tool["output_format"]) != "png" {
		if compression, ok := imageOutputCompressionFromBody(payload["output_compression"]); ok {
			tool["output_compression"] = compression
		}
	}
	if quality := util.Clean(payload["quality"]); quality != "" && util.Clean(payload["model"]) != util.ImageModelCodex {
		tool["quality"] = quality
	}
	body := map[string]any{
		"model":           firstNonEmpty(util.Clean(payload["model"]), util.ImageModelAuto),
		"input":           input,
		"tools":           []map[string]any{tool},
		"tool_choice":     "required",
		"n":               util.ToInt(payload["n"], 1),
		"owner_name":      util.Clean(payload["owner_name"]),
		"response_format": "b64_json",
	}
	if messages := util.AsMapSlice(payload["messages"]); len(messages) > 0 {
		body["instructions"] = responseImageTaskInstructions(messages, prompt)
	}
	return body
}

func responseImageTaskAction(images []string) string {
	if len(images) > 0 {
		return "edit"
	}
	return "generate"
}

func normalizeImageRequestPayload(body map[string]any) {
	if body == nil {
		return
	}
	resolution := protocol.NormalizeImageResolutionTier(firstNonEmpty(util.Clean(body["resolution"]), util.Clean(body["image_resolution"])))
	if resolution != "" {
		body["resolution"] = resolution
		body["image_resolution"] = resolution
	}
	body["size"] = protocol.ResolveImageSizeWithResolution(util.Clean(body["size"]), resolution)
}

func asyncImageTaskResponse(task map[string]any) map[string]any {
	taskID := util.Clean(task["id"])
	return map[string]any{
		"object":   "image.task",
		"task_id":  taskID,
		"id":       taskID,
		"status":   util.Clean(task["status"]),
		"created":  time.Now().Unix(),
		"endpoint": "/api/creation-tasks?ids=" + taskID,
		"task":     task,
	}
}

func responseImageTaskDataURLs(raw any) []string {
	var out []string
	for _, item := range util.AsStringSlice(raw) {
		text := strings.TrimSpace(util.Clean(item))
		lower := strings.ToLower(text)
		if strings.HasPrefix(lower, "data:image/") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			out = append(out, text)
		}
	}
	if len(out) > 16 {
		out = out[:16]
	}
	return out
}

func responseImageTaskInstructions(messages []map[string]any, prompt string) string {
	var history []string
	for index, message := range messages {
		if index == len(messages)-1 && strings.TrimSpace(util.Clean(message["content"])) == strings.TrimSpace(prompt) {
			continue
		}
		text := strings.TrimSpace(util.Clean(message["content"]))
		if text == "" {
			continue
		}
		history = append(history, firstNonEmpty(util.Clean(message["role"]), "user")+": "+text)
	}
	if len(history) == 0 {
		return ""
	}
	return "Use this conversation history only as context for image generation. Do not render the history text unless the current request explicitly asks for it.\n\n" + strings.Join(history, "\n")
}

func responsesImageTaskResult(engine *protocol.Engine, completed map[string]any, payload map[string]any) map[string]any {
	created := int64(util.ToInt(completed["created_at"], int(time.Now().Unix())))
	items := responseImageOutputItems(completed["output"])
	return engine.FormatImageResultWithOptions(items, util.Clean(payload["prompt"]), util.Clean(payload["response_format"]), util.Clean(payload["base_url"]), util.Clean(payload["owner_id"]), util.Clean(payload["owner_name"]), created, "", protocol.ImageOutputOptionsFromPayload(payload))
}

func responseImageOutputItems(output any) []map[string]any {
	var items []map[string]any
	for _, item := range util.AsMapSlice(output) {
		if util.Clean(item["type"]) != "image_generation_call" {
			continue
		}
		b64 := util.Clean(item["result"])
		if b64 == "" {
			continue
		}
		items = append(items, map[string]any{"b64_json": b64, "revised_prompt": util.Clean(item["revised_prompt"])})
	}
	return items
}

func responseOutputText(output any) string {
	var parts []string
	for _, item := range util.AsMapSlice(output) {
		if util.Clean(item["type"]) != "message" {
			continue
		}
		for _, content := range util.AsMapSlice(item["content"]) {
			if util.Clean(content["type"]) == "output_text" {
				if text := strings.TrimSpace(util.Clean(content["text"])); text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func (a *App) runLoggedChatTask(ctx context.Context, identity service.Identity, payload map[string]any) (map[string]any, error) {
	start := time.Now()
	payload["owner_id"] = identityScope(identity)
	payload["owner_name"] = identityDisplayName(identity)
	payload["stream"] = false
	model := firstNonEmpty(util.Clean(payload["model"]), util.ImageModelAuto)
	result, stream, err := a.engine.HandleChatCompletions(ctx, payload)
	if stream != nil {
		err = errors.New("chat task streaming is not supported")
	}
	if err != nil {
		a.logCall(identity, "鏂囨湰鐢熸垚", http.MethodPost, "/api/creation-tasks/chat-completions", model, start, "failed", protocolErrorHTTPStatus(err), err.Error(), nil)
		return result, err
	}
	text := chatCompletionResultText(result)
	if text == "" {
		err = errors.New("妯″瀷娌℃湁杩斿洖鏂囨湰鍐呭")
		a.logCall(identity, "鏂囨湰鐢熸垚", http.MethodPost, "/api/creation-tasks/chat-completions", model, start, "failed", http.StatusBadGateway, err.Error(), nil)
		return result, err
	}
	a.logCall(identity, "鏂囨湰鐢熸垚", http.MethodPost, "/api/creation-tasks/chat-completions", model, start, "success", http.StatusOK, "", nil)
	return map[string]any{
		"created":     result["created"],
		"output_type": "text",
		"data":        []map[string]any{{"text_response": text}},
	}, nil
}

func chatCompletionResultText(result map[string]any) string {
	for _, choice := range util.AsMapSlice(result["choices"]) {
		message := util.StringMap(choice["message"])
		if text := chatCompletionContentText(message["content"]); text != "" {
			return text
		}
	}
	return ""
}

func chatCompletionContentText(content any) string {
	if text, ok := content.(string); ok {
		return strings.TrimSpace(text)
	}
	var parts []string
	for _, item := range anyList(content) {
		block := util.StringMap(item)
		if text := util.Clean(block["text"]); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func collectURLs(v any) []string {
	switch x := v.(type) {
	case map[string]any:
		var urls []string
		for key, value := range x {
			if key == "url" || key == "local_url" || key == "r2_url" {
				if u := util.Clean(value); u != "" {
					urls = append(urls, u)
				}
			} else if key == "urls" {
				for _, raw := range anyList(value) {
					if u := util.Clean(raw); u != "" {
						urls = append(urls, u)
					}
				}
			} else {
				urls = append(urls, collectURLs(value)...)
			}
		}
		return urls
	case []any:
		var urls []string
		for _, item := range x {
			urls = append(urls, collectURLs(item)...)
		}
		return urls
	case []map[string]any:
		var urls []string
		for _, item := range x {
			urls = append(urls, collectURLs(item)...)
		}
		return urls
	default:
		return nil
	}
}

func dedupe(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func anyList(v any) []any {
	if list, ok := v.([]any); ok {
		return list
	}
	if list, ok := v.([]map[string]any); ok {
		out := make([]any, len(list))
		for i, item := range list {
			out[i] = item
		}
		return out
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (a *App) serveWeb(w http.ResponseWriter, r *http.Request) {
	if isWebIndexRequest(r.URL.Path) && a.serveBrandedWebIndex(w) {
		return
	}
	frontend.Handler().ServeHTTP(w, r)
}

func (a *App) serveBrandedWebIndex(w http.ResponseWriter) bool {
	indexHTML, err := frontend.IndexHTML()
	if err != nil {
		return false
	}
	body := rewriteWebIndexBrand(string(indexHTML), a.config.BrandSiteName(), a.config.BrandSiteLogoURL())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(body))
	return true
}

func isWebIndexRequest(rawPath string) bool {
	clean := strings.Trim(strings.TrimPrefix(path.Clean("/"+rawPath), "/"), "/")
	if clean == "" {
		return true
	}
	last := path.Base(clean)
	return !strings.HasPrefix(clean, "assets/") && !strings.Contains(last, ".")
}

func rewriteWebIndexBrand(indexHTML, siteName, iconURL string) string {
	escapedSiteName := html.EscapeString(firstNonEmpty(strings.TrimSpace(siteName), "GPT生图站"))
	escapedIconURL := html.EscapeString(firstNonEmpty(strings.TrimSpace(iconURL), "/logo-mark.svg"))
	result := replaceHTMLTagContent(indexHTML, "<title>", "</title>", escapedSiteName)
	replacement := `<link rel="icon" href="` + escapedIconURL + `" />`
	result = strings.Replace(result, `<link rel="icon" href="/favicon.ico" />`, replacement, 1)
	result = strings.Replace(result, `<link rel="icon" href="/logo-mark.svg" type="image/svg+xml" />`, replacement, 1)
	if script := initialAppMetaScript(siteName, iconURL); script != "" {
		result = strings.Replace(result, "</head>", script+"</head>", 1)
	}
	return result
}

func initialAppMetaScript(siteName, iconURL string) string {
	payload, err := json.Marshal(map[string]string{
		"app_title":         firstNonEmpty(strings.TrimSpace(siteName), "GPT生图站"),
		"project_name":      firstNonEmpty(strings.TrimSpace(siteName), "GPT生图站"),
		"top_left_logo_url": firstNonEmpty(strings.TrimSpace(iconURL), "/logo-mark.svg"),
		"site_logo_url":     firstNonEmpty(strings.TrimSpace(iconURL), "/logo-mark.svg"),
	})
	if err != nil {
		return ""
	}
	return "<script>window.__APP_META__=" + string(payload) + ";</script>"
}

func replaceHTMLTagContent(src, startTag, endTag, replacement string) string {
	start := strings.Index(src, startTag)
	if start < 0 {
		return src
	}
	contentStart := start + len(startTag)
	end := strings.Index(src[contentStart:], endTag)
	if end < 0 {
		return src
	}
	contentEnd := contentStart + end
	return src[:contentStart] + replacement + src[contentEnd:]
}
