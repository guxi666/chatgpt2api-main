"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ArrowRight,
  CheckCircle2,
  CreditCard,
  LoaderCircle,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  Wallet,
} from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  createSubscriptionOrder,
  fetchSubscriptionPlans,
  type PayType,
  type SubscriptionPlan,
  type SubscriptionStatus,
  type WalletInfo,
} from "@/lib/api";
import { parseDateTime } from "@/lib/datetime";
import { useAuthGuard } from "@/lib/use-auth-guard";

const fallbackPlans: SubscriptionPlan[] = [
  {
    key: "monthly",
    name: "包月套餐",
    description: "适合轻量和日常创作",
    badge: "灵活",
    price_cents: 1990,
    period_label: "每月",
    features: ["无限生图", "套餐期内不扣余额", "支持余额支付"],
  },
  {
    key: "quarterly",
    name: "包季套餐",
    description: "性价比更高，推荐多数用户",
    badge: "推荐",
    price_cents: 4990,
    period_label: "每季",
    features: ["无限生图", "套餐期内不扣余额", "支持余额支付"],
  },
  {
    key: "yearly",
    name: "包年套餐",
    description: "长期使用最省心",
    badge: "最优惠",
    price_cents: 15900,
    period_label: "每年",
    features: ["无限生图", "套餐期内不扣余额", "支持余额支付"],
  },
];

function normalizePlans(source: SubscriptionPlan[]) {
  const map = new Map<string, SubscriptionPlan>();
  for (const item of source) {
    const key = String(item?.key || "").trim();
    if (!key) continue;
    map.set(key, item);
  }
  return fallbackPlans.map((base) => {
    const hit = map.get(String(base.key));
    if (!hit) return base;
    return {
      ...base,
      ...hit,
      key: base.key,
      name: String(hit.name || base.name),
      description: String(hit.description || base.description || ""),
      badge: String(hit.badge || base.badge || ""),
      price_cents: Number.isFinite(Number(hit.price_cents))
        ? Number(hit.price_cents)
        : base.price_cents,
      period_label: String(hit.period_label || base.period_label || ""),
      features:
        Array.isArray(hit.features) && hit.features.length > 0
          ? hit.features
          : base.features,
    } as SubscriptionPlan;
  });
}

function normalizeSafetyText(value: string) {
  return String(value || "")
    .replaceAll("随时可取消", "无生图数量限制")
    .trim();
}

function centsToYuan(cents: number) {
  return (Math.max(0, Number(cents || 0)) / 100).toFixed(2);
}

function formatDateTime(value?: string) {
  const date = parseDateTime(value);
  if (!date) return "-";
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day} ${hour}:${minute}`;
}

function payTypeLabel(payType: string) {
  switch (payType) {
    case "balance":
      return "余额支付";
    case "wxpay":
      return "微信支付";
    case "paypal":
      return "PayPal";
    case "usdt":
      return "USDT";
    default:
      return "支付宝";
  }
}

function tierLabel(tier?: string) {
  switch (tier) {
    case "monthly":
      return "包月";
    case "quarterly":
      return "包季";
    case "yearly":
      return "包年";
    default:
      return "-";
  }
}

function SubscriptionPageContent() {
  const [plans, setPlans] = useState<SubscriptionPlan[]>([]);
  const [status, setStatus] = useState<SubscriptionStatus>({ active: false });
  const [wallet, setWallet] = useState<WalletInfo | null>(null);
  const [channels, setChannels] = useState<string[]>([]);
  const [heading, setHeading] = useState("选择适合你的套餐");
  const [subheading, setSubheading] = useState("套餐有效期内无限生图，不扣余额");
  const [safetyText, setSafetyText] = useState(
    "安全支付保障·无生图数量限制·无隐藏费用",
  );
  const [agentHint, setAgentHint] = useState("购买代理充值更优惠");
  const [selectedTier, setSelectedTier] = useState<string>("");
  const [selectedPayType, setSelectedPayType] = useState<string>("balance");
  const [isLoading, setIsLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const displayPlans = useMemo(() => normalizePlans(plans), [plans]);

  const reload = useCallback(async () => {
    const data = await fetchSubscriptionPlans();
    const incomingPlans = Array.isArray(data.plans) ? data.plans : [];
    const incomingChannels = Array.isArray(data.pay_channels)
      ? data.pay_channels
      : [];
    const finalPlans = normalizePlans(
      incomingPlans.length > 0 ? incomingPlans : fallbackPlans,
    );
    const finalChannels =
      incomingChannels.length > 0 ? incomingChannels : ["balance", "alipay"];

    setPlans(finalPlans);
    setStatus(data.status || { active: false });
    setWallet(data.wallet || null);
    setChannels(finalChannels);
    setHeading(String(data.heading || "选择适合你的套餐"));
    setSubheading(String(data.subheading || "套餐有效期内无限生图，不扣余额"));
    setSafetyText(
      normalizeSafetyText(
        String(data.safety_text || "安全支付保障·无生图数量限制·无隐藏费用"),
      ),
    );
    setAgentHint(String(data.agent_hint || "购买代理充值更优惠"));

    setSelectedTier((current) => {
      if (current && finalPlans.some((item) => item.key === current)) {
        return current;
      }
      return String(finalPlans[0]?.key || "monthly");
    });

    setSelectedPayType((current) => {
      if (current && finalChannels.includes(current)) return current;
      if (finalChannels.includes("balance")) return "balance";
      return String(finalChannels[0] || "alipay");
    });
  }, []);

  useEffect(() => {
    let active = true;
    const load = async () => {
      setIsLoading(true);
      try {
        await reload();
      } catch (error) {
        if (active) {
          toast.error(error instanceof Error ? error.message : "加载套餐失败");
        }
      } finally {
        if (active) setIsLoading(false);
      }
    };
    void load();
    return () => {
      active = false;
    };
  }, [reload]);

  const selectedPlan = useMemo(
    () => displayPlans.find((item) => item.key === selectedTier) || null,
    [displayPlans, selectedTier],
  );

  const handleBuy = async () => {
    if (!selectedPlan) {
      toast.error("请选择套餐");
      return;
    }
    if (!selectedPayType) {
      toast.error("请选择支付方式");
      return;
    }
    setIsSubmitting(true);
    try {
      const data = await createSubscriptionOrder({
        tier: selectedPlan.key as "monthly" | "quarterly" | "yearly",
        pay_type: selectedPayType as PayType | "balance",
      });
      if (data.order?.pay_url) {
        window.open(
          String(data.order.pay_url),
          "_blank",
          "noopener,noreferrer",
        );
        toast.success("订单已创建，请在新窗口完成支付");
      } else {
        toast.success("套餐购买成功");
      }
      await reload();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "购买失败");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <section className="mx-auto flex w-full max-w-6xl flex-col gap-6 pb-10">
      <PageHeader
        eyebrow="Subscription"
        title="套餐订阅"
        actions={
          <Button
            variant="outline"
            className="h-10 rounded-lg"
            onClick={() => void reload()}
            disabled={isLoading || isSubmitting}
          >
            <RefreshCw className={`size-4 ${isLoading ? "animate-spin" : ""}`} />
            刷新
          </Button>
        }
      />

      <Card className="overflow-hidden rounded-[24px] border-0 bg-gradient-to-br from-sky-50 via-white to-indigo-50 shadow-[0_20px_60px_-30px_rgba(31,66,176,0.35)]">
        <CardHeader className="items-center space-y-3 pb-1 text-center">
          <Badge className="rounded-full bg-indigo-600 px-3 py-1 text-xs text-white hover:bg-indigo-600">
            推荐订阅
          </Badge>
          <CardTitle className="text-3xl font-semibold tracking-tight">{heading}</CardTitle>
          <CardDescription className="text-sm text-muted-foreground">
            {subheading}
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-5 pt-3">
          <div className="mx-auto grid w-full max-w-[1120px] gap-4 lg:grid-cols-3">
            {displayPlans.map((plan) => {
                const active = selectedTier === plan.key;
                return (
                  <button
                    type="button"
                    key={String(plan.key)}
                    onClick={() => setSelectedTier(String(plan.key))}
                    className={`relative min-h-[260px] rounded-[20px] border p-5 text-left transition ${
                      active
                        ? "border-indigo-500 bg-white shadow-[0_18px_32px_-26px_rgba(79,70,229,0.8)] ring-2 ring-indigo-200"
                        : "border-border/80 bg-white/90 hover:border-indigo-300"
                    }`}
                  >
                    {plan.badge ? (
                      <Badge className="absolute right-4 top-4 rounded-full bg-indigo-100 text-indigo-700 hover:bg-indigo-100">
                        {plan.badge}
                      </Badge>
                    ) : null}
                    <div className="text-lg font-semibold text-foreground">{plan.name}</div>
                    <div className="mt-1 text-sm text-muted-foreground">
                      {plan.description || "订阅有效期内无限生图"}
                    </div>
                    <div className="mt-4 flex items-end gap-1">
                      <span className="text-4xl font-semibold leading-none text-indigo-700">
                        ￥{centsToYuan(plan.price_cents)}
                      </span>
                      <span className="pb-1 text-sm text-muted-foreground">
                        /{plan.period_label || "周期"}
                      </span>
                    </div>
                    {plan.price_note ? (
                      <div className="mt-1 text-xs text-muted-foreground">{plan.price_note}</div>
                    ) : null}
                    <div className="mt-4 space-y-2 text-sm">
                      {(plan.features || []).map((feature) => (
                        <div
                          key={`${plan.key}-${feature}`}
                          className="flex items-center gap-2 text-foreground"
                        >
                          <CheckCircle2 className="size-4 text-indigo-600" />
                          <span>{feature}</span>
                        </div>
                      ))}
                    </div>
                  </button>
                );
              })}
          </div>

          <div className="grid gap-3 rounded-[18px] border border-indigo-100 bg-white/90 p-4 sm:grid-cols-[1fr_auto] sm:items-end">
            <div className="grid gap-3 sm:grid-cols-2">
              <div>
                <div className="mb-1 text-xs text-muted-foreground">套餐档位</div>
                <Select value={selectedTier || undefined} onValueChange={setSelectedTier}>
                  <SelectTrigger className="h-11 rounded-[12px]">
                    <SelectValue placeholder="选择套餐" />
                  </SelectTrigger>
                  <SelectContent>
                    {displayPlans.map((plan) => (
                      <SelectItem key={String(plan.key)} value={String(plan.key)}>
                        {plan.name} ￥{centsToYuan(plan.price_cents)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div>
                <div className="mb-1 text-xs text-muted-foreground">支付方式</div>
                <Select
                  value={selectedPayType || undefined}
                  onValueChange={setSelectedPayType}
                >
                  <SelectTrigger className="h-11 rounded-[12px]">
                    <SelectValue placeholder="支付方式" />
                  </SelectTrigger>
                  <SelectContent>
                    {channels.map((channel) => (
                      <SelectItem key={channel} value={channel}>
                        {payTypeLabel(channel)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <Button
              className="h-11 min-w-[180px] rounded-[12px] bg-gradient-to-r from-indigo-600 to-violet-600 text-white hover:from-indigo-500 hover:to-violet-500"
              disabled={isSubmitting || !selectedTier || !selectedPayType}
              onClick={() => void handleBuy()}
            >
              {isSubmitting ? (
                <LoaderCircle className="size-4 animate-spin" />
              ) : (
                <Sparkles className="size-4" />
              )}
              立即开通
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 lg:grid-cols-[1.3fr_1fr]">
        <Card className="rounded-[18px] border-indigo-100 bg-gradient-to-r from-indigo-50/60 to-white">
          <CardContent className="flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-start gap-3">
              <div className="rounded-full bg-indigo-100 p-2">
                <Wallet className="size-5 text-indigo-700" />
              </div>
              <div>
                <div className="text-base font-semibold">余额支付已支持</div>
                <div className="text-sm text-muted-foreground">
                  当前余额：￥{centsToYuan(wallet?.balance_cents || 0)}
                </div>
                <div className="mt-1 text-xs text-muted-foreground">
                  余额不足时可先去钱包充值后再购买套餐
                </div>
              </div>
            </div>
            <Button
              variant="outline"
              className="h-10 rounded-[12px] border-indigo-200 text-indigo-700"
              onClick={() => {
                window.location.href = "/wallet";
              }}
            >
              去钱包充值
              <ArrowRight className="size-4" />
            </Button>
          </CardContent>
        </Card>

        <Card className="rounded-[18px]">
          <CardHeader className="pb-2">
            <CardTitle className="text-base">当前套餐状态</CardTitle>
            <CardDescription>套餐到期后恢复按次扣费</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex items-center gap-2">
              <span className="text-muted-foreground">状态：</span>
              {status.active ? (
                <Badge variant="success">已生效</Badge>
              ) : (
                <Badge variant="warning">未生效</Badge>
              )}
            </div>
            <div>
              <span className="text-muted-foreground">套餐：</span>
              {tierLabel(status.tier)}
            </div>
            <div>
              <span className="text-muted-foreground">生效时间：</span>
              {formatDateTime(status.start_at)}
            </div>
            <div>
              <span className="text-muted-foreground">到期时间：</span>
              {formatDateTime(status.expire_at)}
            </div>
            <div>
              <span className="text-muted-foreground">剩余天数：</span>
              {status.remaining_days ?? 0}
            </div>
          </CardContent>
        </Card>
      </div>

      <Card className="rounded-[18px] border-dashed border-indigo-300 bg-indigo-50/40">
        <CardContent className="flex items-center justify-between gap-3 p-4">
          <div className="inline-flex items-center gap-2 text-sm font-medium text-indigo-700">
            <CreditCard className="size-4" />
            {agentHint}
          </div>
          <Button
            variant="outline"
            className="h-9 rounded-[10px] border-indigo-300 text-indigo-700"
            onClick={() => {
              window.location.href = "/agency";
            }}
          >
            去代理页面
            <ArrowRight className="size-4" />
          </Button>
        </CardContent>
      </Card>

      <div className="flex items-center justify-center gap-5 rounded-[14px] border border-border bg-card px-4 py-3 text-sm text-muted-foreground">
        <span className="inline-flex items-center gap-1">
          <ShieldCheck className="size-4 text-indigo-600" />
          安全支付保障
        </span>
        <span className="inline-flex items-center gap-1">
          <RefreshCw className="size-4 text-indigo-600" />
          无生图数量限制
        </span>
        <span className="inline-flex items-center gap-1">
          <CheckCircle2 className="size-4 text-indigo-600" />
          无隐藏费用
        </span>
        <span>{safetyText}</span>
      </div>
    </section>
  );
}

export default function SubscriptionPage() {
  const { isCheckingAuth, session } = useAuthGuard(undefined, "/subscription");
  if (isCheckingAuth || !session) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
      </div>
    );
  }
  return <SubscriptionPageContent />;
}
