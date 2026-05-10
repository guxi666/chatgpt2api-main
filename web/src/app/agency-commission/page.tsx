"use client";

import { useEffect, useMemo, useState } from "react";
import { Copy, LoaderCircle, RefreshCw } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { fetchAgencyCommissionDashboard, type AgencyCommissionDashboard } from "@/lib/api";
import { formatBeijingDateTime } from "@/lib/datetime";
import { useAuthGuard } from "@/lib/use-auth-guard";

function formatDateTime(value?: string) {
  return formatBeijingDateTime(value);
}

function CommissionContent() {
  const [isLoading, setIsLoading] = useState(true);
  const [dashboard, setDashboard] = useState<AgencyCommissionDashboard | null>(null);

  const load = async () => {
    setIsLoading(true);
    try {
      const data = await fetchAgencyCommissionDashboard();
      setDashboard(data);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载代理分成失败");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const orders = useMemo(() => dashboard?.orders || [], [dashboard?.orders]);
  const summary = dashboard?.summary;
  const agent = dashboard?.agent;

  return (
    <section className="flex flex-col gap-5">
      <PageHeader
        eyebrow="代理"
        title="代理分成"
        actions={(
          <Button variant="outline" className="h-10 rounded-lg" onClick={() => void load()} disabled={isLoading}>
            <RefreshCw className={`size-4 ${isLoading ? "animate-spin" : ""}`} />
            刷新
          </Button>
        )}
      />

      <Card className="rounded-2xl border-border/80">
        <CardHeader>
          <CardTitle className="text-lg">我的专属渠道链接</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="text-sm text-muted-foreground">邀请用户通过此链接注册并消费后，可在本页查看分成明细。</div>
          <div className="flex flex-col gap-2 md:flex-row md:items-center">
            <code className="flex-1 rounded-lg border border-border bg-muted/30 px-3 py-2 text-sm">{agent?.channel_link || "-"}</code>
            <Button
              type="button"
              variant="outline"
              className="h-10 rounded-lg"
              onClick={async () => {
                if (!agent?.channel_link) {
                  toast.error("暂无可复制链接");
                  return;
                }
                try {
                  await navigator.clipboard.writeText(agent.channel_link);
                  toast.success("渠道链接已复制");
                } catch {
                  toast.error("复制失败");
                }
              }}
            >
              <Copy className="size-4" />
              复制链接
            </Button>
          </div>
          <div className="text-sm text-muted-foreground">
            当前代理等级：<span className="font-medium text-foreground">{agent?.tier || "-"}</span>
            {" · "}
            分成比例：<span className="font-medium text-foreground">{((agent?.commission_bp || 0) / 100).toFixed(2)}%</span>
            {" · "}
            充值优惠：<span className="font-medium text-foreground">{((agent?.discount_bp || 0) / 100).toFixed(2)}%</span>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 lg:grid-cols-4">
        <Card className="rounded-2xl">
          <CardContent className="pt-6">
            <div className="text-sm text-muted-foreground">今日分成</div>
            <div className="mt-2 text-3xl font-bold">￥{summary?.today_commission_yuan || "0.00"}</div>
          </CardContent>
        </Card>
        <Card className="rounded-2xl">
          <CardContent className="pt-6">
            <div className="text-sm text-muted-foreground">本月分成</div>
            <div className="mt-2 text-3xl font-bold">￥{summary?.month_commission_yuan || "0.00"}</div>
          </CardContent>
        </Card>
        <Card className="rounded-2xl">
          <CardContent className="pt-6">
            <div className="text-sm text-muted-foreground">累计分成</div>
            <div className="mt-2 text-3xl font-bold">￥{summary?.total_commission_yuan || "0.00"}</div>
          </CardContent>
        </Card>
        <Card className="rounded-2xl">
          <CardContent className="pt-6">
            <div className="text-sm text-muted-foreground">可提现金额</div>
            <div className="mt-2 text-3xl font-bold">￥{summary?.available_yuan || "0.00"}</div>
          </CardContent>
        </Card>
      </div>

      <Card className="rounded-2xl">
        <CardHeader>
          <CardTitle className="text-lg">被邀请用户充值订单</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <Table className="min-w-[980px]">
              <TableHeader>
                <TableRow>
                  <TableHead>用户</TableHead>
                  <TableHead>订单号</TableHead>
                  <TableHead>充值金额</TableHead>
                  <TableHead>分成金额</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {orders.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{item.user_email || item.user_id || "-"}</TableCell>
                    <TableCell>{item.out_trade_no || "-"}</TableCell>
                    <TableCell>￥{item.amount_yuan || "0.00"}</TableCell>
                    <TableCell>￥{item.commission_yuan || "0.00"}</TableCell>
                    <TableCell>{item.status || "-"}</TableCell>
                    <TableCell>{formatDateTime(item.created_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          {!isLoading && orders.length === 0 ? <div className="px-6 py-10 text-center text-sm text-muted-foreground">暂无分成订单</div> : null}
          {isLoading ? (
            <div className="flex items-center justify-center py-10">
              <LoaderCircle className="size-5 animate-spin text-stone-400" />
            </div>
          ) : null}
        </CardContent>
      </Card>
    </section>
  );
}

export default function AgencyCommissionPage() {
  const { isCheckingAuth, session } = useAuthGuard(["admin", "user"], "/agency-commission");
  if (isCheckingAuth || !session) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <LoaderCircle className="size-5 animate-spin text-stone-400" />
      </div>
    );
  }
  return <CommissionContent />;
}
