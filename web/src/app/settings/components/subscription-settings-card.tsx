"use client";

import { useEffect, useState } from "react";
import { Gem, LoaderCircle, Save } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

import { useSettingsStore } from "../store";
import { SettingsCard, settingsInputClassName } from "./settings-ui";

function centsToYuanText(value: unknown) {
  const cents = Number(value || 0);
  const safe = Number.isFinite(cents) ? Math.max(0, cents) : 0;
  return (safe / 100).toFixed(2);
}

function yuanToCents(value: string) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return 0;
  }
  return Math.round(parsed * 100);
}

function PlanPriceInput({
  id,
  value,
  onChange,
}: {
  id: string;
  value: unknown;
  onChange: (cents: number) => void;
}) {
  const [draft, setDraft] = useState(centsToYuanText(value));

  useEffect(() => {
    setDraft(centsToYuanText(value));
  }, [value]);

  return (
    <Field className="gap-1.5">
      <FieldLabel htmlFor={id}>价格（元）</FieldLabel>
      <Input
        id={id}
        type="text"
        min="0"
        inputMode="decimal"
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={() => {
          const cents = yuanToCents(draft);
          onChange(cents);
          setDraft(centsToYuanText(cents));
        }}
        className={settingsInputClassName}
      />
    </Field>
  );
}

function PlanBlock({
  title,
  nameKey,
  badgeKey,
  descKey,
  priceCentsKey,
  priceNoteKey,
  featuresKey,
}: {
  title: string;
  nameKey:
    | "subscription_monthly_name"
    | "subscription_quarterly_name"
    | "subscription_yearly_name";
  badgeKey:
    | "subscription_monthly_badge"
    | "subscription_quarterly_badge"
    | "subscription_yearly_badge";
  descKey:
    | "subscription_monthly_desc"
    | "subscription_quarterly_desc"
    | "subscription_yearly_desc";
  priceCentsKey:
    | "subscription_monthly_price_cents"
    | "subscription_quarterly_price_cents"
    | "subscription_yearly_price_cents";
  priceNoteKey:
    | "subscription_monthly_price_note"
    | "subscription_quarterly_price_note"
    | "subscription_yearly_price_note";
  featuresKey:
    | "subscription_monthly_features"
    | "subscription_quarterly_features"
    | "subscription_yearly_features";
}) {
  const config = useSettingsStore((state) => state.config);
  const setConfigField = useSettingsStore((state) => state.setConfigField);

  return (
    <div className="rounded-[14px] border border-border/80 p-4">
      <div className="mb-3 text-sm font-semibold">{title}</div>
      <div className="grid gap-3 sm:grid-cols-2">
        <PlanPriceInput
          id={`${priceCentsKey}-yuan`}
          value={config?.[priceCentsKey]}
          onChange={(cents) => setConfigField(priceCentsKey, cents)}
        />

        <Field className="gap-1.5">
          <FieldLabel htmlFor={badgeKey}>角标</FieldLabel>
          <Input
            id={badgeKey}
            value={String(config?.[badgeKey] || "")}
            onChange={(event) => setConfigField(badgeKey, event.target.value)}
            className={settingsInputClassName}
          />
        </Field>

        <Field className="gap-1.5">
          <FieldLabel htmlFor={nameKey}>套餐名称</FieldLabel>
          <Input
            id={nameKey}
            value={String(config?.[nameKey] || "")}
            onChange={(event) => setConfigField(nameKey, event.target.value)}
            className={settingsInputClassName}
          />
        </Field>

        <Field className="gap-1.5">
          <FieldLabel htmlFor={priceNoteKey}>价格说明（可选）</FieldLabel>
          <Input
            id={priceNoteKey}
            value={String(config?.[priceNoteKey] || "")}
            onChange={(event) =>
              setConfigField(priceNoteKey, event.target.value)
            }
            className={settingsInputClassName}
          />
        </Field>

        <Field className="gap-1.5 sm:col-span-2">
          <FieldLabel htmlFor={descKey}>套餐描述</FieldLabel>
          <Input
            id={descKey}
            value={String(config?.[descKey] || "")}
            onChange={(event) => setConfigField(descKey, event.target.value)}
            className={settingsInputClassName}
          />
        </Field>

        <Field className="gap-1.5 sm:col-span-2">
          <FieldLabel htmlFor={featuresKey}>权益文案（每行一条）</FieldLabel>
          <Textarea
            id={featuresKey}
            value={String(config?.[featuresKey] || "")}
            onChange={(event) =>
              setConfigField(featuresKey, event.target.value)
            }
            className={settingsInputClassName}
            rows={3}
          />
        </Field>
      </div>
    </div>
  );
}

export function SubscriptionSettingsCard() {
  const config = useSettingsStore((state) => state.config);
  const isLoadingConfig = useSettingsStore((state) => state.isLoadingConfig);
  const isSavingConfig = useSettingsStore((state) => state.isSavingConfig);
  const saveConfig = useSettingsStore((state) => state.saveConfig);
  const setConfigField = useSettingsStore((state) => state.setConfigField);

  if (isLoadingConfig && !config) {
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
      description="在这里可以直接自定义包月/包季/包年金额（单位：元）、文案和权益。"
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
        <label className="flex min-h-12 flex-col gap-2 rounded-[14px] border border-border/80 bg-background px-4 py-3 text-sm font-medium sm:flex-row sm:items-center sm:gap-3">
          <Checkbox
            checked={config?.subscription_enabled !== false}
            onCheckedChange={(value) =>
              setConfigField("subscription_enabled", Boolean(value))
            }
          />
          <span>启用套餐订阅</span>
          <span className="text-xs font-normal leading-5 text-muted-foreground sm:ml-auto">
            关闭后用户仍可查看当前套餐状态，但不能购买新套餐
          </span>
        </label>

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
            <FieldLabel htmlFor="subscription-agent-hint">代理提示文案</FieldLabel>
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
            <FieldLabel htmlFor="subscription-safety-text">底部安全文案</FieldLabel>
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

        <PlanBlock
          title="包月套餐"
          nameKey="subscription_monthly_name"
          badgeKey="subscription_monthly_badge"
          descKey="subscription_monthly_desc"
          priceCentsKey="subscription_monthly_price_cents"
          priceNoteKey="subscription_monthly_price_note"
          featuresKey="subscription_monthly_features"
        />

        <PlanBlock
          title="包季套餐"
          nameKey="subscription_quarterly_name"
          badgeKey="subscription_quarterly_badge"
          descKey="subscription_quarterly_desc"
          priceCentsKey="subscription_quarterly_price_cents"
          priceNoteKey="subscription_quarterly_price_note"
          featuresKey="subscription_quarterly_features"
        />

        <PlanBlock
          title="包年套餐"
          nameKey="subscription_yearly_name"
          badgeKey="subscription_yearly_badge"
          descKey="subscription_yearly_desc"
          priceCentsKey="subscription_yearly_price_cents"
          priceNoteKey="subscription_yearly_price_note"
          featuresKey="subscription_yearly_features"
        />
      </div>
    </SettingsCard>
  );
}
