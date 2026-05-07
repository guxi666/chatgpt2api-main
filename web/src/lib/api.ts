import { httpRequest } from "@/lib/request";
import type { LoginPageImageMode } from "@/lib/login-page-image-layout";

export type AccountType = "Free" | "Plus" | "ProLite" | "Pro" | "Team";
export type AccountStatus = "正常" | "限流" | "异常" | "禁用";
export const IMAGE_MODEL_OPTIONS = [
  { value: "auto", label: "Auto" },
  { value: "gpt-image-2", label: "gpt-image-2" },
  { value: "codex-gpt-image-2", label: "codex-gpt-image-2" },
  { value: "gpt-5-mini", label: "gpt-5-mini" },
  { value: "gpt-5-3-mini", label: "gpt-5-3-mini" },
  { value: "gpt-5", label: "gpt-5" },
  { value: "gpt-5-1", label: "gpt-5-1" },
  { value: "gpt-5-2", label: "gpt-5-2" },
  { value: "gpt-5-3", label: "gpt-5-3" },
  { value: "gpt-5.4", label: "gpt-5.4" },
  { value: "gpt-5.5", label: "gpt-5.5" },
] as const;
export type ImageModel = (typeof IMAGE_MODEL_OPTIONS)[number]["value"];
export const DEFAULT_IMAGE_MODEL: ImageModel = "auto";
export const DEFAULT_CHAT_MODEL: ImageModel = "auto";
const IMAGE_MODEL_VALUES = new Set<string>(IMAGE_MODEL_OPTIONS.map((option) => option.value));
const IMAGE_TASK_MODEL_VALUES = new Set<ImageModel>(["auto", "gpt-image-2", "codex-gpt-image-2"]);
const CHAT_MODEL_VALUES = new Set<ImageModel>([
  "auto",
  "gpt-5-mini",
  "gpt-5-3-mini",
  "gpt-5",
  "gpt-5-1",
  "gpt-5-2",
  "gpt-5-3",
  "gpt-5.4",
  "gpt-5.5",
]);
export const IMAGE_TASK_MODEL_OPTIONS = IMAGE_MODEL_OPTIONS.filter((option) => IMAGE_TASK_MODEL_VALUES.has(option.value));
export const CHAT_MODEL_OPTIONS = IMAGE_MODEL_OPTIONS.filter((option) => CHAT_MODEL_VALUES.has(option.value));

export function isImageModel(value: unknown): value is ImageModel {
  return typeof value === "string" && IMAGE_MODEL_VALUES.has(value);
}

export function isImageTaskModel(value: unknown): value is ImageModel {
  return isImageModel(value) && IMAGE_TASK_MODEL_VALUES.has(value);
}

export function isChatModel(value: unknown): value is ImageModel {
  return isImageModel(value) && CHAT_MODEL_VALUES.has(value);
}

export type ImageQuality = "low" | "medium" | "high";

const IMAGE_QUALITY_VALUES = new Set<string>(["low", "medium", "high"]);

export function isImageQuality(value: unknown): value is ImageQuality {
  return typeof value === "string" && IMAGE_QUALITY_VALUES.has(value);
}

export type AuthRole = "admin" | "user";
export type AnnouncementTarget = "login" | "image";

export type Account = {
  id: string;
  access_token: string;
  type: AccountType;
  status: AccountStatus;
  quota: number;
  imageQuotaUnknown?: boolean;
  email?: string | null;
  user_id?: string | null;
  limits_progress?: Array<{
    feature_name?: string;
    remaining?: number;
    reset_after?: string;
  }>;
  default_model_slug?: string | null;
  restoreAt?: string | null;
  success: number;
  fail: number;
  lastUsedAt: string | null;
};

type AccountListResponse = {
  items: Account[];
};

type AccountMutationResponse = {
  items: Account[];
  added?: number;
  skipped?: number;
  removed?: number;
  refreshed?: number;
  errors?: Array<{ access_token: string; error: string }>;
};

type AccountRefreshResponse = {
  items: Account[];
  refreshed: number;
  errors: Array<{ access_token: string; error: string }>;
};

type AccountUpdateResponse = {
  item: Account;
  items: Account[];
};

export type SettingsConfig = {
  app_title?: string;
  project_name?: string;
  app_logo_url?: string;
  site_icon_url?: string;
  proxy: string;
  base_url?: string;
  refresh_account_interval_minute?: number | string;
  image_concurrent_limit?: number | string;
  user_default_concurrent_limit?: number | string;
  user_default_rpm_limit?: number | string;
  image_retention_days?: number | string;
  auto_remove_invalid_accounts?: boolean;
  auto_remove_rate_limited_accounts?: boolean;
  log_levels?: string[];
  linuxdo_enabled?: boolean;
  linuxdo_client_id?: string;
  linuxdo_client_secret?: string;
  linuxdo_client_secret_configured?: boolean;
  linuxdo_redirect_url?: string;
  linuxdo_frontend_redirect_url?: string;
  email_smtp_enabled?: boolean;
  email_smtp_host?: string;
  email_smtp_port?: number | string;
  email_smtp_use_ssl?: boolean;
  email_smtp_username?: string;
  email_smtp_auth_code?: string;
  email_smtp_auth_code_configured?: boolean;
  email_smtp_from_email?: string;
  email_smtp_from_name?: string;
  yipay_enabled?: boolean;
  yipay_pid?: string;
  yipay_key?: string;
  yipay_key_configured?: boolean;
  yipay_submit_url?: string;
  yipay_notify_url?: string;
  yipay_return_url?: string;
  yipay_site_name?: string;
  paypal_enabled?: boolean;
  paypal_checkout_url?: string;
  usdt_enabled?: boolean;
  usdt_network?: string;
  usdt_address?: string;
  usdt_payment_url?: string;
  login_page_image_url?: string;
  login_page_image_mode?: LoginPageImageMode | string;
  login_page_image_zoom?: number | string;
  login_page_image_position_x?: number | string;
  login_page_image_position_y?: number | string;
  [key: string]: unknown;
};

export type LoginPageImageSettings = {
  login_page_image_url: string;
  login_page_image_mode: LoginPageImageMode;
  login_page_image_zoom: number;
  login_page_image_position_x: number;
  login_page_image_position_y: number;
};

export type ManagedImage = {
  name: string;
  path: string;
  owner_id?: string;
  date: string;
  size: number;
  url: string;
  thumbnail_url?: string;
  width?: number;
  height?: number;
  created_at: string;
};

export type SystemLog = {
  time: string;
  type: "call" | "account" | string;
  summary?: string;
  detail?: Record<string, unknown>;
  [key: string]: unknown;
};

export type ImageResponse = {
  created: number;
  data: Array<{ b64_json?: string; url?: string; revised_prompt?: string }>;
};

export type ImageTask = {
  id: string;
  status: "queued" | "running" | "success" | "error" | "cancelled";
  mode: "generate" | "edit";
  model?: ImageModel;
  size?: string;
  quality?: ImageQuality;
  created_at: string;
  updated_at: string;
  data?: Array<{ b64_json?: string; url?: string; revised_prompt?: string }>;
  error?: string;
  output_type?: "text";
};

export type ImageTaskMessage = {
  role: "system" | "user" | "assistant" | "tool";
  content: string;
};

export type ChatCompletionResponse = {
  choices?: Array<{
    message?: {
      content?: string | Array<{ type?: string; text?: string }>;
    };
  }>;
};

type ImageTaskListResponse = {
  items?: ImageTask[] | null;
  missing_ids?: string[] | null;
};

export type LoginResponse = {
  ok: boolean;
  version: string;
  role: AuthRole;
  subject_id: string;
  name: string;
  provider?: string;
  credential_id?: string;
  credential_name?: string;
};

export type EmailUserProfile = {
  id: string;
  email: string;
  name: string;
  enabled: boolean;
  provider: string;
  role: AuthRole;
  balance_cents: number;
  total_recharge_cents: number;
  total_consume_cents: number;
  created_at?: string;
  updated_at?: string;
  last_login_at?: string;
};

export type WalletInfo = {
  user_id: string;
  email: string;
  name: string;
  invite_code?: string;
  invited_by?: string;
  invitee_count?: number;
  invitees?: Array<{
    id: string;
    email: string;
    name?: string;
    created_at?: string;
  }>;
  balance_cents: number;
  total_recharge_cents: number;
  total_consume_cents: number;
  image_price_note?: string;
  last_login_at?: string;
  updated_at?: string;
  billing_provider_hint?: string;
};

export type WalletResponse = {
  wallet: WalletInfo;
  image_price: number;
  pay_channels?: string[];
};

export type EmailAuthResponse = LoginResponse & {
  key: string;
  user: EmailUserProfile;
  wallet: WalletInfo;
  image_price: number;
  allowed_domains: string[];
};

export type PayType = "alipay" | "wxpay" | "qqpay" | "paypal" | "usdt";
export type PayOrderStatus = "pending" | "paid" | "failed";

export type PayOrder = {
  id: string;
  provider: string;
  status: PayOrderStatus;
  out_trade_no: string;
  trade_no?: string;
  user_id?: string;
  user_email?: string;
  pay_type: PayType | string;
  amount_cents: number;
  amount_yuan: string;
  pay_url?: string;
  usdt_address?: string;
  usdt_network?: string;
  created_at: string;
  updated_at: string;
  paid_at?: string;
  source?: "order" | "transaction";
  tx_type?: "recharge" | "consume" | "adjust" | string;
  note?: string;
};

export type AuthProviders = {
  linuxdo: {
    enabled: boolean;
  };
};

export type Announcement = {
  id: string;
  title: string;
  content: string;
  enabled?: boolean;
  show_login: boolean;
  show_image: boolean;
  created_at?: string | null;
  updated_at?: string | null;
};

export type UserKey = {
  id: string;
  name: string;
  role: "user";
  kind?: "api_key";
  provider?: "local" | "linuxdo" | string;
  owner_id?: string;
  owner_name?: string;
  enabled: boolean;
  created_at: string | null;
  last_used_at: string | null;
};

export type ManagedUser = {
  id: string;
  name: string;
  role: "user";
  provider: "local" | "linuxdo" | string;
  owner_id?: string;
  owner_name?: string;
  linuxdo_level?: string;
  enabled: boolean;
  has_api_key: boolean;
  has_session: boolean;
  api_key_id?: string;
  api_key_name?: string;
  session_id?: string;
  session_name?: string;
  credential_count: number;
  created_at: string | null;
  last_used_at: string | null;
  updated_at?: string | null;
  call_count?: number;
  success_count?: number;
  failure_count?: number;
  quota_used?: number;
  usage_curve?: Array<{
    date: string;
    calls: number;
    success: number;
    failure: number;
    quota_used: number;
  }>;
  balance_cents?: number;
  total_recharge_cents?: number;
  total_consume_cents?: number;
  billing_user?: boolean;
  image_price_cents?: number;
};

export type RegisterConfig = {
  enabled: boolean;
  mail: {
    request_timeout: number;
    wait_timeout: number;
    wait_interval: number;
    providers: Array<Record<string, unknown>>;
  };
  proxy: string;
  total: number;
  threads: number;
  mode: "total" | "quota" | "available";
  target_quota: number;
  target_available: number;
  check_interval: number;
  stats: {
    job_id?: string;
    success: number;
    fail: number;
    done: number;
    running: number;
    threads: number;
    elapsed_seconds?: number;
    avg_seconds?: number;
    success_rate?: number;
    current_quota?: number;
    current_available?: number;
    started_at?: string;
    updated_at?: string;
    finished_at?: string;
  };
  logs?: Array<{
    time: string;
    text: string;
    level: string;
  }>;
};

export async function login(authKey: string) {
  const normalizedAuthKey = String(authKey || "").trim();
  return httpRequest<LoginResponse>("/auth/login", {
    method: "POST",
    body: {},
    headers: {
      Authorization: `Bearer ${normalizedAuthKey}`,
    },
    redirectOnUnauthorized: false,
  });
}

export async function registerEmail(payload: { email: string; password: string; name?: string; verification_code?: string; invite_code?: string }) {
  return httpRequest<EmailAuthResponse>("/auth/register", {
    method: "POST",
    body: {
      email: payload.email,
      password: payload.password,
      name: payload.name || "",
      verification_code: payload.verification_code || "",
      invite_code: payload.invite_code || "",
    },
    redirectOnUnauthorized: false,
  });
}

export async function sendEmailRegisterCode(payload: { email: string }) {
  return httpRequest<{ ok: boolean; message: string }>("/auth/email-register-code", {
    method: "POST",
    body: {
      email: payload.email,
    },
    redirectOnUnauthorized: false,
  });
}

export async function loginByEmail(payload: { email: string; password: string }) {
  return httpRequest<EmailAuthResponse>("/auth/email-login", {
    method: "POST",
    body: {
      email: payload.email,
      password: payload.password,
    },
    redirectOnUnauthorized: false,
  });
}

export async function fetchWallet() {
  return httpRequest<WalletResponse>("/api/wallet");
}

export async function redeemWalletCode(payload: { code: string }) {
  return httpRequest<{ ok: boolean; wallet: WalletInfo }>("/api/wallet/redeem", {
    method: "POST",
    body: payload,
  });
}

export async function createPayOrder(payload: { amount?: string; amount_cents?: number; pay_type: PayType }) {
  return httpRequest<{ order: PayOrder }>("/api/pay/orders", {
    method: "POST",
    body: payload,
  });
}

export async function fetchPayOrders() {
  return httpRequest<{ items: PayOrder[] }>("/api/pay/orders");
}

export async function fetchAuthProviders() {
  return httpRequest<AuthProviders>("/auth/providers", {
    redirectOnUnauthorized: false,
  });
}

export async function fetchVisibleAnnouncements(target: AnnouncementTarget) {
  const params = new URLSearchParams({ target });
  return httpRequest<{ items: Announcement[] }>(`/api/announcements?${params.toString()}`, {
    redirectOnUnauthorized: false,
  });
}

export async function fetchAdminAnnouncements() {
  return httpRequest<{ items: Announcement[] }>("/api/admin/announcements");
}

export async function createAnnouncement(announcement: {
  title: string;
  content: string;
  enabled: boolean;
  show_login: boolean;
  show_image: boolean;
}) {
  return httpRequest<{ item: Announcement; items: Announcement[] }>("/api/admin/announcements", {
    method: "POST",
    body: announcement,
  });
}

export async function updateAnnouncement(
  announcementId: string,
  updates: Partial<Pick<Announcement, "title" | "content" | "enabled" | "show_login" | "show_image">>,
) {
  return httpRequest<{ item: Announcement; items: Announcement[] }>(`/api/admin/announcements/${announcementId}`, {
    method: "POST",
    body: updates,
  });
}

export async function deleteAnnouncement(announcementId: string) {
  return httpRequest<{ items: Announcement[] }>(`/api/admin/announcements/${announcementId}`, {
    method: "DELETE",
  });
}

export async function fetchAccounts() {
  return httpRequest<AccountListResponse>("/api/accounts");
}

export async function createAccounts(tokens: string[]) {
  return httpRequest<AccountMutationResponse>("/api/accounts", {
    method: "POST",
    body: { tokens },
  });
}

export async function deleteAccounts(tokens: string[]) {
  return httpRequest<AccountMutationResponse>("/api/accounts", {
    method: "DELETE",
    body: { tokens },
  });
}

export async function refreshAccounts(accessTokens: string[]) {
  return httpRequest<AccountRefreshResponse>("/api/accounts/refresh", {
    method: "POST",
    body: { access_tokens: accessTokens },
  });
}

export async function updateAccount(
  accessToken: string,
  updates: {
    type?: AccountType;
    status?: AccountStatus;
    quota?: number;
  },
) {
  return httpRequest<AccountUpdateResponse>("/api/accounts/update", {
    method: "POST",
    body: {
      access_token: accessToken,
      ...updates,
    },
  });
}

export async function generateImage(prompt: string, model?: ImageModel, size?: string, quality?: ImageQuality) {
  return httpRequest<ImageResponse>(
    "/v1/images/generations",
    {
      method: "POST",
      body: {
        prompt,
        ...(model ? { model } : {}),
        ...(size ? { size } : {}),
        ...(quality ? { quality } : {}),
        n: 1,
        response_format: "b64_json",
      },
    },
  );
}

export async function editImage(files: File | File[], prompt: string, model?: ImageModel, size?: string, quality?: ImageQuality) {
  const formData = new FormData();
  const uploadFiles = Array.isArray(files) ? files : [files];

  uploadFiles.forEach((file) => {
    formData.append("image", file);
  });
  formData.append("prompt", prompt);
  if (model) {
    formData.append("model", model);
  }
  if (size) {
    formData.append("size", size);
  }
  if (quality) {
    formData.append("quality", quality);
  }
  formData.append("n", "1");

  return httpRequest<ImageResponse>(
    "/v1/images/edits",
    {
      method: "POST",
      body: formData,
    },
  );
}

export async function createImageGenerationTask(
  clientTaskId: string,
  prompt: string,
  model?: ImageModel,
  size?: string,
  quality?: ImageQuality,
  count = 1,
  messages?: ImageTaskMessage[],
) {
  return httpRequest<ImageTask>("/api/image-tasks/generations", {
    method: "POST",
    body: {
      client_task_id: clientTaskId,
      prompt,
      ...(model ? { model } : {}),
      ...(size ? { size } : {}),
      ...(quality ? { quality } : {}),
      ...(messages?.length ? { messages } : {}),
      n: count,
    },
  });
}

export async function createImageEditTask(
  clientTaskId: string,
  files: File | File[],
  prompt: string,
  model?: ImageModel,
  size?: string,
  quality?: ImageQuality,
  count = 1,
  messages?: ImageTaskMessage[],
) {
  const formData = new FormData();
  const uploadFiles = Array.isArray(files) ? files : [files];

  uploadFiles.forEach((file) => {
    formData.append("image", file);
  });
  formData.append("client_task_id", clientTaskId);
  formData.append("prompt", prompt);
  if (model) {
    formData.append("model", model);
  }
  if (size) {
    formData.append("size", size);
  }
  if (quality) {
    formData.append("quality", quality);
  }
  if (messages?.length) {
    formData.append("messages", JSON.stringify(messages));
  }
  formData.append("n", String(count));

  return httpRequest<ImageTask>("/api/image-tasks/edits", {
    method: "POST",
    body: formData,
  });
}

export async function createChatCompletion(model: ImageModel, messages: ImageTaskMessage[]) {
  return httpRequest<ChatCompletionResponse>("/v1/chat/completions", {
    method: "POST",
    body: {
      model,
      messages,
      stream: false,
    },
  });
}

export async function fetchImageTasks(ids: string[]) {
  const params = new URLSearchParams();
  if (ids.length > 0) {
    params.set("ids", ids.join(","));
  }
  const data = await httpRequest<ImageTaskListResponse>(`/api/image-tasks${params.toString() ? `?${params.toString()}` : ""}`, {
    headers: {
      "Cache-Control": "no-cache",
      Pragma: "no-cache",
    },
  });
  return {
    items: Array.isArray(data.items) ? data.items : [],
    missing_ids: Array.isArray(data.missing_ids) ? data.missing_ids : [],
  };
}

export async function cancelImageTask(clientTaskId: string) {
  return httpRequest<ImageTask>(`/api/image-tasks/${encodeURIComponent(clientTaskId)}/cancel`, {
    method: "POST",
    body: {},
  });
}

export async function fetchSettingsConfig() {
  return httpRequest<{ config: SettingsConfig }>("/api/settings");
}

export async function updateSettingsConfig(settings: SettingsConfig) {
  return httpRequest<{ config: SettingsConfig }>("/api/settings", {
    method: "POST",
    body: settings,
  });
}

export async function updateLoginPageImageSettings(
  settings: LoginPageImageSettings,
  options: { action: "keep" | "replace" | "remove"; file?: File | null },
) {
  const formData = new FormData();
  formData.append("login_page_image_url", settings.login_page_image_url);
  formData.append("login_page_image_mode", settings.login_page_image_mode);
  formData.append("login_page_image_zoom", String(settings.login_page_image_zoom));
  formData.append("login_page_image_position_x", String(settings.login_page_image_position_x));
  formData.append("login_page_image_position_y", String(settings.login_page_image_position_y));
  formData.append("login_page_image_action", options.action);
  if (options.file) {
    formData.append("login_page_image_file", options.file);
  }
  return httpRequest<{ config: SettingsConfig }>("/api/settings/login-page-image", {
    method: "POST",
    body: formData,
  });
}

export async function fetchManagedImages(
  filters: { start_date?: string; end_date?: string },
  options: { signal?: AbortSignal } = {},
) {
  const params = new URLSearchParams();
  if (filters.start_date) params.set("start_date", filters.start_date);
  if (filters.end_date) params.set("end_date", filters.end_date);
  const data = await httpRequest<{ items?: ManagedImage[] | null; groups?: Array<{ date: string; items: ManagedImage[] }> | null }>(
    `/api/images${params.toString() ? `?${params.toString()}` : ""}`,
    { signal: options.signal },
  );
  return {
    items: Array.isArray(data.items) ? data.items : [],
    groups: Array.isArray(data.groups) ? data.groups : [],
  };
}

export async function deleteManagedImages(paths: string[]) {
  return httpRequest<{ deleted: number; missing: number; paths: string[] }>("/api/images", {
    method: "DELETE",
    body: { paths },
  });
}

export async function fetchSystemLogs(filters: { type?: string; start_date?: string; end_date?: string }) {
  const params = new URLSearchParams();
  if (filters.type) params.set("type", filters.type);
  if (filters.start_date) params.set("start_date", filters.start_date);
  if (filters.end_date) params.set("end_date", filters.end_date);
  return httpRequest<{ items: SystemLog[] }>(`/api/logs${params.toString() ? `?${params.toString()}` : ""}`);
}

export async function fetchUserKeys() {
  return httpRequest<{ items: UserKey[] }>("/api/auth/users");
}

export async function createUserKey(name: string) {
  return httpRequest<{ item: UserKey; key: string; items: UserKey[] }>("/api/auth/users", {
    method: "POST",
    body: { name },
  });
}

export async function revealUserKey(keyId: string) {
  return httpRequest<{ key: string }>(`/api/auth/users/${keyId}/key`);
}

export async function updateUserKey(keyId: string, updates: { enabled?: boolean; name?: string }) {
  return httpRequest<{ item: UserKey; items: UserKey[] }>(`/api/auth/users/${keyId}`, {
    method: "POST",
    body: updates,
  });
}

export async function deleteUserKey(keyId: string) {
  return httpRequest<{ items: UserKey[] }>(`/api/auth/users/${keyId}`, {
    method: "DELETE",
  });
}

function managedUserPath(userId: string) {
  return `/api/admin/users/${encodeURIComponent(userId)}`;
}

export async function fetchManagedUsers() {
  return httpRequest<{ items: ManagedUser[] }>("/api/admin/users");
}

export async function createManagedUser(name: string) {
  return httpRequest<{ item: ManagedUser; api_key: UserKey; key: string; items: ManagedUser[] }>("/api/admin/users", {
    method: "POST",
    body: { name },
  });
}

export async function updateManagedUser(userId: string, updates: { enabled?: boolean; name?: string }) {
  return httpRequest<{ item: ManagedUser; items: ManagedUser[] }>(managedUserPath(userId), {
    method: "POST",
    body: updates,
  });
}

export async function adjustManagedUserBalance(
  userId: string,
  payload: { delta_cents?: number; balance_cents?: number; note?: string },
) {
  return httpRequest<{ wallet: WalletInfo; items: ManagedUser[] }>(
    `/api/admin/billing/users/${encodeURIComponent(userId)}/balance`,
    {
      method: "POST",
      body: payload,
    },
  );
}

export type RedeemCode = {
  code: string;
  amount_cents: number;
  amount_yuan: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
  expires_at?: string;
  used_by?: string;
  used_at?: string;
  note?: string;
};

export async function fetchRedeemCodes(limit = 200) {
  return httpRequest<{ items: RedeemCode[] }>(`/api/admin/billing/redeem-codes?limit=${encodeURIComponent(String(limit))}`);
}

export async function createRedeemCodes(payload: {
  amount_cents?: number;
  amount?: string;
  count?: number;
  expires_at?: string;
  note?: string;
}) {
  return httpRequest<{ items: RedeemCode[]; created: RedeemCode[] }>("/api/admin/billing/redeem-codes", {
    method: "POST",
    body: payload,
  });
}

export async function updateRedeemCode(
  code: string,
  payload: { enabled?: boolean; expires_at?: string; note?: string },
) {
  return httpRequest<{ item: RedeemCode; items: RedeemCode[] }>(`/api/admin/billing/redeem-codes/${encodeURIComponent(code)}`, {
    method: "POST",
    body: payload,
  });
}

export async function deleteRedeemCode(code: string) {
  return httpRequest<{ items: RedeemCode[] }>(`/api/admin/billing/redeem-codes/${encodeURIComponent(code)}`, {
    method: "DELETE",
  });
}

export async function revealManagedUserKey(userId: string) {
  return httpRequest<{ key: string }>(`${managedUserPath(userId)}/key`);
}

export async function resetManagedUserKey(userId: string, name?: string) {
  return httpRequest<{ item: ManagedUser; api_key: UserKey; key: string; items: ManagedUser[] }>(
    `${managedUserPath(userId)}/reset-key`,
    {
      method: "POST",
      body: { name: name ?? "" },
    },
  );
}

export async function deleteManagedUser(userId: string) {
  return httpRequest<{ items: ManagedUser[] }>(managedUserPath(userId), {
    method: "DELETE",
  });
}

export async function fetchRegisterConfig() {
  return httpRequest<{ register: RegisterConfig }>("/api/register");
}

export async function updateRegisterConfig(updates: Partial<RegisterConfig>) {
  return httpRequest<{ register: RegisterConfig }>("/api/register", {
    method: "POST",
    body: updates,
  });
}

export async function startRegister() {
  return httpRequest<{ register: RegisterConfig }>("/api/register/start", { method: "POST" });
}

export async function stopRegister() {
  return httpRequest<{ register: RegisterConfig }>("/api/register/stop", { method: "POST" });
}

export async function resetRegister() {
  return httpRequest<{ register: RegisterConfig }>("/api/register/reset", { method: "POST" });
}

// ── CPA (CLIProxyAPI) ──────────────────────────────────────────────

export type CPAPool = {
  id: string;
  name: string;
  base_url: string;
  import_job?: CPAImportJob | null;
};

export type CPARemoteFile = {
  name: string;
  email: string;
};

export type CPAImportJob = {
  job_id: string;
  status: "pending" | "running" | "completed" | "failed";
  created_at: string;
  updated_at: string;
  total: number;
  completed: number;
  added: number;
  skipped: number;
  refreshed: number;
  failed: number;
  errors: Array<{ name: string; error: string }>;
};

export async function fetchCPAPools() {
  return httpRequest<{ pools: CPAPool[] }>("/api/cpa/pools");
}

export async function createCPAPool(pool: { name: string; base_url: string; secret_key: string }) {
  return httpRequest<{ pool: CPAPool; pools: CPAPool[] }>("/api/cpa/pools", {
    method: "POST",
    body: pool,
  });
}

export async function updateCPAPool(
  poolId: string,
  updates: { name?: string; base_url?: string; secret_key?: string },
) {
  return httpRequest<{ pool: CPAPool; pools: CPAPool[] }>(`/api/cpa/pools/${poolId}`, {
    method: "POST",
    body: updates,
  });
}

export async function deleteCPAPool(poolId: string) {
  return httpRequest<{ pools: CPAPool[] }>(`/api/cpa/pools/${poolId}`, {
    method: "DELETE",
  });
}

export async function fetchCPAPoolFiles(poolId: string) {
  return httpRequest<{ pool_id: string; files: CPARemoteFile[] }>(`/api/cpa/pools/${poolId}/files`);
}

export async function startCPAImport(poolId: string, names: string[]) {
  return httpRequest<{ import_job: CPAImportJob | null }>(`/api/cpa/pools/${poolId}/import`, {
    method: "POST",
    body: { names },
  });
}

export async function fetchCPAPoolImportJob(poolId: string) {
  return httpRequest<{ import_job: CPAImportJob | null }>(`/api/cpa/pools/${poolId}/import`);
}

// ── Sub2API ────────────────────────────────────────────────────────

export type Sub2APIServer = {
  id: string;
  name: string;
  base_url: string;
  email: string;
  has_api_key: boolean;
  group_id: string;
  import_job?: CPAImportJob | null;
};

export type Sub2APIRemoteAccount = {
  id: string;
  name: string;
  email: string;
  plan_type: string;
  status: string;
  expires_at: string;
  has_refresh_token: boolean;
};

export type Sub2APIRemoteGroup = {
  id: string;
  name: string;
  description: string;
  platform: string;
  status: string;
  account_count: number;
  active_account_count: number;
};

export async function fetchSub2APIServers() {
  return httpRequest<{ servers: Sub2APIServer[] }>("/api/sub2api/servers");
}

export async function createSub2APIServer(server: {
  name: string;
  base_url: string;
  email: string;
  password: string;
  api_key: string;
  group_id: string;
}) {
  return httpRequest<{ server: Sub2APIServer; servers: Sub2APIServer[] }>("/api/sub2api/servers", {
    method: "POST",
    body: server,
  });
}

export async function updateSub2APIServer(
  serverId: string,
  updates: {
    name?: string;
    base_url?: string;
    email?: string;
    password?: string;
    api_key?: string;
    group_id?: string;
  },
) {
  return httpRequest<{ server: Sub2APIServer; servers: Sub2APIServer[] }>(`/api/sub2api/servers/${serverId}`, {
    method: "POST",
    body: updates,
  });
}

export async function fetchSub2APIServerGroups(serverId: string) {
  return httpRequest<{ server_id: string; groups: Sub2APIRemoteGroup[] }>(
    `/api/sub2api/servers/${serverId}/groups`,
  );
}

export async function deleteSub2APIServer(serverId: string) {
  return httpRequest<{ servers: Sub2APIServer[] }>(`/api/sub2api/servers/${serverId}`, {
    method: "DELETE",
  });
}

export async function fetchSub2APIServerAccounts(serverId: string) {
  return httpRequest<{ server_id: string; accounts: Sub2APIRemoteAccount[] }>(
    `/api/sub2api/servers/${serverId}/accounts`,
  );
}

export async function startSub2APIImport(serverId: string, accountIds: string[]) {
  return httpRequest<{ import_job: CPAImportJob | null }>(`/api/sub2api/servers/${serverId}/import`, {
    method: "POST",
    body: { account_ids: accountIds },
  });
}

export async function fetchSub2APIImportJob(serverId: string) {
  return httpRequest<{ import_job: CPAImportJob | null }>(`/api/sub2api/servers/${serverId}/import`);
}

// ── Upstream proxy ────────────────────────────────────────────────

export type ProxySettings = {
  enabled: boolean;
  url: string;
};

export type ProxyTestResult = {
  ok: boolean;
  status: number;
  latency_ms: number;
  error: string | null;
};

export async function fetchProxy() {
  return httpRequest<{ proxy: ProxySettings }>("/api/proxy");
}

export async function updateProxy(updates: { enabled?: boolean; url?: string }) {
  return httpRequest<{ proxy: ProxySettings }>("/api/proxy", {
    method: "POST",
    body: updates,
  });
}

export async function testProxy(url?: string) {
  return httpRequest<{ result: ProxyTestResult }>("/api/proxy/test", {
    method: "POST",
    body: { url: url ?? "" },
  });
}
