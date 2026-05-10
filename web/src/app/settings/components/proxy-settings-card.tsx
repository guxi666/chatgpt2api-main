"use client";

import { useMemo, useState } from "react";
import { Link2, LoaderCircle, PlugZap, Save } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { testProxy, updateProxy, type ProxyTestResult } from "@/lib/api";
import { cn } from "@/lib/utils";

import { useSettingsStore } from "../store";
import {
  SettingsCard,
  SettingsNotice,
  settingsInlineCodeClassName,
  settingsInputClassName,
} from "./settings-ui";

const SOCKS5_EXAMPLES = [
  "socks5h://127.0.0.1:10808",
  "socks5://127.0.0.1:10808",
  "socks5h://user:pass@127.0.0.1:10808",
] as const;

function isSocksProxy(value: string) {
  const normalized = value.trim().toLowerCase();
  return normalized.startsWith("socks5://") || normalized.startsWith("socks5h://");
}

export function ProxySettingsCard() {
  const [isTesting, setIsTesting] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [testResult, setTestResult] = useState<ProxyTestResult | null>(null);
  const config = useSettingsStore((state) => state.config);
  const isLoadingConfig = useSettingsStore((state) => state.isLoadingConfig);
  const setProxy = useSettingsStore((state) => state.setProxy);

  const proxy = String(config?.proxy ?? "");
  const trimmedProxy = proxy.trim();
  const proxyStatus = useMemo(() => {
    if (!trimmedProxy) return "未配置";
    if (isSocksProxy(trimmedProxy)) return "SOCKS5 已配置";
    return "HTTP 代理已配置";
  }, [trimmedProxy]);

  const handleTest = async () => {
    if (!trimmedProxy) {
      toast.error("请先填写代理地址");
      return;
    }
    setIsTesting(true);
    setTestResult(null);
    try {
      const data = await testProxy(trimmedProxy);
      setTestResult(data.result);
      if (data.result.ok) {
        toast.success(`代理可用：${data.result.latency_ms} ms，HTTP ${data.result.status}`);
      } else {
        toast.error(`代理不可用：${data.result.error ?? "未知错误"}`);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "测试代理失败");
    } finally {
      setIsTesting(false);
    }
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      const data = await updateProxy({ url: trimmedProxy });
      setProxy(data.proxy.url || "");
      toast.success(trimmedProxy ? "代理配置已保存" : "已清空代理配置");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存代理失败");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <SettingsCard
      icon={Link2}
      title="SOCKS5 出站代理"
      tone="slate"
      meta={
        <Badge variant={trimmedProxy ? "success" : "secondary"} className="rounded-md px-2.5 py-1">
          {proxyStatus}
        </Badge>
      }
      action={
        <Button size="lg" onClick={() => void handleSave()} disabled={isSaving || isLoadingConfig}>
          {isSaving ? (
            <LoaderCircle data-icon="inline-start" className="animate-spin" />
          ) : (
            <Save data-icon="inline-start" />
          )}
          保存代理
        </Button>
      }
    >
      {isLoadingConfig ? (
        <div className="flex items-center justify-center py-10">
          <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          <Field className="gap-1.5">
            <FieldLabel htmlFor="settings-socks5-proxy">代理地址</FieldLabel>
            <Input
              id="settings-socks5-proxy"
              value={proxy}
              onChange={(event) => {
                setProxy(event.target.value);
                setTestResult(null);
              }}
              placeholder="socks5h://127.0.0.1:10808"
              className={settingsInputClassName}
            />
          </Field>

          <div className="grid gap-2 sm:grid-cols-3">
            {SOCKS5_EXAMPLES.map((example) => (
              <Button
                key={example}
                type="button"
                variant="outline"
                className="h-auto justify-start whitespace-normal rounded-[13px] px-3 py-2 text-left font-mono text-xs"
                onClick={() => {
                  setProxy(example);
                  setTestResult(null);
                }}
              >
                {example}
              </Button>
            ))}
          </div>

          <SettingsNotice>
            支持 <span className={settingsInlineCodeClassName}>socks5://</span>、
            <span className={settingsInlineCodeClassName}>socks5h://</span>、
            <span className={settingsInlineCodeClassName}>http://</span> 和
            <span className={settingsInlineCodeClassName}>https://</span>。推荐使用
            <span className={settingsInlineCodeClassName}>socks5h://</span>，这样域名解析也会交给代理端处理。
          </SettingsNotice>

          {testResult ? (
            <div
              className={cn(
                "rounded-[13px] border px-3 py-2 text-xs leading-5",
                testResult.ok
                  ? "border-emerald-200 bg-emerald-50 text-emerald-800"
                  : "border-rose-200 bg-rose-50 text-rose-800",
              )}
            >
              {testResult.ok
                ? `代理可用：HTTP ${testResult.status}，用时 ${testResult.latency_ms} ms`
                : `代理不可用：${testResult.error ?? "未知错误"}，用时 ${testResult.latency_ms} ms`}
            </div>
          ) : null}

          <div className="flex flex-wrap justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => void handleTest()}
              disabled={isTesting || isLoadingConfig || !trimmedProxy}
            >
              {isTesting ? (
                <LoaderCircle data-icon="inline-start" className="animate-spin" />
              ) : (
                <PlugZap data-icon="inline-start" />
              )}
              测试代理
            </Button>
          </div>
        </div>
      )}
    </SettingsCard>
  );
}
