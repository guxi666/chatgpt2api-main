"use client";

import { useEffect, useState } from "react";
import { Moon, Sun } from "lucide-react";
import { Link, NavLink, useLocation, useNavigate } from "react-router-dom";

import { AnnouncementNotifications } from "@/components/announcement-banner";
import webConfig from "@/constants/common-env";
import { clearVerifiedAuthSession, getCachedAuthSession, getVerifiedAuthSession } from "@/lib/session";
import type { StoredAuthSession } from "@/store/auth";
import { Button } from "@/components/ui/button";
import { fetchAccounts, type Account } from "@/lib/api";
import { resolveAppAssetSrc } from "@/lib/app-meta";
import { useAppMeta } from "@/lib/use-app-meta";
import { usePreferredLanguage } from "@/lib/language";
import { cn } from "@/lib/utils";
import {
  applyColorTheme,
  getPreferredColorTheme,
  saveColorTheme,
  type ColorTheme,
} from "@/lib/theme";

type Language = "zh" | "en";
type NavItemKey = "create" | "accounts" | "register" | "images" | "users" | "logs" | "settings" | "wallet";

const navLabels: Record<Language, Record<NavItemKey, string>> = {
  zh: {
    create: "创作台",
    accounts: "账号池",
    register: "注册机",
    images: "图片库",
    users: "用户管理",
    logs: "日志",
    settings: "设置",
    wallet: "钱包充值",
  },
  en: {
    create: "Create",
    accounts: "Accounts",
    register: "Register",
    images: "Images",
    users: "Users",
    logs: "Logs",
    settings: "Settings",
    wallet: "Wallet",
  },
};

const roleLabels: Record<Language, { admin: string; linuxdo: string; email: string; key: string; quota: string; logout: string }> = {
  zh: {
    admin: "管理员",
    linuxdo: "Linuxdo 用户",
    email: "邮箱用户",
    key: "密钥用户",
    quota: "剩余额度",
    logout: "退出",
  },
  en: {
    admin: "Admin",
    linuxdo: "Linuxdo User",
    email: "Email User",
    key: "API User",
    quota: "Remaining",
    logout: "Logout",
  },
};

function buildNavItems(
  language: Language,
  mode: "admin" | "linuxdo" | "email" | "user",
) {
  const labels = navLabels[language];
  if (mode === "admin") {
    return [
      { href: "/image", label: labels.create },
      { href: "/accounts", label: labels.accounts },
      { href: "/register", label: labels.register },
      { href: "/image-manager", label: labels.images },
      { href: "/users", label: labels.users },
      { href: "/logs", label: labels.logs },
      { href: "/settings", label: labels.settings },
    ];
  }
  return [
    { href: "/image", label: labels.create },
    { href: "/wallet", label: labels.wallet },
    { href: "/image-manager", label: labels.images },
  ];
}

const QUOTA_REFRESH_EVENT = "chatgpt2api:quota-refresh";

function formatAvailableQuota(accounts: Account[]) {
  const availableAccounts = accounts.filter((account) => account.status !== "禁用");
  return String(availableAccounts.reduce((sum, account) => sum + Math.max(0, account.quota), 0));
}

function ThemeToggleButton({
  theme,
  onToggle,
  className,
}: {
  theme: ColorTheme;
  onToggle: () => void;
  className?: string;
}) {
  const dark = theme === "dark";

  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className={cn("size-8 rounded-full", className)}
      onClick={onToggle}
      aria-label={dark ? "切换到浅色模式" : "切换到深色模式"}
      title={dark ? "浅色模式" : "深色模式"}
    >
      {dark ? <Sun /> : <Moon />}
    </Button>
  );
}

export function TopNav() {
  const location = useLocation();
  const navigate = useNavigate();
  const appMeta = useAppMeta();
  const { language, setLanguage } = usePreferredLanguage();
  const pathname = location.pathname.replace(/\/+$/, "") || "/";
  const [session, setSession] = useState<StoredAuthSession | null | undefined>(() => getCachedAuthSession());
  const [theme, setTheme] = useState<ColorTheme>(() => getPreferredColorTheme());
  const [availableQuota, setAvailableQuota] = useState("--");

  useEffect(() => {
    applyColorTheme(theme);
  }, [theme]);

  useEffect(() => {
    let active = true;

    const load = async () => {
      if (pathname === "/login") {
        if (!active) {
          return;
        }
        setSession(null);
        return;
      }

      const storedSession = await getVerifiedAuthSession();
      if (!active) {
        return;
      }
      setSession(storedSession);
    };

    void load();
    return () => {
      active = false;
    };
  }, [pathname]);

  useEffect(() => {
    if (!session || session.role !== "admin") {
      setAvailableQuota("--");
      return;
    }

    let active = true;
    const loadQuota = async () => {
      try {
        const data = await fetchAccounts();
        if (active) {
          setAvailableQuota(formatAvailableQuota(data.items));
        }
      } catch {
        if (active) {
          setAvailableQuota((current) => (current === "加载中..." ? "--" : current));
        }
      }
    };
    const handleRefresh = () => {
      void loadQuota();
    };

    setAvailableQuota("加载中...");
    void loadQuota();
    window.addEventListener("focus", handleRefresh);
    window.addEventListener(QUOTA_REFRESH_EVENT, handleRefresh);
    return () => {
      active = false;
      window.removeEventListener("focus", handleRefresh);
      window.removeEventListener(QUOTA_REFRESH_EVENT, handleRefresh);
    };
  }, [session]);

  const handleLogout = async () => {
    await clearVerifiedAuthSession();
    navigate("/login", { replace: true });
  };

  const handleThemeToggle = () => {
    setTheme((currentTheme) => {
      const nextTheme = currentTheme === "dark" ? "light" : "dark";
      applyColorTheme(nextTheme);
      saveColorTheme(nextTheme);
      return nextTheme;
    });
  };

  if (pathname === "/login" || pathname === "/auth/linuxdo/callback" || session === undefined || !session) {
    return null;
  }

  const roleText = roleLabels[language];
  const navItems = session.role === "admin"
    ? buildNavItems(language, "admin")
    : session.provider === "linuxdo"
      ? buildNavItems(language, "linuxdo")
      : session.provider === "email"
        ? buildNavItems(language, "email")
        : buildNavItems(language, "user");
  const roleLabel = session.role === "admin"
    ? roleText.admin
    : session.provider === "linuxdo"
      ? roleText.linuxdo
      : session.provider === "email"
        ? roleText.email
        : roleText.key;

  return (
    <header className="sticky top-3 z-40 rounded-[24px] border border-border bg-card/90 shadow-[0_0_22.576px_rgba(44,74,116,0.09)] backdrop-blur dark:border-border dark:bg-card/92">
      <div className="flex min-h-14 flex-col gap-2 px-3 py-2 sm:flex-row sm:items-center sm:justify-between sm:gap-4 sm:px-4">
        <div className="flex items-center justify-between gap-3 sm:justify-start">
          <Link
            to="/image"
            className="font-display inline-flex shrink-0 items-center gap-2 py-1 text-[15px] font-semibold text-[#18181b] transition hover:text-[#1456f0] dark:text-foreground dark:hover:text-sky-300"
          >
            <img
              src={resolveAppAssetSrc(appMeta.app_logo_url)}
              alt=""
              aria-hidden="true"
              className="size-7 rounded-[10px] shadow-[0_4px_10px_rgba(184,90,127,0.16)]"
            />
            {appMeta.app_title || "chatgpt2api"}
          </Link>
          <div className="ml-auto flex shrink-0 items-center gap-1 sm:hidden">
            <AnnouncementNotifications target="image" className="size-8" />
            <ThemeToggleButton theme={theme} onToggle={handleThemeToggle} />
            <button
              type="button"
              className="rounded-full px-3 py-1 text-xs font-medium text-[#45515e] transition hover:bg-black/[0.05] hover:text-[#18181b] dark:text-muted-foreground dark:hover:bg-accent dark:hover:text-accent-foreground"
              onClick={() => void handleLogout()}
            >
              {roleText.logout}
            </button>
          </div>
        </div>
        <nav className="hide-scrollbar -mx-1 flex min-w-0 flex-1 gap-1 overflow-x-auto px-1 sm:mx-0 sm:justify-center sm:gap-1.5 sm:overflow-visible sm:px-0">
          {navItems.map((item) => {
            const active = pathname === item.href;
            return (
              <NavLink
                key={item.href}
                to={item.href}
                className={() =>
                  cn(
                    "relative shrink-0 whitespace-nowrap rounded-full px-3 py-1.5 text-[13px] font-medium transition sm:text-sm",
                    active
                      ? "bg-black/[0.06] text-[#18181b] dark:bg-accent dark:text-accent-foreground"
                      : "text-[#45515e] hover:bg-black/[0.05] hover:text-[#18181b] dark:text-muted-foreground dark:hover:bg-accent dark:hover:text-accent-foreground",
                  )
                }
              >
                {item.label}
              </NavLink>
            );
          })}
        </nav>
        <div className="hidden items-center justify-end gap-2 sm:flex sm:gap-3">
          <div className="inline-flex items-center rounded-full border border-border bg-white p-0.5 text-xs dark:bg-input/30">
            <button
              type="button"
              className={cn(
                "rounded-full px-2.5 py-1 font-medium transition",
                language === "zh" ? "bg-[#181e25] text-white" : "text-muted-foreground hover:text-foreground",
              )}
              onClick={() => setLanguage("zh")}
            >
              中文
            </button>
            <button
              type="button"
              className={cn(
                "rounded-full px-2.5 py-1 font-medium transition",
                language === "en" ? "bg-[#181e25] text-white" : "text-muted-foreground hover:text-foreground",
              )}
              onClick={() => setLanguage("en")}
            >
              EN
            </button>
          </div>
          <AnnouncementNotifications target="image" className="size-8" />
          <ThemeToggleButton theme={theme} onToggle={handleThemeToggle} />
          <span className="hidden rounded-full bg-[#f0f0f0] px-2.5 py-1 text-[11px] font-medium text-[#45515e] sm:inline-block dark:bg-secondary dark:text-secondary-foreground">
            {roleLabel}
          </span>
          {session.role === "admin" ? (
            <span className="hidden rounded-full bg-[#f0f0f0] px-2.5 py-1 text-[11px] font-medium text-[#45515e] sm:inline-block dark:bg-secondary dark:text-secondary-foreground">
              {roleText.quota} {availableQuota}
            </span>
          ) : null}
          <span className="hidden rounded-full bg-[#f0f0f0] px-2.5 py-1 text-[11px] font-medium text-[#45515e] sm:inline-block dark:bg-secondary dark:text-secondary-foreground">
            v{webConfig.appVersion}
          </span>
          <button
            type="button"
            className="rounded-full px-3 py-1 text-sm text-[#45515e] transition hover:bg-black/[0.05] hover:text-[#18181b] dark:text-muted-foreground dark:hover:bg-accent dark:hover:text-accent-foreground"
            onClick={() => void handleLogout()}
          >
            {roleText.logout}
          </button>
        </div>
      </div>
    </header>
  );
}
