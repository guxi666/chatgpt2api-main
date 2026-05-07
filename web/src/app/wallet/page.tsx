"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ExternalLink, Gift, LoaderCircle, RefreshCw, Wallet } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { createPayOrder, fetchPayOrders, fetchWallet, redeemWalletCode, type PayOrder, type PayType, type WalletInfo } from "@/lib/api";
import { useAuthGuard } from "@/lib/use-auth-guard";

function centsToYuan(cents: number) {
  return (Math.max(0, cents) / 100).toFixed(2);
}

function orderStatusLabel(status: string) {
  if (status === "paid") return { text: "成功", variant: "success" as const };
  if (status === "failed") return { text: "失败", variant: "danger" as const };
  return { text: "待支付", variant: "warning" as const };
}

function payTypeLabel(type: string) {
  switch (type) {
    case "redeem_code":
      return "卡密兑换";
    case "admin_adjust":
      return "后台调账";
    case "register_bonus":
      return "注册赠送";
    case "invite_bonus":
      return "邀请奖励";
    case "wxpay":
      return "微信";
    case "paypal":
      return "PayPal";
    case "usdt":
      return "USDT";
    default:
      return "支付宝";
  }
}

function defaultPayType(channels: string[]): PayType {
  const allowed = new Set(channels);
  const priority: PayType[] = ["alipay", "wxpay", "paypal", "usdt"];
  for (const item of priority) {
    if (allowed.has(item)) {
      return item;
    }
  }
  return "alipay";
}

function WalletPageContent() {
  const [wallet, setWallet] = useState<WalletInfo | null>(null);
  const [imagePriceCents, setImagePriceCents] = useState(8);
  const [orders, setOrders] = useState<PayOrder[]>([]);
  const [latestOrder, setLatestOrder] = useState<PayOrder | null>(null);
  const [amountYuan, setAmountYuan] = useState("10");
  const [payType, setPayType] = useState<PayType>("alipay");
  const [payChannels, setPayChannels] = useState<string[]>([]);
  const [redeemCode, setRedeemCode] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isRedeeming, setIsRedeeming] = useState(false);
  const [showInvitees, setShowInvitees] = useState(false);

  const reload = useCallback(async () => {
    const [walletData, orderData] = await Promise.all([fetchWallet(), fetchPayOrders()]);
    setWallet(walletData.wallet);
    setImagePriceCents(walletData.image_price);
    const channels = Array.isArray(walletData.pay_channels) ? walletData.pay_channels : [];
    setPayChannels(channels);
    setPayType((current) => (channels.length > 0 && !channels.includes(current) ? defaultPayType(channels) : current));
    setOrders(Array.isArray(orderData.items) ? orderData.items : []);
  }, []);

  useEffect(() => {
    let active = true;
    const load = async () => {
      setIsLoading(true);
      try {
        await reload();
      } catch (error) {
        if (active) {
          toast.error(error instanceof Error ? error.message : "加载钱包失败");
        }
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
  }, [reload]);

  const handleCreateOrder = async () => {
    const normalizedAmount = amountYuan.trim();
    if (!normalizedAmount) {
      toast.error("请输入充值金额");
      return;
    }
    const parsedAmount = Number(normalizedAmount);
    if (!Number.isFinite(parsedAmount) || parsedAmount <= 0) {
      toast.error("充值金额不正确");
      return;
    }

    setIsSubmitting(true);
    try {
      const data = await createPayOrder({
        amount: parsedAmount.toFixed(2),
        pay_type: payType,
      });
      const createdOrder = data.order;
      const payUrl = createdOrder.pay_url || "";
      setLatestOrder(createdOrder);
      if (payUrl) {
        window.open(payUrl, "_blank", "noopener,noreferrer");
        toast.success("订单已创建，请在新窗口完成支付");
      } else if (createdOrder.pay_type === "usdt") {
        toast.success("USDT 订单已创建，请按页面收款信息转账并等待到账。");
      } else {
        toast.success("订单已创建");
      }
      await reload();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "创建订单失败");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleRedeem = async () => {
    const code = redeemCode.trim();
    if (!code) {
      toast.error("请输入卡密");
      return;
    }
    setIsRedeeming(true);
    try {
      await redeemWalletCode({ code });
      setRedeemCode("");
      toast.success("兑换成功，余额已更新");
      await reload();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "兑换失败");
    } finally {
      setIsRedeeming(false);
    }
  };

  const pendingCount = useMemo(() => orders.filter((order) => order.status === "pending").length, [orders]);
  const effectiveChannels = payChannels;

  return (
    <section className="flex flex-col gap-5">
      <PageHeader
        eyebrow="Wallet"
        title="钱包充值"
        actions={(
          <Button variant="outline" className="h-10 rounded-lg" onClick={() => void reload()} disabled={isLoading || isSubmitting || isRedeeming}>
            <RefreshCw className={`size-4 ${isLoading ? "animate-spin" : ""}`} />
            刷新
          </Button>
        )}
      />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <Card className="rounded-[20px]">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">余额</CardTitle>
            <CardDescription>当前可用余额</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-semibold text-foreground">￥{centsToYuan(wallet?.balance_cents || 0)}</div>
          </CardContent>
        </Card>
        <Card className="rounded-[20px]">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">累计充值</CardTitle>
            <CardDescription>历史总充值</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-semibold text-foreground">￥{centsToYuan(wallet?.total_recharge_cents || 0)}</div>
          </CardContent>
        </Card>
        <Card className="rounded-[20px]">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">单次扣费</CardTitle>
            <CardDescription>生图/对话消耗</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-semibold text-foreground">￥{centsToYuan(imagePriceCents)}</div>
            <div className="mt-2 text-xs text-muted-foreground">文本对话每次固定消耗 ￥0.10</div>
          </CardContent>
        </Card>
      </div>
      <Card className="rounded-[20px]">
        <CardHeader>
          <CardTitle className="text-base">我的邀请码</CardTitle>
          <CardDescription>邀请他人注册可获赠 10 次生图点数</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-2 text-sm">
          <div className="font-mono text-foreground">{wallet?.invite_code || "-"}</div>
          <div className="text-muted-foreground">我的邀请人：{wallet?.invited_by || "无"}</div>
          <div className="text-muted-foreground">已邀请注册：{wallet?.invitee_count ?? 0} 人</div>
          <div>
            <Button type="button" variant="outline" className="h-8 rounded-[10px] px-3 text-xs" onClick={() => setShowInvitees((current) => !current)}>
              {showInvitees ? "收起邀请详情" : "查看邀请详情"}
            </Button>
          </div>
          {showInvitees ? (
            <div className="rounded-[12px] border border-border bg-muted/40 p-3">
              {wallet?.invitees && wallet.invitees.length > 0 ? (
                <div className="flex flex-col gap-1 text-xs text-muted-foreground">
                  {wallet.invitees.map((item) => (
                    <div key={item.id || item.email} className="flex items-center justify-between gap-3">
                      <span className="font-mono text-foreground">{item.email || "-"}</span>
                      <span>{item.created_at || "-"}</span>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-xs text-muted-foreground">暂无被邀请用户</div>
              )}
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card className="rounded-[20px]">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Gift className="size-4" />
            卡密兑换
          </CardTitle>
          <CardDescription>输入管理员发放的卡密可直接增加余额</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,1fr)_180px]">
          <Input
            value={redeemCode}
            onChange={(event) => setRedeemCode(event.target.value)}
            placeholder="请输入卡密"
            className="h-11 rounded-[14px]"
          />
          <Button className="h-11 rounded-[14px]" disabled={isRedeeming || isLoading} onClick={() => void handleRedeem()}>
            {isRedeeming ? <LoaderCircle className="size-4 animate-spin" /> : <Gift className="size-4" />}
            立即兑换
          </Button>
        </CardContent>
      </Card>

      <Card className="rounded-[20px]">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Wallet className="size-4" />
            创建充值订单
          </CardTitle>
          <CardDescription>创建后会跳转到对应支付页面</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,1fr)_180px_160px]">
          <Input
            value={amountYuan}
            onChange={(event) => setAmountYuan(event.target.value)}
            inputMode="decimal"
            placeholder="充值金额（元）"
            className="h-11 rounded-[14px]"
          />
          <Select value={payType} onValueChange={(value) => setPayType(value as PayType)}>
            <SelectTrigger className="h-11 rounded-[14px]">
              <SelectValue placeholder="支付方式" />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {effectiveChannels.includes("alipay") ? <SelectItem value="alipay">支付宝</SelectItem> : null}
                {effectiveChannels.includes("wxpay") ? <SelectItem value="wxpay">微信</SelectItem> : null}
                {effectiveChannels.includes("paypal") ? <SelectItem value="paypal">PayPal</SelectItem> : null}
                {effectiveChannels.includes("usdt") ? <SelectItem value="usdt">USDT</SelectItem> : null}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Button
            className="h-11 rounded-[14px]"
            disabled={isSubmitting || isLoading || effectiveChannels.length === 0}
            onClick={() => void handleCreateOrder()}
          >
            {isSubmitting ? <LoaderCircle className="size-4 animate-spin" /> : <ExternalLink className="size-4" />}
            立即充值
          </Button>
          {effectiveChannels.length === 0 ? (
            <div className="sm:col-span-3 rounded-[12px] border border-dashed border-border px-3 py-2 text-sm text-muted-foreground">
              暂未开放支付渠道，请联系管理员在系统设置中启用易支付。
            </div>
          ) : null}
          {latestOrder && latestOrder.pay_type === "usdt" && !latestOrder.pay_url ? (
            <div className="sm:col-span-3 rounded-[12px] border border-border bg-muted/40 px-3 py-3 text-sm">
              <div className="font-medium">USDT 收款信息</div>
              <div className="mt-1 text-muted-foreground">订单号：{latestOrder.out_trade_no}</div>
              <div className="mt-1 text-muted-foreground">网络：{latestOrder.usdt_network || "-"}</div>
              <div className="mt-1 break-all text-muted-foreground">地址：{latestOrder.usdt_address || "-"}</div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card className="rounded-[20px]">
        <CardHeader>
          <CardTitle className="text-base">充值订单</CardTitle>
          <CardDescription>最近 30 条（待支付 {pendingCount} 条）</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="flex min-h-[180px] items-center justify-center">
              <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : orders.length === 0 ? (
            <div className="rounded-[14px] border border-dashed border-border p-8 text-center text-sm text-muted-foreground">暂无充值订单</div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>订单号</TableHead>
                    <TableHead>金额</TableHead>
                    <TableHead>方式</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>说明</TableHead>
                    <TableHead>时间</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {orders.map((order) => {
                    const status = orderStatusLabel(order.status);
                    return (
                      <TableRow key={order.id}>
                        <TableCell className="font-mono text-xs">{order.out_trade_no}</TableCell>
                        <TableCell>￥{order.amount_yuan || centsToYuan(order.amount_cents)}</TableCell>
                        <TableCell>{payTypeLabel(order.pay_type)}</TableCell>
                        <TableCell>
                          <Badge variant={status.variant}>{status.text}</Badge>
                        </TableCell>
                        <TableCell className="max-w-[280px] truncate text-muted-foreground">{order.note || "-"}</TableCell>
                        <TableCell className="text-muted-foreground">{order.created_at || "-"}</TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </section>
  );
}

export default function WalletPage() {
  const { isCheckingAuth, session } = useAuthGuard(["user"]);

  if (isCheckingAuth || !session) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <LoaderCircle className="size-5 animate-spin text-stone-400" />
      </div>
    );
  }

  if (session.provider === "linuxdo") {
    return (
      <Card className="mx-auto mt-12 w-full max-w-2xl rounded-[20px]">
        <CardHeader>
          <CardTitle>当前账号不支持钱包充值</CardTitle>
          <CardDescription>Linuxdo 账号暂不支持余额充值与按次扣费。</CardDescription>
        </CardHeader>
      </Card>
    );
  }

  return <WalletPageContent />;
}
