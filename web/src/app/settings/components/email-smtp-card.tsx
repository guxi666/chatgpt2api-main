"use client";

import { LoaderCircle, Mail, Save } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

import { useSettingsStore } from "../store";
import { SettingsCard, settingsInputClassName } from "./settings-ui";

export function EmailSMTPCard() {
  const config = useSettingsStore((state) => state.config);
  const isLoadingConfig = useSettingsStore((state) => state.isLoadingConfig);
  const isSavingConfig = useSettingsStore((state) => state.isSavingConfig);
  const saveConfig = useSettingsStore((state) => state.saveConfig);
  const setEmailSMTPEnabled = useSettingsStore((state) => state.setEmailSMTPEnabled);
  const setEmailSMTPHost = useSettingsStore((state) => state.setEmailSMTPHost);
  const setEmailSMTPPort = useSettingsStore((state) => state.setEmailSMTPPort);
  const setEmailSMTPUseSSL = useSettingsStore((state) => state.setEmailSMTPUseSSL);
  const setEmailSMTPUsername = useSettingsStore((state) => state.setEmailSMTPUsername);
  const setEmailSMTPAuthCode = useSettingsStore((state) => state.setEmailSMTPAuthCode);
  const setEmailSMTPFromEmail = useSettingsStore((state) => state.setEmailSMTPFromEmail);
  const setEmailSMTPFromName = useSettingsStore((state) => state.setEmailSMTPFromName);

  if (isLoadingConfig) {
    return (
      <SettingsCard icon={Mail} title="邮件发信配置" description="配置 SMTP 发送邮箱（推荐 QQ 邮箱）">
        <div className="flex items-center justify-center py-10">
          <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
        </div>
      </SettingsCard>
    );
  }

  const smtpEnabled = Boolean(config?.email_smtp_enabled);
  const authCodeConfigured = Boolean(config?.email_smtp_auth_code_configured);

  return (
    <SettingsCard
      icon={Mail}
      title="邮件发信配置"
      description="用于验证码、通知邮件等后续功能。QQ 邮箱推荐 smtp.qq.com:465 + SSL。"
      action={(
        <Button size="lg" onClick={() => void saveConfig()} disabled={isSavingConfig}>
          {isSavingConfig ? (
            <LoaderCircle data-icon="inline-start" className="animate-spin" />
          ) : (
            <Save data-icon="inline-start" />
          )}
          保存
        </Button>
      )}
    >
      <div className="flex flex-col gap-3">
        <label className="flex min-h-11 items-center gap-3 rounded-[13px] border border-border/80 bg-background px-3 py-2 text-sm">
          <Checkbox checked={smtpEnabled} onCheckedChange={(value) => setEmailSMTPEnabled(Boolean(value))} />
          启用 SMTP 发信
        </label>

        <div className="grid gap-3 sm:grid-cols-2">
          <Field className="gap-1.5">
            <FieldLabel htmlFor="email-smtp-host">SMTP 主机</FieldLabel>
            <Input
              id="email-smtp-host"
              value={String(config?.email_smtp_host || "")}
              onChange={(event) => setEmailSMTPHost(event.target.value)}
              placeholder="smtp.qq.com"
              className={settingsInputClassName}
            />
          </Field>

          <Field className="gap-1.5">
            <FieldLabel htmlFor="email-smtp-port">SMTP 端口</FieldLabel>
            <Input
              id="email-smtp-port"
              value={String(config?.email_smtp_port || "")}
              onChange={(event) => setEmailSMTPPort(event.target.value)}
              placeholder="465"
              className={settingsInputClassName}
            />
          </Field>

          <Field className="gap-1.5">
            <FieldLabel htmlFor="email-smtp-username">发信账号</FieldLabel>
            <Input
              id="email-smtp-username"
              value={String(config?.email_smtp_username || "")}
              onChange={(event) => setEmailSMTPUsername(event.target.value)}
              placeholder="your_account@qq.com"
              className={settingsInputClassName}
            />
          </Field>

          <Field className="gap-1.5">
            <FieldLabel htmlFor="email-smtp-auth-code">
              授权码 {authCodeConfigured ? "(已配置)" : "(未配置)"}
            </FieldLabel>
            <Input
              id="email-smtp-auth-code"
              type="password"
              value={String(config?.email_smtp_auth_code || "")}
              onChange={(event) => setEmailSMTPAuthCode(event.target.value)}
              placeholder="不修改可留空"
              className={settingsInputClassName}
            />
          </Field>

          <Field className="gap-1.5">
            <FieldLabel htmlFor="email-smtp-from-email">发件邮箱</FieldLabel>
            <Input
              id="email-smtp-from-email"
              value={String(config?.email_smtp_from_email || "")}
              onChange={(event) => setEmailSMTPFromEmail(event.target.value)}
              placeholder="your_account@qq.com"
              className={settingsInputClassName}
            />
          </Field>

          <Field className="gap-1.5">
            <FieldLabel htmlFor="email-smtp-from-name">发件名称</FieldLabel>
            <Input
              id="email-smtp-from-name"
              value={String(config?.email_smtp_from_name || "")}
              onChange={(event) => setEmailSMTPFromName(event.target.value)}
              placeholder="chatgpt2api"
              className={settingsInputClassName}
            />
          </Field>
        </div>

        <label className="flex min-h-11 items-center gap-3 rounded-[13px] border border-border/80 bg-background px-3 py-2 text-sm">
          <Checkbox
            checked={Boolean(config?.email_smtp_use_ssl)}
            onCheckedChange={(value) => setEmailSMTPUseSSL(Boolean(value))}
          />
          使用 SSL（QQ 邮箱建议开启）
        </label>
      </div>
    </SettingsCard>
  );
}
