"use client";

import { create } from "zustand";
import { toast } from "sonner";

import {
  cleanupLogs,
  createCPAPool,
  deleteCPAPool,
  fetchCPAPoolFiles,
  fetchCPAPools,
  fetchLogGovernance,
  fetchRegisterConfig,
  resetRegister as resetRegisterApi,
  fetchSettingsConfig,
  startRegister,
  startCPAImport,
  stopRegister,
  updateCPAPool,
  updateLoginPageImageSettings,
  updateRegisterConfig,
  updateSettingsConfig,
  type CPAPool,
  type CPARemoteFile,
  type LogCleanupResult,
  type LogGovernanceSummary,
  type LoginPageImageSettings,
  type RegisterConfig,
  type SettingsConfig,
} from "@/lib/api";
import { dispatchAppMetaUpdated } from "@/lib/app-meta";
import {
  LOGIN_PAGE_IMAGE_DEFAULT_TRANSFORM,
  normalizeLoginPageImageMode,
  normalizeLoginPageImageTransform,
  type LoginPageImageMode,
} from "@/lib/login-page-image-layout";

export const PAGE_SIZE_OPTIONS = ["50", "100", "200"] as const;

export type PageSizeOption = (typeof PAGE_SIZE_OPTIONS)[number];

function centsToYuanInput(value: unknown, fallbackCents: number) {
  const cents = Number(value || fallbackCents);
  const safe = Number.isFinite(cents)
    ? Math.max(0, cents)
    : Math.max(0, fallbackCents);
  return (safe / 100).toFixed(2);
}

function yuanInputToCents(value: unknown, fallbackCents: number) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return Math.max(1, fallbackCents);
  }
  return Math.max(1, Math.round(parsed * 100));
}

function normalizeConfig(config: SettingsConfig): SettingsConfig {
  const loginImageTransform = normalizeLoginPageImageTransform({
    zoom: Number(config.login_page_image_zoom),
    positionX: Number(config.login_page_image_position_x),
    positionY: Number(config.login_page_image_position_y),
  });
  const registrationBonusImageTimes = Number(
    config.registration_bonus_image_times,
  );
  return {
    ...config,
    refresh_account_interval_minute: Number(
      config.refresh_account_interval_minute || 5,
    ),
    image_concurrent_limit: Number(config.image_concurrent_limit || 4),
    image_single_count_limit: Number(config.image_single_count_limit || 10),
    image_task_timeout_seconds: Number(
      config.image_task_timeout_seconds || 300,
    ),
    user_default_concurrent_limit: Number(
      config.user_default_concurrent_limit || 0,
    ),
    user_default_rpm_limit: Number(config.user_default_rpm_limit || 0),
    image_retention_days: Number(config.image_retention_days || 30),
    log_retention_days: Number(config.log_retention_days || 7),
    auto_remove_invalid_accounts: Boolean(config.auto_remove_invalid_accounts),
    auto_remove_rate_limited_accounts: Boolean(
      config.auto_remove_rate_limited_accounts,
    ),
    log_levels: Array.isArray(config.log_levels) ? config.log_levels : [],
    proxy: typeof config.proxy === "string" ? config.proxy : "",
    base_url: typeof config.base_url === "string" ? config.base_url : "",
    brand_top_left_name:
      typeof config.brand_top_left_name === "string"
        ? config.brand_top_left_name
        : "GPT生图站",
    brand_site_name:
      typeof config.brand_site_name === "string"
        ? config.brand_site_name
        : "GPT生图站",
    brand_top_left_logo_url:
      typeof config.brand_top_left_logo_url === "string"
        ? config.brand_top_left_logo_url
        : "/logo-mark.svg",
    brand_site_logo_url:
      typeof config.brand_site_logo_url === "string"
        ? config.brand_site_logo_url
        : "/logo-mark.svg",
    registration_enabled: Boolean(config.registration_enabled),
    registration_allowed_email_domains:
      typeof config.registration_allowed_email_domains === "string"
        ? config.registration_allowed_email_domains
        : "qq.com,163.com,126.com,gmail.com,outlook.com,hotmail.com,icloud.com,yahoo.com,foxmail.com,sina.com",
    registration_bonus_image_times: Number.isFinite(registrationBonusImageTimes)
      ? registrationBonusImageTimes
      : 20,
    agency_enabled: config.agency_enabled !== false,
    show_ecommerce_entry: config.show_ecommerce_entry !== false,
    show_new_ecommerce_window_entry:
      config.show_new_ecommerce_window_entry !== false,
    cf_turnstile_enabled: Boolean(config.cf_turnstile_enabled),
    cf_turnstile_site_key:
      typeof config.cf_turnstile_site_key === "string"
        ? config.cf_turnstile_site_key
        : "",
    cf_turnstile_secret_key: "",
    cf_turnstile_secret_key_configured: Boolean(
      config.cf_turnstile_secret_key_configured,
    ),
    email_smtp_enabled: Boolean(config.email_smtp_enabled),
    email_smtp_host:
      typeof config.email_smtp_host === "string"
        ? config.email_smtp_host
        : "smtp.qq.com",
    email_smtp_port: Number(config.email_smtp_port || 465),
    email_smtp_use_ssl: Boolean(config.email_smtp_use_ssl ?? true),
    email_smtp_username:
      typeof config.email_smtp_username === "string"
        ? config.email_smtp_username
        : "",
    email_smtp_auth_code: "",
    email_smtp_auth_code_configured: Boolean(
      config.email_smtp_auth_code_configured,
    ),
    email_smtp_from_email:
      typeof config.email_smtp_from_email === "string"
        ? config.email_smtp_from_email
        : "",
    email_smtp_from_name:
      typeof config.email_smtp_from_name === "string"
        ? config.email_smtp_from_name
        : "GPT生图站",
    image_price_cents: Number(config.image_price_cents || 8),
    image_price_1k_cents: centsToYuanInput(
      config.image_price_1k_cents || config.image_price_cents || 8,
      8,
    ),
    image_price_2k_cents: centsToYuanInput(
      config.image_price_2k_cents || 16,
      16,
    ),
    image_price_4k_cents: centsToYuanInput(
      config.image_price_4k_cents || 32,
      32,
    ),
    agency_tier_basic_cents: Number(config.agency_tier_basic_cents || 19900),
    agency_tier_pro_cents: Number(config.agency_tier_pro_cents || 49900),
    agency_tier_premium_cents: Number(
      config.agency_tier_premium_cents || 99900,
    ),
    subscription_enabled: config.subscription_enabled !== false,
    subscription_heading:
      typeof config.subscription_heading === "string"
        ? config.subscription_heading
        : "选择适合你的订阅套餐",
    subscription_subheading:
      typeof config.subscription_subheading === "string"
        ? config.subscription_subheading
        : "在有效期内无限生图，不扣余额",
    subscription_safety_text:
      typeof config.subscription_safety_text === "string"
        ? config.subscription_safety_text
        : "安全支付保障·随时可取消·无隐藏费用",
    subscription_agent_hint:
      typeof config.subscription_agent_hint === "string"
        ? config.subscription_agent_hint
        : "购买代理充值更优惠",
    subscription_monthly_name:
      typeof config.subscription_monthly_name === "string"
        ? config.subscription_monthly_name
        : "包月套餐",
    subscription_monthly_desc:
      typeof config.subscription_monthly_desc === "string"
        ? config.subscription_monthly_desc
        : "",
    subscription_monthly_badge:
      typeof config.subscription_monthly_badge === "string"
        ? config.subscription_monthly_badge
        : "",
    subscription_monthly_price_cents: Number(
      config.subscription_monthly_price_cents || 2990,
    ),
    subscription_monthly_price_note:
      typeof config.subscription_monthly_price_note === "string"
        ? config.subscription_monthly_price_note
        : "",
    subscription_monthly_features:
      typeof config.subscription_monthly_features === "string"
        ? config.subscription_monthly_features
        : "无限生图\n高峰稳定排队\n专属客服支持",
    subscription_quarterly_name:
      typeof config.subscription_quarterly_name === "string"
        ? config.subscription_quarterly_name
        : "包季套餐",
    subscription_quarterly_desc:
      typeof config.subscription_quarterly_desc === "string"
        ? config.subscription_quarterly_desc
        : "",
    subscription_quarterly_badge:
      typeof config.subscription_quarterly_badge === "string"
        ? config.subscription_quarterly_badge
        : "推荐",
    subscription_quarterly_price_cents: Number(
      config.subscription_quarterly_price_cents || 7990,
    ),
    subscription_quarterly_price_note:
      typeof config.subscription_quarterly_price_note === "string"
        ? config.subscription_quarterly_price_note
        : "",
    subscription_quarterly_features:
      typeof config.subscription_quarterly_features === "string"
        ? config.subscription_quarterly_features
        : "无限生图\n优先出图通道\n专属客服支持",
    subscription_yearly_name:
      typeof config.subscription_yearly_name === "string"
        ? config.subscription_yearly_name
        : "包年套餐",
    subscription_yearly_desc:
      typeof config.subscription_yearly_desc === "string"
        ? config.subscription_yearly_desc
        : "",
    subscription_yearly_badge:
      typeof config.subscription_yearly_badge === "string"
        ? config.subscription_yearly_badge
        : "最划算",
    subscription_yearly_price_cents: Number(
      config.subscription_yearly_price_cents || 27990,
    ),
    subscription_yearly_price_note:
      typeof config.subscription_yearly_price_note === "string"
        ? config.subscription_yearly_price_note
        : "",
    subscription_yearly_features:
      typeof config.subscription_yearly_features === "string"
        ? config.subscription_yearly_features
        : "无限生图\n全年优先保障\n专属客服支持",
    yipay_enabled: Boolean(config.yipay_enabled),
    yipay_pid: typeof config.yipay_pid === "string" ? config.yipay_pid : "",
    yipay_key: "",
    yipay_key_configured: Boolean(config.yipay_key_configured),
    yipay_submit_url:
      typeof config.yipay_submit_url === "string"
        ? config.yipay_submit_url
        : "",
    yipay_notify_url:
      typeof config.yipay_notify_url === "string"
        ? config.yipay_notify_url
        : "",
    yipay_return_url:
      typeof config.yipay_return_url === "string"
        ? config.yipay_return_url
        : "",
    yipay_site_name:
      typeof config.yipay_site_name === "string"
        ? config.yipay_site_name
        : "GPT生图站",
    paypal_enabled: Boolean(config.paypal_enabled),
    paypal_checkout_url:
      typeof config.paypal_checkout_url === "string"
        ? config.paypal_checkout_url
        : "",
    usdt_enabled: Boolean(config.usdt_enabled),
    usdt_network:
      typeof config.usdt_network === "string" ? config.usdt_network : "TRC20",
    usdt_address:
      typeof config.usdt_address === "string" ? config.usdt_address : "",
    usdt_payment_url:
      typeof config.usdt_payment_url === "string"
        ? config.usdt_payment_url
        : "",
    image_r2_enabled: Boolean(config.image_r2_enabled),
    image_r2_endpoint:
      typeof config.image_r2_endpoint === "string"
        ? config.image_r2_endpoint
        : "",
    image_r2_bucket:
      typeof config.image_r2_bucket === "string" ? config.image_r2_bucket : "",
    image_r2_region:
      typeof config.image_r2_region === "string"
        ? config.image_r2_region
        : "auto",
    image_r2_access_key_id:
      typeof config.image_r2_access_key_id === "string"
        ? config.image_r2_access_key_id
        : "",
    image_r2_secret_access_key: "",
    image_r2_secret_access_key_configured: Boolean(
      config.image_r2_secret_access_key_configured,
    ),
    image_r2_public_base_url:
      typeof config.image_r2_public_base_url === "string"
        ? config.image_r2_public_base_url
        : "",
    image_r2_prefix:
      typeof config.image_r2_prefix === "string"
        ? config.image_r2_prefix
        : "images",
    image_r2_secondary_enabled: Boolean(config.image_r2_secondary_enabled),
    image_r2_secondary_endpoint:
      typeof config.image_r2_secondary_endpoint === "string"
        ? config.image_r2_secondary_endpoint
        : "",
    image_r2_secondary_bucket:
      typeof config.image_r2_secondary_bucket === "string"
        ? config.image_r2_secondary_bucket
        : "",
    image_r2_secondary_region:
      typeof config.image_r2_secondary_region === "string"
        ? config.image_r2_secondary_region
        : "auto",
    image_r2_secondary_access_key_id:
      typeof config.image_r2_secondary_access_key_id === "string"
        ? config.image_r2_secondary_access_key_id
        : "",
    image_r2_secondary_secret_access_key: "",
    image_r2_secondary_secret_access_key_configured: Boolean(
      config.image_r2_secondary_secret_access_key_configured,
    ),
    image_r2_secondary_public_base_url:
      typeof config.image_r2_secondary_public_base_url === "string"
        ? config.image_r2_secondary_public_base_url
        : "",
    image_r2_secondary_prefix:
      typeof config.image_r2_secondary_prefix === "string"
        ? config.image_r2_secondary_prefix
        : "images",
    image_imgbed_enabled: Boolean(config.image_imgbed_enabled),
    image_imgbed_upload_url:
      typeof config.image_imgbed_upload_url === "string"
        ? config.image_imgbed_upload_url
        : "",
    image_imgbed_auth_code: "",
    image_imgbed_auth_code_configured: Boolean(
      config.image_imgbed_auth_code_configured,
    ),
    image_imgbed_upload_channel:
      typeof config.image_imgbed_upload_channel === "string"
        ? config.image_imgbed_upload_channel
        : "cfr2",
    linuxdo_enabled: Boolean(config.linuxdo_enabled),
    linuxdo_client_id:
      typeof config.linuxdo_client_id === "string"
        ? config.linuxdo_client_id
        : "",
    linuxdo_client_secret: "",
    linuxdo_client_secret_configured: Boolean(
      config.linuxdo_client_secret_configured,
    ),
    linuxdo_redirect_url:
      typeof config.linuxdo_redirect_url === "string"
        ? config.linuxdo_redirect_url
        : "",
    linuxdo_frontend_redirect_url:
      typeof config.linuxdo_frontend_redirect_url === "string"
        ? config.linuxdo_frontend_redirect_url
        : "/auth/linuxdo/callback",
    update_repo:
      typeof config.update_repo === "string"
        ? config.update_repo
        : "ZyphrZero/chatgpt2api",
    update_github_token: "",
    update_github_token_configured: Boolean(
      config.update_github_token_configured,
    ),
    login_page_image_url:
      typeof config.login_page_image_url === "string"
        ? config.login_page_image_url
        : "",
    login_page_image_mode: normalizeLoginPageImageMode(
      config.login_page_image_mode,
    ),
    login_page_image_zoom: loginImageTransform.zoom,
    login_page_image_position_x: loginImageTransform.positionX,
    login_page_image_position_y: loginImageTransform.positionY,
  };
}

function normalizeFiles(items: CPARemoteFile[]) {
  const seen = new Set<string>();
  const files: CPARemoteFile[] = [];
  for (const item of items) {
    const name = String(item.name || "").trim();
    if (!name || seen.has(name)) {
      continue;
    }
    seen.add(name);
    files.push({
      name,
      email: String(item.email || "").trim(),
    });
  }
  return files;
}

type SettingsStore = {
  config: SettingsConfig | null;
  isLoadingConfig: boolean;
  isSavingConfig: boolean;
  logGovernance: LogGovernanceSummary | null;
  lastLogCleanup: LogCleanupResult | null;
  isLoadingLogGovernance: boolean;
  isCleaningLogs: boolean;

  registerConfig: RegisterConfig | null;
  isLoadingRegister: boolean;
  isSavingRegister: boolean;

  pools: CPAPool[];
  isLoadingPools: boolean;
  deletingId: string | null;
  loadingFilesId: string | null;

  dialogOpen: boolean;
  editingPool: CPAPool | null;
  formName: string;
  formBaseUrl: string;
  formSecretKey: string;
  showSecret: boolean;
  isSavingPool: boolean;

  browserOpen: boolean;
  browserPool: CPAPool | null;
  remoteFiles: CPARemoteFile[];
  selectedNames: string[];
  fileQuery: string;
  filePage: number;
  pageSize: PageSizeOption;
  isStartingImport: boolean;

  initialize: () => Promise<void>;
  loadConfig: () => Promise<void>;
  saveConfig: () => Promise<void>;
  setRefreshAccountIntervalMinute: (value: string) => void;
  setImageConcurrentLimit: (value: string) => void;
  setImageSingleCountLimit: (value: string) => void;
  setImageTaskTimeoutSeconds: (value: string) => void;
  setUserDefaultConcurrentLimit: (value: string) => void;
  setUserDefaultRpmLimit: (value: string) => void;
  setImageRetentionDays: (value: string) => void;
  setLogRetentionDays: (value: string) => void;
  setAutoRemoveInvalidAccounts: (value: boolean) => void;
  setAutoRemoveRateLimitedAccounts: (value: boolean) => void;
  setLogLevel: (level: string, enabled: boolean) => void;
  setProxy: (value: string) => void;
  setBaseUrl: (value: string) => void;
  setBrandTopLeftName: (value: string) => void;
  setBrandSiteName: (value: string) => void;
  setBrandTopLeftLogoURL: (value: string) => void;
  setBrandSiteLogoURL: (value: string) => void;
  setRegistrationEnabled: (value: boolean) => void;
  setRegistrationAllowedEmailDomains: (value: string) => void;
  setRegistrationBonusImageTimes: (value: string) => void;
  setShowEcommerceEntry: (value: boolean) => void;
  setShowNewEcommerceWindowEntry: (value: boolean) => void;
  setCFTurnstileEnabled: (value: boolean) => void;
  setCFTurnstileSiteKey: (value: string) => void;
  setCFTurnstileSecretKey: (value: string) => void;
  setEmailSMTPEnabled: (value: boolean) => void;
  setEmailSMTPHost: (value: string) => void;
  setEmailSMTPPort: (value: string) => void;
  setEmailSMTPUseSSL: (value: boolean) => void;
  setEmailSMTPUsername: (value: string) => void;
  setEmailSMTPAuthCode: (value: string) => void;
  setEmailSMTPFromEmail: (value: string) => void;
  setEmailSMTPFromName: (value: string) => void;
  setImagePrice1K: (value: string) => void;
  setImagePrice2K: (value: string) => void;
  setImagePrice4K: (value: string) => void;
  setYiPayEnabled: (value: boolean) => void;
  setYiPayPID: (value: string) => void;
  setYiPayKey: (value: string) => void;
  setYiPaySubmitUrl: (value: string) => void;
  setYiPayNotifyUrl: (value: string) => void;
  setYiPayReturnUrl: (value: string) => void;
  setYiPaySiteName: (value: string) => void;
  setImageR2Enabled: (value: boolean) => void;
  setImageR2Endpoint: (value: string) => void;
  setImageR2Bucket: (value: string) => void;
  setImageR2Region: (value: string) => void;
  setImageR2AccessKeyId: (value: string) => void;
  setImageR2SecretAccessKey: (value: string) => void;
  setImageR2PublicBaseUrl: (value: string) => void;
  setImageR2Prefix: (value: string) => void;
  setLinuxDoEnabled: (value: boolean) => void;
  setLinuxDoClientId: (value: string) => void;
  setLinuxDoClientSecret: (value: string) => void;
  setLinuxDoRedirectUrl: (value: string) => void;
  setLinuxDoFrontendRedirectUrl: (value: string) => void;
  setUpdateRepo: (value: string) => void;
  setUpdateGitHubToken: (value: string) => void;
  setConfigField: <K extends keyof SettingsConfig>(
    key: K,
    value: SettingsConfig[K],
  ) => void;
  setLoginPageImageUrl: (value: string) => void;
  setLoginPageImageMode: (value: LoginPageImageMode) => void;
  setLoginPageImageTransform: (transform: {
    zoom: number;
    positionX: number;
    positionY: number;
  }) => void;
  restoreDefaultLoginPageImage: () => void;
  saveLoginPageImage: (options: {
    file?: File | null;
    action: "keep" | "replace" | "remove";
  }) => Promise<boolean>;
  loadLogGovernance: (silent?: boolean) => Promise<void>;
  cleanupLogsByRetention: () => Promise<void>;

  loadRegister: (silent?: boolean) => Promise<void>;
  setRegisterConfig: (config: RegisterConfig) => void;
  setRegisterProxy: (value: string) => void;
  setRegisterTotal: (value: string) => void;
  setRegisterThreads: (value: string) => void;
  setRegisterMode: (value: "total" | "quota" | "available") => void;
  setRegisterTargetQuota: (value: string) => void;
  setRegisterTargetAvailable: (value: string) => void;
  setRegisterCheckInterval: (value: string) => void;
  setRegisterAccountCheckInterval: (value: string) => void;
  setRegisterMailField: (
    key: "request_timeout" | "wait_timeout" | "wait_interval",
    value: string,
  ) => void;
  addRegisterProvider: () => void;
  updateRegisterProvider: (
    index: number,
    updates: Record<string, unknown>,
  ) => void;
  deleteRegisterProvider: (index: number) => void;
  saveRegister: () => Promise<void>;
  toggleRegister: () => Promise<void>;
  resetRegister: () => Promise<void>;

  loadPools: (silent?: boolean) => Promise<void>;
  openAddDialog: () => void;
  openEditDialog: (pool: CPAPool) => void;
  setDialogOpen: (open: boolean) => void;
  setFormName: (value: string) => void;
  setFormBaseUrl: (value: string) => void;
  setFormSecretKey: (value: string) => void;
  setShowSecret: (checked: boolean) => void;
  savePool: () => Promise<void>;
  deletePool: (pool: CPAPool) => Promise<void>;

  browseFiles: (pool: CPAPool) => Promise<void>;
  setBrowserOpen: (open: boolean) => void;
  toggleFile: (name: string, checked: boolean) => void;
  replaceSelectedNames: (names: string[]) => void;
  setFileQuery: (value: string) => void;
  setFilePage: (page: number) => void;
  setPageSize: (value: PageSizeOption) => void;
  startImport: () => Promise<void>;
};

export const useSettingsStore = create<SettingsStore>((set, get) => ({
  config: null,
  isLoadingConfig: true,
  isSavingConfig: false,
  logGovernance: null,
  lastLogCleanup: null,
  isLoadingLogGovernance: true,
  isCleaningLogs: false,

  registerConfig: null,
  isLoadingRegister: true,
  isSavingRegister: false,

  pools: [],
  isLoadingPools: true,
  deletingId: null,
  loadingFilesId: null,

  dialogOpen: false,
  editingPool: null,
  formName: "",
  formBaseUrl: "",
  formSecretKey: "",
  showSecret: false,
  isSavingPool: false,

  browserOpen: false,
  browserPool: null,
  remoteFiles: [],
  selectedNames: [],
  fileQuery: "",
  filePage: 1,
  pageSize: "100",
  isStartingImport: false,

  initialize: async () => {
    await Promise.allSettled([get().loadConfig()]);
  },

  loadConfig: async () => {
    set({ isLoadingConfig: true });
    try {
      const data = await fetchSettingsConfig();
      set({
        config: normalizeConfig(data.config),
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载系统配置失败");
    } finally {
      set({ isLoadingConfig: false });
    }
  },

  saveConfig: async () => {
    const { config, isLoadingConfig } = get();
    if (!config) {
      return;
    }
    if (isLoadingConfig) {
      toast.info("配置还在加载中，请稍后再保存");
      return;
    }

    set({ isSavingConfig: true });
    try {
      const linuxDoClientSecret = String(
        config.linuxdo_client_secret || "",
      ).trim();
      const updateGitHubToken = String(config.update_github_token || "").trim();
      const cfTurnstileSecretKey = String(
        config.cf_turnstile_secret_key || "",
      ).trim();
      const emailSMTPAuthCode = String(
        config.email_smtp_auth_code || "",
      ).trim();
      const yiPayKey = String(config.yipay_key || "").trim();
      const imageR2SecretAccessKey = String(
        config.image_r2_secret_access_key || "",
      ).trim();
      const imageSecondaryR2SecretAccessKey = String(
        config.image_r2_secondary_secret_access_key || "",
      ).trim();
      const imageImgBedAuthCode = String(
        config.image_imgbed_auth_code || "",
      ).trim();
      const payload: SettingsConfig = {
        ...config,
        refresh_account_interval_minute: Math.max(
          1,
          Number(config.refresh_account_interval_minute) || 1,
        ),
        image_concurrent_limit: Math.max(
          1,
          Number(config.image_concurrent_limit) || 4,
        ),
        image_single_count_limit: Math.min(
          10,
          Math.max(1, Number(config.image_single_count_limit) || 10),
        ),
        image_task_timeout_seconds: Math.min(
          3600,
          Math.max(30, Number(config.image_task_timeout_seconds) || 300),
        ),
        user_default_concurrent_limit: Math.max(
          0,
          Number(config.user_default_concurrent_limit) || 0,
        ),
        user_default_rpm_limit: Math.max(
          0,
          Number(config.user_default_rpm_limit) || 0,
        ),
        image_retention_days: Math.max(
          1,
          Number(config.image_retention_days) || 30,
        ),
        log_retention_days: Math.min(
          3650,
          Math.max(1, Number(config.log_retention_days) || 7),
        ),
        auto_remove_invalid_accounts: Boolean(
          config.auto_remove_invalid_accounts,
        ),
        auto_remove_rate_limited_accounts: Boolean(
          config.auto_remove_rate_limited_accounts,
        ),
        proxy: config.proxy.trim(),
        base_url: String(config.base_url || "").trim(),
        brand_top_left_name: String(config.brand_top_left_name || "").trim(),
        brand_site_name: String(config.brand_site_name || "").trim(),
        brand_top_left_logo_url: String(
          config.brand_top_left_logo_url || "",
        ).trim(),
        brand_site_logo_url: String(config.brand_site_logo_url || "").trim(),
        registration_enabled: Boolean(config.registration_enabled),
        registration_allowed_email_domains: String(
          config.registration_allowed_email_domains || "",
        ).trim(),
        registration_bonus_image_times: Math.max(
          0,
          Number(config.registration_bonus_image_times) || 0,
        ),
        agency_enabled: Boolean(config.agency_enabled),
        show_ecommerce_entry: Boolean(config.show_ecommerce_entry),
        show_new_ecommerce_window_entry: Boolean(
          config.show_new_ecommerce_window_entry,
        ),
        cf_turnstile_enabled: Boolean(config.cf_turnstile_enabled),
        cf_turnstile_site_key: String(
          config.cf_turnstile_site_key || "",
        ).trim(),
        cf_turnstile_secret_key: cfTurnstileSecretKey,
        email_smtp_enabled: Boolean(config.email_smtp_enabled),
        email_smtp_host: String(config.email_smtp_host || "").trim(),
        email_smtp_port: Math.max(1, Number(config.email_smtp_port) || 465),
        email_smtp_use_ssl: Boolean(config.email_smtp_use_ssl),
        email_smtp_username: String(config.email_smtp_username || "").trim(),
        email_smtp_auth_code: emailSMTPAuthCode,
        email_smtp_from_email: String(
          config.email_smtp_from_email || "",
        ).trim(),
        email_smtp_from_name: String(config.email_smtp_from_name || "").trim(),
        image_price_cents: Math.max(1, Number(config.image_price_cents) || 8),
        image_price_1k_cents: yuanInputToCents(
          config.image_price_1k_cents,
          Number(config.image_price_cents) || 8,
        ),
        image_price_2k_cents: yuanInputToCents(config.image_price_2k_cents, 16),
        image_price_4k_cents: yuanInputToCents(config.image_price_4k_cents, 32),
        agency_tier_basic_cents: Math.max(
          0,
          Number(config.agency_tier_basic_cents) || 19900,
        ),
        agency_tier_pro_cents: Math.max(
          0,
          Number(config.agency_tier_pro_cents) || 49900,
        ),
        agency_tier_premium_cents: Math.max(
          0,
          Number(config.agency_tier_premium_cents) || 99900,
        ),
        subscription_enabled: config.subscription_enabled !== false,
        subscription_heading: String(config.subscription_heading || "").trim(),
        subscription_subheading: String(
          config.subscription_subheading || "",
        ).trim(),
        subscription_safety_text: String(
          config.subscription_safety_text || "",
        ).trim(),
        subscription_agent_hint: String(
          config.subscription_agent_hint || "",
        ).trim(),
        subscription_monthly_name: String(
          config.subscription_monthly_name || "",
        ).trim(),
        subscription_monthly_desc: String(
          config.subscription_monthly_desc || "",
        ).trim(),
        subscription_monthly_badge: String(
          config.subscription_monthly_badge || "",
        ).trim(),
        subscription_monthly_price_cents: Math.max(
          0,
          Number(config.subscription_monthly_price_cents) || 0,
        ),
        subscription_monthly_price_note: String(
          config.subscription_monthly_price_note || "",
        ).trim(),
        subscription_monthly_features: String(
          config.subscription_monthly_features || "",
        ).trim(),
        subscription_quarterly_name: String(
          config.subscription_quarterly_name || "",
        ).trim(),
        subscription_quarterly_desc: String(
          config.subscription_quarterly_desc || "",
        ).trim(),
        subscription_quarterly_badge: String(
          config.subscription_quarterly_badge || "",
        ).trim(),
        subscription_quarterly_price_cents: Math.max(
          0,
          Number(config.subscription_quarterly_price_cents) || 0,
        ),
        subscription_quarterly_price_note: String(
          config.subscription_quarterly_price_note || "",
        ).trim(),
        subscription_quarterly_features: String(
          config.subscription_quarterly_features || "",
        ).trim(),
        subscription_yearly_name: String(
          config.subscription_yearly_name || "",
        ).trim(),
        subscription_yearly_desc: String(
          config.subscription_yearly_desc || "",
        ).trim(),
        subscription_yearly_badge: String(
          config.subscription_yearly_badge || "",
        ).trim(),
        subscription_yearly_price_cents: Math.max(
          0,
          Number(config.subscription_yearly_price_cents) || 0,
        ),
        subscription_yearly_price_note: String(
          config.subscription_yearly_price_note || "",
        ).trim(),
        subscription_yearly_features: String(
          config.subscription_yearly_features || "",
        ).trim(),
        yipay_enabled: Boolean(config.yipay_enabled),
        yipay_pid: String(config.yipay_pid || "").trim(),
        yipay_key: yiPayKey,
        yipay_submit_url: String(config.yipay_submit_url || "").trim(),
        yipay_notify_url: String(config.yipay_notify_url || "").trim(),
        yipay_return_url: String(config.yipay_return_url || "").trim(),
        yipay_site_name: String(config.yipay_site_name || "").trim(),
        paypal_enabled: Boolean(config.paypal_enabled),
        paypal_checkout_url: String(config.paypal_checkout_url || "").trim(),
        usdt_enabled: Boolean(config.usdt_enabled),
        usdt_network: String(config.usdt_network || "").trim(),
        usdt_address: String(config.usdt_address || "").trim(),
        usdt_payment_url: String(config.usdt_payment_url || "").trim(),
        image_r2_enabled: Boolean(config.image_r2_enabled),
        image_r2_endpoint: String(config.image_r2_endpoint || "").trim(),
        image_r2_bucket: String(config.image_r2_bucket || "").trim(),
        image_r2_region: String(config.image_r2_region || "auto").trim(),
        image_r2_access_key_id: String(
          config.image_r2_access_key_id || "",
        ).trim(),
        image_r2_secret_access_key: imageR2SecretAccessKey,
        image_r2_public_base_url: String(
          config.image_r2_public_base_url || "",
        ).trim(),
        image_r2_prefix: String(config.image_r2_prefix || "images").trim(),
        image_r2_secondary_enabled: Boolean(config.image_r2_secondary_enabled),
        image_r2_secondary_endpoint: String(
          config.image_r2_secondary_endpoint || "",
        ).trim(),
        image_r2_secondary_bucket: String(
          config.image_r2_secondary_bucket || "",
        ).trim(),
        image_r2_secondary_region: String(
          config.image_r2_secondary_region || "auto",
        ).trim(),
        image_r2_secondary_access_key_id: String(
          config.image_r2_secondary_access_key_id || "",
        ).trim(),
        image_r2_secondary_secret_access_key: imageSecondaryR2SecretAccessKey,
        image_r2_secondary_public_base_url: String(
          config.image_r2_secondary_public_base_url || "",
        ).trim(),
        image_r2_secondary_prefix: String(
          config.image_r2_secondary_prefix || "images",
        ).trim(),
        image_imgbed_enabled: Boolean(config.image_imgbed_enabled),
        image_imgbed_upload_url: String(
          config.image_imgbed_upload_url || "",
        ).trim(),
        image_imgbed_auth_code: imageImgBedAuthCode,
        image_imgbed_upload_channel: String(
          config.image_imgbed_upload_channel || "cfr2",
        ).trim(),
        linuxdo_enabled: Boolean(config.linuxdo_enabled),
        linuxdo_client_id: String(config.linuxdo_client_id || "").trim(),
        linuxdo_client_secret: linuxDoClientSecret,
        linuxdo_redirect_url: String(config.linuxdo_redirect_url || "").trim(),
        linuxdo_frontend_redirect_url: String(
          config.linuxdo_frontend_redirect_url || "",
        ).trim(),
        update_repo: String(
          config.update_repo ?? "ZyphrZero/chatgpt2api",
        ).trim(),
        update_github_token: updateGitHubToken,
      };
      if (!linuxDoClientSecret) {
        delete payload.linuxdo_client_secret;
      }
      if (!updateGitHubToken) {
        delete payload.update_github_token;
      }
      if (!cfTurnstileSecretKey) {
        delete payload.cf_turnstile_secret_key;
      }
      if (!emailSMTPAuthCode) {
        delete payload.email_smtp_auth_code;
      }
      if (!yiPayKey) {
        delete payload.yipay_key;
      }
      if (!imageR2SecretAccessKey) {
        delete payload.image_r2_secret_access_key;
      }
      if (!imageSecondaryR2SecretAccessKey) {
        delete payload.image_r2_secondary_secret_access_key;
      }
      if (!imageImgBedAuthCode) {
        delete payload.image_imgbed_auth_code;
      }
      delete payload.linuxdo_client_secret_configured;
      delete payload.update_github_token_configured;
      delete payload.cf_turnstile_secret_key_configured;
      delete payload.email_smtp_auth_code_configured;
      delete payload.yipay_key_configured;
      delete payload.image_r2_secret_access_key_configured;
      delete payload.image_r2_secondary_secret_access_key_configured;
      delete payload.image_imgbed_auth_code_configured;

      const data = await updateSettingsConfig(payload);
      const nextConfig = normalizeConfig(data.config);
      set({ config: nextConfig });
      dispatchAppMetaUpdated({
        app_title: String(nextConfig.brand_top_left_name || "GPT生图站"),
        project_name: String(nextConfig.brand_site_name || "GPT生图站"),
        top_left_logo_url: String(
          nextConfig.brand_top_left_logo_url || "/logo-mark.svg",
        ),
        site_logo_url: String(
          nextConfig.brand_site_logo_url || "/logo-mark.svg",
        ),
        show_ecommerce_entry: Boolean(nextConfig.show_ecommerce_entry),
        show_new_ecommerce_window_entry: Boolean(
          nextConfig.show_new_ecommerce_window_entry,
        ),
      });
      toast.success("配置已保存");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存系统配置失败");
    } finally {
      set({ isSavingConfig: false });
    }
  },

  setRefreshAccountIntervalMinute: (value) => {
    set((state) => {
      if (!state.config) {
        return {};
      }
      return {
        config: {
          ...state.config,
          refresh_account_interval_minute: value,
        },
      };
    });
  },

  setImageRetentionDays: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_retention_days: value } }
        : {},
    );
  },

  setLogRetentionDays: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, log_retention_days: value } }
        : {},
    );
  },

  setImageConcurrentLimit: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_concurrent_limit: value } }
        : {},
    );
  },

  setImageSingleCountLimit: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_single_count_limit: value } }
        : {},
    );
  },

  setImageTaskTimeoutSeconds: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_task_timeout_seconds: value } }
        : {},
    );
  },

  setUserDefaultConcurrentLimit: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, user_default_concurrent_limit: value } }
        : {},
    );
  },

  setUserDefaultRpmLimit: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, user_default_rpm_limit: value } }
        : {},
    );
  },

  setAutoRemoveInvalidAccounts: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, auto_remove_invalid_accounts: value } }
        : {},
    );
  },

  setAutoRemoveRateLimitedAccounts: (value) => {
    set((state) =>
      state.config
        ? {
            config: {
              ...state.config,
              auto_remove_rate_limited_accounts: value,
            },
          }
        : {},
    );
  },

  setLogLevel: (level, enabled) => {
    set((state) => {
      if (!state.config) return {};
      const levels = new Set(state.config.log_levels || []);
      if (enabled) levels.add(level);
      else levels.delete(level);
      return { config: { ...state.config, log_levels: Array.from(levels) } };
    });
  },

  setProxy: (value) => {
    set((state) => {
      if (!state.config) {
        return {};
      }
      return {
        config: {
          ...state.config,
          proxy: value,
        },
      };
    });
  },

  setBaseUrl: (value) => {
    set((state) => {
      if (!state.config) {
        return {};
      }
      return {
        config: {
          ...state.config,
          base_url: value,
        },
      };
    });
  },

  setBrandTopLeftName: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, brand_top_left_name: value } }
        : {},
    );
  },

  setBrandSiteName: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, brand_site_name: value } }
        : {},
    );
  },

  setBrandTopLeftLogoURL: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, brand_top_left_logo_url: value } }
        : {},
    );
  },

  setBrandSiteLogoURL: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, brand_site_logo_url: value } }
        : {},
    );
  },

  setRegistrationEnabled: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, registration_enabled: value } }
        : {},
    );
  },

  setRegistrationAllowedEmailDomains: (value) => {
    set((state) =>
      state.config
        ? {
            config: {
              ...state.config,
              registration_allowed_email_domains: value,
            },
          }
        : {},
    );
  },

  setRegistrationBonusImageTimes: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, registration_bonus_image_times: value } }
        : {},
    );
  },

  setShowEcommerceEntry: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, show_ecommerce_entry: value } }
        : {},
    );
  },

  setShowNewEcommerceWindowEntry: (value) => {
    set((state) =>
      state.config
        ? {
            config: {
              ...state.config,
              show_new_ecommerce_window_entry: value,
            },
          }
        : {},
    );
  },

  setCFTurnstileEnabled: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, cf_turnstile_enabled: value } }
        : {},
    );
  },

  setCFTurnstileSiteKey: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, cf_turnstile_site_key: value } }
        : {},
    );
  },

  setCFTurnstileSecretKey: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, cf_turnstile_secret_key: value } }
        : {},
    );
  },

  setEmailSMTPEnabled: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, email_smtp_enabled: value } }
        : {},
    );
  },

  setEmailSMTPHost: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, email_smtp_host: value } }
        : {},
    );
  },

  setEmailSMTPPort: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, email_smtp_port: value } }
        : {},
    );
  },

  setEmailSMTPUseSSL: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, email_smtp_use_ssl: value } }
        : {},
    );
  },

  setEmailSMTPUsername: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, email_smtp_username: value } }
        : {},
    );
  },

  setEmailSMTPAuthCode: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, email_smtp_auth_code: value } }
        : {},
    );
  },

  setEmailSMTPFromEmail: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, email_smtp_from_email: value } }
        : {},
    );
  },

  setEmailSMTPFromName: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, email_smtp_from_name: value } }
        : {},
    );
  },

  setImagePrice1K: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_price_1k_cents: value } }
        : {},
    );
  },

  setImagePrice2K: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_price_2k_cents: value } }
        : {},
    );
  },

  setImagePrice4K: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_price_4k_cents: value } }
        : {},
    );
  },

  setYiPayEnabled: (value) => {
    set((state) =>
      state.config ? { config: { ...state.config, yipay_enabled: value } } : {},
    );
  },

  setYiPayPID: (value) => {
    set((state) =>
      state.config ? { config: { ...state.config, yipay_pid: value } } : {},
    );
  },

  setYiPayKey: (value) => {
    set((state) =>
      state.config ? { config: { ...state.config, yipay_key: value } } : {},
    );
  },

  setYiPaySubmitUrl: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, yipay_submit_url: value } }
        : {},
    );
  },

  setYiPayNotifyUrl: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, yipay_notify_url: value } }
        : {},
    );
  },

  setYiPayReturnUrl: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, yipay_return_url: value } }
        : {},
    );
  },

  setYiPaySiteName: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, yipay_site_name: value } }
        : {},
    );
  },

  setImageR2Enabled: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_r2_enabled: value } }
        : {},
    );
  },

  setImageR2Endpoint: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_r2_endpoint: value } }
        : {},
    );
  },

  setImageR2Bucket: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_r2_bucket: value } }
        : {},
    );
  },

  setImageR2Region: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_r2_region: value } }
        : {},
    );
  },

  setImageR2AccessKeyId: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_r2_access_key_id: value } }
        : {},
    );
  },

  setImageR2SecretAccessKey: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_r2_secret_access_key: value } }
        : {},
    );
  },

  setImageR2PublicBaseUrl: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_r2_public_base_url: value } }
        : {},
    );
  },

  setImageR2Prefix: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, image_r2_prefix: value } }
        : {},
    );
  },

  setLinuxDoEnabled: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, linuxdo_enabled: value } }
        : {},
    );
  },

  setLinuxDoClientId: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, linuxdo_client_id: value } }
        : {},
    );
  },

  setLinuxDoClientSecret: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, linuxdo_client_secret: value } }
        : {},
    );
  },

  setLinuxDoRedirectUrl: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, linuxdo_redirect_url: value } }
        : {},
    );
  },

  setLinuxDoFrontendRedirectUrl: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, linuxdo_frontend_redirect_url: value } }
        : {},
    );
  },

  setUpdateRepo: (value) => {
    set((state) =>
      state.config ? { config: { ...state.config, update_repo: value } } : {},
    );
  },

  setUpdateGitHubToken: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, update_github_token: value } }
        : {},
    );
  },

  setConfigField: (key, value) => {
    set((state) => {
      if (!state.config) {
        return {};
      }
      return {
        config: {
          ...state.config,
          [key]: value,
        },
      };
    });
  },

  setLoginPageImageUrl: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, login_page_image_url: value } }
        : {},
    );
  },

  setLoginPageImageMode: (value) => {
    set((state) =>
      state.config
        ? { config: { ...state.config, login_page_image_mode: value } }
        : {},
    );
  },

  setLoginPageImageTransform: (transform) => {
    const normalized = normalizeLoginPageImageTransform(transform);
    set((state) =>
      state.config
        ? {
            config: {
              ...state.config,
              login_page_image_zoom: normalized.zoom,
              login_page_image_position_x: normalized.positionX,
              login_page_image_position_y: normalized.positionY,
            },
          }
        : {},
    );
  },

  restoreDefaultLoginPageImage: () => {
    set((state) =>
      state.config
        ? {
            config: {
              ...state.config,
              login_page_image_url: "",
              login_page_image_zoom: LOGIN_PAGE_IMAGE_DEFAULT_TRANSFORM.zoom,
              login_page_image_position_x:
                LOGIN_PAGE_IMAGE_DEFAULT_TRANSFORM.positionX,
              login_page_image_position_y:
                LOGIN_PAGE_IMAGE_DEFAULT_TRANSFORM.positionY,
            },
          }
        : {},
    );
  },

  saveLoginPageImage: async ({ file, action }) => {
    const { config } = get();
    if (!config) {
      return false;
    }
    const transform = normalizeLoginPageImageTransform({
      zoom: Number(config.login_page_image_zoom),
      positionX: Number(config.login_page_image_position_x),
      positionY: Number(config.login_page_image_position_y),
    });
    const settings: LoginPageImageSettings = {
      login_page_image_url: String(config.login_page_image_url || "").trim(),
      login_page_image_mode: normalizeLoginPageImageMode(
        config.login_page_image_mode,
      ),
      login_page_image_zoom: transform.zoom,
      login_page_image_position_x: transform.positionX,
      login_page_image_position_y: transform.positionY,
    };

    set({ isSavingConfig: true });
    try {
      const data = await updateLoginPageImageSettings(settings, {
        action,
        file,
      });
      const nextConfig = normalizeConfig(data.config);
      set({ config: nextConfig });
      dispatchAppMetaUpdated({
        app_title: String(nextConfig.brand_top_left_name || "GPT生图站"),
        project_name: String(nextConfig.brand_site_name || "GPT生图站"),
        top_left_logo_url: String(
          nextConfig.brand_top_left_logo_url || "/logo-mark.svg",
        ),
        site_logo_url: String(
          nextConfig.brand_site_logo_url || "/logo-mark.svg",
        ),
        login_page_image_url: String(nextConfig.login_page_image_url || ""),
        login_page_image_mode: normalizeLoginPageImageMode(
          nextConfig.login_page_image_mode,
        ),
        login_page_image_zoom: Number(nextConfig.login_page_image_zoom),
        login_page_image_position_x: Number(
          nextConfig.login_page_image_position_x,
        ),
        login_page_image_position_y: Number(
          nextConfig.login_page_image_position_y,
        ),
      });
      toast.success("登录页图片已保存");
      return true;
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "保存登录页图片失败",
      );
      return false;
    } finally {
      set({ isSavingConfig: false });
    }
  },

  loadLogGovernance: async (silent = false) => {
    if (!silent) set({ isLoadingLogGovernance: true });
    try {
      const data = await fetchLogGovernance();
      set({ logGovernance: data.governance });
    } catch (error) {
      if (!silent)
        toast.error(
          error instanceof Error ? error.message : "加载日志治理数据失败",
        );
    } finally {
      if (!silent) set({ isLoadingLogGovernance: false });
    }
  },

  cleanupLogsByRetention: async () => {
    const { config } = get();
    if (!config) {
      return;
    }
    const retentionDays = Math.min(
      3650,
      Math.max(1, Number(config.log_retention_days) || 7),
    );
    set({ isCleaningLogs: true });
    try {
      const data = await cleanupLogs(retentionDays);
      set({
        lastLogCleanup: data.cleanup,
        logGovernance: data.governance,
      });
      toast.success(`已清理 ${data.cleanup.deleted} 条历史日志`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "清理日志失败");
    } finally {
      set({ isCleaningLogs: false });
    }
  },

  loadRegister: async (silent = false) => {
    if (!silent) set({ isLoadingRegister: true });
    try {
      const data = await fetchRegisterConfig();
      set({ registerConfig: data.register });
    } catch (error) {
      if (!silent)
        toast.error(
          error instanceof Error ? error.message : "加载注册配置失败",
        );
    } finally {
      if (!silent) set({ isLoadingRegister: false });
    }
  },

  setRegisterConfig: (config) => {
    set({ registerConfig: config, isLoadingRegister: false });
  },

  setRegisterProxy: (value) => {
    set((state) =>
      state.registerConfig
        ? { registerConfig: { ...state.registerConfig, proxy: value } }
        : {},
    );
  },

  setRegisterTotal: (value) => {
    set((state) =>
      state.registerConfig
        ? {
            registerConfig: {
              ...state.registerConfig,
              total: Number(value) || 0,
            },
          }
        : {},
    );
  },

  setRegisterThreads: (value) => {
    set((state) =>
      state.registerConfig
        ? {
            registerConfig: {
              ...state.registerConfig,
              threads: Number(value) || 0,
            },
          }
        : {},
    );
  },

  setRegisterMode: (value) => {
    set((state) =>
      state.registerConfig
        ? { registerConfig: { ...state.registerConfig, mode: value } }
        : {},
    );
  },

  setRegisterTargetQuota: (value) => {
    set((state) =>
      state.registerConfig
        ? {
            registerConfig: {
              ...state.registerConfig,
              target_quota: Number(value) || 0,
            },
          }
        : {},
    );
  },

  setRegisterTargetAvailable: (value) => {
    set((state) =>
      state.registerConfig
        ? {
            registerConfig: {
              ...state.registerConfig,
              target_available: Number(value) || 0,
            },
          }
        : {},
    );
  },

  setRegisterCheckInterval: (value) => {
    set((state) =>
      state.registerConfig
        ? {
            registerConfig: {
              ...state.registerConfig,
              check_interval: Number(value) || 0,
            },
          }
        : {},
    );
  },

  setRegisterAccountCheckInterval: (value) => {
    set((state) =>
      state.registerConfig
        ? {
            registerConfig: {
              ...state.registerConfig,
              account_check_interval: Number(value) || 0,
            },
          }
        : {},
    );
  },

  setRegisterMailField: (key, value) => {
    set((state) =>
      state.registerConfig
        ? {
            registerConfig: {
              ...state.registerConfig,
              mail: { ...state.registerConfig.mail, [key]: Number(value) || 0 },
            },
          }
        : {},
    );
  },

  addRegisterProvider: () => {
    set((state) =>
      state.registerConfig
        ? {
            registerConfig: {
              ...state.registerConfig,
              mail: {
                ...state.registerConfig.mail,
                providers: [
                  ...(state.registerConfig.mail.providers || []),
                  {
                    enable: true,
                    type: "tempmail_lol",
                    api_key: "",
                    domain: [],
                  },
                ],
              },
            },
          }
        : {},
    );
  },

  updateRegisterProvider: (index, updates) => {
    set((state) => {
      if (!state.registerConfig) return {};
      const providers = [...(state.registerConfig.mail.providers || [])];
      providers[index] = { ...(providers[index] || {}), ...updates };
      return {
        registerConfig: {
          ...state.registerConfig,
          mail: { ...state.registerConfig.mail, providers },
        },
      };
    });
  },

  deleteRegisterProvider: (index) => {
    set((state) =>
      state.registerConfig
        ? {
            registerConfig: {
              ...state.registerConfig,
              mail: {
                ...state.registerConfig.mail,
                providers: (state.registerConfig.mail.providers || []).filter(
                  (_, itemIndex) => itemIndex !== index,
                ),
              },
            },
          }
        : {},
    );
  },

  saveRegister: async () => {
    const { registerConfig } = get();
    if (!registerConfig) return;
    try {
      set({ isSavingRegister: true });
      const data = await updateRegisterConfig({
        mail: registerConfig.mail,
        proxy: registerConfig.proxy.trim(),
        total: Math.max(1, Number(registerConfig.total) || 1),
        threads: Math.max(1, Number(registerConfig.threads) || 1),
        mode: registerConfig.mode,
        target_quota: Math.max(1, Number(registerConfig.target_quota) || 1),
        target_available: Math.max(
          1,
          Number(registerConfig.target_available) || 1,
        ),
        check_interval: Math.max(1, Number(registerConfig.check_interval) || 5),
        account_check_interval: Math.max(
          0,
          Number(registerConfig.account_check_interval) || 0,
        ),
      });
      set({ registerConfig: data.register });
      toast.success("注册配置已保存");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存注册配置失败");
    } finally {
      set({ isSavingRegister: false });
    }
  },

  toggleRegister: async () => {
    const { registerConfig } = get();
    if (!registerConfig) return;
    set({ isSavingRegister: true });
    try {
      if (!registerConfig.enabled) {
        await updateRegisterConfig({
          mail: registerConfig.mail,
          proxy: registerConfig.proxy.trim(),
          total: Math.max(1, Number(registerConfig.total) || 1),
          threads: Math.max(1, Number(registerConfig.threads) || 1),
          mode: registerConfig.mode,
          target_quota: Math.max(1, Number(registerConfig.target_quota) || 1),
          target_available: Math.max(
            1,
            Number(registerConfig.target_available) || 1,
          ),
          check_interval: Math.max(
            1,
            Number(registerConfig.check_interval) || 5,
          ),
          account_check_interval: Math.max(
            0,
            Number(registerConfig.account_check_interval) || 0,
          ),
        });
      }
      const data = registerConfig.enabled
        ? await stopRegister()
        : await startRegister();
      set({ registerConfig: data.register });
      toast.success(
        registerConfig.enabled ? "注册任务已停止" : "注册任务已启动",
      );
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "切换注册状态失败");
    } finally {
      set({ isSavingRegister: false });
    }
  },

  resetRegister: async () => {
    set({ isSavingRegister: true });
    try {
      const data = await resetRegisterApi();
      set({ registerConfig: data.register });
      toast.success("注册统计已重置");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "重置注册统计失败");
    } finally {
      set({ isSavingRegister: false });
    }
  },

  loadPools: async (silent = false) => {
    if (!silent) {
      set({ isLoadingPools: true });
    }
    try {
      const data = await fetchCPAPools();
      set({ pools: data.pools });
    } catch (error) {
      if (!silent) {
        toast.error(
          error instanceof Error ? error.message : "加载 CPA 连接失败",
        );
      }
    } finally {
      if (!silent) {
        set({ isLoadingPools: false });
      }
    }
  },

  openAddDialog: () => {
    set({
      editingPool: null,
      formName: "",
      formBaseUrl: "",
      formSecretKey: "",
      showSecret: false,
      dialogOpen: true,
    });
  },

  openEditDialog: (pool) => {
    set({
      editingPool: pool,
      formName: pool.name,
      formBaseUrl: pool.base_url,
      formSecretKey: "",
      showSecret: false,
      dialogOpen: true,
    });
  },

  setDialogOpen: (open) => {
    set({ dialogOpen: open });
  },

  setFormName: (value) => {
    set({ formName: value });
  },

  setFormBaseUrl: (value) => {
    set({ formBaseUrl: value });
  },

  setFormSecretKey: (value) => {
    set({ formSecretKey: value });
  },

  setShowSecret: (checked) => {
    set({ showSecret: checked });
  },

  savePool: async () => {
    const { editingPool, formName, formBaseUrl, formSecretKey } = get();
    if (!formBaseUrl.trim()) {
      toast.error("请输入 CPA 地址");
      return;
    }
    if (!editingPool && !formSecretKey.trim()) {
      toast.error("请输入 Secret Key");
      return;
    }

    set({ isSavingPool: true });
    try {
      if (editingPool) {
        const data = await updateCPAPool(editingPool.id, {
          name: formName.trim(),
          base_url: formBaseUrl.trim(),
          secret_key: formSecretKey.trim() || undefined,
        });
        set({ pools: data.pools, dialogOpen: false });
        toast.success("连接已更新");
      } else {
        const data = await createCPAPool({
          name: formName.trim(),
          base_url: formBaseUrl.trim(),
          secret_key: formSecretKey.trim(),
        });
        set({ pools: data.pools, dialogOpen: false });
        toast.success("连接已添加");
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存失败");
    } finally {
      set({ isSavingPool: false });
    }
  },

  deletePool: async (pool) => {
    set({ deletingId: pool.id });
    try {
      const data = await deleteCPAPool(pool.id);
      set({ pools: data.pools });
      toast.success("连接已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除失败");
    } finally {
      set({ deletingId: null });
    }
  },

  browseFiles: async (pool) => {
    set({ loadingFilesId: pool.id });
    try {
      const data = await fetchCPAPoolFiles(pool.id);
      const files = normalizeFiles(data.files);
      set({
        browserPool: pool,
        remoteFiles: files,
        selectedNames: [],
        fileQuery: "",
        filePage: 1,
        browserOpen: true,
      });
      toast.success(`读取成功，共 ${files.length} 个远程账号`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取远程账号失败");
    } finally {
      set({ loadingFilesId: null });
    }
  },

  setBrowserOpen: (open) => {
    set({ browserOpen: open });
  },

  toggleFile: (name, checked) => {
    set((state) => {
      if (checked) {
        return {
          selectedNames: Array.from(new Set([...state.selectedNames, name])),
        };
      }
      return {
        selectedNames: state.selectedNames.filter((item) => item !== name),
      };
    });
  },

  replaceSelectedNames: (names) => {
    set({ selectedNames: Array.from(new Set(names)) });
  },

  setFileQuery: (value) => {
    set({ fileQuery: value, filePage: 1 });
  },

  setFilePage: (page) => {
    set({ filePage: page });
  },

  setPageSize: (value) => {
    set({ pageSize: value, filePage: 1 });
  },

  startImport: async () => {
    const { browserPool, selectedNames, pools } = get();
    if (!browserPool) {
      return;
    }
    if (selectedNames.length === 0) {
      toast.error("请先选择要导入的账号");
      return;
    }

    set({ isStartingImport: true });
    try {
      const result = await startCPAImport(browserPool.id, selectedNames);
      set({
        pools: pools.map((pool) =>
          pool.id === browserPool.id
            ? { ...pool, import_job: result.import_job }
            : pool,
        ),
        browserOpen: false,
      });
      toast.success("导入任务已启动");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "启动导入失败");
    } finally {
      set({ isStartingImport: false });
    }
  },
}));
