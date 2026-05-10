"use client";

import { useEffect, useMemo, useState } from "react";
import { LoaderCircle, Plus, Save, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  adjustManagedUserBalance,
  fetchAdminBillingOrders,
  createRedeemCodes,
  deleteRedeemCode,
  fetchManagedUsers,
  fetchRedeemCodes,
  updateRedeemCode,
  type AdminBillingStats,
  type ManagedUser,
  type PayOrder,
  type RedeemCode,
} from "@/lib/api";
import { parseDateTime } from "@/lib/datetime";

import { SettingsCard, settingsInputClassName } from "./settings-ui";

function centsToYuan(cents: number) {
  return (Math.max(0, cents) / 100).toFixed(2);
}

function payTypeLabel(type: string) {
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

function orderStatusLabel(status: string) {
  if (status === "paid") return { text: "已支付", variant: "success" as const };
  if (status === "failed") return { text: "失败", variant: "danger" as const };
  return { text: "待支付", variant: "warning" as const };
}

function formatBeijingDateTime(value?: string | null) {
  const date = parseDateTime(value);
  if (!date) {
    return "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

export function BillingAdminCard() {
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [users, setUsers] = useState<ManagedUser[]>([]);
  const [codes, setCodes] = useState<RedeemCode[]>([]);
  const [orders, setOrders] = useState<PayOrder[]>([]);
  const [stats, setStats] = useState<AdminBillingStats | null>(null);

  const [selectedUserId, setSelectedUserId] = useState("");
  const [adjustMode, setAdjustMode] = useState<"delta" | "set">("delta");
  const [adjustValue, setAdjustValue] = useState("100");
  const [adjustNote, setAdjustNote] = useState("");

  const [createAmount, setCreateAmount] = useState("10.00");
  const [createCount, setCreateCount] = useState("10");
  const [createExpireAt, setCreateExpireAt] = useState("");
  const [createNote, setCreateNote] = useState("");

  const emailUsers = useMemo(
    () => users.filter((item) => item.provider === "email"),
    [users],
  );

  const load = async () => {
    setIsLoading(true);
    try {
      const [usersData, codeData, ordersData] = await Promise.all([
        fetchManagedUsers(),
        fetchRedeemCodes(200),
        fetchAdminBillingOrders(0),
      ]);
      const nextUsers = Array.isArray(usersData.items) ? usersData.items : [];
      setUsers(nextUsers);
      setCodes(Array.isArray(codeData.items) ? codeData.items : []);
      setOrders(Array.isArray(ordersData.items) ? ordersData.items : []);
      setStats(ordersData.stats || null);
      if (!selectedUserId && nextUsers.length > 0) {
        const firstEmailUser = nextUsers.find((item) => item.provider === "email");
        if (firstEmailUser) {
          setSelectedUserId(firstEmailUser.id);
        }
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载计费管理失败");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleAdjustBalance = async () => {
    if (!selectedUserId) {
      toast.error("请先选择邮箱用户");
      return;
    }
    const value = Number(adjustValue.trim());
    if (!Number.isFinite(value)) {
      toast.error("请输入有效数字");
      return;
    }
    setIsSaving(true);
    try {
      if (adjustMode === "delta") {
        const deltaCents = Math.round(value);
        await adjustManagedUserBalance(selectedUserId, {
          delta_cents: deltaCents,
          note: adjustNote.trim(),
        });
      } else {
        const balanceCents = Math.round(value);
        await adjustManagedUserBalance(selectedUserId, {
          balance_cents: balanceCents,
          note: adjustNote.trim(),
        });
      }
      toast.success("余额调整成功");
      await load();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "余额调整失败");
    } finally {
      setIsSaving(false);
    }
  };

  const handleCreateCodes = async () => {
    const amount = Number(createAmount.trim());
    const count = Number(createCount.trim());
    if (!Number.isFinite(amount) || amount <= 0) {
      toast.error("充值金额不正确");
      return;
    }
    if (!Number.isFinite(count) || count < 1) {
      toast.error("卡密数量不正确");
      return;
    }
    setIsSaving(true);
    try {
      await createRedeemCodes({
        amount: amount.toFixed(2),
        count: Math.round(count),
        expires_at: createExpireAt.trim() || undefined,
        note: createNote.trim() || undefined,
      });
      toast.success("卡密已生成");
      await load();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "生成卡密失败");
    } finally {
      setIsSaving(false);
    }
  };

  const handleToggleCode = async (code: RedeemCode) => {
    setIsSaving(true);
    try {
      await updateRedeemCode(code.code, { enabled: !code.enabled });
      await load();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新卡密失败");
    } finally {
      setIsSaving(false);
    }
  };

  const handleDeleteCode = async (code: RedeemCode) => {
    setIsSaving(true);
    try {
      await deleteRedeemCode(code.code);
      await load();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除卡密失败");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <SettingsCard
      icon={Save}
      title="计费管理"
      description="管理员可直接调整邮箱用户余额，并生成/禁用/删除卡密。"
    >
      {isLoading ? (
        <div className="flex items-center justify-center py-10">
          <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <div className="flex flex-col gap-5">
          <section className="flex flex-col gap-3 rounded-[14px] border border-border/80 p-4">
            <h3 className="text-sm font-semibold">用户余额调整</h3>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field className="gap-1.5">
                <FieldLabel htmlFor="billing-user">邮箱用户</FieldLabel>
                <Select value={selectedUserId} onValueChange={setSelectedUserId}>
                  <SelectTrigger id="billing-user" className="h-10 rounded-[12px]">
                    <SelectValue placeholder="选择用户" />
                  </SelectTrigger>
                  <SelectContent>
                    {emailUsers.map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        {item.name || item.id}（￥{centsToYuan(Number(item.balance_cents || 0))}）
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </Field>

              <Field className="gap-1.5">
                <FieldLabel htmlFor="billing-mode">调整方式</FieldLabel>
                <Select value={adjustMode} onValueChange={(value) => setAdjustMode(value as "delta" | "set")}>
                  <SelectTrigger id="billing-mode" className="h-10 rounded-[12px]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="delta">增减金额（分）</SelectItem>
                    <SelectItem value="set">设置为固定余额（分）</SelectItem>
                  </SelectContent>
                </Select>
              </Field>

              <Field className="gap-1.5">
                <FieldLabel htmlFor="billing-value">数值</FieldLabel>
                <Input id="billing-value" value={adjustValue} onChange={(event) => setAdjustValue(event.target.value)} className={settingsInputClassName} />
              </Field>

              <Field className="gap-1.5">
                <FieldLabel htmlFor="billing-note">备注（可选）</FieldLabel>
                <Input id="billing-note" value={adjustNote} onChange={(event) => setAdjustNote(event.target.value)} className={settingsInputClassName} />
              </Field>
            </div>
            <Button className="h-10 rounded-[12px]" disabled={isSaving} onClick={() => void handleAdjustBalance()}>
              {isSaving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}
              提交余额调整
            </Button>
          </section>

          <section className="flex flex-col gap-3 rounded-[14px] border border-border/80 p-4">
            <h3 className="text-sm font-semibold">卡密生成</h3>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field className="gap-1.5">
                <FieldLabel htmlFor="redeem-amount">金额（元）</FieldLabel>
                <Input id="redeem-amount" value={createAmount} onChange={(event) => setCreateAmount(event.target.value)} className={settingsInputClassName} />
              </Field>
              <Field className="gap-1.5">
                <FieldLabel htmlFor="redeem-count">数量</FieldLabel>
                <Input id="redeem-count" value={createCount} onChange={(event) => setCreateCount(event.target.value)} className={settingsInputClassName} />
              </Field>
              <Field className="gap-1.5">
                <FieldLabel htmlFor="redeem-expire">过期时间（RFC3339，可选）</FieldLabel>
                <Input id="redeem-expire" value={createExpireAt} onChange={(event) => setCreateExpireAt(event.target.value)} placeholder="2026-12-31T23:59:59Z" className={settingsInputClassName} />
              </Field>
              <Field className="gap-1.5">
                <FieldLabel htmlFor="redeem-note">备注（可选）</FieldLabel>
                <Input id="redeem-note" value={createNote} onChange={(event) => setCreateNote(event.target.value)} className={settingsInputClassName} />
              </Field>
            </div>
            <Button className="h-10 rounded-[12px]" disabled={isSaving} onClick={() => void handleCreateCodes()}>
              {isSaving ? <LoaderCircle className="size-4 animate-spin" /> : <Plus className="size-4" />}
              生成卡密
            </Button>
          </section>

          <section className="flex flex-col gap-3 rounded-[14px] border border-border/80 p-4">
            <h3 className="text-sm font-semibold">卡密列表</h3>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>卡密</TableHead>
                    <TableHead>金额</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>使用者</TableHead>
                    <TableHead>过期</TableHead>
                    <TableHead>操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {codes.map((code) => (
                    <TableRow key={code.code}>
                      <TableCell className="font-mono text-xs">{code.code}</TableCell>
                      <TableCell>￥{code.amount_yuan}</TableCell>
                      <TableCell>{code.enabled ? "启用" : "禁用"}</TableCell>
                      <TableCell>{code.used_by || "-"}</TableCell>
                      <TableCell>{code.expires_at || "-"}</TableCell>
                      <TableCell className="flex gap-2">
                        <Button variant="outline" className="h-8 rounded-[10px] px-3" disabled={isSaving} onClick={() => void handleToggleCode(code)}>
                          {code.enabled ? "禁用" : "启用"}
                        </Button>
                        <Button variant="outline" className="h-8 rounded-[10px] border-rose-200 px-3 text-rose-600" disabled={isSaving} onClick={() => void handleDeleteCode(code)}>
                          <Trash2 className="size-4" />
                          删除
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </section>

          <section className="flex flex-col gap-3 rounded-[14px] border border-border/80 p-4">
            <h3 className="text-sm font-semibold">充值统计（北京时间）</h3>
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-[12px] border border-border/80 bg-muted/30 p-3">
                <div className="text-xs text-muted-foreground">今日收益</div>
                <div className="mt-1 text-lg font-semibold text-foreground">￥{stats?.today_revenue_yuan || "0.00"}</div>
                <div className="mt-1 text-xs text-muted-foreground">已支付 {stats?.today_paid_count || 0} 笔</div>
              </div>
              <div className="rounded-[12px] border border-border/80 bg-muted/30 p-3">
                <div className="text-xs text-muted-foreground">累计收益</div>
                <div className="mt-1 text-lg font-semibold text-foreground">￥{stats?.total_revenue_yuan || "0.00"}</div>
                <div className="mt-1 text-xs text-muted-foreground">已支付 {stats?.total_paid_count || 0} 笔</div>
              </div>
              <div className="rounded-[12px] border border-border/80 bg-muted/30 p-3">
                <div className="text-xs text-muted-foreground">待支付订单</div>
                <div className="mt-1 text-lg font-semibold text-foreground">{stats?.pending_count || 0}</div>
              </div>
              <div className="rounded-[12px] border border-border/80 bg-muted/30 p-3">
                <div className="text-xs text-muted-foreground">失败订单</div>
                <div className="mt-1 text-lg font-semibold text-foreground">{stats?.failed_count || 0}</div>
              </div>
            </div>
            <div className="text-xs text-muted-foreground">
              订单总数：{stats?.record_count || 0}，统计更新时间：{formatBeijingDateTime(stats?.updated_at)}
            </div>
          </section>

          <section className="flex flex-col gap-3 rounded-[14px] border border-border/80 p-4">
            <h3 className="text-sm font-semibold">全时段用户充值记录</h3>
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>订单号</TableHead>
                    <TableHead>用户邮箱</TableHead>
                    <TableHead>金额</TableHead>
                    <TableHead>方式</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>支付时间</TableHead>
                    <TableHead>创建时间</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {orders.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="py-6 text-center text-sm text-muted-foreground">
                        暂无充值记录
                      </TableCell>
                    </TableRow>
                  ) : (
                    orders.map((order) => {
                      const status = orderStatusLabel(order.status);
                      return (
                        <TableRow key={order.id}>
                          <TableCell className="font-mono text-xs">{order.out_trade_no}</TableCell>
                          <TableCell className="text-xs">{order.user_email || "-"}</TableCell>
                          <TableCell>￥{order.amount_yuan || centsToYuan(order.amount_cents)}</TableCell>
                          <TableCell>{payTypeLabel(order.pay_type)}</TableCell>
                          <TableCell>
                            <Badge variant={status.variant}>{status.text}</Badge>
                          </TableCell>
                          <TableCell className="text-xs text-muted-foreground">{formatBeijingDateTime(order.paid_at)}</TableCell>
                          <TableCell className="text-xs text-muted-foreground">{formatBeijingDateTime(order.created_at)}</TableCell>
                        </TableRow>
                      );
                    })
                  )}
                </TableBody>
              </Table>
            </div>
          </section>
        </div>
      )}
    </SettingsCard>
  );
}
