"use client";

import { Gem, LoaderCircle, Save } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

import { useSettingsStore } from "../store";
import { SettingsCard, settingsInputClassName } from "./settings-ui";

function NumberInput({
  id,
  label,
  value,
  onChange,
}: {
  id: string;
  label: string;
  value: unknown;
  onChange: (value: string) => void;
}) {
  return (
    <Field className="gap-1.5">
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Input
        id={id}
        value={String(value || "")}
        onChange={(event) => onChange(event.target.value)}
        className={settingsInputClassName}
      />
    </Field>
  );
}

export function SubscriptionSettingsCard() {
  const config = useSettingsStore((state) => state.config);
  const isLoadingConfig = useSettingsStore((state) => state.isLoadingConfig);
  const isSavingConfig = useSettingsStore((state) => state.isSavingConfig);
  const saveConfig = useSettingsStore((state) => state.saveConfig);
  const setConfigField = useSettingsStore((state) => state.setConfigField);

  if (isLoadingConfig) {
    return (
      <SettingsCard icon={Gem} title="套餐订阅配置">
        <div className="flex items-center justify-center py-10">
          <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
        </div>
      </SettingsCard>
    );
  }

  return (
    <SettingsCard
      icon={Gem}
      title="套餐订阅配置"
      description="配置前台套餐页展示文案、三档价格与权益文案。"
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
      <div className="flex flex-col gap-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field className="gap-1.5 sm:col-span-2">
            <FieldLabel htmlFor="subscription-heading">主标题</FieldLabel>
            <Input
              id="subscription-heading"
              value={String(config?.subscription_heading || "")}
              onChange={(event) =>
                setConfigField("subscription_heading", event.target.value)
              }
              className={settingsInputClassName}
            />
          </Field>
          <Field className="gap-1.5 sm:col-span-2">
            <FieldLabel htmlFor="subscription-subheading">副标题</FieldLabel>
            <Input
              id="subscription-subheading"
              value={String(config?.subscription_subheading || "")}
              onChange={(event) =>
                setConfigField("subscription_subheading", event.target.value)
              }
              className={settingsInputClassName}
            />
          </Field>
          <Field className="gap-1.5 sm:col-span-2">
            <FieldLabel htmlFor="subscription-agent-hint">
              优惠提示文案
            </FieldLabel>
            <Input
              id="subscription-agent-hint"
              value={String(config?.subscription_agent_hint || "")}
              onChange={(event) =>
                setConfigField("subscription_agent_hint", event.target.value)
              }
              className={settingsInputClassName}
            />
          </Field>
          <Field className="gap-1.5 sm:col-span-2">
            <FieldLabel htmlFor="subscription-safety-text">安全文案</FieldLabel>
            <Input
              id="subscription-safety-text"
              value={String(config?.subscription_safety_text || "")}
              onChange={(event) =>
                setConfigField("subscription_safety_text", event.target.value)
              }
              className={settingsInputClassName}
            />
          </Field>
        </div>

        <div className="rounded-[14px] border border-border/80 p-4">
          <div className="mb-3 text-sm font-semibold">包月套餐</div>
          <div className="grid gap-3 sm:grid-cols-2">
            <NumberInput
              id="subscription-monthly-price"
              label="价格（分）"
              value={config?.subscription_monthly_price_cents}
              onChange={(value) =>
                setConfigField("subscription_monthly_price_cents", value)
              }
            />
            <Field className="gap-1.5">
              <FieldLabel htmlFor="subscription-monthly-badge">角标</FieldLabel>
              <Input
                id="subscription-monthly-badge"
                value={String(config?.subscription_monthly_badge || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_monthly_badge",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="subscription-monthly-name">名称</FieldLabel>
              <Input
                id="subscription-monthly-name"
                value={String(config?.subscription_monthly_name || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_monthly_name",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="subscription-monthly-price-note">
                价格说明
              </FieldLabel>
              <Input
                id="subscription-monthly-price-note"
                value={String(config?.subscription_monthly_price_note || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_monthly_price_note",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5 sm:col-span-2">
              <FieldLabel htmlFor="subscription-monthly-desc">描述</FieldLabel>
              <Input
                id="subscription-monthly-desc"
                value={String(config?.subscription_monthly_desc || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_monthly_desc",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5 sm:col-span-2">
              <FieldLabel htmlFor="subscription-monthly-features">
                权益文案（每行一条）
              </FieldLabel>
              <Textarea
                id="subscription-monthly-features"
                value={String(config?.subscription_monthly_features || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_monthly_features",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
                rows={3}
              />
            </Field>
          </div>
        </div>

        <div className="rounded-[14px] border border-border/80 p-4">
          <div className="mb-3 text-sm font-semibold">包季套餐</div>
          <div className="grid gap-3 sm:grid-cols-2">
            <NumberInput
              id="subscription-quarterly-price"
              label="价格（分）"
              value={config?.subscription_quarterly_price_cents}
              onChange={(value) =>
                setConfigField("subscription_quarterly_price_cents", value)
              }
            />
            <Field className="gap-1.5">
              <FieldLabel htmlFor="subscription-quarterly-badge">
                角标
              </FieldLabel>
              <Input
                id="subscription-quarterly-badge"
                value={String(config?.subscription_quarterly_badge || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_quarterly_badge",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="subscription-quarterly-name">
                名称
              </FieldLabel>
              <Input
                id="subscription-quarterly-name"
                value={String(config?.subscription_quarterly_name || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_quarterly_name",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="subscription-quarterly-price-note">
                价格说明
              </FieldLabel>
              <Input
                id="subscription-quarterly-price-note"
                value={String(config?.subscription_quarterly_price_note || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_quarterly_price_note",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5 sm:col-span-2">
              <FieldLabel htmlFor="subscription-quarterly-desc">
                描述
              </FieldLabel>
              <Input
                id="subscription-quarterly-desc"
                value={String(config?.subscription_quarterly_desc || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_quarterly_desc",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5 sm:col-span-2">
              <FieldLabel htmlFor="subscription-quarterly-features">
                权益文案（每行一条）
              </FieldLabel>
              <Textarea
                id="subscription-quarterly-features"
                value={String(config?.subscription_quarterly_features || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_quarterly_features",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
                rows={3}
              />
            </Field>
          </div>
        </div>

        <div className="rounded-[14px] border border-border/80 p-4">
          <div className="mb-3 text-sm font-semibold">包年套餐</div>
          <div className="grid gap-3 sm:grid-cols-2">
            <NumberInput
              id="subscription-yearly-price"
              label="价格（分）"
              value={config?.subscription_yearly_price_cents}
              onChange={(value) =>
                setConfigField("subscription_yearly_price_cents", value)
              }
            />
            <Field className="gap-1.5">
              <FieldLabel htmlFor="subscription-yearly-badge">角标</FieldLabel>
              <Input
                id="subscription-yearly-badge"
                value={String(config?.subscription_yearly_badge || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_yearly_badge",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="subscription-yearly-name">名称</FieldLabel>
              <Input
                id="subscription-yearly-name"
                value={String(config?.subscription_yearly_name || "")}
                onChange={(event) =>
                  setConfigField("subscription_yearly_name", event.target.value)
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5">
              <FieldLabel htmlFor="subscription-yearly-price-note">
                价格说明
              </FieldLabel>
              <Input
                id="subscription-yearly-price-note"
                value={String(config?.subscription_yearly_price_note || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_yearly_price_note",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5 sm:col-span-2">
              <FieldLabel htmlFor="subscription-yearly-desc">描述</FieldLabel>
              <Input
                id="subscription-yearly-desc"
                value={String(config?.subscription_yearly_desc || "")}
                onChange={(event) =>
                  setConfigField("subscription_yearly_desc", event.target.value)
                }
                className={settingsInputClassName}
              />
            </Field>
            <Field className="gap-1.5 sm:col-span-2">
              <FieldLabel htmlFor="subscription-yearly-features">
                权益文案（每行一条）
              </FieldLabel>
              <Textarea
                id="subscription-yearly-features"
                value={String(config?.subscription_yearly_features || "")}
                onChange={(event) =>
                  setConfigField(
                    "subscription_yearly_features",
                    event.target.value,
                  )
                }
                className={settingsInputClassName}
                rows={3}
              />
            </Field>
          </div>
        </div>
      </div>
    </SettingsCard>
  );
}
