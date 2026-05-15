"use client";

import { CloudUpload, LoaderCircle, Save } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

import { useSettingsStore } from "../store";
import { SettingsCard, SettingsNotice, settingsInputClassName } from "./settings-ui";

export function R2StorageCard() {
  const config = useSettingsStore((state) => state.config);
  const isLoadingConfig = useSettingsStore((state) => state.isLoadingConfig);
  const isSavingConfig = useSettingsStore((state) => state.isSavingConfig);
  const saveConfig = useSettingsStore((state) => state.saveConfig);
  const setImageR2Enabled = useSettingsStore((state) => state.setImageR2Enabled);
  const setImageR2Endpoint = useSettingsStore((state) => state.setImageR2Endpoint);
  const setImageR2Bucket = useSettingsStore((state) => state.setImageR2Bucket);
  const setImageR2Region = useSettingsStore((state) => state.setImageR2Region);
  const setImageR2AccessKeyId = useSettingsStore((state) => state.setImageR2AccessKeyId);
  const setImageR2SecretAccessKey = useSettingsStore((state) => state.setImageR2SecretAccessKey);
  const setImageR2PublicBaseUrl = useSettingsStore((state) => state.setImageR2PublicBaseUrl);
  const setImageR2Prefix = useSettingsStore((state) => state.setImageR2Prefix);

  if (isLoadingConfig) {
    return (
      <SettingsCard icon={CloudUpload} title="Cloudflare R2 图床">
        <div className="flex items-center justify-center py-10">
          <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
        </div>
      </SettingsCard>
    );
  }

  const secretConfigured = Boolean(config?.image_r2_secret_access_key_configured);

  return (
    <SettingsCard
      icon={CloudUpload}
      title="Cloudflare R2 图床"
      tone="amber"
      action={(
        <Button size="lg" onClick={() => void saveConfig()} disabled={isSavingConfig}>
          {isSavingConfig ? <LoaderCircle data-icon="inline-start" className="animate-spin" /> : <Save data-icon="inline-start" />}
          保存
        </Button>
      )}
    >
      <div className="flex flex-col gap-5">
        <label className="flex min-h-10 items-center gap-3 rounded-[12px] border border-border/70 bg-background px-3 py-2 text-sm">
          <Checkbox checked={Boolean(config?.image_r2_enabled)} onCheckedChange={(value) => setImageR2Enabled(Boolean(value))} />
          开启生成图片后自动上传到 Cloudflare R2
        </label>

        <div className="grid gap-3 sm:grid-cols-2">
          <Field className="gap-1.5 sm:col-span-2">
            <FieldLabel htmlFor="r2-endpoint">R2 S3 Endpoint</FieldLabel>
            <Input
              id="r2-endpoint"
              value={String(config?.image_r2_endpoint || "")}
              onChange={(event) => setImageR2Endpoint(event.target.value)}
              placeholder="https://<account_id>.r2.cloudflarestorage.com"
              className={settingsInputClassName}
            />
          </Field>
          <Field className="gap-1.5">
            <FieldLabel htmlFor="r2-bucket">Bucket 名称</FieldLabel>
            <Input id="r2-bucket" value={String(config?.image_r2_bucket || "")} onChange={(event) => setImageR2Bucket(event.target.value)} className={settingsInputClassName} />
          </Field>
          <Field className="gap-1.5">
            <FieldLabel htmlFor="r2-region">Region</FieldLabel>
            <Input id="r2-region" value={String(config?.image_r2_region || "auto")} onChange={(event) => setImageR2Region(event.target.value)} placeholder="auto" className={settingsInputClassName} />
          </Field>
          <Field className="gap-1.5">
            <FieldLabel htmlFor="r2-access-key">Access Key ID</FieldLabel>
            <Input id="r2-access-key" value={String(config?.image_r2_access_key_id || "")} onChange={(event) => setImageR2AccessKeyId(event.target.value)} className={settingsInputClassName} />
          </Field>
          <Field className="gap-1.5">
            <FieldLabel htmlFor="r2-secret-key">Secret Access Key {secretConfigured ? "(已配置)" : "(未配置)"}</FieldLabel>
            <Input
              id="r2-secret-key"
              type="password"
              value={String(config?.image_r2_secret_access_key || "")}
              onChange={(event) => setImageR2SecretAccessKey(event.target.value)}
              placeholder="不修改可留空"
              className={settingsInputClassName}
            />
          </Field>
          <Field className="gap-1.5">
            <FieldLabel htmlFor="r2-public-base-url">图床访问域名</FieldLabel>
            <Input
              id="r2-public-base-url"
              value={String(config?.image_r2_public_base_url || "")}
              onChange={(event) => setImageR2PublicBaseUrl(event.target.value)}
              placeholder="https://img.example.com"
              className={settingsInputClassName}
            />
          </Field>
          <Field className="gap-1.5">
            <FieldLabel htmlFor="r2-prefix">对象前缀</FieldLabel>
            <Input id="r2-prefix" value={String(config?.image_r2_prefix || "images")} onChange={(event) => setImageR2Prefix(event.target.value)} placeholder="images" className={settingsInputClassName} />
          </Field>
        </div>

        <SettingsNotice>
          上传成功后会默认删除服务器本地原图以节省磁盘，图片库会保留 R2 图床记录。请务必填写可访问的图床访问域名，否则无法安全清理本地图片。
        </SettingsNotice>
      </div>
    </SettingsCard>
  );
}
