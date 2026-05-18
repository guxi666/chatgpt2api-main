"use client";

import { CloudUpload, LoaderCircle, Save } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

import { useSettingsStore } from "../store";
import {
  SettingsCard,
  SettingsNotice,
  settingsInputClassName,
} from "./settings-ui";

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
  const setConfigField = useSettingsStore((state) => state.setConfigField);

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
  const secondarySecretConfigured = Boolean(
    config?.image_r2_secondary_secret_access_key_configured,
  );
  const imgBedSecretConfigured = Boolean(
    config?.image_imgbed_auth_code_configured,
  );

  return (
    <SettingsCard
      icon={CloudUpload}
      title="图床存储配置"
      tone="amber"
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
        <div className="rounded-[14px] border border-border/80 p-4">
          <div className="mb-3 text-sm font-semibold">
            第 1 优先级：CloudFlare-ImgBed
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="flex min-h-10 items-center gap-3 rounded-[12px] border border-border/70 bg-background px-3 py-2 text-sm sm:col-span-2">
              <Checkbox
                checked={Boolean(config?.image_imgbed_enabled)}
                onCheckedChange={(value) =>
                  setConfigField("image_imgbed_enabled", Boolean(value))
                }
              />
              启用外部图床优先上传
            </label>
            <Field className="gap-1.5 sm:col-span-2">
              <FieldLabel htmlFor="imgbed-upload-url">上传接口 URL</FieldLabel>
              <Input
                id="imgbed-upload-url"
                value={String(config?.image_imgbed_upload_url || "")}
                onChange={(event) =>
                  setConfigField("image_imgbed_upload_url", event.target.value)
                }
                placeholder="https://your-imgbed-domain/upload"
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="imgbed-auth-code">
                Auth Code {imgBedSecretConfigured ? "(已配置)" : "(未配置)"}
              </FieldLabel>
              <Input
                id="imgbed-auth-code"
                type="password"
                value={String(config?.image_imgbed_auth_code || "")}
                onChange={(event) =>
                  setConfigField("image_imgbed_auth_code", event.target.value)
                }
                placeholder="不修改可留空"
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="imgbed-upload-channel">上传通道</FieldLabel>
              <Input
                id="imgbed-upload-channel"
                value={String(config?.image_imgbed_upload_channel || "cfr2")}
                onChange={(event) =>
                  setConfigField(
                    "image_imgbed_upload_channel",
                    event.target.value,
                  )
                }
                placeholder="cfr2"
                className={settingsInputClassName}
              />
            </Field>
          </div>
        </div>

        <div className="rounded-[14px] border border-border/80 p-4">
          <div className="mb-3 flex items-center justify-between gap-3">
            <div className="text-sm font-semibold">第 2 优先级：新增 CFR2（备用主力）</div>
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => {
                setConfigField("image_r2_secondary_endpoint", String(config?.image_r2_endpoint || ""));
                setConfigField("image_r2_secondary_bucket", String(config?.image_r2_bucket || ""));
                setConfigField("image_r2_secondary_region", String(config?.image_r2_region || "auto"));
                setConfigField("image_r2_secondary_access_key_id", String(config?.image_r2_access_key_id || ""));
                setConfigField(
                  "image_r2_secondary_secret_access_key",
                  String(config?.image_r2_secret_access_key || ""),
                );
                setConfigField(
                  "image_r2_secondary_public_base_url",
                  String(config?.image_r2_public_base_url || ""),
                );
                setConfigField("image_r2_secondary_prefix", String(config?.image_r2_prefix || "images"));
              }}
            >
              复制老 CFR2 配置
            </Button>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="flex min-h-10 items-center gap-3 rounded-[12px] border border-border/70 bg-background px-3 py-2 text-sm sm:col-span-2">
              <Checkbox
                checked={Boolean(config?.image_r2_secondary_enabled)}
                onCheckedChange={(value) =>
                  setConfigField("image_r2_secondary_enabled", Boolean(value))
                }
              />
              启用新增 CFR2 上传
            </label>
            <Field className="gap-1.5 sm:col-span-2">
              <FieldLabel htmlFor="secondary-r2-endpoint">
                R2 S3 Endpoint
              </FieldLabel>
              <Input
                id="secondary-r2-endpoint"
                value={String(config?.image_r2_secondary_endpoint || "")}
                onChange={(event) =>
                  setConfigField("image_r2_secondary_endpoint", event.target.value)
                }
                placeholder="https://<account_id>.r2.cloudflarestorage.com"
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="secondary-r2-bucket">Bucket 名称</FieldLabel>
              <Input
                id="secondary-r2-bucket"
                value={String(config?.image_r2_secondary_bucket || "")}
                onChange={(event) =>
                  setConfigField("image_r2_secondary_bucket", event.target.value)
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="secondary-r2-region">Region</FieldLabel>
              <Input
                id="secondary-r2-region"
                value={String(config?.image_r2_secondary_region || "auto")}
                onChange={(event) =>
                  setConfigField("image_r2_secondary_region", event.target.value)
                }
                placeholder="auto"
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="secondary-r2-access-key">Access Key ID</FieldLabel>
              <Input
                id="secondary-r2-access-key"
                value={String(config?.image_r2_secondary_access_key_id || "")}
                onChange={(event) =>
                  setConfigField("image_r2_secondary_access_key_id", event.target.value)
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="secondary-r2-secret-key">
                Secret Access Key{" "}
                {secondarySecretConfigured ? "(已配置)" : "(未配置)"}
              </FieldLabel>
              <Input
                id="secondary-r2-secret-key"
                type="password"
                value={String(config?.image_r2_secondary_secret_access_key || "")}
                onChange={(event) =>
                  setConfigField(
                    "image_r2_secondary_secret_access_key",
                    event.target.value,
                  )
                }
                placeholder="不修改可留空"
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="secondary-r2-public-base-url">
                图床访问域名
              </FieldLabel>
              <Input
                id="secondary-r2-public-base-url"
                value={String(config?.image_r2_secondary_public_base_url || "")}
                onChange={(event) =>
                  setConfigField(
                    "image_r2_secondary_public_base_url",
                    event.target.value,
                  )
                }
                placeholder="https://img-backup.example.com"
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="secondary-r2-prefix">对象前缀</FieldLabel>
              <Input
                id="secondary-r2-prefix"
                value={String(config?.image_r2_secondary_prefix || "images")}
                onChange={(event) =>
                  setConfigField("image_r2_secondary_prefix", event.target.value)
                }
                placeholder="images"
                className={settingsInputClassName}
              />
            </Field>
          </div>
        </div>

        <div className="rounded-[14px] border border-border/80 p-4">
          <div className="mb-3 text-sm font-semibold">
            第 3 优先级：老 CFR2（最后兜底）
          </div>
          <label className="mb-3 flex min-h-10 items-center gap-3 rounded-[12px] border border-border/70 bg-background px-3 py-2 text-sm">
            <Checkbox
              checked={Boolean(config?.image_r2_enabled)}
              onCheckedChange={(value) => setImageR2Enabled(Boolean(value))}
            />
            启用老 CFR2 兜底上传
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
              <Input
                id="r2-bucket"
                value={String(config?.image_r2_bucket || "")}
                onChange={(event) => setImageR2Bucket(event.target.value)}
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="r2-region">Region</FieldLabel>
              <Input
                id="r2-region"
                value={String(config?.image_r2_region || "auto")}
                onChange={(event) => setImageR2Region(event.target.value)}
                placeholder="auto"
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="r2-access-key">Access Key ID</FieldLabel>
              <Input
                id="r2-access-key"
                value={String(config?.image_r2_access_key_id || "")}
                onChange={(event) => setImageR2AccessKeyId(event.target.value)}
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="r2-secret-key">
                Secret Access Key {secretConfigured ? "(已配置)" : "(未配置)"}
              </FieldLabel>
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
              <Input
                id="r2-prefix"
                value={String(config?.image_r2_prefix || "images")}
                onChange={(event) => setImageR2Prefix(event.target.value)}
                placeholder="images"
                className={settingsInputClassName}
              />
            </Field>
          </div>
        </div>

        <SettingsNotice>
          图片上传顺序：依次尝试 CloudFlare-ImgBed、新增 CFR2、老 CFR2。每一级失败会自动回退到下一级。
        </SettingsNotice>
      </div>
    </SettingsCard>
  );
}
