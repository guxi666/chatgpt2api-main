"use client";

import { useEffect, useMemo, useState } from "react";
import { Link2, LoaderCircle, PlugZap, Save } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { fetchProxy, testProxy, updateProxy, type ProxyTestResult } from "@/lib/api";
import { cn } from "@/lib/utils";

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
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isTesting, setIsTesting] = useState(false);
  const [testResult, setTestResult] = useState<ProxyTestResult | null>(null);

  const [proxy, setProxy] = useState("");
  const [poolEnabled, setPoolEnabled] = useState(false);
  const [poolUrls, setPoolUrls] = useState("");
  const [poolCooldownSeconds, setPoolCooldownSeconds] = useState("600");

  const [savedProxy, setSavedProxy] = useState("");
  const [savedPoolEnabled, setSavedPoolEnabled] = useState(false);
  const [savedPoolUrls, setSavedPoolUrls] = useState("");
  const [savedPoolCooldownSeconds, setSavedPoolCooldownSeconds] = useState("600");

  useEffect(() => {
    let active = true;
    const load = async () => {
      setIsLoading(true);
      try {
        const data = await fetchProxy();
        if (!active) return;
        const nextProxy = String(data.proxy.url || "");
        const nextPoolEnabled = Boolean(data.proxy.pool_enabled);
        const nextPoolUrls = String(data.proxy.pool_urls || "");
        const nextCooldown = String(data.proxy.pool_cooldown_seconds || 600);
        setProxy(nextProxy);
        setPoolEnabled(nextPoolEnabled);
        setPoolUrls(nextPoolUrls);
        setPoolCooldownSeconds(nextCooldown);
        setSavedProxy(nextProxy);
        setSavedPoolEnabled(nextPoolEnabled);
        setSavedPoolUrls(nextPoolUrls);
        setSavedPoolCooldownSeconds(nextCooldown);
      } catch (error) {
        toast.error(error instanceof Error ? error.message : "加载代理配置失败");
      } finally {
        if (active) {
          setIsLoading(false);
        }
      }
    };
    void load();
    return () => {
      active = false;
    };
  }, []);

  const trimmedProxy = proxy.trim();
  const trimmedPoolUrls = poolUrls.trim();
  const proxyStatus = useMemo(() => {
    if (!trimmedProxy) return "未配置";
    if (isSocksProxy(trimmedProxy)) return "SOCKS5 已配置";
    return "HTTP 代理已配置";
  }, [trimmedProxy]);

  const dirty =
    trimmedProxy !== savedProxy ||
    poolEnabled !== savedPoolEnabled ||
    trimmedPoolUrls !== savedPoolUrls ||
    String(poolCooldownSeconds || "").trim() !== savedPoolCooldownSeconds;

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
      const data = await updateProxy({
        url: trimmedProxy,
        pool_enabled: poolEnabled,
        pool_urls: trimmedPoolUrls,
        pool_cooldown_seconds: Math.max(30, Number(poolCooldownSeconds) || 600),
      });
      const nextProxy = String(data.proxy.url || "");
      const nextPoolEnabled = Boolean(data.proxy.pool_enabled);
      const nextPoolUrls = String(data.proxy.pool_urls || "");
      const nextCooldown = String(data.proxy.pool_cooldown_seconds || 600);
      setProxy(nextProxy);
      setPoolEnabled(nextPoolEnabled);
      setPoolUrls(nextPoolUrls);
      setPoolCooldownSeconds(nextCooldown);
      setSavedProxy(nextProxy);
      setSavedPoolEnabled(nextPoolEnabled);
      setSavedPoolUrls(nextPoolUrls);
      setSavedPoolCooldownSeconds(nextCooldown);
      toast.success("代理配置已保存");
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
        <Button size="lg" onClick={() => void handleSave()} disabled={isSaving || isLoading || !dirty}>
          {isSaving ? (
            <LoaderCircle data-icon="inline-start" className="animate-spin" />
          ) : (
            <Save data-icon="inline-start" />
          )}
          保存代理
        </Button>
      }
    >
      {isLoading ? (
        <div className="flex items-center justify-center py-10">
          <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          <Field className="gap-1.5">
            <FieldLabel htmlFor="settings-socks5-proxy">主代理地址</FieldLabel>
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

          <label className="flex min-h-12 items-start gap-3 rounded-[13px] border border-border/80 bg-background px-3 py-3 text-sm">
            <Checkbox
              checked={poolEnabled}
              onCheckedChange={(checked) => setPoolEnabled(Boolean(checked))}
            />
            <span className="leading-6 text-foreground">启用多代理池调度</span>
          </label>

          <Field className="gap-1.5">
            <FieldLabel htmlFor="settings-proxy-pool-urls">多代理列表</FieldLabel>
            <Textarea
              id="settings-proxy-pool-urls"
              value={poolUrls}
              onChange={(event) => setPoolUrls(event.target.value)}
              placeholder={"socks5h://user:pass@1.2.3.4:1080\nsocks5h://user:pass@5.6.7.8:1080"}
              className="min-h-28 rounded-[13px] font-mono text-xs"
            />
          </Field>

          <Field className="gap-1.5">
            <FieldLabel htmlFor="settings-proxy-pool-cooldown">失败冷却（秒）</FieldLabel>
            <Input
              id="settings-proxy-pool-cooldown"
              value={poolCooldownSeconds}
              onChange={(event) => setPoolCooldownSeconds(event.target.value)}
              className={settingsInputClassName}
            />
          </Field>

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
              disabled={isTesting || isLoading || !trimmedProxy}
            >
              {isTesting ? (
                <LoaderCircle data-icon="inline-start" className="animate-spin" />
              ) : (
                <PlugZap data-icon="inline-start" />
              )}
              测试主代理
            </Button>
          </div>
        </div>
      )}
    </SettingsCard>
  );
}
