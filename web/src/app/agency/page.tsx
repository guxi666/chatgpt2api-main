"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  BadgeCheck,
  CheckCircle2,
  Copy,
  Crown,
  Gem,
  LoaderCircle,
  RefreshCw,
  Save,
  ShieldCheck,
  Sparkles,
  Users,
  Wallet,
} from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  activateAgencyUser,
  createAgencyWithdrawal,
  fetchAgencyAdminWithdrawals,
  fetchAgencyAdminUsers,
  fetchAgencyCommissionDashboard,
  fetchAgencyConfig,
  fetchAgencyWithdrawProfile,
  fetchAgencyWithdrawals,
  fetchManagedUsers,
  fetchWallet,
  joinAgencyTier,
  updateAgencyConfig,
  updateAgencyAdminWithdrawal,
  updateAgencyWithdrawProfile,
  uploadAgencyWithdrawQRCode,
  upgradeAgencyTier,
  verifySession,
  type AgencyAdminUser,
  type AgencyCommissionDashboard,
  type AgencyCommissionOrder,
  type AgencyConfig,
  type AgencyMaterial,
  type AgencyMaterialQRConfig,
  type AgencyTier,
  type AgencyWithdrawalRequest,
  type ManagedUser,
  type PayType,
} from "@/lib/api";
import { resolveBrandAssetURL } from "@/lib/app-meta";
import { formatBeijingDateTime } from "@/lib/datetime";
import { useAppMeta } from "@/lib/use-app-meta";
import { getCachedAuthSession } from "@/lib/session";
import { getStoredSessionToken } from "@/store/auth";
import { cn } from "@/lib/utils";

type TierKey = "basic" | "pro" | "premium";
type AgencyMenuKey = "overview" | "promotion" | "team" | "income" | "withdraw" | "materials" | "benefits" | "settings";

const TIER_META: Record<TierKey, { subtitle: string; tag?: string; icon: typeof Gem }> = {
  basic: { subtitle: "基础代理套餐，适合个人起步", icon: Gem },
  pro: { subtitle: "进阶代理套餐，适合团队运营", tag: "性价比最高", icon: Gem },
  premium: { subtitle: "旗舰代理套餐，享受更高分成", tag: "分成最高", icon: Crown as unknown as typeof Gem },
};

const STATUS_TEXT: Record<string, string> = {
  paid: "已结算",
  pending: "待结算",
  failed: "失败",
};

const WITHDRAW_STATUS_TEXT: Record<string, string> = {
  pending: "待审核",
  approved: "已通过",
  paid: "已打款",
  rejected: "已驳回",
};

const AGENCY_MENU: Array<{ key: AgencyMenuKey; label: string; icon: typeof Crown }> = [
  { key: "overview", label: "代理总览", icon: Crown },
  { key: "promotion", label: "推广链接", icon: Copy },
  { key: "team", label: "我的团队", icon: Users },
  { key: "income", label: "收益明细", icon: Wallet },
  { key: "withdraw", label: "提现管理", icon: Wallet },
  { key: "materials", label: "推广素材", icon: Sparkles },
  { key: "benefits", label: "等级权益", icon: ShieldCheck },
  { key: "settings", label: "设置中心", icon: CheckCircle2 },
];

function parsePositiveInt(value: string) {
  const numberValue = Number(value);
  if (!Number.isFinite(numberValue)) return 0;
  return Math.max(0, Math.trunc(numberValue));
}

function parseYuanToCents(value: string) {
  const amount = Number(value);
  if (!Number.isFinite(amount) || amount <= 0) return 0;
  return Math.max(0, Math.round(amount * 100));
}

function formatPercentByBP(bp?: number) {
  return `${((bp || 0) / 100).toFixed(2)}%`;
}

function formatYuan(value: string | number | undefined) {
  const n = Number(value || 0);
  if (!Number.isFinite(n)) return "0.00";
  return n.toFixed(2);
}

function tierRank(tier: string) {
  if (tier === "basic") return 1;
  if (tier === "pro") return 2;
  if (tier === "premium") return 3;
  return 0;
}

function tierLabel(tierKey: string, tiers: AgencyTier[]) {
  const tier = tiers.find((item) => item.key === tierKey);
  return tier?.name || tierKey || "未开通";
}

function tierFromRole(roleName: string, roleID: string) {
  const name = String(roleName || "").trim();
  if (name.includes("代理") && name.includes("旗舰")) return "premium";
  if (name.includes("代理") && name.includes("进阶")) return "pro";
  if (name.includes("代理") && name.includes("基础")) return "basic";
  const id = String(roleID || "").trim().toLowerCase();
  if (id === "admin" || id === "default-user") return "";
  return "";
}

function orderedTiers(tiers: AgencyTier[]) {
  const orderMap = new Map<TierKey, number>([
    ["basic", 1],
    ["pro", 2],
    ["premium", 3],
  ]);
  return [...tiers].sort((a, b) => (orderMap.get(a.key as TierKey) || 99) - (orderMap.get(b.key as TierKey) || 99));
}

function formatDateTime(value?: string) {
  return formatBeijingDateTime(value);
}

function sumOrderAmountYuan(orders: AgencyCommissionOrder[]) {
  return orders.reduce((sum, item) => {
    if (item.amount_yuan) {
      const n = Number(item.amount_yuan);
      return Number.isFinite(n) ? sum + n : sum;
    }
    return sum + Number(item.amount_cents || 0) / 100;
  }, 0);
}

function sumPendingCommissionYuan(orders: AgencyCommissionOrder[]) {
  return orders.reduce((sum, item) => {
    if ((item.status || "").toLowerCase() !== "pending") return sum;
    if (item.commission_yuan) {
      const n = Number(item.commission_yuan);
      return Number.isFinite(n) ? sum + n : sum;
    }
    return sum + Number(item.commission_cents || 0) / 100;
  }, 0);
}

function payoutSummary(item: AgencyWithdrawalRequest) {
  const parts = [
    item.alipay_qr_code ? "支付宝收款码" : "",
    item.wechat_qr_code ? "微信收款码" : "",
    item.phone ? `手机号 ${item.phone}` : "",
    item.wechat_id ? `微信号 ${item.wechat_id}` : "",
  ].filter(Boolean);
  return parts.join(" / ") || "-";
}

function calcTrendRows(orders: AgencyCommissionOrder[]) {
  const map = new Map<string, number>();
  for (const item of orders) {
    const created = String(item.created_at || "").trim();
    if (!created) continue;
    const dt = new Date(created);
    if (Number.isNaN(dt.getTime())) continue;
    const key = `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, "0")}-${String(dt.getDate()).padStart(2, "0")}`;
    const commission = item.commission_yuan ? Number(item.commission_yuan) : Number(item.commission_cents || 0) / 100;
    map.set(key, (map.get(key) || 0) + (Number.isFinite(commission) ? commission : 0));
  }
  const out: Array<{ label: string; value: number; key: string }> = [];
  for (let i = 6; i >= 0; i -= 1) {
    const dt = new Date();
    dt.setHours(0, 0, 0, 0);
    dt.setDate(dt.getDate() - i);
    const key = `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, "0")}-${String(dt.getDate()).padStart(2, "0")}`;
    out.push({
      key,
      label: `${String(dt.getMonth() + 1).padStart(2, "0")}-${String(dt.getDate()).padStart(2, "0")}`,
      value: Number((map.get(key) || 0).toFixed(2)),
    });
  }
  return out;
}

function normalizeAgencyMaterialQRConfig(
  value?: AgencyMaterialQRConfig | null,
): Required<AgencyMaterialQRConfig> {
  const normalizePercent = (raw: unknown, fallback: number) => {
    const n = Number(raw);
    if (!Number.isFinite(n)) return fallback;
    return Math.min(100, Math.max(0, Math.round(n)));
  };
  return {
    enabled: value?.enabled !== false,
    x_percent: normalizePercent(value?.x_percent, 72),
    y_percent: normalizePercent(value?.y_percent, 72),
    size_percent: normalizePercent(value?.size_percent, 26),
    logo_percent: normalizePercent(value?.logo_percent, 24),
  };
}

function loadImageElement(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.crossOrigin = "anonymous";
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error(`image_load_failed:${src}`));
    image.src = src;
  });
}

function drawRoundedRect(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number,
) {
  const r = Math.max(0, Math.min(radius, Math.min(width, height) / 2));
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.lineTo(x + width - r, y);
  ctx.quadraticCurveTo(x + width, y, x + width, y + r);
  ctx.lineTo(x + width, y + height - r);
  ctx.quadraticCurveTo(x + width, y + height, x + width - r, y + height);
  ctx.lineTo(x + r, y + height);
  ctx.quadraticCurveTo(x, y + height, x, y + height - r);
  ctx.lineTo(x, y + r);
  ctx.quadraticCurveTo(x, y, x + r, y);
  ctx.closePath();
}

async function buildPromotionMaterialPreview(
  baseImageURL: string,
  qrImageURL: string,
  brandLogoURL: string,
  config: Required<AgencyMaterialQRConfig>,
): Promise<string> {
  const [baseImage, qrImage] = await Promise.all([loadImageElement(baseImageURL), loadImageElement(qrImageURL)]);
  const brandLogo = brandLogoURL ? await loadImageElement(brandLogoURL).catch(() => null) : null;

  const canvas = document.createElement("canvas");
  canvas.width = Math.max(1, baseImage.naturalWidth || baseImage.width || 1);
  canvas.height = Math.max(1, baseImage.naturalHeight || baseImage.height || 1);
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    return baseImageURL;
  }

  ctx.drawImage(baseImage, 0, 0, canvas.width, canvas.height);

  const shorterSide = Math.min(canvas.width, canvas.height);
  const qrSize = Math.max(40, Math.round((shorterSide * config.size_percent) / 100));
  const qrX = Math.round(((canvas.width - qrSize) * config.x_percent) / 100);
  const qrY = Math.round(((canvas.height - qrSize) * config.y_percent) / 100);
  const radius = Math.max(8, Math.round(qrSize * 0.08));

  ctx.save();
  drawRoundedRect(ctx, qrX, qrY, qrSize, qrSize, radius);
  ctx.fillStyle = "rgba(255,255,255,0.96)";
  ctx.fill();
  ctx.restore();

  const qrPadding = Math.max(4, Math.round(qrSize * 0.08));
  ctx.drawImage(qrImage, qrX + qrPadding, qrY + qrPadding, qrSize - qrPadding * 2, qrSize - qrPadding * 2);

  if (brandLogo) {
    const logoSize = Math.max(10, Math.round((qrSize * config.logo_percent) / 100));
    const logoX = qrX + Math.round((qrSize - logoSize) / 2);
    const logoY = qrY + Math.round((qrSize - logoSize) / 2);
    const logoRadius = Math.max(4, Math.round(logoSize * 0.2));
    ctx.save();
    drawRoundedRect(ctx, logoX, logoY, logoSize, logoSize, logoRadius);
    ctx.clip();
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(logoX, logoY, logoSize, logoSize);
    ctx.drawImage(brandLogo, logoX, logoY, logoSize, logoSize);
    ctx.restore();
  }

  return canvas.toDataURL("image/png");
}

function AgencyPackageShowcase({
  tiers,
  currentTier,
  joiningTier,
  onJoin,
}: {
  tiers: AgencyTier[];
  currentTier: string;
  joiningTier: string;
  onJoin: (tier: TierKey) => Promise<void>;
}) {
  const currentRank = tierRank(currentTier);
  const sorted = [...tiers].sort((a, b) => tierRank(a.key) - tierRank(b.key));
  const byKey = new Map(sorted.map((t) => [t.key, t]));
  return (
    <section className="rounded-2xl border border-[#dde3f5] bg-[linear-gradient(160deg,#f8f9ff_0%,#f3f5ff_45%,#f6f7ff_100%)] p-6 shadow-[0_16px_40px_-28px_rgba(68,87,150,0.35)]">
      <div className="space-y-6">
        <div className="grid gap-5 xl:grid-cols-[1fr_auto] xl:items-start">
          <div>
            <div className="inline-flex rounded-full bg-[#ececff] px-3 py-1 text-xs font-semibold text-[#5d63ff]">开启你的 AI 业务</div>
            <h2 className="mt-3 text-3xl font-black tracking-tight text-[#0f2550] md:text-4xl">代理加盟</h2>
            <p className="mt-3 text-base text-[#365180] md:text-lg">选择适合你的代理方案，享受高额分成与专属支持，共同拓展 AI 服务业务。</p>
          </div>
          <div className="grid gap-4 text-[#334f7d] sm:grid-cols-2 xl:grid-cols-4">
            <div className="space-y-1 text-sm"><div className="font-bold text-[#6a45ff]">高额分成</div><div>利润空间更高</div></div>
            <div className="space-y-1 text-sm"><div className="font-bold text-[#6a45ff]">专属支持</div><div>1v1 运营指导</div></div>
            <div className="space-y-1 text-sm"><div className="font-bold text-[#6a45ff]">稳定可靠</div><div>系统安全可控</div></div>
            <div className="space-y-1 text-sm"><div className="font-bold text-[#6a45ff]">快速结算</div><div>收益按周期到账</div></div>
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-3">
          {sorted.map((tier) => {
            const meta = TIER_META[tier.key as TierKey] || TIER_META.basic;
            const Icon = meta.icon;
            const targetRank = tierRank(tier.key);
            const isCurrentTier = currentRank > 0 && targetRank === currentRank;
            const disabled = joiningTier === tier.key || isCurrentTier;
            const isMiddle = tier.key === "pro";
            const isPremium = tier.key === "premium";
            const features = tier.key === "basic"
              ? ["个人代理授权", "基础技术支持", "标准分成比例", "基础数据看板"]
              : tier.key === "pro"
                ? ["团队代理授权", "优先技术支持", "更高分成比例", "高级数据看板", "运营指导手册"]
                : ["全域代理授权", "专属客户经理", "最高分成比例", "全维数据看板", "定制运营方案", "专属 API 支持"];
            return (
              <Card
                key={tier.key}
                className={cn(
                  "relative min-h-[520px] overflow-hidden rounded-2xl border bg-none p-0 [background:linear-gradient(180deg,#ffffff,#f8fbff)] shadow-[0_8px_24px_rgba(79,104,166,0.12)]",
                  isPremium ? "border-[#f4c978]" : "border-[#d9e3fb]",
                  isMiddle ? "ring-1 ring-[#7ca5ff]/60" : "",
                )}
              >
                {meta.tag ? (
                  <div className={cn("absolute left-8 top-0 rounded-b-lg px-3 py-1 text-xs font-bold", isPremium ? "bg-[#f8b133] text-white" : "bg-[#4e6dff] text-white")}>
                    {meta.tag}
                  </div>
                ) : null}
                <CardContent className="flex min-h-[520px] flex-col gap-4 p-6 pt-8 text-[#1f3d70]">
                  <div className={cn("inline-flex size-14 items-center justify-center rounded-2xl border", isPremium ? "border-[#f6d08d] bg-[#fff5e1]" : "border-[#d9e3fb] bg-[#eef3ff]")}>
                    <Icon className={cn("size-7", isPremium ? "text-[#f29a00]" : "text-[#2f6bff]")} />
                  </div>
                  <div>
                    <div className="text-3xl leading-none font-black text-[#0f2f62]">{tier.name}</div>
                    <div className="mt-3 text-base text-[#3d5b8d]">{meta.subtitle}</div>
                  </div>
                  <div className="h-px bg-[#e5ebfb]" />
                  <div className={cn("text-5xl leading-none font-black", isPremium ? "text-[#f29500]" : isMiddle ? "text-[#275fe6]" : "text-[#4d43df]")}>
                    ¥{(Number(tier.price_cents || 0) / 100).toLocaleString("zh-CN", { minimumFractionDigits: 0, maximumFractionDigits: 0 })}
                  </div>
                  <div className="text-base text-[#4c6794]">一次性</div>
                  <div className="h-px bg-[#e5ebfb]" />
                  <div className="space-y-3 text-base min-h-[320px]">
                    {features.map((item) => (
                      <div key={item} className="flex items-center gap-2">
                        <BadgeCheck className={cn("size-5", isPremium ? "text-[#f29500]" : "text-[#2f6bff]")} />
                        <span>{item}</span>
                      </div>
                    ))}
                    <div className="flex items-center gap-2">
                      <BadgeCheck className={cn("size-5", isPremium ? "text-[#f29500]" : "text-[#2f6bff]")} />
                      <span>分成比例 {tier.commission_percent || formatPercentByBP(tier.commission_bp)}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <BadgeCheck className={cn("size-5", isPremium ? "text-[#f29500]" : "text-[#2f6bff]")} />
                      <span>充值优惠 {tier.discount_percent || formatPercentByBP(tier.discount_bp)}</span>
                    </div>
                  </div>
                  <Button
                    className={cn(
                      "mt-2 h-11 rounded-full text-base font-semibold",
                      isPremium
                        ? "border border-[#f4c978] bg-white text-[#f29a00] hover:bg-[#fff7e7]"
                        : isMiddle
                          ? "bg-[linear-gradient(90deg,#4774f8,#8e39ef)] text-white"
                          : "border border-[#c9d8ff] bg-white text-[#4d43df] hover:bg-[#f7f9ff]",
                    )}
                    onClick={() => void onJoin(tier.key as TierKey)}
                    disabled={disabled}
                  >
                    {joiningTier === tier.key ? "处理中..." : isCurrentTier ? "当前已开通" : "立即加入"}
                  </Button>
                </CardContent>
              </Card>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function AgencyCommissionCenter({
  dashboard,
  tiers,
  currentTier,
  currentTierName,
  brandTitle,
  brandLogoURL,
  materials,
  materialQRConfig,
  isLoading,
  onReload,
  onRequestUpgrade,
}: {
  dashboard: AgencyCommissionDashboard | null;
  tiers: AgencyTier[];
  currentTier: string;
  currentTierName: string;
  brandTitle: string;
  brandLogoURL: string;
  materials: AgencyMaterial[];
  materialQRConfig: Required<AgencyMaterialQRConfig>;
  isLoading: boolean;
  onReload: () => Promise<boolean>;
  onRequestUpgrade: (tier: TierKey) => void;
}) {
  const [activeMenu, setActiveMenu] = useState<AgencyMenuKey>("overview");
  const [selectedTierKey, setSelectedTierKey] = useState<TierKey>("basic");
  const [withdrawAmount, setWithdrawAmount] = useState("");
  const [withdrawAlipayQRCode, setWithdrawAlipayQRCode] = useState("");
  const [withdrawWechatQRCode, setWithdrawWechatQRCode] = useState("");
  const [withdrawPhone, setWithdrawPhone] = useState("");
  const [withdrawWechatID, setWithdrawWechatID] = useState("");
  const [withdrawals, setWithdrawals] = useState<AgencyWithdrawalRequest[]>([]);
  const [materialPreviewByID, setMaterialPreviewByID] = useState<Record<string, string>>({});
  const [isLoadingWithdrawals, setIsLoadingWithdrawals] = useState(false);
  const [isSubmittingWithdraw, setIsSubmittingWithdraw] = useState(false);
  const [uploadingQRCodeKind, setUploadingQRCodeKind] = useState<"" | "alipay" | "wechat">("");
  const alipayQRCodeInputRef = useRef<HTMLInputElement | null>(null);
  const wechatQRCodeInputRef = useRef<HTMLInputElement | null>(null);

  const summary = dashboard?.summary;
  const agent = dashboard?.agent;
  const orders = dashboard?.orders || [];

  useEffect(() => {
    setWithdrawals(dashboard?.withdrawals || []);
  }, [dashboard?.withdrawals]);

  useEffect(() => {
    if (currentTier === "basic" || currentTier === "pro" || currentTier === "premium") {
      setSelectedTierKey(currentTier);
    }
  }, [currentTier]);

  const fallbackLink = useMemo(() => {
    const code = String(agent?.invite_code || "").trim();
    if (!code || typeof window === "undefined") return "";
    return `${window.location.origin}/login?invite_code=${encodeURIComponent(code)}`;
  }, [agent?.invite_code]);

  const link = String(agent?.channel_link || "").trim() || fallbackLink;
  const qrURL = useMemo(() => {
    if (!link) return "";
    return `https://api.qrserver.com/v1/create-qr-code/?size=260x260&margin=8&data=${encodeURIComponent(link)}`;
  }, [link]);

  useEffect(() => {
    let cancelled = false;

    const buildPreviews = async () => {
      if (!materialQRConfig.enabled || !qrURL) {
        setMaterialPreviewByID({});
        return;
      }
      const next: Record<string, string> = {};
      for (const item of materials) {
        const id = String(item.id || "").trim();
        const imageURL = String(item.image_url || "").trim();
        if (!id || !imageURL) continue;
        try {
          next[id] = await buildPromotionMaterialPreview(
            imageURL,
            qrURL,
            brandLogoURL,
            materialQRConfig,
          );
        } catch {
          next[id] = imageURL;
        }
      }
      if (!cancelled) {
        setMaterialPreviewByID(next);
      }
    };

    void buildPreviews();
    return () => {
      cancelled = true;
    };
  }, [brandLogoURL, materialQRConfig, materials, qrURL]);

  const invitedCount = Number(agent?.invited_count || agent?.invited_users?.length || 0);
  const registeredCount = Number(agent?.invited_users?.length || 0);
  const rechargeUserCount = useMemo(() => {
    const set = new Set<string>();
    for (const item of orders) {
      const key = String(item.user_id || item.user_email || "").trim();
      if (key) set.add(key);
    }
    return set.size;
  }, [orders]);
  const rechargeAmount = sumOrderAmountYuan(orders);
  const pendingAmount = sumPendingCommissionYuan(orders);

  const totalCommission = Number(summary?.total_commission_yuan || 0);
  const monthCommission = Number(summary?.month_commission_yuan || 0);
  const withdrawable = Number(summary?.available_yuan || 0);

  const loadWithdrawals = useCallback(async () => {
    setIsLoadingWithdrawals(true);
    try {
      const data = await fetchAgencyWithdrawals(100);
      setWithdrawals(data.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "提现明细加载失败");
    } finally {
      setIsLoadingWithdrawals(false);
    }
  }, []);

  const applyWithdrawProfile = useCallback((profile?: { alipay_qr_code?: string; wechat_qr_code?: string; phone?: string; wechat_id?: string }) => {
    setWithdrawAlipayQRCode(String(profile?.alipay_qr_code || ""));
    setWithdrawWechatQRCode(String(profile?.wechat_qr_code || ""));
    setWithdrawPhone(String(profile?.phone || ""));
    setWithdrawWechatID(String(profile?.wechat_id || ""));
  }, []);

  const loadWithdrawProfile = useCallback(async () => {
    try {
      const data = await fetchAgencyWithdrawProfile();
      applyWithdrawProfile(data.profile);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "提现资料加载失败");
    }
  }, [applyWithdrawProfile]);

  useEffect(() => {
    void loadWithdrawProfile();
  }, [loadWithdrawProfile]);

  const openWithdrawDetails = useCallback(() => {
    setActiveMenu("withdraw");
    void loadWithdrawals();
    void loadWithdrawProfile();
  }, [loadWithdrawProfile, loadWithdrawals]);

  const handleQRCodeUpload = async (kind: "alipay" | "wechat", file?: File | null) => {
    if (!file) return;
    setUploadingQRCodeKind(kind);
    try {
      const data = await uploadAgencyWithdrawQRCode(kind, file);
      applyWithdrawProfile(data.profile);
      toast.success(kind === "alipay" ? "支付宝收款码已上传" : "微信收款码已上传");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "收款码上传失败");
    } finally {
      setUploadingQRCodeKind("");
      if (kind === "alipay" && alipayQRCodeInputRef.current) alipayQRCodeInputRef.current.value = "";
      if (kind === "wechat" && wechatQRCodeInputRef.current) wechatQRCodeInputRef.current.value = "";
    }
  };

  const handleWithdrawApply = async () => {
    const amount = Number(withdrawAmount);
    if (!Number.isFinite(amount) || amount <= 0) {
      toast.error("请输入正确的提现金额");
      return;
    }
    if (amount > withdrawable) {
      toast.error("提现金额不能超过可提现余额");
      return;
    }
    const payload = {
      amount_cents: parseYuanToCents(withdrawAmount),
      alipay_qr_code: withdrawAlipayQRCode.trim(),
      wechat_qr_code: withdrawWechatQRCode.trim(),
      phone: withdrawPhone.trim(),
      wechat_id: withdrawWechatID.trim(),
    };
    if (!payload.alipay_qr_code && !payload.wechat_qr_code && !payload.phone && !payload.wechat_id) {
      toast.error("请至少填写一种收款联系方式");
      return;
    }
    setIsSubmittingWithdraw(true);
    try {
      await updateAgencyWithdrawProfile(payload);
      const data = await createAgencyWithdrawal(payload);
      toast.success("提现申请已提交，等待管理员审核");
      setWithdrawAmount("");
      setWithdrawAlipayQRCode("");
      setWithdrawWechatQRCode("");
      setWithdrawPhone("");
      setWithdrawWechatID("");
      setWithdrawals((items) => [data.item, ...items.filter((item) => item.id !== data.item.id)]);
      await onReload();
      await loadWithdrawals();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "提现申请提交失败");
    } finally {
      setIsSubmittingWithdraw(false);
    }
  };

  const trendRows = useMemo(() => calcTrendRows(orders), [orders]);
  const maxTrend = Math.max(1, ...trendRows.map((item) => item.value));
  const points = trendRows.map((item, idx) => {
    const x = (idx / Math.max(1, trendRows.length - 1)) * 100;
    const y = 100 - (item.value / maxTrend) * 70;
    return { x, y, ...item };
  });
  const pointsText = points.map((p) => `${p.x},${p.y}`).join(" ");

  const composition = useMemo(() => {
    const base = Math.max(0, totalCommission);
    if (base <= 0) return { recharge: 0, member: 0, consume: 0, rechargePct: 60, memberPct: 25, consumePct: 15 };
    const recharge = Number((base * 0.6).toFixed(2));
    const member = Number((base * 0.25).toFixed(2));
    const consume = Number((base - recharge - member).toFixed(2));
    return { recharge, member, consume, rechargePct: 60, memberPct: 25, consumePct: 15 };
  }, [totalCommission]);

  const ringStyle = {
    background: `conic-gradient(#e9cf9e 0% ${composition.rechargePct}%, #9f7e4c ${composition.rechargePct}% ${composition.rechargePct + composition.memberPct}%, #3c5a8b ${composition.rechargePct + composition.memberPct}% 100%)`,
  };

  const tierByKey = useMemo(() => {
    const map = new Map<TierKey, AgencyTier>();
    for (const t of tiers) {
      if (t.key === "basic" || t.key === "pro" || t.key === "premium") {
        map.set(t.key, t);
      }
    }
    return map;
  }, [tiers]);

  const currentTierConfig = currentTier === "basic" || currentTier === "pro" || currentTier === "premium" ? tierByKey.get(currentTier) : undefined;
  const effectiveCommissionBP = Number(agent?.commission_bp || 0) || Number(currentTierConfig?.commission_bp || 0);

  const upgradeCandidates = useMemo(() => {
    const baseRank = tierRank(currentTier) > 0 ? tierRank(currentTier) : tierRank(selectedTierKey);
    return tiers.filter((tier) => tierRank(tier.key) > baseRank && (tier.key === "basic" || tier.key === "pro" || tier.key === "premium")) as Array<AgencyTier & { key: TierKey }>;
  }, [currentTier, selectedTierKey, tiers]);

  const renderOverview = () => (
    <div className="space-y-4 min-w-0">
      <div className="relative overflow-hidden rounded-xl border border-[#d9e3fb] bg-[linear-gradient(120deg,#eef3ff,#e9edff_55%,#f1f4ff)] p-5">
        <div className="grid gap-4 lg:grid-cols-[1.2fr_1fr]">
          <div>
            <div className="text-2xl font-black tracking-tight text-[#102e60] md:text-3xl">代理分成中心</div>
            <div className="mt-2 text-base text-[#45639a]">邀请好友加入，享受高额分成奖励</div>
          </div>
          <div className="flex items-center justify-end">
            <div className="flex h-28 w-28 items-center justify-center rounded-full border border-[#f1cd8a] bg-[radial-gradient(circle,#ffe7ba_0%,#f2c77a_55%,#d89f46_100%)] shadow-[0_0_20px_rgba(234,196,129,0.35)]">
              <Crown className="size-10 text-[#2d1d09]" />
            </div>
          </div>
        </div>
      </div>

      <div className="rounded-xl border border-[#2a446f] bg-[linear-gradient(90deg,#f1e3cb_0%,#ead7b7_40%,#e3cfaa_100%)] p-4 text-[#24180e]">
        <div className="grid gap-3 md:grid-cols-4">
          <div>
            <div className="text-sm text-[#6b4f2a]">可提现收益（元）</div>
            <div className="mt-1 text-2xl font-black md:text-3xl">¥ {formatYuan(withdrawable)}</div>
            <Button className="mt-3 h-10 rounded-lg bg-[#151515] px-6 text-[#f3dfbe] hover:bg-black" onClick={openWithdrawDetails}>立即提现</Button>
          </div>
          <div><div className="text-sm text-[#6b4f2a]">累计收益（元）</div><div className="mt-3 text-2xl font-black md:text-3xl">{formatYuan(totalCommission)}</div></div>
          <div><div className="text-sm text-[#6b4f2a]">本月预估（元）</div><div className="mt-3 text-2xl font-black md:text-3xl">{formatYuan(monthCommission)}</div></div>
          <div><div className="text-sm text-[#6b4f2a]">可提现余额（元）</div><div className="mt-3 text-2xl font-black md:text-3xl">{formatYuan(withdrawable)}</div><button
              type="button"
              onClick={openWithdrawDetails}
              className="mt-2 ml-auto block rounded-full px-2 py-1 text-right text-sm font-semibold text-[#6b4f2a] transition hover:-translate-y-0.5 hover:bg-white/45 hover:text-[#24180e] hover:shadow-sm"
            >
              提现明细
            </button></div>
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.25fr)_minmax(0,1fr)]">
        <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f6f9ff)] text-[#16335f]">
          <CardContent className="space-y-4 pt-5">
            <div className="text-xl font-bold text-[#0f2f62]">我的专属推广</div>
            <div className="text-sm text-[#5b77a5]">专属链接</div>
            <div className="flex flex-col gap-2 lg:flex-row">
              <div className="flex-1 rounded-lg border border-[#c7d7f8] bg-white px-3 py-2 font-mono text-sm break-all text-[#1f3e73]">{link || "暂无推广链接"}</div>
              <Button type="button" className="h-10 rounded-lg bg-[#e6c894] text-[#2b1f10] hover:bg-[#f1d8ae]" onClick={async () => {
                if (!link) return;
                await navigator.clipboard.writeText(link);
                toast.success("专属链接已复制");
              }}>
                <Copy className="size-4" />
                复制链接
              </Button>
            </div>
            <div className="grid gap-3 md:grid-cols-[170px_1fr]">
              <div className="rounded-lg border border-[#d3def8] bg-[#f5f8ff] p-2">
                {qrURL ? <img src={qrURL} alt="专属推广二维码" className="h-[150px] w-[150px] rounded bg-white p-1" /> : <div className="flex h-[150px] w-[150px] items-center justify-center rounded border border-dashed border-[#35537d] text-xs text-[#9db1d1]">暂无二维码</div>}
              </div>
              <div className="space-y-2">
                <div className="text-base font-semibold text-[#193d73]">专属二维码</div>
                <p className="text-sm text-[#5877a8]">扫码注册自动绑定代理关系，后续充值消费将进入分成统计。</p>
                <div className="flex gap-2">
                  <Button variant="outline" className="h-9 rounded-lg border-[#c9d8ff] bg-white text-[#3d56d8]" asChild><a href={qrURL || "#"} download="agency-invite-qrcode.png" target="_blank" rel="noreferrer">下载二维码</a></Button>
                  <Button variant="outline" className="h-9 rounded-lg border-[#c9d8ff] bg-white text-[#3d56d8]">分享链接</Button>
                </div>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f6f9ff)] text-[#16335f]">
          <CardContent className="space-y-3 pt-5">
            <div className="flex items-center justify-between">
              <div className="text-xl font-bold text-[#0f2f62]">团队数据概览</div>
              <button type="button" className="text-xs text-[#5d7caf] hover:underline" onClick={() => setActiveMenu("team")}>查看全部</button>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-xl border border-[#d4e0fa] bg-white p-3"><div className="text-xs text-[#5878a8]">新增注册</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">{registeredCount.toLocaleString("zh-CN")}</div></div>
              <div className="rounded-xl border border-[#d4e0fa] bg-white p-3"><div className="text-xs text-[#5878a8]">充值金额</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">¥ {formatYuan(rechargeAmount)}</div></div>
              <div className="rounded-xl border border-[#d4e0fa] bg-white p-3"><div className="text-xs text-[#5878a8]">有效付费用户</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">{rechargeUserCount.toLocaleString("zh-CN")}</div></div>
              <div className="rounded-xl border border-[#d4e0fa] bg-white p-3"><div className="text-xs text-[#5878a8]">分成订单数</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">{orders.length.toLocaleString("zh-CN")}</div></div>
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="rounded-xl border border-[#d6d9f5] bg-[linear-gradient(120deg,#e8e8ff,#f0efff)] px-5 py-4 text-[#143765]">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div>
            <div className="text-xl font-bold">升级更高等级，享受更高分成比例</div>
            <div className="mt-1 text-sm text-[#5a77a8]">当前等级：{currentTierName || "未开通"}（分成比例 {formatPercentByBP(effectiveCommissionBP)}）</div>
          </div>
          <Button className="h-10 rounded-lg bg-[#e4c794] px-5 text-[#271a0e] hover:bg-[#f0d6ab]" onClick={() => setActiveMenu("benefits")}>查看等级权益</Button>
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
        <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f6f9ff)] text-[#16335f]">
          <CardHeader><CardTitle className="text-lg text-[#16335f]">收益趋势</CardTitle></CardHeader>
          <CardContent>
              <div className="rounded-xl border border-[#d4dff8] bg-[#f8fbff] p-4">
              <svg viewBox="0 0 100 100" preserveAspectRatio="none" className="h-52 w-full">
                <defs>
                  <linearGradient id="lineFill" x1="0" x2="0" y1="0" y2="1">
                    <stop offset="0%" stopColor="#7fa4ff" stopOpacity="0.25" />
                    <stop offset="100%" stopColor="#7fa4ff" stopOpacity="0" />
                  </linearGradient>
                </defs>
                <polyline points={`0,100 ${pointsText} 100,100`} fill="url(#lineFill)" stroke="none" />
                <polyline points={pointsText} fill="none" stroke="#4a6fff" strokeWidth="1.4" />
              </svg>
              <div className="mt-2 grid grid-cols-7 text-center text-xs text-[#5d7cad]">
                {trendRows.map((item) => <span key={item.key}>{item.label}</span>)}
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f6f9ff)] text-[#16335f]">
          <CardHeader><CardTitle className="text-lg text-[#16335f]">收益构成</CardTitle></CardHeader>
          <CardContent className="space-y-4">
            <div className="mx-auto flex h-44 w-44 items-center justify-center rounded-full p-5" style={ringStyle}>
              <div className="flex h-full w-full flex-col items-center justify-center rounded-full bg-[#08172c] text-center">
                <div className="text-sm text-[#5a79aa]">总收益</div>
                <div className="text-2xl font-black text-[#1d3f71]">{formatYuan(totalCommission)}</div>
              </div>
            </div>
              <div className="space-y-2 text-sm text-[#4a689b]">
                <div className="flex items-center justify-between"><span>充值分成</span><span>{composition.rechargePct}%</span><span>¥ {formatYuan(composition.recharge)}</span></div>
                <div className="flex items-center justify-between"><span>会员分成</span><span>{composition.memberPct}%</span><span>¥ {formatYuan(composition.member)}</span></div>
                <div className="flex items-center justify-between"><span>消费分成</span><span>{composition.consumePct}%</span><span>¥ {formatYuan(composition.consume)}</span></div>
              </div>
            </CardContent>
          </Card>
      </div>

      <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f8fbff)] text-[#16335f]">
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-lg text-[#16335f]">分成明细</CardTitle>
          <div className="text-xs text-[#c5a97a]">全部类型</div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="min-w-[860px] w-full text-sm">
              <thead>
                <tr className="border-b border-[#d7e2fb] text-left text-[#5d7cad]">
                  <th className="px-2 py-3">订单号</th>
                  <th className="px-2 py-3">用户</th>
                  <th className="px-2 py-3">类型</th>
                  <th className="px-2 py-3">金额（元）</th>
                  <th className="px-2 py-3">分成比例</th>
                  <th className="px-2 py-3">分成金额（元）</th>
                  <th className="px-2 py-3">时间</th>
                </tr>
              </thead>
              <tbody>
                {orders.length === 0 ? (
                  <tr><td className="px-2 py-10 text-center text-[#7892bf]" colSpan={7}>暂无分成订单</td></tr>
                ) : orders.slice(0, 10).map((item) => (
                  <tr key={item.id} className="border-b border-[#edf2ff] text-[#1f3f72]">
                    <td className="px-2 py-3">{item.out_trade_no || item.id || "-"}</td>
                    <td className="px-2 py-3">{item.user_email || item.user_id || "-"}</td>
                    <td className="px-2 py-3">充值分成</td>
                    <td className="px-2 py-3">¥ {item.amount_yuan || formatYuan((item.amount_cents || 0) / 100)}</td>
                    <td className="px-2 py-3">{formatPercentByBP(item.commission_bp || effectiveCommissionBP)}</td>
                    <td className="px-2 py-3">¥ {item.commission_yuan || formatYuan((item.commission_cents || 0) / 100)}</td>
                    <td className="px-2 py-3">{formatDateTime(item.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="mt-4 flex items-center justify-between text-sm text-[#6c87b6]">
            <span>共 {orders.length} 条</span>
            <span>10 条/页</span>
          </div>
        </CardContent>
      </Card>
    </div>
  );

  const renderPromotion = () => (
    <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f6f9ff)] text-[#16335f]">
      <CardHeader><CardTitle className="text-lg text-[#16335f]">推广链接</CardTitle></CardHeader>
      <CardContent className="space-y-3">
        <div className="rounded-lg border border-[#c7d7f8] bg-white px-3 py-2 font-mono text-sm break-all text-[#1f3e73]">{link || "暂无推广链接"}</div>
        <div className="grid gap-4 md:grid-cols-[280px_1fr]">
          <div>{qrURL ? <img src={qrURL} alt="专属推广二维码" className="h-[260px] w-[260px] rounded-md bg-white p-2" /> : null}</div>
          <div className="space-y-3">
            <div className="text-sm text-[#5877a8]">二维码会随着你的推广链接自动生成。正式上线后使用正式域名访问，二维码会自动切换到正式域名地址。</div>
            <div className="flex gap-2"><Button className="h-9 rounded-lg bg-[#e6c894] text-[#2b1f10] hover:bg-[#f1d8ae]">复制链接</Button><Button variant="outline" className="h-9 rounded-lg border-[#c9d8ff] bg-white text-[#3d56d8]">下载二维码</Button></div>
          </div>
        </div>
      </CardContent>
    </Card>
  );

  const renderTeam = () => (
    <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f6f9ff)] text-[#16335f]">
      <CardHeader><CardTitle className="text-lg text-[#16335f]">我的团队</CardTitle></CardHeader>
      <CardContent className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <div className="rounded-xl border border-[#d4e0fa] bg-white p-3"><div className="text-sm text-[#5878a8]">邀请用户总数</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">{invitedCount.toLocaleString("zh-CN")}</div></div>
        <div className="rounded-xl border border-[#d4e0fa] bg-white p-3"><div className="text-sm text-[#5878a8]">新增注册</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">{registeredCount.toLocaleString("zh-CN")}</div></div>
        <div className="rounded-xl border border-[#d4e0fa] bg-white p-3"><div className="text-sm text-[#5878a8]">有效付费用户</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">{rechargeUserCount.toLocaleString("zh-CN")}</div></div>
        <div className="rounded-xl border border-[#d4e0fa] bg-white p-3"><div className="text-sm text-[#5878a8]">充值金额</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">¥ {formatYuan(rechargeAmount)}</div></div>
      </CardContent>
    </Card>
  );

  const renderIncome = () => (
    <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f8fbff)] text-[#16335f]">
      <CardHeader><CardTitle className="text-lg text-[#16335f]">收益明细</CardTitle></CardHeader>
      <CardContent>
        <div className="overflow-x-auto">
          <table className="min-w-[860px] w-full text-sm">
            <thead>
              <tr className="border-b border-[#d7e2fb] text-left text-[#5d7cad]">
                <th className="px-2 py-3">订单号</th>
                <th className="px-2 py-3">用户</th>
                <th className="px-2 py-3">类型</th>
                <th className="px-2 py-3">金额（元）</th>
                <th className="px-2 py-3">分成比例</th>
                <th className="px-2 py-3">分成金额（元）</th>
                <th className="px-2 py-3">状态</th>
                <th className="px-2 py-3">时间</th>
              </tr>
            </thead>
            <tbody>
              {orders.length === 0 ? (
                <tr><td className="px-2 py-9 text-center text-[#7892bf]" colSpan={8}>暂无收益订单</td></tr>
              ) : orders.slice(0, 20).map((item) => (
                <tr key={item.id} className="border-b border-[#edf2ff] text-[#1f3f72]">
                  <td className="px-2 py-3">{item.out_trade_no || item.id || "-"}</td>
                  <td className="px-2 py-3">{item.user_email || item.user_id || "-"}</td>
                  <td className="px-2 py-3">充值分成</td>
                  <td className="px-2 py-3">¥ {item.amount_yuan || formatYuan((item.amount_cents || 0) / 100)}</td>
                  <td className="px-2 py-3">{formatPercentByBP(item.commission_bp || effectiveCommissionBP)}</td>
                  <td className="px-2 py-3">¥ {item.commission_yuan || formatYuan((item.commission_cents || 0) / 100)}</td>
                  <td className="px-2 py-3">{STATUS_TEXT[(item.status || "").toLowerCase()] || item.status || "-"}</td>
                  <td className="px-2 py-3">{formatDateTime(item.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  );

  const renderWithdraw = () => (
    <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f8fbff)] text-[#16335f]">
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-lg text-[#16335f]">提现管理</CardTitle>
        <Button variant="outline" className="h-8 rounded-lg border-[#c9d8ff] bg-white text-[#3d56d8]" onClick={() => void loadWithdrawals()} disabled={isLoadingWithdrawals}>
          <RefreshCw className={cn("size-4", isLoadingWithdrawals ? "animate-spin" : "")} />
          刷新明细
        </Button>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 md:grid-cols-3">
          <div className="rounded-xl border border-[#d4e0fa] bg-white p-4"><div className="text-sm text-[#5878a8]">可提现余额</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">¥ {formatYuan(withdrawable)}</div></div>
          <div className="rounded-xl border border-[#d4e0fa] bg-white p-4"><div className="text-sm text-[#5878a8]">待结算收益</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">¥ {formatYuan(pendingAmount)}</div></div>
          <div className="rounded-xl border border-[#d4e0fa] bg-white p-4"><div className="text-sm text-[#5878a8]">累计收益</div><div className="mt-2 text-2xl font-black text-[#1a3f72]">¥ {formatYuan(totalCommission)}</div></div>
        </div>
        <div className="rounded-xl border border-[#d4e0fa] bg-white p-4">
          <div className="mb-3 text-base font-bold text-[#16335f]">提交提现申请</div>
          <div className="grid gap-3 md:grid-cols-2">
            <Input value={withdrawAmount} onChange={(event) => setWithdrawAmount(event.target.value)} inputMode="decimal" placeholder="提现金额（元）" className="h-10 rounded-lg border-[#c9d8ff]" />
            <Input value={withdrawPhone} onChange={(event) => setWithdrawPhone(event.target.value)} placeholder="手机号" className="h-10 rounded-lg border-[#c9d8ff]" />
            <Input value={withdrawWechatID} onChange={(event) => setWithdrawWechatID(event.target.value)} placeholder="微信号" className="h-10 rounded-lg border-[#c9d8ff]" />
            <div className="space-y-2">
              <div className="flex gap-2">
                <Input value={withdrawAlipayQRCode} onChange={(event) => setWithdrawAlipayQRCode(event.target.value)} placeholder={"\u652f\u4ed8\u5b9d\u6536\u6b3e\u7801\u94fe\u63a5\u6216\u5907\u6ce8"} className="h-10 rounded-lg border-[#c9d8ff]" />
                <Button type="button" variant="outline" className="h-10 shrink-0 rounded-lg border-[#c9d8ff] bg-white text-[#3d56d8]" onClick={() => alipayQRCodeInputRef.current?.click()} disabled={uploadingQRCodeKind === "alipay"}>
                  {uploadingQRCodeKind === "alipay" ? <LoaderCircle className="size-4 animate-spin" /> : null}
                  {"\u4e0a\u4f20\u652f\u4ed8\u5b9d\u7801"}
                </Button>
              </div>
              <input ref={alipayQRCodeInputRef} type="file" accept="image/*" className="hidden" onChange={(event) => void handleQRCodeUpload("alipay", event.target.files?.[0])} />
            </div>
            <div className="space-y-2 md:col-span-2">
              <div className="flex gap-2">
                <Input value={withdrawWechatQRCode} onChange={(event) => setWithdrawWechatQRCode(event.target.value)} placeholder={"\u5fae\u4fe1\u6536\u6b3e\u7801\u94fe\u63a5\u6216\u5907\u6ce8"} className="h-10 rounded-lg border-[#c9d8ff]" />
                <Button type="button" variant="outline" className="h-10 shrink-0 rounded-lg border-[#c9d8ff] bg-white text-[#3d56d8]" onClick={() => wechatQRCodeInputRef.current?.click()} disabled={uploadingQRCodeKind === "wechat"}>
                  {uploadingQRCodeKind === "wechat" ? <LoaderCircle className="size-4 animate-spin" /> : null}
                  {"\u4e0a\u4f20\u5fae\u4fe1\u7801"}
                </Button>
              </div>
              <input ref={wechatQRCodeInputRef} type="file" accept="image/*" className="hidden" onChange={(event) => void handleQRCodeUpload("wechat", event.target.files?.[0])} />
            </div>
          </div>
          <div className="mt-3 flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
            <div className="text-xs text-[#6d89b6]">至少填写一种收款联系方式；提交后进入管理员审核，不会自动打款。</div>
            <Button className="h-10 rounded-lg bg-[#151515] px-6 text-[#f3dfbe] hover:bg-black" onClick={handleWithdrawApply} disabled={isSubmittingWithdraw}>
              {isSubmittingWithdraw ? <LoaderCircle className="size-4 animate-spin" /> : null}
              提交提现
            </Button>
          </div>
        </div>
        <div className="rounded-xl border border-[#d4e0fa] bg-white p-4">
          <div className="mb-3 flex items-center justify-between">
            <div className="text-base font-bold text-[#16335f]">提现明细</div>
            <div className="text-xs text-[#6d89b6]">共 {withdrawals.length} 条</div>
          </div>
          <div className="overflow-x-auto">
            <table className="min-w-[760px] w-full text-sm">
              <thead>
                <tr className="border-b border-[#d7e2fb] text-left text-[#5d7cad]">
                  <th className="px-2 py-3">申请编号</th>
                  <th className="px-2 py-3">金额</th>
                  <th className="px-2 py-3">状态</th>
                  <th className="px-2 py-3">收款信息</th>
                  <th className="px-2 py-3">申请时间</th>
                  <th className="px-2 py-3">处理时间</th>
                  <th className="px-2 py-3">备注</th>
                </tr>
              </thead>
              <tbody>
                {isLoadingWithdrawals ? (
                  <tr><td className="px-2 py-9 text-center text-[#7892bf]" colSpan={7}><LoaderCircle className="mr-2 inline size-4 animate-spin" />正在加载提现明细</td></tr>
                ) : withdrawals.length === 0 ? (
                  <tr><td className="px-2 py-9 text-center text-[#7892bf]" colSpan={7}>暂无提现明细</td></tr>
                ) : withdrawals.map((item) => (
                  <tr key={item.id} className="border-b border-[#edf2ff] text-[#1f3f72]">
                    <td className="px-2 py-3 font-mono text-xs">{item.id || "-"}</td>
                    <td className="px-2 py-3">¥ {item.amount_yuan || formatYuan((item.amount_cents || 0) / 100)}</td>
                    <td className="px-2 py-3">{WITHDRAW_STATUS_TEXT[(item.status || "").toLowerCase()] || item.status || "-"}</td>
                    <td className="px-2 py-3">{payoutSummary(item)}</td>
                    <td className="px-2 py-3">{formatDateTime(item.created_at)}</td>
                    <td className="px-2 py-3">{formatDateTime(item.processed_at)}</td>
                    <td className="px-2 py-3">{item.admin_note || "-"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </CardContent>
    </Card>
  );

  const renderMaterials = () => (
    <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f8fbff)] text-[#16335f]">
      <CardHeader><CardTitle className="text-lg text-[#16335f]">推广素材</CardTitle></CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-4 md:grid-cols-3">
          {materials.map((item) => (
            <div key={item.id} className="rounded-xl border border-[#d4e0fa] bg-white p-3">
              <div className="mb-2 text-sm font-semibold text-[#274a7c]">{item.title || "推广素材"}</div>
              {item.image_url ? (
                <img src={materialPreviewByID[item.id] || item.image_url} alt={item.title || "推广素材"} className="h-28 w-full rounded-lg object-cover" />
              ) : (
                <div className="h-28 rounded-lg bg-[linear-gradient(120deg,#f8edd8,#eef4ff)]" />
              )}
              {item.description ? <div className="mt-2 text-xs leading-5 text-[#6d86ae]">{item.description}</div> : null}
              {item.copy ? <div className="mt-2 rounded-lg bg-[#f6f9ff] p-2 text-xs leading-6 text-[#5f7faf]">{item.copy}</div> : null}
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );

  const renderBenefits = () => (
    <div className="space-y-4">
      <div className="rounded-xl border border-[#d6d9f5] bg-[linear-gradient(120deg,#eef0ff,#f5f6ff)] px-5 py-4 text-[#133764]">
        <div className="text-xl font-bold">等级权益与升级</div>
        <div className="mt-1 text-sm text-[#5a77a8]">当前等级：{currentTierName || "未开通"}（分成比例 {formatPercentByBP(effectiveCommissionBP)}）</div>
      </div>

      <div className="grid gap-3 md:grid-cols-3">
        {(["basic", "pro", "premium"] as const).map((key) => {
          const tier = tierByKey.get(key);
          if (!tier) return null;
          const selected = selectedTierKey === key;
          return (
            <button
              key={key}
              type="button"
              onClick={() => setSelectedTierKey(key)}
              className={cn("rounded-xl border p-4 text-left transition", selected ? "border-[#e4c794] bg-[#f9f2e5] text-[#8a5b1e]" : "border-[#d4e0fa] bg-white text-[#5f7faf]")}
            >
              <div className="text-lg font-bold">{tier.name}</div>
              <div className="mt-2 text-sm">分成：{tier.commission_percent || formatPercentByBP(tier.commission_bp)}</div>
              <div className="text-sm">优惠：{tier.discount_percent || formatPercentByBP(tier.discount_bp)}</div>
            </button>
          );
        })}
      </div>

      <Card className="min-w-0 rounded-xl border-[#d9e3fb] bg-none [background:linear-gradient(160deg,#ffffff,#f8fbff)] text-[#16335f]">
        <CardHeader><CardTitle className="text-lg text-[#16335f]">可升级选项</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          {upgradeCandidates.length === 0 ? (
            <div className="text-sm text-[#6b86b5]">当前档位已是最高或无可升级方案。</div>
          ) : (
            <div className="grid gap-3 md:grid-cols-2">
              {upgradeCandidates.map((tier) => (
                <div key={tier.key} className="rounded-xl border border-[#d4e0fa] bg-white p-3">
                  <div className="text-lg font-bold">{tier.name}</div>
                  <div className="mt-1 text-sm text-[#5d7cad]">升级价格：¥ {formatYuan((tier.price_cents || 0) / 100)}</div>
                  <div className="text-sm text-[#5d7cad]">分成比例：{tier.commission_percent || formatPercentByBP(tier.commission_bp)}</div>
                  <Button className="mt-3 h-9 rounded-lg bg-[#e6c894] text-[#2b1f10] hover:bg-[#f1d8ae]" onClick={() => onRequestUpgrade(tier.key)}>升级到 {tier.name}</Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );

  const renderSettings = () => (
    <div className="rounded-xl border border-[#d6d9f5] bg-[linear-gradient(120deg,#eef0ff,#f5f6ff)] p-4 text-center text-[#173a68]">
      <div className="flex items-center justify-center gap-2"><img src={brandLogoURL} alt="" className="size-7 rounded-md border border-[#876437]" /><span className="font-semibold">{brandTitle}</span></div>
      <div className="mt-1 text-xs text-[#5f7faf]">当前等级：{currentTierName || "未开通"}，待结算：¥ {formatYuan(pendingAmount)}，邀请总数：{invitedCount}</div>
      <Button variant="outline" className="mt-3 h-8 rounded-lg border-[#c9d8ff] bg-white text-[#3d56d8]" onClick={() => void onReload()} disabled={isLoading}><RefreshCw className={cn("size-4", isLoading ? "animate-spin" : "")} />刷新数据</Button>
    </div>
  );

  const renderContent = () => {
    if (activeMenu === "overview") return renderOverview();
    if (activeMenu === "promotion") return renderPromotion();
    if (activeMenu === "team") return renderTeam();
    if (activeMenu === "income") return renderIncome();
    if (activeMenu === "withdraw") return renderWithdraw();
    if (activeMenu === "materials") return renderMaterials();
    if (activeMenu === "benefits") return renderBenefits();
    return renderSettings();
  };

  return (
    <section className="overflow-hidden rounded-2xl border border-[#dde3f5] bg-[linear-gradient(160deg,#f7f9ff_0%,#f2f4ff_55%,#f8f9ff_100%)] p-4 text-[#16335f]">
      <div className="grid gap-4 xl:grid-cols-[220px_minmax(0,1fr)]">
        <aside className="space-y-4">
          <div className="rounded-2xl border border-[#dde3f5] bg-white p-4">
            <div className="flex items-center gap-2"><Crown className="size-4 text-[#5a70ff]" /><span className="text-lg font-bold text-[#16335f]">至尊代理</span></div>
            <div className="mt-2 text-sm text-[#5f7faf]">高级合伙人</div>
          </div>

          <div className="rounded-2xl border border-[#dde3f5] bg-white p-2">
            {AGENCY_MENU.map((item) => {
              const Icon = item.icon;
              const active = activeMenu === item.key;
              return (
                <button
                  key={item.key}
                  type="button"
                  onClick={() => setActiveMenu(item.key)}
                  className={cn("mb-1.5 flex w-full items-center gap-2 rounded-xl px-3 py-2.5 text-left text-sm transition", active ? "bg-[#edf2ff] text-[#214175]" : "text-[#617faa] hover:bg-[#f3f6ff]")}
                >
                  <Icon className={cn("size-4", active ? "text-[#4f66ff]" : "text-[#8ea8cf]")} />
                  <span>{item.label}</span>
                </button>
              );
            })}
          </div>
        </aside>

        <div className="min-w-0">{renderContent()}</div>
      </div>
    </section>
  );
}

export default function AgencyPage() {
  const appMeta = useAppMeta();
  const session = useMemo(() => getCachedAuthSession(), []);
  const isAdmin = session?.role === "admin";

  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [config, setConfig] = useState<AgencyConfig | null>(null);
  const [prices, setPrices] = useState<Record<string, string>>({});
  const [commissionBPs, setCommissionBPs] = useState<Record<string, string>>({});
  const [discountBPs, setDiscountBPs] = useState<Record<string, string>>({});
  const [materialDrafts, setMaterialDrafts] = useState<AgencyMaterial[]>([]);
  const [materialQRConfig, setMaterialQRConfig] = useState<Required<AgencyMaterialQRConfig>>(
    normalizeAgencyMaterialQRConfig(),
  );

  const [currentTier, setCurrentTier] = useState("");
  const [agencyEnabled, setAgencyEnabled] = useState(false);
  const [joiningTier, setJoiningTier] = useState("");
  const [agencyPayChannels, setAgencyPayChannels] = useState<PayType[]>([]);
  const [agencyPayType, setAgencyPayType] = useState<PayType | "">("");
  const [isPayDialogOpen, setIsPayDialogOpen] = useState(false);
  const [pendingJoinTier, setPendingJoinTier] = useState<TierKey | "">("");
  const [dashboard, setDashboard] = useState<AgencyCommissionDashboard | null>(null);
  const [isDashboardLoading, setIsDashboardLoading] = useState(false);

  const [allUsers, setAllUsers] = useState<ManagedUser[]>([]);
  const [agencyUsers, setAgencyUsers] = useState<AgencyAdminUser[]>([]);
  const [isLoadingAdminUsers, setIsLoadingAdminUsers] = useState(false);
  const [activatingUserID, setActivatingUserID] = useState("");
  const [adminWithdrawals, setAdminWithdrawals] = useState<AgencyWithdrawalRequest[]>([]);
  const [isLoadingAdminWithdrawals, setIsLoadingAdminWithdrawals] = useState(false);
  const [processingWithdrawalID, setProcessingWithdrawalID] = useState("");
  const [withdrawalNotes, setWithdrawalNotes] = useState<Record<string, string>>({});

  const loadDashboard = useCallback(async (): Promise<boolean> => {
    if (isAdmin) return true;
    setIsDashboardLoading(true);
    try {
      const data = await fetchAgencyCommissionDashboard();
      setDashboard(data);
      return true;
    } catch (error) {
      setDashboard(null);
      const message = error instanceof Error ? error.message : "加载代理分成失败";
      if (/permission denied|forbidden/i.test(message)) {
        setAgencyEnabled(false);
        toast.info("当前账号无代理分成权限，已切换到代理开通页");
        return false;
      }
      toast.error(message);
      return false;
    } finally {
      setIsDashboardLoading(false);
    }
  }, [isAdmin]);

  const loadAdminUsers = useCallback(async () => {
    if (!isAdmin) return;
    setIsLoadingAdminUsers(true);
    try {
      const [usersRes, agentsRes] = await Promise.all([fetchManagedUsers(), fetchAgencyAdminUsers()]);
      setAllUsers(usersRes.items || []);
      setAgencyUsers(agentsRes.items || []);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载代理用户失败");
    } finally {
      setIsLoadingAdminUsers(false);
    }
  }, [isAdmin]);

  const loadAdminWithdrawals = useCallback(async () => {
    if (!isAdmin) return;
    setIsLoadingAdminWithdrawals(true);
    try {
      const res = await fetchAgencyAdminWithdrawals(500);
      const items = res.items || [];
      setAdminWithdrawals(items);
      setWithdrawalNotes((current) => {
        const next = { ...current };
        for (const item of items) {
          if (next[item.id] === undefined) {
            next[item.id] = item.admin_note || "";
          }
        }
        return next;
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载提现申请失败");
    } finally {
      setIsLoadingAdminWithdrawals(false);
    }
  }, [isAdmin]);

  const load = useCallback(async () => {
    setIsLoading(true);
    try {
      const data = await fetchAgencyConfig();
      setConfig(data);

      const nextPrices: Record<string, string> = {};
      const nextCommissionBPs: Record<string, string> = {};
      const nextDiscountBPs: Record<string, string> = {};
      for (const tier of data.tiers || []) {
        nextPrices[tier.key] = (Math.max(0, Number(tier.price_cents || 0)) / 100).toFixed(2);
        nextCommissionBPs[tier.key] = String(tier.commission_bp ?? 0);
        nextDiscountBPs[tier.key] = String(tier.discount_bp ?? 0);
      }
      setPrices(nextPrices);
      setCommissionBPs(nextCommissionBPs);
      setDiscountBPs(nextDiscountBPs);
      setMaterialDrafts((data.materials || []).map((item) => ({ ...item })));
      setMaterialQRConfig(normalizeAgencyMaterialQRConfig(data.material_qr));

      if (isAdmin) {
        setAgencyEnabled(false);
        setDashboard(null);
      } else {
        try {
          const wallet = await fetchWallet();
          const token = await getStoredSessionToken();
          let roleName = String(session?.roleName || "").trim();
          let roleID = String(session?.roleId || "").trim();
          if (token) {
            try {
              const sessionData = await verifySession(token);
              roleName = String(sessionData.role_name || "").trim();
              roleID = String(sessionData.role_id || "").trim();
            } catch {}
          }
          const walletTier = wallet.wallet?.agency_enabled ? String(wallet.wallet?.agency_tier || "").trim() : "";
          const tier = tierRank(walletTier) > 0 ? walletTier : tierFromRole(roleName, roleID);
          const enabled = tier !== "";
          const channels = Array.isArray(wallet.pay_channels) ? (wallet.pay_channels.filter(Boolean) as PayType[]) : [];
          setAgencyPayChannels(channels);
          setCurrentTier(tier);
          if (!enabled) {
            setAgencyEnabled(false);
            setDashboard(null);
          } else {
            setAgencyEnabled(true);
            const ok = await loadDashboard();
            if (!ok) setAgencyEnabled(false);
          }
        } catch {
          setCurrentTier("");
          setAgencyEnabled(false);
          setDashboard(null);
        }
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "加载代理配置失败");
    } finally {
      setIsLoading(false);
    }
  }, [isAdmin, loadDashboard, session?.roleId, session?.roleName]);

  useEffect(() => {
    void load();
  }, [load]);

  const tiers = useMemo(() => orderedTiers(config?.tiers || []), [config]);

  const handleSubmitJoin = async (tier: TierKey, payType: PayType) => {
    setJoiningTier(tier);
    try {
      const currentRank = tierRank(currentTier);
      const targetRank = tierRank(tier);
      const response = currentRank > 0 && targetRank > currentRank ? await upgradeAgencyTier(tier, payType) : await joinAgencyTier(tier, payType);
      const payURL = String(response.order?.pay_url || "").trim();
      if (payURL) {
        window.open(payURL, "_blank", "noopener,noreferrer");
        toast.success("请先完成支付，支付成功后会自动开通代理权限");
      } else {
        toast.error("未获取到支付链接，请检查支付配置");
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "开通代理失败";
      if (/already activated/i.test(message)) {
        await load();
        return;
      }
      toast.error(message);
    } finally {
      setJoiningTier("");
    }
  };

  const handleJoin = async (tier: TierKey) => {
    if (isAdmin) {
      toast.info("管理员账号无需购买代理套餐");
      return;
    }
    const currentRank = tierRank(currentTier);
    const targetRank = tierRank(tier);
    if (currentRank > 0 && targetRank <= currentRank) return;
    setPendingJoinTier(tier);
    setIsPayDialogOpen(true);
  };

  const handleConfirmJoin = async () => {
    if (!pendingJoinTier) return;
    if (agencyPayChannels.length === 0) {
      toast.error("当前未启用支付渠道，请联系管理员在后台支付配置中开启");
      return;
    }
    if (!agencyPayType) {
      toast.error("请选择您需要的支付方式");
      return;
    }
    setIsPayDialogOpen(false);
    await handleSubmitJoin(pendingJoinTier, agencyPayType);
    setPendingJoinTier("");
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      await updateAgencyConfig({
        agency_tier_basic_cents: parseYuanToCents(prices.basic),
        agency_tier_pro_cents: parseYuanToCents(prices.pro),
        agency_tier_premium_cents: parseYuanToCents(prices.premium),
        agency_tier_basic_commission_bp: parsePositiveInt(commissionBPs.basic),
        agency_tier_pro_commission_bp: parsePositiveInt(commissionBPs.pro),
        agency_tier_premium_commission_bp: parsePositiveInt(commissionBPs.premium),
        agency_tier_basic_discount_bp: parsePositiveInt(discountBPs.basic),
        agency_tier_pro_discount_bp: parsePositiveInt(discountBPs.pro),
        agency_tier_premium_discount_bp: parsePositiveInt(discountBPs.premium),
        agency_materials: materialDrafts,
        agency_material_qr_enabled: materialQRConfig.enabled,
        agency_material_qr_x_percent: materialQRConfig.x_percent,
        agency_material_qr_y_percent: materialQRConfig.y_percent,
        agency_material_qr_size_percent: materialQRConfig.size_percent,
        agency_material_qr_logo_percent: materialQRConfig.logo_percent,
      });
      toast.success("代理配置已更新");
      await load();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "保存代理配置失败");
    } finally {
      setIsSaving(false);
    }
  };

  const agencyTierByUserID = useMemo(() => {
    const map = new Map<string, string>();
    for (const item of agencyUsers) {
      if (item.id) map.set(item.id, item.agency_tier || "");
    }
    return map;
  }, [agencyUsers]);

  const sortedAdminUsers = useMemo(
    () => [...allUsers].sort((a, b) => (a.email || a.username || a.name || a.id || "").localeCompare((b.email || b.username || b.name || b.id || ""), "zh-CN")),
    [allUsers],
  );

  const handleActivateAgency = async (userID: string, tier: TierKey) => {
    setActivatingUserID(`${userID}:${tier}`);
    try {
      await activateAgencyUser({ user_id: userID, tier });
      toast.success("已更新该用户代理等级");
      await loadAdminUsers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新代理等级失败");
    } finally {
      setActivatingUserID("");
    }
  };

  const handleUpdateWithdrawal = async (id: string, status: "pending" | "approved" | "paid" | "rejected") => {
    setProcessingWithdrawalID(`${id}:${status}`);
    try {
      const res = await updateAgencyAdminWithdrawal({
        id,
        status,
        admin_note: withdrawalNotes[id] || "",
      });
      setAdminWithdrawals((current) => current.map((item) => (item.id === id ? res.item : item)));
      setWithdrawalNotes((current) => ({ ...current, [id]: res.item.admin_note || "" }));
      toast.success("提现申请状态已更新");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新提现申请失败");
    } finally {
      setProcessingWithdrawalID("");
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-16">
        <LoaderCircle className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const showAgencyPackage = isAdmin || !agencyEnabled;
  const currentTierName = tierLabel(currentTier, tiers);
  const brandTitle = (appMeta.app_title || "chatgpt2api").trim() || "chatgpt2api";
  const brandLogoURL = resolveBrandAssetURL(appMeta.top_left_logo_url || "/logo-mark.svg") || "/logo-mark.svg";

  return (
    <div className="mx-auto w-full max-w-[1280px] space-y-5">
      {showAgencyPackage ? (
        <AgencyPackageShowcase tiers={tiers} currentTier={currentTier} joiningTier={joiningTier} onJoin={handleJoin} />
      ) : (
        <AgencyCommissionCenter
          dashboard={dashboard}
          tiers={tiers}
          currentTier={currentTier}
          currentTierName={currentTierName}
          brandTitle={brandTitle}
          brandLogoURL={brandLogoURL}
          materials={config?.materials || []}
          materialQRConfig={materialQRConfig}
          isLoading={isDashboardLoading}
          onReload={loadDashboard}
          onRequestUpgrade={(tier) => void handleJoin(tier)}
        />
      )}

      {showAgencyPackage && !isAdmin ? (
        <Dialog
          open={isPayDialogOpen}
          onOpenChange={(open) => {
            setIsPayDialogOpen(open);
            if (!open) setPendingJoinTier("");
          }}
        >
          <DialogContent className="rounded-2xl p-6 sm:max-w-md">
            <DialogHeader><DialogTitle>选择支付方式</DialogTitle></DialogHeader>
            <div className="space-y-3">
              <Select value={agencyPayType || undefined} onValueChange={(value) => setAgencyPayType(value as PayType)}>
                <SelectTrigger className="h-11 rounded-xl"><SelectValue placeholder="请选择支付方式" /></SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {agencyPayChannels.includes("alipay") ? <SelectItem value="alipay">支付宝</SelectItem> : null}
                    {agencyPayChannels.includes("wxpay") ? <SelectItem value="wxpay">微信支付</SelectItem> : null}
                    {agencyPayChannels.includes("qqpay") ? <SelectItem value="qqpay">QQ钱包</SelectItem> : null}
                    {agencyPayChannels.includes("paypal") ? <SelectItem value="paypal">PayPal</SelectItem> : null}
                    {agencyPayChannels.includes("usdt") ? <SelectItem value="usdt">USDT</SelectItem> : null}
                  </SelectGroup>
                </SelectContent>
              </Select>
              {agencyPayChannels.length === 0 ? <p className="text-sm text-muted-foreground">当前未启用支付渠道，请联系管理员在后台支付配置中开启。</p> : null}
            </div>
            <DialogFooter>
              <Button type="button" variant="secondary" className="h-10 rounded-xl px-5" onClick={() => setIsPayDialogOpen(false)}>取消</Button>
              <Button type="button" className="h-10 rounded-xl px-5" onClick={() => void handleConfirmJoin()} disabled={joiningTier !== ""}>
                {joiningTier ? <LoaderCircle className="size-4 animate-spin" /> : null}
                确认并支付
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      ) : null}

      {isAdmin ? (
        <>
          <Card className="rounded-2xl border-border/80">
            <CardHeader><CardTitle className="text-lg">代理价格与比例配置</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-3 md:grid-cols-3">
                {tiers.map((tier) => (
                  <div key={tier.key} className="space-y-2 rounded-xl border border-border/60 p-3">
                    <div className="text-sm font-medium">{tier.name}</div>
                    <Field className="gap-1.5"><FieldLabel htmlFor={`agency-price-${tier.key}`}>价格（元）</FieldLabel><Input id={`agency-price-${tier.key}`} value={prices[tier.key] ?? ""} onChange={(event) => setPrices((prev) => ({ ...prev, [tier.key]: event.target.value }))} /></Field>
                    <Field className="gap-1.5"><FieldLabel htmlFor={`agency-commission-${tier.key}`}>分成比例（BP）</FieldLabel><Input id={`agency-commission-${tier.key}`} value={commissionBPs[tier.key] ?? ""} onChange={(event) => setCommissionBPs((prev) => ({ ...prev, [tier.key]: event.target.value }))} /></Field>
                    <Field className="gap-1.5"><FieldLabel htmlFor={`agency-discount-${tier.key}`}>充值优惠（BP）</FieldLabel><Input id={`agency-discount-${tier.key}`} value={discountBPs[tier.key] ?? ""} onChange={(event) => setDiscountBPs((prev) => ({ ...prev, [tier.key]: event.target.value }))} /></Field>
                  </div>
                ))}
              </div>
              <Button onClick={() => void handleSave()} disabled={isSaving}>{isSaving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}保存代理配置</Button>
            </CardContent>
          </Card>


          <Card className="rounded-2xl border-border/80">
            <CardHeader><CardTitle className="text-lg">推广素材配置</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-3 lg:grid-cols-3">
                {materialDrafts.map((item, index) => (
                  <div key={item.id || index} className="space-y-2 rounded-xl border border-border/60 p-3">
                    <Field className="gap-1.5">
                      <FieldLabel htmlFor={`agency-material-title-${index}`}>标题</FieldLabel>
                      <Input id={`agency-material-title-${index}`} value={item.title || ""} onChange={(event) => setMaterialDrafts((current) => current.map((draft, i) => i === index ? { ...draft, title: event.target.value } : draft))} />
                    </Field>
                    <Field className="gap-1.5">
                      <FieldLabel htmlFor={`agency-material-desc-${index}`}>简介</FieldLabel>
                      <Input id={`agency-material-desc-${index}`} value={item.description || ""} onChange={(event) => setMaterialDrafts((current) => current.map((draft, i) => i === index ? { ...draft, description: event.target.value } : draft))} />
                    </Field>
                    <Field className="gap-1.5">
                      <FieldLabel htmlFor={`agency-material-image-${index}`}>图片地址</FieldLabel>
                      <Input id={`agency-material-image-${index}`} value={item.image_url || ""} onChange={(event) => setMaterialDrafts((current) => current.map((draft, i) => i === index ? { ...draft, image_url: event.target.value } : draft))} />
                    </Field>
                    <Field className="gap-1.5">
                      <FieldLabel htmlFor={`agency-material-copy-${index}`}>文案内容</FieldLabel>
                      <Input id={`agency-material-copy-${index}`} value={item.copy || ""} onChange={(event) => setMaterialDrafts((current) => current.map((draft, i) => i === index ? { ...draft, copy: event.target.value } : draft))} />
                    </Field>
                  </div>
                ))}
              </div>
              <div className="rounded-xl border border-border/60 p-3">
                <div className="mb-3 text-sm font-semibold">二维码叠加设置</div>
                <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-5">
                  <Field className="gap-1.5">
                    <FieldLabel htmlFor="agency-material-qr-enabled">启用叠加</FieldLabel>
                    <Select
                      value={materialQRConfig.enabled ? "on" : "off"}
                      onValueChange={(value) =>
                        setMaterialQRConfig((current) => ({ ...current, enabled: value === "on" }))
                      }
                    >
                      <SelectTrigger id="agency-material-qr-enabled">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="on">开启</SelectItem>
                        <SelectItem value="off">关闭</SelectItem>
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field className="gap-1.5">
                    <FieldLabel htmlFor="agency-material-qr-x">横向位置 (%)</FieldLabel>
                    <Input
                      id="agency-material-qr-x"
                      value={String(materialQRConfig.x_percent)}
                      onChange={(event) =>
                        setMaterialQRConfig((current) => ({
                          ...current,
                          x_percent: Math.min(100, Math.max(0, parsePositiveInt(event.target.value))),
                        }))
                      }
                    />
                  </Field>
                  <Field className="gap-1.5">
                    <FieldLabel htmlFor="agency-material-qr-y">纵向位置 (%)</FieldLabel>
                    <Input
                      id="agency-material-qr-y"
                      value={String(materialQRConfig.y_percent)}
                      onChange={(event) =>
                        setMaterialQRConfig((current) => ({
                          ...current,
                          y_percent: Math.min(100, Math.max(0, parsePositiveInt(event.target.value))),
                        }))
                      }
                    />
                  </Field>
                  <Field className="gap-1.5">
                    <FieldLabel htmlFor="agency-material-qr-size">二维码大小 (%)</FieldLabel>
                    <Input
                      id="agency-material-qr-size"
                      value={String(materialQRConfig.size_percent)}
                      onChange={(event) =>
                        setMaterialQRConfig((current) => ({
                          ...current,
                          size_percent: Math.min(100, Math.max(0, parsePositiveInt(event.target.value))),
                        }))
                      }
                    />
                  </Field>
                  <Field className="gap-1.5">
                    <FieldLabel htmlFor="agency-material-qr-logo">Logo比例 (%)</FieldLabel>
                    <Input
                      id="agency-material-qr-logo"
                      value={String(materialQRConfig.logo_percent)}
                      onChange={(event) =>
                        setMaterialQRConfig((current) => ({
                          ...current,
                          logo_percent: Math.min(100, Math.max(0, parsePositiveInt(event.target.value))),
                        }))
                      }
                    />
                  </Field>
                </div>
                <p className="mt-2 text-xs text-muted-foreground">
                  保存后，代理端“推广素材”中的图片会自动叠加专属二维码，二维码中心自动使用后台站点 Logo。
                </p>
              </div>
              <Button onClick={() => void handleSave()} disabled={isSaving}>{isSaving ? <LoaderCircle className="size-4 animate-spin" /> : <Save className="size-4" />}保存推广素材</Button>
            </CardContent>
          </Card>
        </>
      ) : null}
    </div>
  );
}
