"use client";

import { Palette, LoaderCircle, Save } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

import { useSettingsStore } from "../store";
import { SettingsCard, settingsInputClassName } from "./settings-ui";

export function SiteBrandCard() {
  const config = useSettingsStore((state) => state.config);
  const isLoadingConfig = useSettingsStore((state) => state.isLoadingConfig);
  const isSavingConfig = useSettingsStore((state) => state.isSavingConfig);

  const setBrandTopLeftName = useSettingsStore((state) => state.setBrandTopLeftName);
  const setBrandSiteName = useSettingsStore((state) => state.setBrandSiteName);
  const setBrandTopLeftLogoURL = useSettingsStore((state) => state.setBrandTopLeftLogoURL);
  const setBrandSiteLogoURL = useSettingsStore((state) => state.setBrandSiteLogoURL);
  const saveConfig = useSettingsStore((state) => state.saveConfig);

  if (isLoadingConfig && !config) {
    return (
      <SettingsCard icon={Palette} title="站点品牌" tone="violet">
        <div className="flex items-center justify-center py-10">
          <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
        </div>
      </SettingsCard>
    );
  }

  return (
    <SettingsCard
      icon={Palette}
      title="站点品牌"
      tone="violet"
      action={
        <Button size="lg" onClick={() => void saveConfig()} disabled={isSavingConfig}>
          {isSavingConfig ? <LoaderCircle data-icon="inline-start" className="animate-spin" /> : <Save data-icon="inline-start" />}
          保存
        </Button>
      }
    >
      <div className="grid gap-3 sm:grid-cols-2">
        <Field className="gap-1.5">
          <FieldLabel htmlFor="settings-brand-top-left-name">左上角名称</FieldLabel>
          <Input
            id="settings-brand-top-left-name"
            value={String(config?.brand_top_left_name || "")}
            onChange={(event) => setBrandTopLeftName(event.target.value)}
            placeholder="GPT生图站"
            className={settingsInputClassName}
          />
        </Field>

        <Field className="gap-1.5">
          <FieldLabel htmlFor="settings-brand-site-name">站点名称</FieldLabel>
          <Input
            id="settings-brand-site-name"
            value={String(config?.brand_site_name || "")}
            onChange={(event) => setBrandSiteName(event.target.value)}
            placeholder="GPT生图站"
            className={settingsInputClassName}
          />
        </Field>

        <Field className="gap-1.5">
          <FieldLabel htmlFor="settings-brand-top-left-logo-url">左上角图标 URL</FieldLabel>
          <Input
            id="settings-brand-top-left-logo-url"
            value={String(config?.brand_top_left_logo_url || "")}
            onChange={(event) => setBrandTopLeftLogoURL(event.target.value)}
            placeholder="/logo-mark.svg 或 https://example.com/logo.png"
            className={settingsInputClassName}
          />
        </Field>

        <Field className="gap-1.5">
          <FieldLabel htmlFor="settings-brand-site-logo-url">站点图标 URL</FieldLabel>
          <Input
            id="settings-brand-site-logo-url"
            value={String(config?.brand_site_logo_url || "")}
            onChange={(event) => setBrandSiteLogoURL(event.target.value)}
            placeholder="/logo-mark.svg 或 https://example.com/favicon.ico"
            className={settingsInputClassName}
          />
        </Field>
      </div>
    </SettingsCard>
  );
}
