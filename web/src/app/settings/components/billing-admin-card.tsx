"use client";

import { useEffect, useState } from "react";
import { LoaderCircle, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  createRedeemCodes,
  deleteRedeemCode,
  fetchRedeemCodes,
  updateRedeemCode,
  type RedeemCode,
} from "@/lib/api";
import { formatBeijingDateTime } from "@/lib/datetime";

import { SettingsCard, settingsInputClassName } from "./settings-ui";

function formatDateTime(value?: string | null) {
  return formatBeijingDateTime(value);
}

export function BillingAdminCard() {
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [codes, setCodes] = useState<RedeemCode[]>([]);
  const [createAmount, setCreateAmount] = useState("10.00");
  const [createCount, setCreateCount] = useState("10");
  const [createExpireAt, setCreateExpireAt] = useState("");
  const [createNote, setCreateNote] = useState("");

  const loadCodes = async () => {
    setIsLoading(true);
    try {
      const codeData = await fetchRedeemCodes(50);
      setCodes(Array.isArray(codeData.items) ? codeData.items : []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载卡密失败");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadCodes();
  }, []);

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
      await loadCodes();
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
      await loadCodes();
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
      await loadCodes();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除卡密失败");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <SettingsCard title="计费管理" icon={Plus} description="只保留卡密生成与卡密管理。">
      <div className="flex flex-col gap-5">
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
              <FieldLabel htmlFor="redeem-expire">过期时间（可选）</FieldLabel>
              <Input id="redeem-expire" type="datetime-local" value={createExpireAt} onChange={(event) => setCreateExpireAt(event.target.value)} className={settingsInputClassName} />
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
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : (
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
                      <TableCell>{formatDateTime(code.expires_at)}</TableCell>
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
          )}
        </section>
      </div>
    </SettingsCard>
  );
}
