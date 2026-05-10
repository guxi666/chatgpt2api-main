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
  resetManagedUserKey,
  revealManagedUserKey,
  updateManagedUser,
  type ManagedUser,
} from "@/lib/api";
import { formatLocalDateTime } from "@/lib/datetime";
import { usePreferredLanguage } from "@/lib/language";
import { useAuthGuard } from "@/lib/use-auth-guard";

type Language = "zh" | "en";

const copy: Record<Language, Record<string, string>> = {
  zh: {
    eyebrow: "用户",
    title: "用户管理",
    refresh: "刷新",
    createUser: "创建用户",
    total: "总计",
    searchPlaceholder: "搜索 用户ID/名称/归属/API密钥/会话",
    allSources: "全部来源",
    allStatus: "全部状态",
    enabled: "启用",
    disabled: "禁用",
    clear: "清空筛选",
    user: "用户",
    source: "来源",
    level: "等级",
    status: "状态",
    quotaUsed: "用量",
    balanceLeft: "余额/可用次数",
    curve: "曲线",
    apiKey: "API 密钥",
    time: "时间",
    actions: "操作",
    today: "今日",
    timesLeft: "次可用",
    calls: "调用",
    fails: "失败",
    created: "创建",
    used: "使用",
    show: "查看",
    hide: "隐藏",
    resetKey: "重置密钥",
    createKey: "创建密钥",
    adjustBalance: "调整余额",
    disable: "禁用",
    enable: "启用",
    delete: "删除",
    noUsers: "暂无用户",
    noMatchedUsers: "没有匹配用户",
    createUserTitle: "创建用户",
    createUserDesc: "会同时为该用户创建新的 API 密钥。",
    name: "名称",
    create: "创建",
    cancel: "取消",
    resetApiKeyTitle: "重置 API 密钥",
    createApiKeyTitle: "创建 API 密钥",
    keyName: "密钥名称",
    confirm: "确认",
    deleteUserTitle: "删除用户",
    adjustBalanceTitle: "调整余额",
    mode: "模式",
    addSubtract: "增减（分）",
    setBalance: "设定余额（分）",
    value: "数值（分）",
    noteOptional: "备注（可选）",
    save: "保存",
    copied: "已复制",
    copyFailed: "复制失败",
    noApiKey: "该用户暂无 API 密钥",
    apiKeyRevealed: "已显示 API 密钥",
    revealFailed: "显示 API 密钥失败",
    userCreated: "用户已创建",
    createUserFailed: "创建用户失败",
    userDisabled: "用户已禁用",
    userEnabled: "用户已启用",
    updateUserFailed: "更新用户失败",
    resetKeySuccess: "API 密钥已重置",
    createKeySuccess: "API 密钥已创建",
    resetKeyFailed: "重置 API 密钥失败",
    userDeleted: "用户已删除",
    deleteUserFailed: "删除用户失败",
    invalidNumber: "请输入有效数字",
    balanceUpdated: "余额已更新",
    balanceUpdateFailed: "余额更新失败",
    loadingUsersFailed: "加载用户失败",
    local: "本地",
    linuxdo: "Linuxdo",
    email: "邮箱",
    unknown: "未知",
    notGenerated: "未生成",
    linuxdoLogin: "Linuxdo 登录",
    keyForName: "例如：ops-user",
  },
  en: {
    eyebrow: "Users",
    title: "User Management",
    refresh: "Refresh",
    createUser: "Create User",
    total: "Total",
    searchPlaceholder: "Search user id/name/owner/key/session",
    allSources: "All Sources",
    allStatus: "All Status",
    enabled: "Enabled",
    disabled: "Disabled",
    clear: "Clear",
    user: "User",
    source: "Source",
    level: "Level",
    status: "Status",
    quotaUsed: "Quota Used",
    balanceLeft: "Balance / Left",
    curve: "Curve",
    apiKey: "API Key",
    time: "Time",
    actions: "Actions",
    today: "Today",
    timesLeft: "times left",
    calls: "Calls",
    fails: "Fails",
    created: "Created",
    used: "Used",
    show: "Show",
    hide: "Hide",
    resetKey: "Reset Key",
    createKey: "Create Key",
    adjustBalance: "Adjust Balance",
    disable: "Disable",
    enable: "Enable",
    delete: "Delete",
    noUsers: "No users",
    noMatchedUsers: "No matching users",
    createUserTitle: "Create User",
    createUserDesc: "A new API key will be created with this user.",
    name: "Name",
    create: "Create",
    cancel: "Cancel",
    resetApiKeyTitle: "Reset API Key",
    createApiKeyTitle: "Create API Key",
    keyName: "Key Name",
    confirm: "Confirm",
    deleteUserTitle: "Delete User",
    adjustBalanceTitle: "Adjust Balance",
    mode: "Mode",
    addSubtract: "Add/Subtract (cents)",
    setBalance: "Set Balance (cents)",
    value: "Value (cents)",
    noteOptional: "Note (optional)",
    save: "Save",
    copied: "Copied",
    copyFailed: "Copy failed",
    noApiKey: "No API key for this user",
    apiKeyRevealed: "API key revealed",
    revealFailed: "Failed to reveal API key",
    userCreated: "User created",
    createUserFailed: "Failed to create user",
    userDisabled: "User disabled",
    userEnabled: "User enabled",
    updateUserFailed: "Failed to update user",
    resetKeySuccess: "API key reset",
    createKeySuccess: "API key created",
    resetKeyFailed: "Failed to reset API key",
    userDeleted: "User deleted",
    deleteUserFailed: "Failed to delete user",
    invalidNumber: "Please input a valid number",
    balanceUpdated: "Balance updated",
    balanceUpdateFailed: "Failed to update balance",
    loadingUsersFailed: "Failed to load users",
    local: "Local",
    linuxdo: "Linuxdo",
    email: "Email",
    unknown: "Unknown",
    notGenerated: "Not generated",
    linuxdoLogin: "Linuxdo login",
    keyForName: "for example: ops-user",
  },
};

function normalizeManagedUsers(items: ManagedUser[] | null | undefined) {
  return Array.isArray(items) ? items : [];
}

function providerLabel(provider?: string) {
  if (provider === "linuxdo") return "linuxdo";
  if (provider === "email") return "email";
  if (provider === "local") return "local";
  return provider || "unknown";
}

function linuxDoLevelLabel(user: ManagedUser) {
  if (user.provider !== "linuxdo") {
    return "-";
  }
  const level = String(user.linuxdo_level || "").trim();
  return level ? `Level ${level}` : "Unknown";
}

function maskToken(hasToken: boolean) {
  return hasToken ? "************************" : "notGenerated";
}

function numeric(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function centsToYuan(cents: number) {
  return (Math.max(0, cents) / 100).toFixed(2);
}

function remainingImageTimes(user: ManagedUser) {
  const price = Math.max(1, numeric(user.image_price_cents) || 8);
  const balance = Math.max(0, numeric(user.balance_cents));
  return Math.floor(balance / price);
}

function todayQuotaUsed(user: ManagedUser) {
  const points = Array.isArray(user.usage_curve) ? user.usage_curve : [];
  if (points.length === 0) {
    return 0;
  }
  return numeric(points[points.length - 1]?.quota_used);
}

function UsageSparkline({ points }: { points?: ManagedUser["usage_curve"] }) {
  const safePoints = Array.isArray(points) ? points : [];
  const maxCalls = Math.max(1, ...safePoints.map((point) => numeric(point.calls)));

  return (
    <div className="flex h-12 w-[170px] items-end gap-1" aria-label="Call curve">
      {safePoints.map((point) => {
        const calls = numeric(point.calls);
        const height = Math.max(4, Math.round((calls / maxCalls) * 40));
        return (
          <div
            key={point.date}
            className="w-2 rounded-t-sm bg-sky-500/70 dark:bg-sky-400/70"
            style={{ height }}
            title={`${point.date} calls ${calls}, quota ${numeric(point.quota_used)}`}
          />
        );
      })}
    </div>
  );
}

function userSearchText(user: ManagedUser) {
  return [
    user.id,
    user.name,
    user.email,
    user.owner_id,
    user.owner_name,
    user.provider,
    user.linuxdo_level,
    user.api_key_id,
    user.api_key_name,
    user.session_id,
    user.session_name,
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

function userSecondaryText(user: ManagedUser) {
  if (user.provider === "email") {
    const email = String(user.email || "").trim();
    if (email) {
      return email;
    }
  }
  return user.id;
}

function UsersContent() {
  const { language } = usePreferredLanguage();
  const t = copy[language];
  const [items, setItems] = useState<ManagedUser[]>([]);
  const [searchText, setSearchText] = useState("");
  const [providerFilter, setProviderFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");
  const [isLoading, setIsLoading] = useState(true);
  const [pendingIds, setPendingIds] = useState<Set<string>>(() => new Set());
  const [revealingIds, setRevealingIds] = useState<Set<string>>(() => new Set());
  const [revealedKeysById, setRevealedKeysById] = useState<Record<string, string>>({});

  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const [resettingUser, setResettingUser] = useState<ManagedUser | null>(null);
  const [resetName, setResetName] = useState("");
  const [deletingUser, setDeletingUser] = useState<ManagedUser | null>(null);

  const [adjustingUser, setAdjustingUser] = useState<ManagedUser | null>(null);
  const [adjustMode, setAdjustMode] = useState<"delta" | "set">("delta");
  const [adjustValue, setAdjustValue] = useState("100");
  const [adjustNote, setAdjustNote] = useState("");

  const loadUsers = useCallback(async () => {
    setIsLoading(true);
    try {
      const data = await fetchManagedUsers();
      setItems(normalizeManagedUsers(data.items));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t.loadingUsersFailed);
    } finally {
      setIsLoading(false);
    }
  }, [t.loadingUsersFailed]);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  const filteredItems = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    return items.filter((user) => {
      if (providerFilter !== "all" && user.provider !== providerFilter) {
        return false;
      }
      if (statusFilter === "enabled" && !user.enabled) {
        return false;
      }
      if (statusFilter === "disabled" && user.enabled) {
        return false;
      }
      return !keyword || userSearchText(user).includes(keyword);
    });
  }, [items, providerFilter, searchText, statusFilter]);

  const hasActiveFilters = searchText.trim() !== "" || providerFilter !== "all" || statusFilter !== "all";

  const setItemPending = (id: string, isPending: boolean) => {
    setPendingIds((current) => {
      const next = new Set(current);
      if (isPending) next.add(id);
      else next.delete(id);
      return next;
    });
  };

  const setRevealPending = (id: string, isPending: boolean) => {
    setRevealingIds((current) => {
      const next = new Set(current);
      if (isPending) next.add(id);
      else next.delete(id);
      return next;
    });
  };

  const handleCreate = async () => {
    setIsCreating(true);
    try {
      const data = await createManagedUser(createName.trim());
      setItems(normalizeManagedUsers(data.items));
      setRevealedKeysById((current) => ({ ...current, [data.item.id]: data.key }));
      setCreateName("");
      setIsCreateDialogOpen(false);
      toast.success(t.userCreated);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t.createUserFailed);
    } finally {
      setIsCreating(false);
    }
  };

  const handleToggle = async (user: ManagedUser) => {
    setItemPending(user.id, true);
    try {
      const data = await updateManagedUser(user.id, { enabled: !user.enabled });
      setItems(normalizeManagedUsers(data.items));
      toast.success(user.enabled ? t.userDisabled : t.userEnabled);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t.updateUserFailed);
    } finally {
      setItemPending(user.id, false);
    }
  };

  const handleReveal = async (user: ManagedUser) => {
    const previousTop = window.scrollY;
    if (revealedKeysById[user.id]) {
      setRevealedKeysById((current) => {
        const next = { ...current };
        delete next[user.id];
        return next;
      });
      window.requestAnimationFrame(() => {
        window.scrollTo({ top: previousTop });
      });
      return;
    }
    if (!user.has_api_key) {
      toast.error(t.noApiKey);
      return;
    }
    setRevealPending(user.id, true);
    try {
      const data = await revealManagedUserKey(user.id);
      setRevealedKeysById((current) => ({ ...current, [user.id]: data.key }));
      toast.success(t.apiKeyRevealed);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t.revealFailed);
    } finally {
      setRevealPending(user.id, false);
      window.requestAnimationFrame(() => {
        window.scrollTo({ top: previousTop });
      });
    }
  };

  const openResetDialog = (user: ManagedUser) => {
    setResetName(user.api_key_name || "");
    setResettingUser(user);
  };

  const handleReset = async () => {
    if (!resettingUser) return;
    const user = resettingUser;
    setItemPending(user.id, true);
    try {
      const data = await resetManagedUserKey(user.id, resetName.trim());
      setItems(normalizeManagedUsers(data.items));
      setRevealedKeysById((current) => ({ ...current, [user.id]: data.key }));
      setResettingUser(null);
      setResetName("");
      toast.success(user.has_api_key ? t.resetKeySuccess : t.createKeySuccess);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t.resetKeyFailed);
    } finally {
      setItemPending(user.id, false);
    }
  };

  const handleDelete = async () => {
    if (!deletingUser) return;
    const user = deletingUser;
    setItemPending(user.id, true);
    try {
      const data = await deleteManagedUser(user.id);
      setItems(normalizeManagedUsers(data.items));
      setDeletingUser(null);
      setRevealedKeysById((current) => {
        const next = { ...current };
        delete next[user.id];
        return next;
      });
      toast.success(t.userDeleted);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t.deleteUserFailed);
    } finally {
      setItemPending(user.id, false);
    }
  };

  const openAdjustDialog = (user: ManagedUser) => {
    setAdjustMode("delta");
    setAdjustValue("100");
    setAdjustNote("");
    setAdjustingUser(user);
  };

  const handleAdjustBalance = async () => {
    if (!adjustingUser) return;
    const value = Number(adjustValue.trim());
    if (!Number.isFinite(value)) {
      toast.error(t.invalidNumber);
      return;
    }
    setItemPending(adjustingUser.id, true);
    try {
      const payload = adjustMode === "delta"
        ? { delta_cents: Math.round(value), note: adjustNote.trim() }
        : { balance_cents: Math.max(0, Math.round(value)), note: adjustNote.trim() };
      const data = await adjustManagedUserBalance(adjustingUser.id, payload);
      setItems(normalizeManagedUsers(data.items));
      setAdjustingUser(null);
      toast.success(t.balanceUpdated);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t.balanceUpdateFailed);
    } finally {
      setItemPending(adjustingUser.id, false);
    }
  };

  const handleCopy = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      toast.success(t.copied);
    } catch {
      toast.error(t.copyFailed);
    }
  };

  return (
    <section className="flex flex-col gap-5">
      <PageHeader
        eyebrow={t.eyebrow}
        title={t.title}
        actions={(
          <>
            <Button variant="outline" onClick={() => void loadUsers()} disabled={isLoading} className="h-10 rounded-lg">
              <RefreshCw className={`size-4 ${isLoading ? "animate-spin" : ""}`} />
              {t.refresh}
            </Button>
            <Button onClick={() => setIsCreateDialogOpen(true)} className="h-10 rounded-lg">
              <Plus className="size-4" />
              {t.createUser}
            </Button>
          </>
        )}
      />

      <Card className="overflow-hidden">
        <CardContent className="p-0">
          <div className="flex flex-col gap-3 border-b border-border px-5 py-4">
            <div className="flex items-center justify-between text-sm text-muted-foreground">
              <span>{t.total} {filteredItems.length} / {items.length}</span>
            </div>
            <div className="grid gap-2 lg:grid-cols-[minmax(18rem,1fr)_160px_160px_auto]">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  value={searchText}
                  onChange={(event) => setSearchText(event.target.value)}
                  placeholder={t.searchPlaceholder}
                  className="h-10 rounded-lg pl-9"
                />
              </div>
              <Select value={providerFilter} onValueChange={setProviderFilter}>
                <SelectTrigger className="h-10 rounded-lg">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t.allSources}</SelectItem>
                  <SelectItem value="linuxdo">Linuxdo</SelectItem>
                  <SelectItem value="email">{t.email}</SelectItem>
                  <SelectItem value="local">{t.local}</SelectItem>
                </SelectContent>
              </Select>
              <Select value={statusFilter} onValueChange={setStatusFilter}>
                <SelectTrigger className="h-10 rounded-lg">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t.allStatus}</SelectItem>
                  <SelectItem value="enabled">{t.enabled}</SelectItem>
                  <SelectItem value="disabled">{t.disabled}</SelectItem>
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
                }}
              >
                <X className="size-4" />
                {t.clear}
              </Button>
            </div>
          </div>

          <div className="overflow-x-auto lg:overflow-visible">
            <Table className="min-w-[1180px] w-full">
              <TableHeader>
                <TableRow>
                  <TableHead>{t.user}</TableHead>
                  <TableHead>{t.source}</TableHead>
                  <TableHead>{t.level}</TableHead>
                  <TableHead>{t.status}</TableHead>
                  <TableHead>{t.quotaUsed}</TableHead>
                  <TableHead>{t.balanceLeft}</TableHead>
                  <TableHead>{t.curve}</TableHead>
                  <TableHead>{t.apiKey}</TableHead>
                  <TableHead>{t.time}</TableHead>
                  <TableHead className="w-[280px]">{t.actions}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredItems.map((user) => {
                  const isPending = pendingIds.has(user.id);
                  const isRevealing = revealingIds.has(user.id);
                  const revealedKey = revealedKeysById[user.id] ?? "";
                  const canManageToken = user.provider !== "linuxdo";
                  const isBillingUser = user.provider === "email" || Boolean(user.billing_user);
                  return (
                    <TableRow key={user.id} className="text-muted-foreground">
                      <TableCell>
                        <div className="min-w-0 space-y-1">
                          <div className="truncate font-medium text-foreground">{user.name || "User"}</div>
                          <code className="block max-w-[260px] truncate font-mono text-xs text-muted-foreground">{userSecondaryText(user)}</code>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={user.provider === "linuxdo" ? "info" : "secondary"} className="rounded-md">
                          {t[providerLabel(user.provider)] || t.unknown}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {user.provider === "linuxdo" ? (
                          <Badge variant={user.linuxdo_level ? "warning" : "secondary"} className="rounded-md">
                            {linuxDoLevelLabel(user)}
                          </Badge>
                        ) : (
                          <span className="text-xs text-muted-foreground">-</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant={user.enabled ? "success" : "danger"} className="rounded-md">
                          {user.enabled ? t.enabled : t.disabled}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="space-y-1">
                          <div className="text-base font-semibold text-foreground">{numeric(user.quota_used)}</div>
                          <div className="text-xs text-muted-foreground">{t.today} {todayQuotaUsed(user)}</div>
                        </div>
                      </TableCell>
                      <TableCell>
                        {isBillingUser ? (
                          <div className="space-y-1">
                            <div className="text-base font-semibold text-foreground">￥{centsToYuan(numeric(user.balance_cents))}</div>
                            <div className="text-xs text-muted-foreground">{remainingImageTimes(user)} {t.timesLeft}</div>
                          </div>
                        ) : (
                          <span className="text-xs text-muted-foreground">-</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-3">
                          <UsageSparkline points={user.usage_curve} />
                          <div className="space-y-1 text-xs text-muted-foreground">
                            <div>{t.calls} {numeric(user.call_count)}</div>
                            <div>{t.fails} {numeric(user.failure_count)}</div>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        {canManageToken ? (
                          <div className="flex max-w-[300px] items-center gap-2 rounded-lg border border-border bg-muted/40 px-3 py-2">
                            <code className="min-w-0 flex-1 truncate font-mono text-xs text-foreground">
                              {revealedKey || t[maskToken(user.has_api_key)] || maskToken(user.has_api_key)}
                            </code>
                            {revealedKey ? (
                              <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                className="size-7 rounded-lg"
                                onClick={() => void handleCopy(revealedKey)}
                                aria-label="Copy API key"
                              >
                                <Copy className="size-3.5" />
                              </Button>
                            ) : null}
                          </div>
                        ) : (
                          <Badge variant="secondary" className="rounded-md">
                            {t.linuxdoLogin}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="space-y-1 text-xs">
                          <div>{t.created} {formatLocalDateTime(user.created_at, { withSeconds: true })}</div>
                          <div>{t.used} {formatLocalDateTime(user.last_used_at, { withSeconds: true })}</div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap justify-end gap-2">
                          {canManageToken ? (
                            <>
                              <Button
                                type="button"
                                variant="outline"
                                className="h-8 rounded-lg px-3"
                                onClick={() => void handleReveal(user)}
                                disabled={isRevealing || !user.has_api_key}
                              >
                                {isRevealing ? <LoaderCircle className="size-4 animate-spin" /> : revealedKey ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                                {revealedKey ? t.hide : t.show}
                              </Button>
                              <Button
                                type="button"
                                variant="outline"
                                className="h-8 rounded-lg px-3"
                                onClick={() => openResetDialog(user)}
                                disabled={isPending}
                              >
                                <RotateCcw className="size-4" />
                                {user.has_api_key ? t.resetKey : t.createKey}
                              </Button>
                            </>
                          ) : null}
                          {isBillingUser ? (
                            <Button
                              type="button"
                              variant="outline"
                              className="h-8 rounded-lg px-3"
                              onClick={() => openAdjustDialog(user)}
                              disabled={isPending}
                            >
                              {t.adjustBalance}
                            </Button>
                          ) : null}
                          <Button
                            type="button"
                            variant="outline"
                            className="h-8 rounded-lg px-3"
                            onClick={() => void handleToggle(user)}
                            disabled={isPending}
                          >
                            {isPending ? <LoaderCircle className="size-4 animate-spin" /> : user.enabled ? <Ban className="size-4" /> : <CheckCircle2 className="size-4" />}
                            {user.enabled ? t.disable : t.enable}
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            className="h-8 rounded-lg border-rose-200 px-3 text-rose-600 hover:bg-rose-50 hover:text-rose-700"
                            onClick={() => setDeletingUser(user)}
                            disabled={isPending}
                          >
                            <Trash2 className="size-4" />
                            {t.delete}
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
            <div className="flex items-center justify-center py-14">
              <LoaderCircle className="size-5 animate-spin text-stone-400" />
            </div>
          ) : null}
          {!isLoading && filteredItems.length === 0 ? (
            <div className="px-6 py-14 text-center text-sm text-stone-500">
              {items.length === 0 ? t.noUsers : t.noMatchedUsers}
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>{t.createUserTitle}</DialogTitle>
            <DialogDescription className="text-sm leading-6">{t.createUserDesc}</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <label className="text-sm font-medium text-stone-700 dark:text-foreground">{t.name}</label>
            <Input
              value={createName}
              onChange={(event) => setCreateName(event.target.value)}
              placeholder={t.keyForName}
              className="h-11 rounded-xl"
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="secondary" className="h-10 rounded-xl px-5" onClick={() => setIsCreateDialogOpen(false)} disabled={isCreating}>
              {t.cancel}
            </Button>
            <Button type="button" className="h-10 rounded-xl px-5" onClick={() => void handleCreate()} disabled={isCreating}>
              {isCreating ? <LoaderCircle className="size-4 animate-spin" /> : <Plus className="size-4" />}
              {t.create}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(resettingUser)} onOpenChange={(open) => (!open ? setResettingUser(null) : null)}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>{resettingUser?.has_api_key ? t.resetApiKeyTitle : t.createApiKeyTitle}</DialogTitle>
            <DialogDescription className="text-sm leading-6">
              {resettingUser?.has_api_key
                ? (language === "zh" ? `确认重置「${resettingUser.name}」的 API 密钥？` : `Reset API key for "${resettingUser.name}"?`)
                : (language === "zh" ? `确认为「${resettingUser?.name}」创建 API 密钥？` : `Create API key for "${resettingUser?.name}"?`)}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <label className="text-sm font-medium text-stone-700 dark:text-foreground">{t.keyName}</label>
            <Input
              value={resetName}
              onChange={(event) => setResetName(event.target.value)}
              placeholder="My API key"
              className="h-11 rounded-xl"
            />
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              className="h-10 rounded-xl px-5"
              onClick={() => setResettingUser(null)}
              disabled={resettingUser ? pendingIds.has(resettingUser.id) : false}
            >
              {t.cancel}
            </Button>
            <Button
              type="button"
              className="h-10 rounded-xl px-5"
              onClick={() => void handleReset()}
              disabled={resettingUser ? pendingIds.has(resettingUser.id) : false}
            >
              {resettingUser && pendingIds.has(resettingUser.id) ? <LoaderCircle className="size-4 animate-spin" /> : <RotateCcw className="size-4" />}
              {t.confirm}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(deletingUser)} onOpenChange={(open) => (!open ? setDeletingUser(null) : null)}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>{t.deleteUserTitle}</DialogTitle>
            <DialogDescription className="text-sm leading-6">
              {language === "zh" ? `确认删除「${deletingUser?.name}」？` : `Delete "${deletingUser?.name}"?`}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              className="h-10 rounded-xl px-5"
              onClick={() => setDeletingUser(null)}
              disabled={deletingUser ? pendingIds.has(deletingUser.id) : false}
            >
              {t.cancel}
            </Button>
            <Button
              type="button"
              className="h-10 rounded-xl bg-rose-600 px-5 text-white hover:bg-rose-700"
              onClick={() => void handleDelete()}
              disabled={deletingUser ? pendingIds.has(deletingUser.id) : false}
            >
              {deletingUser && pendingIds.has(deletingUser.id) ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
              {t.delete}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(adjustingUser)} onOpenChange={(open) => (!open ? setAdjustingUser(null) : null)}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>{t.adjustBalanceTitle}</DialogTitle>
            <DialogDescription className="text-sm leading-6">
              {adjustingUser ? (language === "zh" ? `用户：${adjustingUser.name || adjustingUser.id}` : `User: ${adjustingUser.name || adjustingUser.id}`) : ""}
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-3">
            <div className="grid gap-2">
              <label className="text-sm font-medium text-stone-700 dark:text-foreground">{t.mode}</label>
              <Select value={adjustMode} onValueChange={(value) => setAdjustMode(value as "delta" | "set")}>
                <SelectTrigger className="h-11 rounded-xl">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="delta">{t.addSubtract}</SelectItem>
                  <SelectItem value="set">{t.setBalance}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <label className="text-sm font-medium text-stone-700 dark:text-foreground">{t.value}</label>
              <Input
                value={adjustValue}
                onChange={(event) => setAdjustValue(event.target.value)}
                placeholder={adjustMode === "delta" ? "e.g. 100 or -100" : "e.g. 1000"}
                className="h-11 rounded-xl"
              />
            </div>
            <div className="grid gap-2">
              <label className="text-sm font-medium text-stone-700 dark:text-foreground">{t.noteOptional}</label>
              <Input
                value={adjustNote}
                onChange={(event) => setAdjustNote(event.target.value)}
                placeholder="manual adjustment"
                className="h-11 rounded-xl"
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              className="h-10 rounded-xl px-5"
              onClick={() => setAdjustingUser(null)}
              disabled={adjustingUser ? pendingIds.has(adjustingUser.id) : false}
            >
              {t.cancel}
            </Button>
            <Button
              type="button"
              className="h-10 rounded-xl px-5"
              onClick={() => void handleAdjustBalance()}
              disabled={adjustingUser ? pendingIds.has(adjustingUser.id) : false}
            >
              {adjustingUser && pendingIds.has(adjustingUser.id) ? <LoaderCircle className="size-4 animate-spin" /> : null}
              {t.save}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}

export default function UsersPage() {
  const { isCheckingAuth, session } = useAuthGuard(["admin"]);
  if (isCheckingAuth || !session || session.role !== "admin") {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <LoaderCircle className="size-5 animate-spin text-stone-400" />
      </div>
    );
  }
  return <UsersContent />;
}
