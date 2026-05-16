"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ChevronDown, ChevronUp, ExternalLink, Gift, LoaderCircle, RefreshCw, Wallet } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  createPayOrder,
  fetchAdminBillingOrders,
  fetchAgencyAdminWithdrawals,
  fetchPayOrders,
  fetchWallet,
  redeemWalletCode,
  updateAgencyAdminWithdrawal,
  type AdminBillingStats,
  type AgencyWithdrawalRequest,
  type PayOrder,
  type PayType,
  type WalletInfo,
} from "@/lib/api";
import { parseDateTime } from "@/lib/datetime";
import { useAuthGuard } from "@/lib/use-auth-guard";

const PAY_ORDER_PENDING_TIMEOUT_MS = 30 * 60 * 1000;
const PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const;

function centsToYuan(cents: number) {
  return (Math.max(0, cents) / 100).toFixed(2);
}

function formatDateTime(value?: string | null) {
  const date = parseDateTime(value);
  if (!date) {
    return "-";
  }
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  const second = String(date.getSeconds()).padStart(2, "0");
  return `${year}-${month}-${day} ${hour}:${minute}:${second}`;
}

function isPendingOrderTimeout(order: PayOrder, nowMs: number) {
  if (order.status !== "pending") {
    return false;
  }
  const createdAt = parseDateTime(order.created_at);
  if (!createdAt) {
    return false;
  }
  return nowMs - createdAt.getTime() >= PAY_ORDER_PENDING_TIMEOUT_MS;
}

function orderStatusLabel(order: PayOrder, nowMs: number) {
  if (order.status === "paid") return { text: "已支付", variant: "success" as const };
  if (order.status === "failed") return { text: "失败", variant: "danger" as const };
  if (isPendingOrderTimeout(order, nowMs)) return { text: "已超时未支付", variant: "danger" as const };
  return { text: "待支付", variant: "warning" as const };
}

function payTypeLabel(type?: string) {
  switch (type) {
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

function providerLabel(provider?: string) {
  switch (provider) {
    case "yipay":
      return "易支付";
    case "redeem_code":
      return "卡密兑换";
    case "admin_adjust":
      return "管理员调整";
    case "invite_bonus":
      return "邀请奖励";
    case "register_bonus":
      return "注册赠送";
    default:
      return provider || "-";
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

function recordAmountText(item: PayOrder) {
  const cents = Number(item.amount_cents || 0);
  if (item.record_type === "transaction" && item.type === "consume") {
    return `-￥${centsToYuan(Math.abs(cents))}`;
  }
  if (item.record_type === "transaction" && cents < 0) {
    return `-￥${centsToYuan(Math.abs(cents))}`;
  }
  return `￥${item.amount_yuan || centsToYuan(Math.abs(cents))}`;
}

function WalletPageContent() {
  const [wallet, setWallet] = useState<WalletInfo | null>(null);
  const [imagePriceCents, setImagePriceCents] = useState(8);
  const [orders, setOrders] = useState<PayOrder[]>([]);
  const [orderTypeFilter, setOrderTypeFilter] = useState<"all" | "recharge" | "consume">("all");
  const [orderStatusFilter, setOrderStatusFilter] = useState<"all" | "pending" | "paid" | "failed">("all");
  const [pageSize, setPageSize] = useState<number>(20);
  const [page, setPage] = useState(1);
  const [latestOrder, setLatestOrder] = useState<PayOrder | null>(null);
  const [amountYuan, setAmountYuan] = useState("10");
  const [payType, setPayType] = useState<PayType>("alipay");
  const [payChannels, setPayChannels] = useState<string[]>([]);
  const [redeemCode, setRedeemCode] = useState("");
  const [showInviteUsers, setShowInviteUsers] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isRedeeming, setIsRedeeming] = useState(false);
  const [nowMs, setNowMs] = useState(() => Date.now());

  const reload = useCallback(async () => {
    const [walletData, orderData] = await Promise.all([fetchWallet(), fetchPayOrders({ limit: 500 })]);
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

  useEffect(() => {
    const timer = window.setInterval(() => {
      setNowMs(Date.now());
    }, 10000);
    return () => {
      window.clearInterval(timer);
    };
  }, []);

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
      const payUrl = String(createdOrder.pay_url || "");
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

  const pendingCount = useMemo(
    () => orders.filter((order) => order.status === "pending" && !isPendingOrderTimeout(order, nowMs)).length,
    [nowMs, orders],
  );
  const filteredOrders = useMemo(() => {
    return orders.filter((order) => {
      const kind = order.record_type === "transaction" ? String(order.type || "").toLowerCase() : "recharge";
      if (orderTypeFilter !== "all") {
        if (orderTypeFilter === "consume" && kind !== "consume") return false;
        if (orderTypeFilter === "recharge" && kind === "consume") return false;
      }
      if (orderStatusFilter !== "all") {
        const normalized = String(order.status || "").toLowerCase();
        if (normalized !== orderStatusFilter) return false;
      }
      return true;
    });
  }, [orderStatusFilter, orderTypeFilter, orders]);
  const totalPages = Math.max(1, Math.ceil(filteredOrders.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  useEffect(() => {
    setPage((prev) => Math.max(1, Math.min(prev, totalPages)));
  }, [totalPages]);
  const pagedOrders = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredOrders.slice(start, start + pageSize);
  }, [currentPage, filteredOrders, pageSize]);
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
            <CardDescription>每次生图消耗</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="text-3xl font-semibold text-foreground">￥{centsToYuan(imagePriceCents)}</div>
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
          <div className="text-muted-foreground">我的邀请人：{wallet?.invited_by_email || wallet?.invited_by || "无"}</div>
          <button
            type="button"
            className="mt-1 inline-flex w-fit items-center gap-1 text-primary hover:underline"
            onClick={() => setShowInviteUsers((value) => !value)}
          >
            已邀请 {wallet?.invited_count || 0} 人 {showInviteUsers ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
          </button>
          {showInviteUsers ? (
            <div className="rounded-[12px] border border-border bg-muted/30 p-3">
              {(wallet?.invited_users || []).length === 0 ? (
                <div className="text-xs text-muted-foreground">暂无邀请记录</div>
              ) : (
                <div className="space-y-1.5 text-xs text-muted-foreground">
                  {(wallet?.invited_users || []).map((item) => (
                    <div key={item.id} className="flex items-center justify-between gap-3">
                      <span className="truncate">{item.email || item.name || item.id}</span>
                      <span>{formatDateTime(item.created_at)}</span>
                    </div>
                  ))}
                </div>
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
          <CardDescription>最近记录（待支付 {pendingCount} 条）</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-3 grid grid-cols-1 gap-2 sm:grid-cols-4">
            <Select value={orderTypeFilter} onValueChange={(value) => { setOrderTypeFilter(value as "all" | "recharge" | "consume"); setPage(1); }}>
              <SelectTrigger className="h-10 rounded-lg">
                <SelectValue placeholder="记录类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部记录</SelectItem>
                <SelectItem value="recharge">充值记录</SelectItem>
                <SelectItem value="consume">使用记录</SelectItem>
              </SelectContent>
            </Select>
            <Select value={orderStatusFilter} onValueChange={(value) => { setOrderStatusFilter(value as "all" | "pending" | "paid" | "failed"); setPage(1); }}>
              <SelectTrigger className="h-10 rounded-lg">
                <SelectValue placeholder="状态筛选" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部状态</SelectItem>
                <SelectItem value="pending">待支付</SelectItem>
                <SelectItem value="paid">已支付</SelectItem>
                <SelectItem value="failed">失败</SelectItem>
              </SelectContent>
            </Select>
            <Select value={String(pageSize)} onValueChange={(value) => { setPageSize(Number(value)); setPage(1); }}>
              <SelectTrigger className="h-10 rounded-lg">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PAGE_SIZE_OPTIONS.map((size) => (
                  <SelectItem key={size} value={String(size)}>
                    每页 {size}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              value={String(currentPage)}
              onChange={(event) => {
                const next = Number(event.target.value);
                if (Number.isFinite(next)) setPage(Math.max(1, Math.min(totalPages, Math.trunc(next))));
              }}
              inputMode="numeric"
              placeholder={`跳转页码 1-${totalPages}`}
              className="h-10 rounded-lg"
            />
          </div>
          {isLoading ? (
            <div className="flex min-h-[180px] items-center justify-center">
              <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : filteredOrders.length === 0 ? (
            <div className="rounded-[14px] border border-dashed border-border p-8 text-center text-sm text-muted-foreground">暂无充值订单</div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>记录号</TableHead>
                    <TableHead>金额</TableHead>
                    <TableHead>来源</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>备注</TableHead>
                    <TableHead>时间</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pagedOrders.map((order) => {
                    const status = orderStatusLabel(order, nowMs);
                    const source = order.record_type === "transaction"
                      ? providerLabel(order.provider)
                      : payTypeLabel(order.pay_type);
                    return (
                      <TableRow key={order.id}>
                        <TableCell className="font-mono text-xs">{order.out_trade_no || order.id}</TableCell>
                        <TableCell>{recordAmountText(order)}</TableCell>
                        <TableCell>{source}</TableCell>
                        <TableCell>
                          <Badge variant={status.variant}>{status.text}</Badge>
                        </TableCell>
                        <TableCell className="text-muted-foreground">{order.note || "-"}</TableCell>
                        <TableCell className="text-muted-foreground">{formatDateTime(order.created_at)}</TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
          {!isLoading && filteredOrders.length > 0 ? (
            <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-sm text-muted-foreground">
              <span>第 {currentPage} / {totalPages} 页，共 {filteredOrders.length} 条</span>
              <div className="flex items-center gap-2">
                <Button type="button" variant="outline" className="h-8 rounded-lg px-3" disabled={currentPage <= 1} onClick={() => setPage((p) => Math.max(1, p - 1))}>上一页</Button>
                <Button type="button" variant="outline" className="h-8 rounded-lg px-3" disabled={currentPage >= totalPages} onClick={() => setPage((p) => Math.min(totalPages, p + 1))}>下一页</Button>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </section>
  );
}

const WITHDRAW_STATUS_LABEL: Record<string, { text: string; variant: "success" | "danger" | "warning" }> = {
  pending: { text: "待审核", variant: "warning" },
  approved: { text: "已通过", variant: "success" },
  paid: { text: "已打款", variant: "success" },
  rejected: { text: "已驳回", variant: "danger" },
};

function orderKindLabel(kind?: string) {
  switch (kind) {
    case "agency_join":
      return "代理开通";
    case "agency_upgrade":
      return "代理升级";
    default:
      return "钱包充值";
  }
}

function payoutText(item: AgencyWithdrawalRequest) {
  const parts = [
    item.phone ? `手机号：${item.phone}` : "",
    item.wechat_id ? `微信号：${item.wechat_id}` : "",
    item.alipay_qr_code ? "支付宝码已提交" : "",
    item.wechat_qr_code ? "微信码已提交" : "",
  ].filter(Boolean);
  return parts.join(" / ") || "-";
}

function AdminWalletPageContent() {
  const [orders, setOrders] = useState<PayOrder[]>([]);
  const [stats, setStats] = useState<AdminBillingStats | null>(null);
  const [withdrawals, setWithdrawals] = useState<AgencyWithdrawalRequest[]>([]);
  const [withdrawalNotes, setWithdrawalNotes] = useState<Record<string, string>>({});
  const [orderStatus, setOrderStatus] = useState<"all" | "pending" | "paid" | "failed">("all");
  const [orderKind, setOrderKind] = useState<"all" | "recharge" | "agency_join" | "agency_upgrade">("all");
  const [orderPageSize, setOrderPageSize] = useState<number>(10);
  const [orderPage, setOrderPage] = useState(1);
  const [orderTotal, setOrderTotal] = useState(0);
  const [orderTotalPages, setOrderTotalPages] = useState(1);
  const [withdrawStatus, setWithdrawStatus] = useState<"all" | "pending" | "approved" | "paid" | "rejected">("all");
  const [isLoading, setIsLoading] = useState(true);
  const [processingWithdrawalID, setProcessingWithdrawalID] = useState("");

  const load = useCallback(async () => {
    setIsLoading(true);
    try {
      const [orderData, withdrawalData] = await Promise.all([
        fetchAdminBillingOrders({
          limit: 0,
          status: orderStatus,
          order_kind: orderKind,
          page: orderPage,
          page_size: orderPageSize,
        }),
        fetchAgencyAdminWithdrawals(500),
      ]);
      setOrders(Array.isArray(orderData.items) ? orderData.items : []);
      setStats(orderData.stats || null);
      setOrderTotal(Number(orderData.total || 0));
      setOrderTotalPages(Math.max(1, Number(orderData.total_page || 1)));
      const nextWithdrawals = Array.isArray(withdrawalData.items) ? withdrawalData.items : [];
      setWithdrawals(nextWithdrawals);
      setWithdrawalNotes((current) => {
        const next = { ...current };
        for (const item of nextWithdrawals) {
          if (next[item.id] === undefined) next[item.id] = item.admin_note || "";
        }
        return next;
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载提现和收益明细失败");
    } finally {
      setIsLoading(false);
    }
  }, [orderKind, orderPage, orderPageSize, orderStatus]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    setOrderPage((prev) => Math.max(1, Math.min(prev, orderTotalPages)));
  }, [orderTotalPages]);

  const filteredWithdrawals = useMemo(() => {
    return withdrawals.filter((item) => withdrawStatus === "all" || String(item.status || "pending") === withdrawStatus);
  }, [withdrawStatus, withdrawals]);

  const updateWithdrawal = async (id: string, status: "pending" | "approved" | "paid" | "rejected") => {
    setProcessingWithdrawalID(`${id}:${status}`);
    try {
      const res = await updateAgencyAdminWithdrawal({ id, status, admin_note: withdrawalNotes[id] || "" });
      setWithdrawals((current) => current.map((item) => (item.id === id ? res.item : item)));
      setWithdrawalNotes((current) => ({ ...current, [id]: res.item.admin_note || "" }));
      toast.success("提现状态已更新");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新提现状态失败");
    } finally {
      setProcessingWithdrawalID("");
    }
  };

  return (
    <section className="flex flex-col gap-5">
      <PageHeader
        eyebrow="Finance"
        title="提现和收益明细"
        actions={(
          <Button variant="outline" className="h-10 rounded-lg" onClick={() => void load()} disabled={isLoading}>
            <RefreshCw className={`size-4 ${isLoading ? "animate-spin" : ""}`} />
            刷新
          </Button>
        )}
      />

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-4">
        <Card className="rounded-[20px]">
          <CardHeader className="pb-3"><CardTitle className="text-base">今日收益</CardTitle></CardHeader>
          <CardContent><div className="text-2xl font-semibold">￥{stats?.today_revenue_yuan || "0.00"}</div></CardContent>
        </Card>
        <Card className="rounded-[20px]">
          <CardHeader className="pb-3"><CardTitle className="text-base">累计收益</CardTitle></CardHeader>
          <CardContent><div className="text-2xl font-semibold">￥{stats?.total_revenue_yuan || "0.00"}</div></CardContent>
        </Card>
        <Card className="rounded-[20px]">
          <CardHeader className="pb-3"><CardTitle className="text-base">待支付订单</CardTitle></CardHeader>
          <CardContent><div className="text-2xl font-semibold">{stats?.pending_count || 0}</div></CardContent>
        </Card>
        <Card className="rounded-[20px]">
          <CardHeader className="pb-3"><CardTitle className="text-base">提现申请</CardTitle></CardHeader>
          <CardContent><div className="text-2xl font-semibold">{withdrawals.length}</div></CardContent>
        </Card>
      </div>

      <Card className="rounded-[20px]">
        <CardHeader>
          <CardTitle className="text-base">代理提现审核</CardTitle>
          <CardDescription>代理提交提现后在这里审核、驳回或标记已打款。</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <Select value={withdrawStatus} onValueChange={(value) => setWithdrawStatus(value as typeof withdrawStatus)}>
              <SelectTrigger className="h-10 w-[160px] rounded-lg"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部状态</SelectItem>
                <SelectItem value="pending">待审核</SelectItem>
                <SelectItem value="approved">已通过</SelectItem>
                <SelectItem value="paid">已打款</SelectItem>
                <SelectItem value="rejected">已驳回</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="overflow-hidden rounded-[14px] border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>申请编号</TableHead>
                  <TableHead>用户邮箱</TableHead>
                  <TableHead>金额</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>收款信息</TableHead>
                  <TableHead>申请时间</TableHead>
                  <TableHead>备注</TableHead>
                  <TableHead>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow><TableCell colSpan={8} className="py-8 text-center text-muted-foreground"><LoaderCircle className="mr-2 inline size-4 animate-spin" />正在加载</TableCell></TableRow>
                ) : filteredWithdrawals.length === 0 ? (
                  <TableRow><TableCell colSpan={8} className="py-8 text-center text-muted-foreground">暂无提现申请</TableCell></TableRow>
                ) : filteredWithdrawals.map((item) => {
                  const status = String(item.status || "pending");
                  const label = WITHDRAW_STATUS_LABEL[status] || { text: status, variant: "warning" as const };
                  const isPaid = status === "paid";
                  return (
                    <TableRow key={item.id}>
                      <TableCell className="font-mono text-xs">{item.id}</TableCell>
                      <TableCell className="text-xs">{item.user_email || item.user_id || "-"}</TableCell>
                      <TableCell className="font-medium">￥{item.amount_yuan || centsToYuan(item.amount_cents || 0)}</TableCell>
                      <TableCell><Badge variant={label.variant}>{label.text}</Badge></TableCell>
                      <TableCell className="max-w-[240px] text-xs text-muted-foreground">
                        <div>{payoutText(item)}</div>
                        <div className="mt-1 flex flex-wrap gap-2">
                          {item.alipay_qr_code ? <a className="text-primary hover:underline" href={item.alipay_qr_code} target="_blank" rel="noreferrer">支付宝码</a> : null}
                          {item.wechat_qr_code ? <a className="text-primary hover:underline" href={item.wechat_qr_code} target="_blank" rel="noreferrer">微信码</a> : null}
                        </div>
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-xs text-muted-foreground">{formatDateTime(item.created_at)}</TableCell>
                      <TableCell>
                        <Input
                          value={withdrawalNotes[item.id] ?? item.admin_note ?? ""}
                          onChange={(event) => setWithdrawalNotes((current) => ({ ...current, [item.id]: event.target.value }))}
                          placeholder="管理员备注"
                          disabled={isPaid}
                          className="h-9 min-w-[140px] rounded-lg"
                        />
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-2">
                          <Button size="sm" variant="outline" disabled={isPaid || processingWithdrawalID !== ""} onClick={() => void updateWithdrawal(item.id, "approved")}>通过</Button>
                          <Button size="sm" disabled={isPaid || processingWithdrawalID !== ""} onClick={() => void updateWithdrawal(item.id, "paid")}>已打款</Button>
                          <Button size="sm" variant="destructive" disabled={isPaid || processingWithdrawalID !== ""} onClick={() => void updateWithdrawal(item.id, "rejected")}>驳回</Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Card className="rounded-[20px]">
        <CardHeader>
          <CardTitle className="text-base">充值和收益明细</CardTitle>
          <CardDescription>包含钱包充值、代理开通、代理升级订单，默认每页 10 条。</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="mb-3 grid grid-cols-1 gap-2 md:grid-cols-[160px_160px_140px_160px]">
            <Select value={orderStatus} onValueChange={(value) => { setOrderStatus(value as typeof orderStatus); setOrderPage(1); }}>
              <SelectTrigger className="h-10 rounded-lg"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部状态</SelectItem>
                <SelectItem value="pending">待支付</SelectItem>
                <SelectItem value="paid">已支付</SelectItem>
                <SelectItem value="failed">失败</SelectItem>
              </SelectContent>
            </Select>
            <Select value={orderKind} onValueChange={(value) => { setOrderKind(value as typeof orderKind); setOrderPage(1); }}>
              <SelectTrigger className="h-10 rounded-lg"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部类型</SelectItem>
                <SelectItem value="recharge">钱包充值</SelectItem>
                <SelectItem value="agency_join">代理开通</SelectItem>
                <SelectItem value="agency_upgrade">代理升级</SelectItem>
              </SelectContent>
            </Select>
            <Select value={String(orderPageSize)} onValueChange={(value) => { setOrderPageSize(Number(value)); setOrderPage(1); }}>
              <SelectTrigger className="h-10 rounded-lg"><SelectValue /></SelectTrigger>
              <SelectContent>
                {PAGE_SIZE_OPTIONS.map((size) => <SelectItem key={size} value={String(size)}>每页 {size}</SelectItem>)}
              </SelectContent>
            </Select>
            <Input
              value={String(orderPage)}
              onChange={(event) => {
                const next = Number(event.target.value);
                if (Number.isFinite(next)) setOrderPage(Math.max(1, Math.min(orderTotalPages, Math.trunc(next))));
              }}
              inputMode="numeric"
              placeholder={`页码 1-${orderTotalPages}`}
              className="h-10 rounded-lg"
            />
          </div>
          <div className="overflow-hidden rounded-[14px] border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>订单号</TableHead>
                  <TableHead>用户邮箱</TableHead>
                  <TableHead>金额</TableHead>
                  <TableHead>类型</TableHead>
                  <TableHead>方式</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>支付时间</TableHead>
                  <TableHead>创建时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow><TableCell colSpan={8} className="py-8 text-center text-muted-foreground"><LoaderCircle className="mr-2 inline size-4 animate-spin" />正在加载</TableCell></TableRow>
                ) : orders.length === 0 ? (
                  <TableRow><TableCell colSpan={8} className="py-8 text-center text-muted-foreground">暂无记录</TableCell></TableRow>
                ) : orders.map((order) => {
                  const status = orderStatusLabel(order, Date.now());
                  return (
                    <TableRow key={order.id}>
                      <TableCell className="font-mono text-xs">{order.out_trade_no || order.id}</TableCell>
                      <TableCell className="text-xs">{order.user_email || "-"}</TableCell>
                      <TableCell>￥{order.amount_yuan || centsToYuan(order.amount_cents || 0)}</TableCell>
                      <TableCell>{orderKindLabel(order.order_kind)}</TableCell>
                      <TableCell>{payTypeLabel(order.pay_type)}</TableCell>
                      <TableCell><Badge variant={status.variant}>{status.text}</Badge></TableCell>
                      <TableCell className="whitespace-nowrap text-xs text-muted-foreground">{formatDateTime(order.paid_at)}</TableCell>
                      <TableCell className="whitespace-nowrap text-xs text-muted-foreground">{formatDateTime(order.created_at)}</TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
          <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-sm text-muted-foreground">
            <span>第 {orderPage} / {orderTotalPages} 页，共 {orderTotal} 条</span>
            <div className="flex items-center gap-2">
              <Button type="button" variant="outline" className="h-8 rounded-lg px-3" disabled={orderPage <= 1} onClick={() => setOrderPage((p) => Math.max(1, p - 1))}>上一页</Button>
              <Button type="button" variant="outline" className="h-8 rounded-lg px-3" disabled={orderPage >= orderTotalPages} onClick={() => setOrderPage((p) => Math.min(orderTotalPages, p + 1))}>下一页</Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}

export default function WalletPage() {
  const { isCheckingAuth, session } = useAuthGuard(undefined, "/wallet");

  if (isCheckingAuth || !session) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <LoaderCircle className="size-5 animate-spin text-stone-400" />
      </div>
    );
  }

  if (session.role === "admin") {
    return <AdminWalletPageContent />;
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
