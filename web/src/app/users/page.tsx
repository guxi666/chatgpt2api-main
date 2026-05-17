"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Ban,
  CheckCircle2,
  Copy,
  Eye,
  EyeOff,
  LoaderCircle,
  Plus,
  RefreshCw,
  RotateCcw,
  Search,
  Trash2,
  X,
} from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  adjustManagedUserBalance,
  createManagedUser,
  deleteManagedUser,
  fetchManagedUsers,
  resetManagedUserPassword,
  resetManagedUserKey,
  revealManagedUserKey,
  updateManagedUser,
  type ManagedUser,
} from "@/lib/api";
import { formatBeijingDateTime } from "@/lib/datetime";
import { useAuthGuard } from "@/lib/use-auth-guard";

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const;

type ProviderFilter = "all" | "local" | "email";
type StatusFilter = "all" | "enabled" | "disabled";

function formatDateTime(value?: string | null) {
  return formatBeijingDateTime(value);
}

function toNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function centsToYuan(cents: number) {
  return (Math.max(0, cents) / 100).toFixed(2);
}

function providerLabel(provider?: string) {
  if (provider === "email") return "邮箱";
  if (provider === "local") return "本地";
  return "其他";
}

function displayEmail(email?: string | null) {
  const raw = String(email || "").trim();
  if (!raw) return "-";
  if (raw.toLowerCase().endsWith("@local.invalid")) return "-";
  const match = raw.match(/^local[a-z0-9-]*_(.+@.+)$/i);
  return match?.[1] || raw;
}

function roleLevelLabel(user: ManagedUser) {
  return (user.role_name || "").trim() || "普通用户";
}

function userSearchText(user: ManagedUser) {
  return [
    user.id,
    user.username,
    user.email,
    user.name,
    user.owner_id,
    user.owner_name,
    user.provider,
    user.role_name,
    user.api_key_id,
    user.api_key_name,
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

async function copyTextToClipboard(value: string) {
  if (navigator.clipboard?.writeText && window.isSecureContext) {
    await navigator.clipboard.writeText(value);
    return;
  }

  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.top = "-9999px";
  textarea.style.left = "-9999px";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();
  textarea.setSelectionRange(0, textarea.value.length);

  try {
    if (!document.execCommand("copy")) {
      throw new Error("copy command rejected");
    }
  } finally {
    document.body.removeChild(textarea);
  }
}

function UsersContent() {
  const [items, setItems] = useState<ManagedUser[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [searchText, setSearchText] = useState("");
  const [providerFilter, setProviderFilter] = useState<ProviderFilter>("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [pageSize, setPageSize] = useState<number>(20);
  const [page, setPage] = useState(1);

  const [pendingIDs, setPendingIDs] = useState<Set<string>>(() => new Set());
  const [revealingIDs, setRevealingIDs] = useState<Set<string>>(() => new Set());
  const [revealedKeysByID, setRevealedKeysByID] = useState<Record<string, string>>({});

  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [createUsername, setCreateUsername] = useState("");
  const [createName, setCreateName] = useState("");
  const [createPassword, setCreatePassword] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const [resettingUser, setResettingUser] = useState<ManagedUser | null>(null);
  const [resetName, setResetName] = useState("");
  const [passwordResetUser, setPasswordResetUser] = useState<ManagedUser | null>(null);
  const [nextPassword, setNextPassword] = useState("");

  const [deletingUser, setDeletingUser] = useState<ManagedUser | null>(null);

  const [adjustingUser, setAdjustingUser] = useState<ManagedUser | null>(null);
  const [adjustMode, setAdjustMode] = useState<"delta" | "set">("delta");
  const [adjustValue, setAdjustValue] = useState("1.00");
  const [adjustNote, setAdjustNote] = useState("");

  const setUserPending = (userID: string, pending: boolean) => {
    setPendingIDs((current) => {
      const next = new Set(current);
      if (pending) next.add(userID);
      else next.delete(userID);
      return next;
    });
  };

  const setRevealPending = (userID: string, pending: boolean) => {
    setRevealingIDs((current) => {
      const next = new Set(current);
      if (pending) next.add(userID);
      else next.delete(userID);
      return next;
    });
  };

  const loadUsers = useCallback(async () => {
    setIsLoading(true);
    try {
      const data = await fetchManagedUsers();
      setItems(Array.isArray(data.items) ? data.items : []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载用户失败");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  const filteredItems = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return items.filter((user) => {
      if (providerFilter !== "all" && user.provider !== providerFilter) return false;
      if (statusFilter === "enabled" && !user.enabled) return false;
      if (statusFilter === "disabled" && user.enabled) return false;
      return !keyword || userSearchText(user).includes(keyword);
    });
  }, [items, providerFilter, searchText, statusFilter]);

  const totalPages = Math.max(1, Math.ceil(filteredItems.length / pageSize));
  const currentPage = Math.min(page, totalPages);

  useEffect(() => {
    setPage((prev) => Math.max(1, Math.min(prev, totalPages)));
  }, [totalPages]);

  const pagedItems = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredItems.slice(start, start + pageSize);
  }, [currentPage, filteredItems, pageSize]);

  const hasActiveFilters = searchText.trim() !== "" || providerFilter !== "all" || statusFilter !== "all";

  const handleCreate = async () => {
    const username = createUsername.trim();
    const password = createPassword.trim();
    if (!username || !password) {
      toast.error("请填写用户名和密码");
      return;
    }
    setIsCreating(true);
    try {
      const data = await createManagedUser({
        username,
        password,
        name: createName.trim() || username,
        enabled: true,
      });
      setItems(Array.isArray(data.items) ? data.items : []);
      setCreateUsername("");
      setCreateName("");
      setCreatePassword("");
      setIsCreateDialogOpen(false);
      toast.success("用户已创建");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "创建用户失败");
    } finally {
      setIsCreating(false);
    }
  };

  const handleToggle = async (user: ManagedUser) => {
    setUserPending(user.id, true);
    try {
      const data = await updateManagedUser(user.id, { enabled: !user.enabled });
      setItems(Array.isArray(data.items) ? data.items : []);
      toast.success(user.enabled ? "用户已禁用" : "用户已启用");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新用户失败");
    } finally {
      setUserPending(user.id, false);
    }
  };

  const handleReveal = async (user: ManagedUser) => {
    if (revealedKeysByID[user.id]) {
      setRevealedKeysByID((current) => {
        const next = { ...current };
        delete next[user.id];
        return next;
      });
      return;
    }
    if (!user.has_api_key || user.provider === "linuxdo") {
      toast.error("该用户暂无可管理密钥");
      return;
    }
    setRevealPending(user.id, true);
    try {
      const data = await revealManagedUserKey(user.id);
      setRevealedKeysByID((current) => ({ ...current, [user.id]: data.key }));
      toast.success("已显示密钥");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "显示密钥失败");
    } finally {
      setRevealPending(user.id, false);
    }
  };

  const handleResetKey = async () => {
    if (!resettingUser) return;
    setUserPending(resettingUser.id, true);
    try {
      const data = await resetManagedUserKey(resettingUser.id, resetName.trim());
      setItems(Array.isArray(data.items) ? data.items : []);
      setRevealedKeysByID((current) => ({ ...current, [resettingUser.id]: data.key }));
      setResettingUser(null);
      setResetName("");
      toast.success("用户密钥已更新");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "重置密钥失败");
    } finally {
      setUserPending(resettingUser.id, false);
    }
  };

  const handleResetPassword = async () => {
    if (!passwordResetUser) return;
    const password = nextPassword.trim();
    if (!password) {
      toast.error("请输入新密码");
      return;
    }
    setUserPending(passwordResetUser.id, true);
    try {
      const data = await resetManagedUserPassword(passwordResetUser.id, password);
      setItems(Array.isArray(data.items) ? data.items : []);
      setPasswordResetUser(null);
      setNextPassword("");
      toast.success("用户密码已重置");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "重置密码失败");
    } finally {
      setUserPending(passwordResetUser.id, false);
    }
  };

  const handleDelete = async () => {
    if (!deletingUser) return;
    setUserPending(deletingUser.id, true);
    try {
      const data = await deleteManagedUser(deletingUser.id);
      setItems(Array.isArray(data.items) ? data.items : []);
      setDeletingUser(null);
      toast.success("用户已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除用户失败");
    } finally {
      setUserPending(deletingUser.id, false);
    }
  };

  const handleAdjustBalance = async () => {
    if (!adjustingUser) return;
    const value = Number(adjustValue.trim());
    if (!Number.isFinite(value)) {
      toast.error("请输入有效金额");
      return;
    }
    const valueCents = Math.round(value * 100);
    setUserPending(adjustingUser.id, true);
    try {
      const payload =
        adjustMode === "delta"
          ? { delta_cents: valueCents, note: adjustNote.trim() }
          : { balance_cents: Math.max(0, valueCents), note: adjustNote.trim() };
      const data = await adjustManagedUserBalance(adjustingUser.id, payload);
      setItems(Array.isArray(data.items) ? data.items : []);
      setAdjustingUser(null);
      toast.success("余额已更新");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "余额更新失败");
    } finally {
      setUserPending(adjustingUser.id, false);
    }
  };

  const handleCopy = async (value: string) => {
    const text = value.trim();
    if (!text) {
      toast.error("没有可复制的内容");
      return;
    }
    try {
      await copyTextToClipboard(text);
      toast.success("已复制");
    } catch {
      toast.error("复制失败");
    }
  };

  return (
    <section className="flex flex-col gap-5">
      <PageHeader
        eyebrow="用户"
        title="用户管理"
        actions={(
          <>
            <Button variant="outline" onClick={() => void loadUsers()} disabled={isLoading} className="h-10 rounded-lg">
              <RefreshCw className={`size-4 ${isLoading ? "animate-spin" : ""}`} />
              刷新
            </Button>
            <Button onClick={() => setIsCreateDialogOpen(true)} className="h-10 rounded-lg">
              <Plus className="size-4" />
              创建用户
            </Button>
          </>
        )}
      />

      <Card className="overflow-hidden">
        <CardContent className="p-0">
          <div className="flex flex-col gap-3 border-b border-border px-5 py-4">
            <div className="flex items-center justify-between text-sm text-muted-foreground">
              <span>总计 {filteredItems.length} / {items.length}</span>
              <div className="flex items-center gap-2">
                <span>每页</span>
                <Select value={String(pageSize)} onValueChange={(value) => { setPageSize(Number(value)); setPage(1); }}>
                  <SelectTrigger className="h-8 w-[96px] rounded-lg"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {PAGE_SIZE_OPTIONS.map((size) => <SelectItem key={size} value={String(size)}>{size}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="grid gap-2 lg:grid-cols-[minmax(18rem,1fr)_160px_160px_auto]">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={searchText}
                  onChange={(event) => { setSearchText(event.target.value); setPage(1); }}
                  placeholder="搜索 ID/用户名/邮箱/密钥"
                  className="h-10 rounded-lg pl-9"
                />
              </div>
              <Select value={providerFilter} onValueChange={(value) => { setProviderFilter(value as ProviderFilter); setPage(1); }}>
                <SelectTrigger className="h-10 rounded-lg"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部来源</SelectItem>
                  <SelectItem value="local">本地</SelectItem>
                  <SelectItem value="email">邮箱</SelectItem>
                </SelectContent>
              </Select>
              <Select value={statusFilter} onValueChange={(value) => { setStatusFilter(value as StatusFilter); setPage(1); }}>
                <SelectTrigger className="h-10 rounded-lg"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部状态</SelectItem>
                  <SelectItem value="enabled">启用</SelectItem>
                  <SelectItem value="disabled">禁用</SelectItem>
                </SelectContent>
              </Select>
              <Button
                type="button"
                variant="outline"
                className="h-10 rounded-lg px-3"
                disabled={!hasActiveFilters}
                onClick={() => {
                  setSearchText("");
                  setProviderFilter("all");
                  setStatusFilter("all");
                  setPage(1);
                }}
              >
                <X className="size-4" />
                清空筛选
              </Button>
            </div>
          </div>

          <div className="overflow-x-auto lg:overflow-visible">
            <Table className="min-w-[1320px] w-full">
              <TableHeader>
                <TableRow>
                  <TableHead>用户</TableHead>
                  <TableHead>来源</TableHead>
                  <TableHead>等级</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>余额</TableHead>
                  <TableHead>用户密钥</TableHead>
                  <TableHead>时间</TableHead>
                  <TableHead className="w-[350px]">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pagedItems.map((user) => {
                  const pending = pendingIDs.has(user.id);
                  const revealing = revealingIDs.has(user.id);
                  const revealedKey = revealedKeysByID[user.id] ?? "";
                  const canManageKey = user.provider !== "linuxdo";
                  return (
                    <TableRow key={user.id}>
                      <TableCell>
                        <div className="space-y-1">
                          <div className="truncate font-medium text-foreground">{user.name || user.username || "用户"}</div>
                          <div className="truncate text-xs text-muted-foreground">{`Email: ${displayEmail(user.email)}`}</div>
                          <div className="truncate text-xs text-muted-foreground">
                            密码状态: {user.has_password ? "已设置" : "未设置"}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell><Badge variant="secondary" className="rounded-md">{providerLabel(user.provider)}</Badge></TableCell>
                      <TableCell><Badge variant="secondary" className="rounded-md">{roleLevelLabel(user)}</Badge></TableCell>
                      <TableCell>
                        <Badge variant={user.enabled ? "success" : "danger"} className="rounded-md">{user.enabled ? "启用" : "禁用"}</Badge>
                      </TableCell>
                      <TableCell>
                        <div className="space-y-1">
                          <div className="text-base font-semibold text-foreground">¥ {centsToYuan(toNumber(user.balance_cents))}</div>
                          <div className="text-xs text-muted-foreground">总充值 ¥ {centsToYuan(toNumber(user.total_recharge_cents))}</div>
                        </div>
                      </TableCell>
                      <TableCell>
                        {canManageKey ? (
                          <div className="max-w-[320px] space-y-1">
                            <div className="text-xs text-muted-foreground truncate">密钥ID: {user.api_key_id || "-"}</div>
                            <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/40 px-3 py-2">
                              <code className="min-w-0 flex-1 truncate font-mono text-xs text-foreground">
                                {revealedKey || (user.has_api_key ? "************************" : "未生成")}
                              </code>
                              {revealedKey ? (
                                <Button type="button" variant="ghost" size="icon" className="size-7 rounded-lg" onClick={() => void handleCopy(revealedKey)}>
                                  <Copy className="size-3.5" />
                                </Button>
                              ) : null}
                            </div>
                          </div>
                        ) : (
                          <Badge variant="secondary" className="rounded-md">第三方用户密钥不可管理</Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="space-y-1 text-xs">
                          <div>创建: {formatDateTime(user.created_at)}</div>
                          <div>最近: {formatDateTime(user.last_used_at)}</div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap justify-end gap-2">
                          {canManageKey ? (
                            <>
                              <Button type="button" variant="outline" className="h-8 rounded-lg px-3" onClick={() => void handleReveal(user)} disabled={revealing || !user.has_api_key}>
                                {revealing ? <LoaderCircle className="size-4 animate-spin" /> : revealedKey ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                                {revealedKey ? "隐藏" : "显示"}
                              </Button>
                              <Button type="button" variant="outline" className="h-8 rounded-lg px-3" onClick={() => { setResetName(user.api_key_name || ""); setResettingUser(user); }} disabled={pending}>
                                <RotateCcw className="size-4" />
                                {user.has_api_key ? "重置密钥" : "创建密钥"}
                              </Button>
                            </>
                          ) : null}
                          <Button type="button" variant="outline" className="h-8 rounded-lg px-3" onClick={() => { setAdjustMode("delta"); setAdjustValue("1.00"); setAdjustNote(""); setAdjustingUser(user); }} disabled={pending}>
                            调整余额
                          </Button>
                          {user.provider === "local" ? (
                            <Button
                              type="button"
                              variant="outline"
                              className="h-8 rounded-lg px-3"
                              onClick={() => {
                                setPasswordResetUser(user);
                                setNextPassword("");
                              }}
                              disabled={pending}
                            >
                              重置密码
                            </Button>
                          ) : null}
                          <Button type="button" variant="outline" className="h-8 rounded-lg px-3" onClick={() => void handleToggle(user)} disabled={pending}>
                            {pending ? <LoaderCircle className="size-4 animate-spin" /> : user.enabled ? <Ban className="size-4" /> : <CheckCircle2 className="size-4" />}
                            {user.enabled ? "禁用" : "启用"}
                          </Button>
                          <Button type="button" variant="outline" className="h-8 rounded-lg border-rose-200 px-3 text-rose-600 hover:bg-rose-50 hover:text-rose-700" onClick={() => setDeletingUser(user)} disabled={pending}>
                            <Trash2 className="size-4" />
                            删除
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>

          {isLoading ? (
            <div className="flex items-center justify-center py-14"><LoaderCircle className="size-5 animate-spin text-stone-400" /></div>
          ) : null}

          {!isLoading && filteredItems.length === 0 ? (
            <div className="px-6 py-14 text-center text-sm text-muted-foreground">{items.length === 0 ? "暂无用户" : "没有匹配的用户"}</div>
          ) : null}

          {!isLoading && filteredItems.length > 0 ? (
            <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-5 py-3 text-sm">
              <span>第 {currentPage} / {totalPages} 页</span>
              <div className="flex items-center gap-2">
                <Input
                  value={String(currentPage)}
                  onChange={(event) => {
                    const next = Number(event.target.value);
                    if (!Number.isFinite(next)) return;
                    setPage(Math.max(1, Math.min(totalPages, Math.trunc(next))));
                  }}
                  inputMode="numeric"
                  className="h-8 w-[120px] rounded-lg"
                  placeholder={`页码 1-${totalPages}`}
                />
                <Button type="button" variant="outline" className="h-8 rounded-lg px-3" disabled={currentPage <= 1} onClick={() => setPage((p) => Math.max(1, p - 1))}>上一页</Button>
                <Button type="button" variant="outline" className="h-8 rounded-lg px-3" disabled={currentPage >= totalPages} onClick={() => setPage((p) => Math.min(totalPages, p + 1))}>下一页</Button>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>创建用户</DialogTitle>
            <DialogDescription className="text-sm leading-6">创建本地账号用于后台管理。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            <Input value={createUsername} onChange={(event) => setCreateUsername(event.target.value)} placeholder="用户名" className="h-11 rounded-xl" />
            <Input value={createName} onChange={(event) => setCreateName(event.target.value)} placeholder="显示名称（可选）" className="h-11 rounded-xl" />
            <Input type="password" value={createPassword} onChange={(event) => setCreatePassword(event.target.value)} placeholder="密码" className="h-11 rounded-xl" />
          </div>
          <DialogFooter>
            <Button type="button" variant="secondary" className="h-10 rounded-xl px-5" onClick={() => setIsCreateDialogOpen(false)} disabled={isCreating}>取消</Button>
            <Button type="button" className="h-10 rounded-xl px-5" onClick={() => void handleCreate()} disabled={isCreating}>{isCreating ? <LoaderCircle className="size-4 animate-spin" /> : <Plus className="size-4" />}创建</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(resettingUser)} onOpenChange={(open) => (!open ? setResettingUser(null) : null)}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>{resettingUser?.has_api_key ? "重置用户密钥" : "创建用户密钥"}</DialogTitle>
            <DialogDescription className="text-sm leading-6">确认后会返回新的密钥。</DialogDescription>
          </DialogHeader>
          <Input value={resetName} onChange={(event) => setResetName(event.target.value)} placeholder="密钥名称（可选）" className="h-11 rounded-xl" />
          <DialogFooter>
            <Button type="button" variant="secondary" className="h-10 rounded-xl px-5" onClick={() => setResettingUser(null)} disabled={resettingUser ? pendingIDs.has(resettingUser.id) : false}>取消</Button>
            <Button type="button" className="h-10 rounded-xl px-5" onClick={() => void handleResetKey()} disabled={resettingUser ? pendingIDs.has(resettingUser.id) : false}>{resettingUser && pendingIDs.has(resettingUser.id) ? <LoaderCircle className="size-4 animate-spin" /> : <RotateCcw className="size-4" />}确认</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(deletingUser)} onOpenChange={(open) => (!open ? setDeletingUser(null) : null)}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>删除用户</DialogTitle>
            <DialogDescription className="text-sm leading-6">确认删除该用户？</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="secondary" className="h-10 rounded-xl px-5" onClick={() => setDeletingUser(null)} disabled={deletingUser ? pendingIDs.has(deletingUser.id) : false}>取消</Button>
            <Button type="button" className="h-10 rounded-xl bg-rose-600 px-5 text-white hover:bg-rose-700" onClick={() => void handleDelete()} disabled={deletingUser ? pendingIDs.has(deletingUser.id) : false}>{deletingUser && pendingIDs.has(deletingUser.id) ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}删除</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(passwordResetUser)} onOpenChange={(open) => (!open ? setPasswordResetUser(null) : null)}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>重置用户密码</DialogTitle>
            <DialogDescription className="text-sm leading-6">
              输入新密码后保存，用户会被强制下线并需要重新登录。
            </DialogDescription>
          </DialogHeader>
          <Input
            type="password"
            value={nextPassword}
            onChange={(event) => setNextPassword(event.target.value)}
            placeholder="请输入新密码（至少 8 位）"
            className="h-11 rounded-xl"
          />
          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              className="h-10 rounded-xl px-5"
              onClick={() => setPasswordResetUser(null)}
              disabled={passwordResetUser ? pendingIDs.has(passwordResetUser.id) : false}
            >
              取消
            </Button>
            <Button
              type="button"
              className="h-10 rounded-xl px-5"
              onClick={() => void handleResetPassword()}
              disabled={passwordResetUser ? pendingIDs.has(passwordResetUser.id) : false}
            >
              {passwordResetUser && pendingIDs.has(passwordResetUser.id) ? <LoaderCircle className="size-4 animate-spin" /> : null}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(adjustingUser)} onOpenChange={(open) => (!open ? setAdjustingUser(null) : null)}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>调整余额</DialogTitle>
            <DialogDescription className="text-sm leading-6">支持增减和直接设定余额。</DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            <Select value={adjustMode} onValueChange={(value) => setAdjustMode(value as "delta" | "set")}>
              <SelectTrigger className="h-11 rounded-xl"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="delta">增减（元）</SelectItem>
                <SelectItem value="set">设定余额（元）</SelectItem>
              </SelectContent>
            </Select>
            <Input value={adjustValue} onChange={(event) => setAdjustValue(event.target.value)} placeholder={adjustMode === "delta" ? "例如 1 或 -1" : "例如 10"} className="h-11 rounded-xl" />
            <Input value={adjustNote} onChange={(event) => setAdjustNote(event.target.value)} placeholder="备注（可选）" className="h-11 rounded-xl" />
          </div>
          <DialogFooter>
            <Button type="button" variant="secondary" className="h-10 rounded-xl px-5" onClick={() => setAdjustingUser(null)} disabled={adjustingUser ? pendingIDs.has(adjustingUser.id) : false}>取消</Button>
            <Button type="button" className="h-10 rounded-xl px-5" onClick={() => void handleAdjustBalance()} disabled={adjustingUser ? pendingIDs.has(adjustingUser.id) : false}>{adjustingUser && pendingIDs.has(adjustingUser.id) ? <LoaderCircle className="size-4 animate-spin" /> : null}保存</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}

export default function UsersPage() {
  const { isCheckingAuth, session } = useAuthGuard(["admin"]);
  if (isCheckingAuth || !session || session.role !== "admin") {
    return <div className="flex min-h-[40vh] items-center justify-center"><LoaderCircle className="size-5 animate-spin text-stone-400" /></div>;
  }
  return <UsersContent />;
}
