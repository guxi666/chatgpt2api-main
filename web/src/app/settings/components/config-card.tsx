"use client";

import type { ReactNode } from "react";
import {
  CircleHelp,
  LoaderCircle,
  PlugZap,
  Save,
  Settings2,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { testProxy, type ProxyTestResult } from "@/lib/api";
import { cn } from "@/lib/utils";

import { useSettingsStore } from "../store";
import { SettingsCard, settingsInputClassName } from "./settings-ui";

const configSectionClassName = "flex flex-col gap-3";
const configFieldClassName = "min-w-0 gap-1.5";

function ConfigTip({ content }: { content: string }) {
  return (
    <span
      aria-label={content}
      title={content}
      className="inline-flex size-5 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
    >
      <CircleHelp className="size-4" />
    </span>
  );
}

function SectionHeading({
  action,
  tip,
  title,
}: {
  action?: ReactNode;
  tip: string;
  title: string;
}) {
  return (
    <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
      <div className="flex min-w-0 items-center gap-1.5">
        <h3 className="truncate text-sm leading-6 font-semibold text-foreground">
          {title}
        </h3>
        <ConfigTip content={tip} />
      </div>
      {action ? (
        <div className="flex w-full shrink-0 sm:w-auto sm:justify-end">
          {action}
        </div>
      ) : null}
    </div>
  );
}

function ConfigFieldLabel({
  children,
  htmlFor,
}: {
  children: ReactNode;
  htmlFor: string;
}) {
  return (
    <FieldLabel htmlFor={htmlFor} className="leading-6">
      {children}
    </FieldLabel>
  );
}

function ConfigOption({
  checked,
  label,
  onCheckedChange,
}: {
  checked: boolean;
  label: string;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <label className="flex min-h-10 min-w-0 items-center gap-2.5 rounded-[12px] border border-border/70 bg-background/75 px-3 py-2 text-sm font-medium text-foreground">
      <Checkbox
        checked={checked}
        onCheckedChange={(value) => onCheckedChange(Boolean(value))}
      />
      <span className="min-w-0 leading-5">{label}</span>
    </label>
  );
}

export function ConfigCard() {
  const [isTestingProxy, setIsTestingProxy] = useState(false);
  const [proxyTestResult, setProxyTestResult] =
    useState<ProxyTestResult | null>(null);
  const config = useSettingsStore((state) => state.config);
  const isLoadingConfig = useSettingsStore((state) => state.isLoadingConfig);
  const isSavingConfig = useSettingsStore((state) => state.isSavingConfig);
  const setRefreshAccountIntervalMinute = useSettingsStore(
    (state) => state.setRefreshAccountIntervalMinute,
  );
  const setImageConcurrentLimit = useSettingsStore(
    (state) => state.setImageConcurrentLimit,
  );
  const setImageSingleCountLimit = useSettingsStore(
    (state) => state.setImageSingleCountLimit,
  );
  const setImageTaskTimeoutSeconds = useSettingsStore(
    (state) => state.setImageTaskTimeoutSeconds,
  );
  const setUserDefaultConcurrentLimit = useSettingsStore(
    (state) => state.setUserDefaultConcurrentLimit,
  );
  const setUserDefaultRpmLimit = useSettingsStore(
    (state) => state.setUserDefaultRpmLimit,
  );
  const setImageRetentionDays = useSettingsStore(
    (state) => state.setImageRetentionDays,
  );
  const setAutoRemoveInvalidAccounts = useSettingsStore(
    (state) => state.setAutoRemoveInvalidAccounts,
  );
  const setAutoRemoveRateLimitedAccounts = useSettingsStore(
    (state) => state.setAutoRemoveRateLimitedAccounts,
  );
  const setProxy = useSettingsStore((state) => state.setProxy);
  const setBaseUrl = useSettingsStore((state) => state.setBaseUrl);
  const setRegistrationEnabled = useSettingsStore(
    (state) => state.setRegistrationEnabled,
  );
  const setRegistrationAllowedEmailDomains = useSettingsStore(
    (state) => state.setRegistrationAllowedEmailDomains,
  );
  const setRegistrationBonusImageTimes = useSettingsStore(
    (state) => state.setRegistrationBonusImageTimes,
  );
  const setShowEcommerceEntry = useSettingsStore(
    (state) => state.setShowEcommerceEntry,
  );
  const setShowNewEcommerceWindowEntry = useSettingsStore(
    (state) => state.setShowNewEcommerceWindowEntry,
  );
  const setCFTurnstileEnabled = useSettingsStore(
    (state) => state.setCFTurnstileEnabled,
  );
  const setCFTurnstileSiteKey = useSettingsStore(
    (state) => state.setCFTurnstileSiteKey,
  );
  const setCFTurnstileSecretKey = useSettingsStore(
    (state) => state.setCFTurnstileSecretKey,
  );
  const setEmailSMTPEnabled = useSettingsStore(
    (state) => state.setEmailSMTPEnabled,
  );
  const setEmailSMTPHost = useSettingsStore(
    (state) => state.setEmailSMTPHost,
  );
  const setEmailSMTPPort = useSettingsStore(
    (state) => state.setEmailSMTPPort,
  );
  const setEmailSMTPUseSSL = useSettingsStore(
    (state) => state.setEmailSMTPUseSSL,
  );
  const setEmailSMTPUsername = useSettingsStore(
    (state) => state.setEmailSMTPUsername,
  );
  const setEmailSMTPAuthCode = useSettingsStore(
    (state) => state.setEmailSMTPAuthCode,
  );
  const setEmailSMTPFromEmail = useSettingsStore(
    (state) => state.setEmailSMTPFromEmail,
  );
  const setEmailSMTPFromName = useSettingsStore(
    (state) => state.setEmailSMTPFromName,
  );
  const setImagePrice1K = useSettingsStore((state) => state.setImagePrice1K);
  const setImagePrice2K = useSettingsStore((state) => state.setImagePrice2K);
  const setImagePrice4K = useSettingsStore((state) => state.setImagePrice4K);
  const saveConfig = useSettingsStore((state) => state.saveConfig);

  const handleTestProxy = async () => {
    const candidate = String(config?.proxy || "").trim();
    if (!candidate) {
      toast.error("请先填写代理地址");
      return;
    }
    setIsTestingProxy(true);
    setProxyTestResult(null);
    try {
      const data = await testProxy(candidate);
      setProxyTestResult(data.result);
      if (data.result.ok) {
        toast.success(
          `代理可用（${data.result.latency_ms} ms，HTTP ${data.result.status}）`,
        );
      } else {
        toast.error(`代理不可用：${data.result.error ?? "未知错误"}`);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "测试代理失败");
    } finally {
      setIsTestingProxy(false);
    }
  };

  if (isLoadingConfig && !config) {
    return (
      <SettingsCard
        icon={Settings2}
        title="系统配置"
        description="调整账号刷新、代理和图片任务。"
      >
        <div className="flex items-center justify-center py-10">
          <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
        </div>
      </SettingsCard>
    );
  }

  return (
    <SettingsCard
      icon={Settings2}
      title="系统配置"
      description="调整账号刷新、代理和图片任务。"
      action={
        <Button
          size="lg"
          onClick={() => void saveConfig()}
          disabled={isSavingConfig}
        >
          {isSavingConfig ? (
            <LoaderCircle data-icon="inline-start" className="animate-spin" />
          ) : (
            <Save data-icon="inline-start" />
          )}
          保存
        </Button>
      }
    >
      <div className="flex flex-col gap-5">
        <section className={configSectionClassName}>
          <SectionHeading
            title="基础参数"
            tip="账号刷新间隔单位分钟；图片访问地址是图片结果访问前缀；同时生成张数控制后台生成槽位；任务超时时间单位秒；图片自动清理会删除指定天数前的本地图片。"
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-refresh-interval">
                账号刷新间隔
              </ConfigFieldLabel>
              <Input
                id="settings-refresh-interval"
                value={String(config?.refresh_account_interval_minute || "")}
                onChange={(event) =>
                  setRefreshAccountIntervalMinute(event.target.value)
                }
                placeholder="分钟"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-base-url">
                图片访问地址
              </ConfigFieldLabel>
              <Input
                id="settings-base-url"
                value={String(config?.base_url || "")}
                onChange={(event) => setBaseUrl(event.target.value)}
                placeholder="https://example.com"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-image-concurrent-limit">
                同时生成张数
              </ConfigFieldLabel>
              <Input
                id="settings-image-concurrent-limit"
                value={String(config?.image_concurrent_limit || "")}
                onChange={(event) =>
                  setImageConcurrentLimit(event.target.value)
                }
                placeholder="4"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-image-single-count-limit">
                单次出图数量限制
              </ConfigFieldLabel>
              <Input
                id="settings-image-single-count-limit"
                value={String(config?.image_single_count_limit || "")}
                onChange={(event) =>
                  setImageSingleCountLimit(event.target.value)
                }
                placeholder="10"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-image-retention-days">
                图片自动清理
              </ConfigFieldLabel>
              <Input
                id="settings-image-retention-days"
                value={String(config?.image_retention_days || "")}
                onChange={(event) => setImageRetentionDays(event.target.value)}
                placeholder="30"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-image-task-timeout-seconds">
                任务超时时间
              </ConfigFieldLabel>
              <Input
                id="settings-image-task-timeout-seconds"
                value={String(config?.image_task_timeout_seconds || "")}
                onChange={(event) =>
                  setImageTaskTimeoutSeconds(event.target.value)
                }
                placeholder="300"
                className={settingsInputClassName}
              />
            </Field>
          </div>
        </section>

        <section className={configSectionClassName}>
          <SectionHeading
            title="用户默认限制"
            tip="限制普通用户创建图片任务的默认并发和速率，管理员不受影响；0 表示不限制。"
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-user-default-concurrent-limit">
                用户默认并发
              </ConfigFieldLabel>
              <Input
                id="settings-user-default-concurrent-limit"
                value={String(config?.user_default_concurrent_limit ?? "")}
                onChange={(event) =>
                  setUserDefaultConcurrentLimit(event.target.value)
                }
                placeholder="0"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-user-default-rpm-limit">
                用户默认 RPM
              </ConfigFieldLabel>
              <Input
                id="settings-user-default-rpm-limit"
                value={String(config?.user_default_rpm_limit ?? "")}
                onChange={(event) =>
                  setUserDefaultRpmLimit(event.target.value)
                }
                placeholder="0"
                className={settingsInputClassName}
              />
            </Field>
          </div>
        </section>

        <section className={configSectionClassName}>
          <SectionHeading
            title="出站代理"
            tip="留空表示不使用代理。"
            action={
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="w-full sm:w-auto"
                onClick={() => void handleTestProxy()}
                disabled={isTestingProxy}
              >
                {isTestingProxy ? (
                  <LoaderCircle
                    data-icon="inline-start"
                    className="animate-spin"
                  />
                ) : (
                  <PlugZap data-icon="inline-start" />
                )}
                测试代理
              </Button>
            }
          />
          <Field className="gap-1.5">
            <ConfigFieldLabel htmlFor="settings-proxy">
              全局代理
            </ConfigFieldLabel>
            <Input
              id="settings-proxy"
              value={String(config?.proxy || "")}
              onChange={(event) => {
                setProxy(event.target.value);
                setProxyTestResult(null);
              }}
              placeholder="http://127.0.0.1:7890"
              className={settingsInputClassName}
            />
            {proxyTestResult ? (
              <div
                className={cn(
                  "rounded-[13px] border px-3 py-2 text-xs leading-5",
                  proxyTestResult.ok
                    ? "border-emerald-200 bg-emerald-50 text-emerald-800"
                    : "border-rose-200 bg-rose-50 text-rose-800",
                )}
              >
                {proxyTestResult.ok
                  ? `代理可用：HTTP ${proxyTestResult.status}，用时 ${proxyTestResult.latency_ms} ms`
                  : `代理不可用：${proxyTestResult.error ?? "未知错误"}（用时 ${proxyTestResult.latency_ms} ms）`}
              </div>
            ) : null}
          </Field>
        </section>

        <section className={configSectionClassName}>
          <SectionHeading
            title="账号入口"
            tip="开启后登录页会显示账号注册入口，新账号默认绑定普通用户角色。"
          />
          <div className="grid gap-2 sm:grid-cols-2">
            <ConfigOption
              checked={Boolean(config?.registration_enabled)}
              onCheckedChange={setRegistrationEnabled}
              label="开放账号注册"
            />
            <ConfigOption
              checked={config?.show_ecommerce_entry !== false}
              onCheckedChange={setShowEcommerceEntry}
              label="显示电商专区入口"
            />
            <ConfigOption
              checked={config?.show_new_ecommerce_window_entry !== false}
              onCheckedChange={setShowNewEcommerceWindowEntry}
              label="显示新建电商窗口"
            />
          </div>
        </section>

        <section className={configSectionClassName}>
          <SectionHeading
            title="邮箱注册与 CF 验证"
            tip="配置注册邮箱白名单和 Cloudflare Turnstile。保存后会应用到登录/注册。"
          />
          <div className="grid gap-2 sm:grid-cols-2">
            <ConfigOption
              checked={Boolean(config?.cf_turnstile_enabled)}
              label="启用 Cloudflare 验证"
              onCheckedChange={setCFTurnstileEnabled}
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-registration-email-domains">
                邮箱白名单域名
              </ConfigFieldLabel>
              <Input
                id="settings-registration-email-domains"
                value={String(config?.registration_allowed_email_domains || "")}
                onChange={(event) =>
                  setRegistrationAllowedEmailDomains(event.target.value)
                }
                placeholder="qq.com,163.com,gmail.com,outlook.com"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-registration-bonus-image-times">
                新用户注册赠送次数
              </ConfigFieldLabel>
              <Input
                id="settings-registration-bonus-image-times"
                value={String(config?.registration_bonus_image_times ?? "")}
                onChange={(event) =>
                  setRegistrationBonusImageTimes(event.target.value)
                }
                placeholder="20"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-cf-turnstile-site-key">
                Turnstile Site Key
              </ConfigFieldLabel>
              <Input
                id="settings-cf-turnstile-site-key"
                value={String(config?.cf_turnstile_site_key || "")}
                onChange={(event) => setCFTurnstileSiteKey(event.target.value)}
                placeholder="0x4AAAAA..."
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-cf-turnstile-secret-key">
                Turnstile Secret Key
              </ConfigFieldLabel>
              <Input
                id="settings-cf-turnstile-secret-key"
                type="password"
                value={String(config?.cf_turnstile_secret_key || "")}
                onChange={(event) =>
                  setCFTurnstileSecretKey(event.target.value)
                }
                placeholder={
                  config?.cf_turnstile_secret_key_configured
                    ? "已配置，留空表示不修改"
                    : "0x4AAAAA..."
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-image-price-1k">
                1K 单价（元）
              </ConfigFieldLabel>
              <Input
                id="settings-image-price-1k"
                value={String(config?.image_price_1k_cents || "")}
                onChange={(event) => setImagePrice1K(event.target.value)}
                placeholder="0.08"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-image-price-2k">
                2K 单价（元）
              </ConfigFieldLabel>
              <Input
                id="settings-image-price-2k"
                value={String(config?.image_price_2k_cents || "")}
                onChange={(event) => setImagePrice2K(event.target.value)}
                placeholder="0.16"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-image-price-4k">
                4K 单价（元）
              </ConfigFieldLabel>
              <Input
                id="settings-image-price-4k"
                value={String(config?.image_price_4k_cents || "")}
                onChange={(event) => setImagePrice4K(event.target.value)}
                placeholder="0.32"
                className={settingsInputClassName}
              />
            </Field>
          </div>
        </section>

        <section className={configSectionClassName}>
          <SectionHeading
            title="邮件发信配置"
            tip="用于注册时的邮箱验证码发送，支持 QQ SMTP。修改后立即生效。"
          />
          <div className="grid gap-2 sm:grid-cols-2">
            <ConfigOption
              checked={Boolean(config?.email_smtp_enabled)}
              onCheckedChange={setEmailSMTPEnabled}
              label="启用 SMTP 发信"
            />
            <ConfigOption
              checked={Boolean(config?.email_smtp_use_ssl)}
              onCheckedChange={setEmailSMTPUseSSL}
              label="使用 SSL（QQ 邮箱建议开启）"
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-email-smtp-host">
                SMTP 主机
              </ConfigFieldLabel>
              <Input
                id="settings-email-smtp-host"
                value={String(config?.email_smtp_host || "")}
                onChange={(event) => setEmailSMTPHost(event.target.value)}
                placeholder="smtp.qq.com"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-email-smtp-port">
                SMTP 端口
              </ConfigFieldLabel>
              <Input
                id="settings-email-smtp-port"
                value={String(config?.email_smtp_port || "")}
                onChange={(event) => setEmailSMTPPort(event.target.value)}
                placeholder="465"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-email-smtp-username">
                发信账号
              </ConfigFieldLabel>
              <Input
                id="settings-email-smtp-username"
                value={String(config?.email_smtp_username || "")}
                onChange={(event) => setEmailSMTPUsername(event.target.value)}
                placeholder="your_account@qq.com"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-email-smtp-auth-code">
                授权码
              </ConfigFieldLabel>
              <Input
                id="settings-email-smtp-auth-code"
                type="password"
                value={String(config?.email_smtp_auth_code || "")}
                onChange={(event) => setEmailSMTPAuthCode(event.target.value)}
                placeholder={
                  config?.email_smtp_auth_code_configured
                    ? "已配置，不修改请留空"
                    : "your_qq_smtp_auth_code"
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-email-smtp-from-email">
                发件邮箱
              </ConfigFieldLabel>
              <Input
                id="settings-email-smtp-from-email"
                value={String(config?.email_smtp_from_email || "")}
                onChange={(event) => setEmailSMTPFromEmail(event.target.value)}
                placeholder="your_account@qq.com"
                className={settingsInputClassName}
              />
            </Field>
            <Field className={configFieldClassName}>
              <ConfigFieldLabel htmlFor="settings-email-smtp-from-name">
                发件名称
              </ConfigFieldLabel>
              <Input
                id="settings-email-smtp-from-name"
                value={String(config?.email_smtp_from_name || "")}
                onChange={(event) => setEmailSMTPFromName(event.target.value)}
                placeholder="GPT生图站"
                className={settingsInputClassName}
              />
            </Field>
          </div>
        </section>

        <section className={configSectionClassName}>
          <SectionHeading
            title="自动维护"
            tip="账号异常或限流时自动从号池移除。"
          />
          <div className="grid gap-2 sm:grid-cols-2">
            <ConfigOption
              checked={Boolean(config?.auto_remove_invalid_accounts)}
              onCheckedChange={setAutoRemoveInvalidAccounts}
              label="自动移除异常账号"
            />
            <ConfigOption
              checked={Boolean(config?.auto_remove_rate_limited_accounts)}
              onCheckedChange={setAutoRemoveRateLimitedAccounts}
              label="自动移除限流账号"
            />
          </div>
        </section>

      </div>
    </SettingsCard>
  );
}
