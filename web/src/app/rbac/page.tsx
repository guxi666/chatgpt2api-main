"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { LoaderCircle, Plus, RefreshCw, Save, Search, ShieldCheck, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/page-header";
import { PermissionEditor } from "@/components/permission-editor";
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
import {
  createManagedRole,
  deleteManagedRole,
  fetchManagedRoles,
  fetchManagedUsers,
  fetchPermissionCatalog,
  updateManagedRole,
  updateManagedUser,
  type ApiPermission,
  type ManagedRole,
  type ManagedUser,
  type PermissionMenu,
} from "@/lib/api";
import { useAuthGuard } from "@/lib/use-auth-guard";
import { cn } from "@/lib/utils";

function normalizeManagedRoles(items: ManagedRole[] | null | undefined) {
  return Array.isArray(items) ? items : [];
}

function uniqueSortedStrings(values: string[] | null | undefined) {
  return Array.from(new Set((Array.isArray(values) ? values : []).map((v) => String(v || "").trim()).filter(Boolean))).sort();
}

function sameStringSet(left: string[], right: string[] | null | undefined) {
  const l = uniqueSortedStrings(left);
  const r = uniqueSortedStrings(right);
  if (l.length !== r.length) return false;
  return l.every((v, i) => v === r[i]);
}

function roleSearchText(role: ManagedRole) {
  return [role.id, role.name, role.description].filter(Boolean).join(" ").toLowerCase();
}

function permissionCountLabel(role: ManagedRole) {
  return `${uniqueSortedStrings(role.menu_paths).length} 菜单 / ${uniqueSortedStrings(role.api_permissions).length} API`;
}

function displayEmail(email?: string | null) {
  const raw = String(email || "").trim();
  if (!raw) return "-";
  const match = raw.match(/^local[a-z0-9-]*_(.+@.+)$/i);
  return match?.[1] || raw;
}

function RBACContent() {
  const selectedRoleIdRef = useRef("");

  const [roles, setRoles] = useState<ManagedRole[]>([]);
  const [users, setUsers] = useState<ManagedUser[]>([]);
  const [catalog, setCatalog] = useState<{ menus: PermissionMenu[]; apis: ApiPermission[] }>({ menus: [], apis: [] });

  const [selectedRoleId, setSelectedRoleId] = useState("");
  const [roleName, setRoleName] = useState("");
  const [roleDescription, setRoleDescription] = useState("");
  const [selectedMenuPaths, setSelectedMenuPaths] = useState<string[]>([]);
  const [selectedApiPermissions, setSelectedApiPermissions] = useState<string[]>([]);

  const [searchText, setSearchText] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);

  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createDescription, setCreateDescription] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const [deletingRole, setDeletingRole] = useState<ManagedRole | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [updatingUserIDs, setUpdatingUserIDs] = useState<Set<string>>(() => new Set());

  useEffect(() => {
    selectedRoleIdRef.current = selectedRoleId;
  }, [selectedRoleId]);

  const applySelectedRole = useCallback((role: ManagedRole | null | undefined) => {
    setSelectedRoleId(role?.id || "");
    setRoleName(role?.name || "");
    setRoleDescription(role?.description || "");
    setSelectedMenuPaths(uniqueSortedStrings(role?.menu_paths));
    setSelectedApiPermissions(uniqueSortedStrings(role?.api_permissions));
  }, []);

  const loadRBAC = useCallback(async () => {
    setIsLoading(true);
    try {
      const [rolesData, catalogData, usersData] = await Promise.all([
        fetchManagedRoles(),
        fetchPermissionCatalog(),
        fetchManagedUsers(),
      ]);
      const nextRoles = normalizeManagedRoles(rolesData.items);
      const nextCatalog = {
        menus: Array.isArray(catalogData.menus) ? catalogData.menus : [],
        apis: Array.isArray(catalogData.apis) ? catalogData.apis : [],
      };
      const nextUsers = Array.isArray(usersData.items) ? usersData.items : [];
      const currentID = selectedRoleIdRef.current;
      const nextSelected = nextRoles.find((item) => item.id === currentID) || nextRoles[0] || null;

      setRoles(nextRoles);
      setCatalog(nextCatalog);
      setUsers(nextUsers);
      applySelectedRole(nextSelected);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载角色权限失败");
    } finally {
      setIsLoading(false);
    }
  }, [applySelectedRole]);

  useEffect(() => {
    void loadRBAC();
  }, [loadRBAC]);

  const selectedRole = useMemo(() => roles.find((item) => item.id === selectedRoleId) || null, [roles, selectedRoleId]);

  const filteredRoles = useMemo(() => {
    const keyword = searchText.trim().toLowerCase();
    if (!keyword) return roles;
    return roles.filter((role) => roleSearchText(role).includes(keyword));
  }, [roles, searchText]);

  const roleMembers = useMemo(() => {
    if (!selectedRoleId) return [];
    return users
      .filter((user) => (user.role_id || "") === selectedRoleId)
      .sort((a, b) => (a.name || a.username || a.email || a.id || "").localeCompare(b.name || b.username || b.email || b.id || "", "zh-CN"));
  }, [selectedRoleId, users]);

  const roleMemberPreviewByRoleID = useMemo(() => {
    const grouped = new Map<string, string[]>();
    for (const user of users) {
      const roleID = String(user.role_id || "").trim();
      if (!roleID) continue;
      const name = String(user.name || user.username || displayEmail(user.email) || user.id || "").trim();
      if (!name) continue;
      const list = grouped.get(roleID) || [];
      list.push(name);
      grouped.set(roleID, list);
    }
    const out = new Map<string, string>();
    for (const [roleID, members] of grouped.entries()) {
      const preview = members.slice(0, 3).join("、");
      out.set(roleID, members.length > 3 ? `${preview} 等 ${members.length} 人` : preview);
    }
    return out;
  }, [users]);

  const isDirty = Boolean(selectedRole)
    && (roleName.trim() !== (selectedRole?.name || "")
      || roleDescription.trim() !== (selectedRole?.description || "")
      || !sameStringSet(selectedMenuPaths, selectedRole?.menu_paths)
      || !sameStringSet(selectedApiPermissions, selectedRole?.api_permissions));

  const handleSave = async () => {
    if (!selectedRole || isSaving) return;
    const name = roleName.trim();
    if (!name) {
      toast.error("角色名称不能为空");
      return;
    }
    setIsSaving(true);
    try {
      const data = await updateManagedRole(selectedRole.id, {
        name,
        description: roleDescription.trim(),
        menu_paths: selectedMenuPaths,
        api_permissions: selectedApiPermissions,
      });
      const nextRoles = normalizeManagedRoles(data.items);
      setRoles(nextRoles);
      applySelectedRole(nextRoles.find((role) => role.id === data.item.id) || data.item);
      toast.success("角色已保存");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存角色失败");
    } finally {
      setIsSaving(false);
    }
  };

  const handleCreate = async () => {
    const name = createName.trim();
    if (!name) {
      toast.error("角色名称不能为空");
      return;
    }
    setIsCreating(true);
    try {
      const data = await createManagedRole({ name, description: createDescription.trim() });
      const nextRoles = normalizeManagedRoles(data.items);
      setRoles(nextRoles);
      applySelectedRole(nextRoles.find((role) => role.id === data.item.id) || data.item);
      setCreateName("");
      setCreateDescription("");
      setIsCreateDialogOpen(false);
      toast.success("角色已创建");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "创建角色失败");
    } finally {
      setIsCreating(false);
    }
  };

  const handleDelete = async () => {
    if (!deletingRole || isDeleting) return;
    setIsDeleting(true);
    try {
      const data = await deleteManagedRole(deletingRole.id);
      const nextRoles = normalizeManagedRoles(data.items);
      setRoles(nextRoles);
      applySelectedRole(nextRoles.find((role) => role.id === selectedRoleId) || nextRoles[0] || null);
      setDeletingRole(null);
      toast.success("角色已删除");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除角色失败");
    } finally {
      setIsDeleting(false);
    }
  };

  const setUserUpdating = (userID: string, pending: boolean) => {
    setUpdatingUserIDs((current) => {
      const next = new Set(current);
      if (pending) next.add(userID);
      else next.delete(userID);
      return next;
    });
  };

  const handleUserRoleChange = async (user: ManagedUser, targetRoleID: string) => {
    const roleID = String(targetRoleID || "").trim();
    if (!roleID || roleID === (user.role_id || "")) return;
    setUserUpdating(user.id, true);
    try {
      const data = await updateManagedUser(user.id, { role_id: roleID });
      setUsers(Array.isArray(data.items) ? data.items : []);
      const rolesData = await fetchManagedRoles();
      setRoles(normalizeManagedRoles(rolesData.items));
      toast.success("用户等级已更新");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新用户等级失败");
    } finally {
      setUserUpdating(user.id, false);
    }
  };

  return (
    <section className="flex flex-col gap-5">
      <PageHeader
        eyebrow="RBAC"
        title="角色权限"
        actions={(
          <>
            <Button variant="outline" onClick={() => void loadRBAC()} disabled={isLoading} className="h-10 rounded-lg">
              <RefreshCw className={cn("size-4", isLoading ? "animate-spin" : "")} />
              刷新
            </Button>
            <Button onClick={() => setIsCreateDialogOpen(true)} disabled={isLoading} className="h-10 rounded-lg">
              <Plus className="size-4" />
              创建角色
            </Button>
            <Button onClick={() => void handleSave()} disabled={!selectedRole || !isDirty || isSaving || isLoading} className="h-10 rounded-lg">
              {isSaving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}
              保存
            </Button>
          </>
        )}
      />

      <div className="grid gap-5 xl:grid-cols-[360px_1fr]">
        <Card className="overflow-hidden">
          <CardContent className="p-0">
            <div className="border-b border-border px-5 py-4">
              <div className="mb-3 flex items-center justify-between text-sm text-muted-foreground">
                <span>角色 {filteredRoles.length} / {roles.length}</span>
                <ShieldCheck className="size-4" />
              </div>
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input value={searchText} onChange={(event) => setSearchText(event.target.value)} placeholder="搜索角色名称或描述" className="h-10 rounded-lg pl-9" />
              </div>
            </div>
            <div className="max-h-[calc(100vh-18rem)] min-h-[360px] overflow-y-auto">
              {isLoading ? <div className="flex min-h-[320px] items-center justify-center"><LoaderCircle className="size-5 animate-spin text-stone-400" /></div> : null}
              {!isLoading && filteredRoles.length === 0 ? <div className="px-5 py-12 text-center text-sm text-muted-foreground">暂无角色</div> : null}
              {!isLoading ? filteredRoles.map((role) => {
                const active = role.id === selectedRoleId;
                return (
                  <button
                    key={role.id}
                    type="button"
                    className={cn("block w-full border-b border-border px-5 py-4 text-left transition hover:bg-muted/50", active ? "bg-[#edf4ff] dark:bg-sky-950/20" : "")}
                    onClick={() => applySelectedRole(role)}
                  >
                    <div className="flex min-w-0 items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="truncate text-sm font-semibold text-foreground">{role.name}</div>
                        <code className="mt-1 block truncate font-mono text-xs text-muted-foreground">{role.id}</code>
                      </div>
                      {role.builtin ? <Badge variant="secondary" className="shrink-0 rounded-md">内置</Badge> : null}
                    </div>
                    <div className="mt-3 flex flex-wrap items-center gap-2">
                      <span className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground">{permissionCountLabel(role)}</span>
                      <span className="rounded-md bg-muted px-2 py-0.5 text-xs text-muted-foreground">{role.user_count || 0} 用户</span>
                    </div>
                    <div className="mt-2 text-xs text-muted-foreground">成员：{roleMemberPreviewByRoleID.get(role.id) || "暂无"}</div>
                  </button>
                );
              }) : null}
            </div>
          </CardContent>
        </Card>

        <Card className="overflow-hidden">
          <CardContent className="p-0">
            <div className="flex flex-col gap-4 border-b border-border px-5 py-4">
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div className="min-w-0">
                  <div className="flex min-w-0 items-center gap-2">
                    <ShieldCheck className="size-5 shrink-0 text-[#1456f0]" />
                    <h2 className="truncate text-base font-semibold text-foreground">{selectedRole?.name || "未选择角色"}</h2>
                  </div>
                  <code className="mt-1 block truncate font-mono text-xs text-muted-foreground">{selectedRole?.id || "-"}</code>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={isDirty ? "warning" : "secondary"} className="w-fit rounded-md">{isDirty ? "未保存" : "已同步"}</Badge>
                  <Button
                    type="button"
                    variant="outline"
                    className="h-9 rounded-lg border-rose-200 px-3 text-rose-600 hover:bg-rose-50 hover:text-rose-700"
                    disabled={!selectedRole || Boolean(selectedRole.builtin) || Boolean(selectedRole.user_count)}
                    onClick={() => selectedRole ? setDeletingRole(selectedRole) : null}
                  >
                    <Trash2 className="size-4" />
                    删除
                  </Button>
                </div>
              </div>
              <div className="grid gap-3 lg:grid-cols-[240px_1fr]">
                <Input value={roleName} onChange={(event) => setRoleName(event.target.value)} placeholder="角色名称" disabled={!selectedRole || isLoading} className="h-10 rounded-lg" />
                <Input value={roleDescription} onChange={(event) => setRoleDescription(event.target.value)} placeholder="角色描述" disabled={!selectedRole || isLoading} className="h-10 rounded-lg" />
              </div>
            </div>

            <div className="border-b border-border px-5 py-4">
              <div className="mb-3 flex items-center justify-between">
                <h3 className="text-sm font-semibold text-foreground">当前角色成员与等级管理</h3>
                <Badge variant="secondary" className="rounded-md">{roleMembers.length} 用户</Badge>
              </div>
              {selectedRole ? (
                roleMembers.length > 0 ? (
                  <div className="overflow-x-auto">
                    <table className="min-w-[720px] w-full text-sm">
                      <thead>
                        <tr className="border-b border-border/60 text-left text-muted-foreground">
                          <th className="px-2 py-2 font-medium">用户</th>
                          <th className="px-2 py-2 font-medium">注册邮箱</th>
                          <th className="px-2 py-2 font-medium">当前等级</th>
                          <th className="px-2 py-2 font-medium">调整等级</th>
                          <th className="px-2 py-2 font-medium">管理</th>
                        </tr>
                      </thead>
                      <tbody>
                        {roleMembers.map((user) => {
                          const roleID = user.role_id || "";
                          const pending = updatingUserIDs.has(user.id);
                          return (
                            <tr key={user.id} className="border-b border-border/40">
                              <td className="px-2 py-2">
                                <div className="font-medium text-foreground">{user.name || user.username || user.id}</div>
                              </td>
                              <td className="px-2 py-2">{displayEmail(user.email)}</td>
                              <td className="px-2 py-2"><Badge variant="secondary" className="rounded-md">{user.role_name || "普通用户"}</Badge></td>
                              <td className="px-2 py-2">
                                <Select value={roleID} onValueChange={(value) => { void handleUserRoleChange(user, value); }} disabled={pending}>
                                  <SelectTrigger className="h-9 w-[220px] rounded-lg"><SelectValue /></SelectTrigger>
                                  <SelectContent>
                                    {roles.map((role) => <SelectItem key={role.id} value={role.id}>{role.name}</SelectItem>)}
                                  </SelectContent>
                                </Select>
                              </td>
                              <td className="px-2 py-2">{pending ? <LoaderCircle className="size-4 animate-spin text-muted-foreground" /> : <span className="text-xs text-muted-foreground">可升/可降</span>}</td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <div className="rounded-lg border border-border/70 bg-muted/20 px-3 py-6 text-sm text-muted-foreground">当前角色下暂无用户。</div>
                )
              ) : (
                <div className="rounded-lg border border-border/70 bg-muted/20 px-3 py-6 text-sm text-muted-foreground">请先选择左侧角色。</div>
              )}
            </div>

            <div className="p-5">
              {isLoading ? (
                <div className="flex min-h-[420px] items-center justify-center"><LoaderCircle className="size-5 animate-spin text-stone-400" /></div>
              ) : selectedRole ? (
                <PermissionEditor
                  menus={catalog.menus}
                  apis={catalog.apis}
                  selectedMenuPaths={selectedMenuPaths}
                  selectedApiPermissions={selectedApiPermissions}
                  onMenuPathsChange={setSelectedMenuPaths}
                  onApiPermissionsChange={setSelectedApiPermissions}
                  className="lg:grid-cols-[300px_1fr]"
                />
              ) : (
                <div className="flex min-h-[420px] items-center justify-center text-sm text-muted-foreground">暂无角色</div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>创建角色</DialogTitle>
            <DialogDescription className="text-sm leading-6">新角色默认从普通用户权限起步，创建后可继续调整。</DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Input value={createName} onChange={(event) => setCreateName(event.target.value)} placeholder="角色名称" className="h-11 rounded-xl" />
          </div>
          <div className="space-y-2">
            <Input value={createDescription} onChange={(event) => setCreateDescription(event.target.value)} placeholder="角色描述" className="h-11 rounded-xl" />
          </div>
          <DialogFooter>
            <Button type="button" variant="secondary" className="h-10 rounded-xl px-5" onClick={() => setIsCreateDialogOpen(false)} disabled={isCreating}>取消</Button>
            <Button type="button" className="h-10 rounded-xl px-5" onClick={() => void handleCreate()} disabled={isCreating}>{isCreating ? <LoaderCircle className="size-4 animate-spin" /> : <Plus className="size-4" />}创建</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(deletingRole)} onOpenChange={(open) => (!open ? setDeletingRole(null) : null)}>
        <DialogContent className="rounded-2xl p-6">
          <DialogHeader className="gap-2">
            <DialogTitle>删除角色</DialogTitle>
            <DialogDescription className="text-sm leading-6">仅未绑定用户的自定义角色可删除。</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="secondary" className="h-10 rounded-xl px-5" onClick={() => setDeletingRole(null)} disabled={isDeleting}>取消</Button>
            <Button type="button" variant="destructive" className="h-10 rounded-xl px-5" onClick={() => void handleDelete()} disabled={isDeleting}>{isDeleting ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}删除</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}

export default function RBACPage() {
  const { isCheckingAuth, session } = useAuthGuard(undefined, "/rbac");
  if (isCheckingAuth || !session) {
    return <div className="flex min-h-[40vh] items-center justify-center"><LoaderCircle className="size-5 animate-spin text-stone-400" /></div>;
  }
  return <RBACContent />;
}
