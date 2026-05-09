"use client";

import { CreditCard, LoaderCircle, Save } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

import { useSettingsStore } from "../store";
import { SettingsCard, settingsInputClassName } from "./settings-ui";

export function PaymentSettingsCard() {
  const config = useSettingsStore((state) => state.config);
  const isLoadingConfig = useSettingsStore((state) => state.isLoadingConfig);
  const isSavingConfig = useSettingsStore((state) => state.isSavingConfig);
  const saveConfig = useSettingsStore((state) => state.saveConfig);

  const setYiPayEnabled = useSettingsStore((state) => state.setYiPayEnabled);
  const setYiPayPID = useSettingsStore((state) => state.setYiPayPID);
  const setYiPayKey = useSettingsStore((state) => state.setYiPayKey);
  const setYiPaySubmitUrl = useSettingsStore((state) => state.setYiPaySubmitUrl);
  const setYiPayNotifyUrl = useSettingsStore((state) => state.setYiPayNotifyUrl);
  const setYiPayReturnUrl = useSettingsStore((state) => state.setYiPayReturnUrl);
  const setYiPaySiteName = useSettingsStore((state) => state.setYiPaySiteName);

  if (isLoadingConfig) {
    return (
      <SettingsCard icon={CreditCard} title="支付配置">
        <div className="flex items-center justify-center py-10">
          <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
        </div>
      </SettingsCard>
    );
  }

  const yipayKeyConfigured = Boolean(config?.yipay_key_configured);

  return (
    <SettingsCard
      icon={CreditCard}
      title="支付配置"
      action={(
        <Button size="lg" onClick={() => void saveConfig()} disabled={isSavingConfig}>
          {isSavingConfig ? <LoaderCircle data-icon="inline-start" className="animate-spin" /> : <Save data-icon="inline-start" />}
          保存
        </Button>
      )}
    >
      <div className="flex flex-col gap-5">
        <section className="flex flex-col gap-3 rounded-[14px] border border-border/80 p-4">
          <label className="flex min-h-10 items-center gap-3 rounded-[12px] border border-border/70 bg-background px-3 py-2 text-sm">
            <Checkbox checked={Boolean(config?.yipay_enabled)} onCheckedChange={(value) => setYiPayEnabled(Boolean(value))} />
            启用易支付
          </label>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field className="gap-1.5">
              <FieldLabel htmlFor="pay-yipay-pid">商户 PID</FieldLabel>
              <Input id="pay-yipay-pid" value={String(config?.yipay_pid || "")} onChange={(event) => setYiPayPID(event.target.value)} className={settingsInputClassName} />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="pay-yipay-key">商户 KEY {yipayKeyConfigured ? "(已配置)" : "(未配置)"}</FieldLabel>
              <Input id="pay-yipay-key" type="password" value={String(config?.yipay_key || "")} onChange={(event) => setYiPayKey(event.target.value)} placeholder="不修改可留空" className={settingsInputClassName} />
            </Field>
            <Field className="gap-1.5 sm:col-span-2">
              <FieldLabel htmlFor="pay-yipay-submit-url">提交地址</FieldLabel>
              <Input id="pay-yipay-submit-url" value={String(config?.yipay_submit_url || "")} onChange={(event) => setYiPaySubmitUrl(event.target.value)} placeholder="https://pay.example.com/submit.php" className={settingsInputClassName} />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="pay-yipay-notify-url">异步回调地址</FieldLabel>
              <Input id="pay-yipay-notify-url" value={String(config?.yipay_notify_url || "")} onChange={(event) => setYiPayNotifyUrl(event.target.value)} placeholder="https://gpt.example.com/api/pay/yipay/notify" className={settingsInputClassName} />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="pay-yipay-return-url">同步返回地址</FieldLabel>
              <Input id="pay-yipay-return-url" value={String(config?.yipay_return_url || "")} onChange={(event) => setYiPayReturnUrl(event.target.value)} placeholder="https://gpt.example.com/wallet" className={settingsInputClassName} />
            </Field>
            <Field className="gap-1.5 sm:col-span-2">
              <FieldLabel htmlFor="pay-yipay-site-name">站点名称</FieldLabel>
              <Input id="pay-yipay-site-name" value={String(config?.yipay_site_name || "")} onChange={(event) => setYiPaySiteName(event.target.value)} className={settingsInputClassName} />
            </Field>
          </div>
        </section>
      </div>
    </SettingsCard>
  );
}
