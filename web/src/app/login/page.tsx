"use client";

import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import {
  ArrowRight,
  KeyRound,
  LoaderCircle,
  MoonStar,
  ShieldCheck,
  Sun,
  UserPlus,
  UserRound,
} from "lucide-react";
import { toast } from "sonner";

import { AnnouncementNotifications } from "@/components/announcement-banner";
import { LoginPageImageStage } from "@/components/login-page-image-stage";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  fetchAuthProviders,
  login,
  registerAccount,
  resetPasswordByEmail,
  sendPasswordResetCode,
  sendRegisterCode,
} from "@/lib/api";
import { resolveBrandAssetURL } from "@/lib/app-meta";
import { setVerifiedAuthSession } from "@/lib/session";
import {
  applyColorTheme,
  getPreferredColorTheme,
  saveColorTheme,
  type ColorTheme,
} from "@/lib/theme";
import { useAppMeta } from "@/lib/use-app-meta";
import { useRedirectIfAuthenticated } from "@/lib/use-auth-guard";
import { getDefaultRouteForSession } from "@/store/auth";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";

const loginBackgroundClass =
  "bg-[#fff9fb] bg-[radial-gradient(rgba(20,86,240,0.12)_1px,transparent_1px),linear-gradient(145deg,#fff8fa_0%,#ffffff_48%,#f4f8ff_100%)] [background-position:0_0,center] [background-size:12px_12px,cover] dark:bg-[#090d16] dark:bg-[radial-gradient(rgba(96,165,250,0.16)_1px,transparent_1px),linear-gradient(145deg,#080b13_0%,#101827_52%,#070b12_100%)]";
const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const REGISTER_DEVICE_ID_STORAGE_KEY = "chatgpt2api:register_device_id";

type LoginMethod = "password" | "key";

type TurnstileApi = {
  render: (container: HTMLElement, options: {
    sitekey: string;
    callback?: (token: string) => void;
    "expired-callback"?: () => void;
    "error-callback"?: () => void;
  }) => string;
  reset: (widgetId?: string) => void;
  remove: (widgetId?: string) => void;
};

declare global {
  interface Window {
    turnstile?: TurnstileApi;
  }
}

function getOrCreateRegisterDeviceID() {
  if (typeof window === "undefined") {
    return "";
  }
  const existing = String(window.localStorage.getItem(REGISTER_DEVICE_ID_STORAGE_KEY) || "").trim();
  if (existing) {
    return existing;
  }
  const next =
    typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
      ? crypto.randomUUID()
      : `dev-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  window.localStorage.setItem(REGISTER_DEVICE_ID_STORAGE_KEY, next);
  return next;
}

export default function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const appMeta = useAppMeta();
  const brandLogoURL = resolveBrandAssetURL(appMeta.top_left_logo_url || "/logo-mark.svg") || "/logo-mark.svg";
  const themeToggleRef = useRef<HTMLButtonElement | null>(null);
  const turnstileRef = useRef<HTMLDivElement | null>(null);
  const turnstileWidgetIDRef = useRef<string | null>(null);

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [registerCode, setRegisterCode] = useState("");
  const [inviteCode, setInviteCode] = useState("");
  const [accessKey, setAccessKey] = useState("");

  const [isRegisterMode, setIsRegisterMode] = useState(false);
  const [loginMethod, setLoginMethod] = useState<LoginMethod>("password");

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSendingCode, setIsSendingCode] = useState(false);
  const [sendCooldown, setSendCooldown] = useState(0);
  const [registrationEnabled, setRegistrationEnabled] = useState(false);
  const [emailVerificationEnabled, setEmailVerificationEnabled] = useState(false);
  const [passwordRecoveryEnabled, setPasswordRecoveryEnabled] = useState(false);
  const [keyLoginEnabled, setKeyLoginEnabled] = useState(true);

  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState("");
  const [turnstileToken, setTurnstileToken] = useState("");
  const [isForgotDialogOpen, setIsForgotDialogOpen] = useState(false);
  const [forgotEmail, setForgotEmail] = useState("");
  const [forgotCode, setForgotCode] = useState("");
  const [forgotPassword, setForgotPassword] = useState("");
  const [isSendingResetCode, setIsSendingResetCode] = useState(false);
  const [isResettingPassword, setIsResettingPassword] = useState(false);
  const [resetCooldown, setResetCooldown] = useState(0);

  const [theme, setTheme] = useState<ColorTheme>(() => getPreferredColorTheme());
  const { isCheckingAuth } = useRedirectIfAuthenticated();

  useEffect(() => {
    const params = new URLSearchParams(location.search);
    const invite = String(params.get("invite_code") || params.get("ref") || "").trim();
    if (invite) {
      setInviteCode(invite);
      setIsRegisterMode(true);
      setLoginMethod("password");
    }
  }, [location.search]);

  useEffect(() => {
    let active = true;
    const loadProviders = async () => {
      try {
        const providers = await fetchAuthProviders();
        if (active) {
          setRegistrationEnabled(Boolean(providers.registration?.enabled));
          setEmailVerificationEnabled(Boolean(providers.email_verification?.enabled));
          setPasswordRecoveryEnabled(Boolean(providers.password_recovery?.enabled ?? providers.email_verification?.enabled));
          setKeyLoginEnabled(Boolean(providers.key_login?.enabled ?? true));
          setTurnstileEnabled(Boolean(providers.turnstile?.enabled));
          setTurnstileSiteKey(String(providers.turnstile?.site_key || "").trim());
        }
      } catch {
        if (active) {
          setRegistrationEnabled(false);
          setEmailVerificationEnabled(false);
          setPasswordRecoveryEnabled(false);
          setKeyLoginEnabled(true);
          setTurnstileEnabled(false);
          setTurnstileSiteKey("");
        }
      }
    };
    void loadProviders();
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!turnstileEnabled || !turnstileSiteKey || !turnstileRef.current) {
      return;
    }

    const mount = () => {
      if (!window.turnstile || !turnstileRef.current) {
        return;
      }
      if (turnstileWidgetIDRef.current) {
        window.turnstile.remove(turnstileWidgetIDRef.current);
      }
      turnstileWidgetIDRef.current = window.turnstile.render(turnstileRef.current, {
        sitekey: turnstileSiteKey,
        callback: (token) => setTurnstileToken(token),
        "expired-callback": () => setTurnstileToken(""),
        "error-callback": () => setTurnstileToken(""),
      });
    };

    if (window.turnstile) {
      mount();
      return;
    }

    const scriptID = "cf-turnstile-script";
    const existing = document.getElementById(scriptID) as HTMLScriptElement | null;
    if (existing) {
      existing.addEventListener("load", mount, { once: true });
      return () => existing.removeEventListener("load", mount);
    }

    const script = document.createElement("script");
    script.id = scriptID;
    script.src = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";
    script.async = true;
    script.defer = true;
    script.addEventListener("load", mount, { once: true });
    document.head.appendChild(script);
    return () => script.removeEventListener("load", mount);
  }, [turnstileEnabled, turnstileSiteKey, isRegisterMode, loginMethod]);

  useEffect(() => {
    if (sendCooldown <= 0) {
      return;
    }
    const timer = window.setTimeout(() => {
      setSendCooldown((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [sendCooldown]);

  useEffect(() => {
    if (resetCooldown <= 0) {
      return;
    }
    const timer = window.setTimeout(() => {
      setResetCooldown((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => window.clearTimeout(timer);
  }, [resetCooldown]);

  const resetTurnstile = () => {
    setTurnstileToken("");
    if (turnstileWidgetIDRef.current && window.turnstile) {
      window.turnstile.reset(turnstileWidgetIDRef.current);
    }
  };

  const handleSendCode = async () => {
    const email = username.trim();
    if (!emailPattern.test(email)) {
      toast.error("请输入有效邮箱地址");
      return;
    }
    if (turnstileEnabled && !turnstileToken) {
      toast.error("请先完成 Cloudflare 验证");
      return;
    }
    if (!emailVerificationEnabled) {
      toast.error("当前未启用邮箱验证码，请联系管理员配置 SMTP");
      return;
    }
    setIsSendingCode(true);
    try {
      await sendRegisterCode(email, turnstileToken);
      setSendCooldown(60);
      toast.success("验证码已发送，请查收邮箱");
    } catch (error) {
      const message = error instanceof Error ? error.message : "发送验证码失败";
      toast.error(message);
    } finally {
      setIsSendingCode(false);
      if (turnstileEnabled) {
        resetTurnstile();
      }
    }
  };

  const handleSubmit = async () => {
    const normalizedUsername = username.trim();
    const normalizedName = displayName.trim();
    const normalizedKey = accessKey.trim();

    if (turnstileEnabled && !turnstileToken) {
      toast.error("请先完成 Cloudflare 验证");
      return;
    }

    if (isRegisterMode) {
      if (!emailPattern.test(normalizedUsername)) {
        toast.error("请输入有效邮箱地址");
        return;
      }
      if (!password) {
        toast.error("请输入密码");
        return;
      }
      if (!emailVerificationEnabled) {
        toast.error("当前未启用邮箱验证码，请联系管理员配置 SMTP");
        return;
      }
      if (!registerCode.trim()) {
        toast.error("请输入邮箱验证码");
        return;
      }
    } else if (loginMethod === "key") {
      if (!normalizedKey) {
        toast.error("请输入密钥");
        return;
      }
    } else {
      if (!normalizedUsername) {
        toast.error("请输入账号或邮箱");
        return;
      }
      if (!password) {
        toast.error("请输入密码");
        return;
      }
    }

    setIsSubmitting(true);
    try {
      const data = isRegisterMode
        ? await registerAccount(
            normalizedUsername,
            password,
            registerCode.trim(),
            normalizedName,
            turnstileToken,
            inviteCode.trim(),
            getOrCreateRegisterDeviceID(),
          )
        : loginMethod === "key"
          ? await login({ key: normalizedKey, cfTurnstileToken: turnstileToken })
          : await login({
              ...(normalizedUsername.includes("@") ? { email: normalizedUsername } : { username: normalizedUsername }),
              password,
              cfTurnstileToken: turnstileToken,
            });

      const token = String(data.token || "").trim();
      if (!token) {
        throw new Error("登录会话签发失败");
      }

      const session = {
        key: token,
        role: data.role,
        roleId: data.role_id,
        roleName: data.role_name,
        subjectId: data.subject_id,
        name: data.name,
        provider: data.provider,
        menuPaths: data.menu_paths || [],
        apiPermissions: data.api_permissions || [],
        menus: data.menus || [],
      };

      await setVerifiedAuthSession(session);
      toast.success(isRegisterMode ? "注册成功" : "登录成功");
      navigate(getDefaultRouteForSession(session), { replace: true });
    } catch (error) {
      const message = error instanceof Error ? error.message : isRegisterMode ? "注册失败" : "登录失败";
      toast.error(message);
    } finally {
      if (turnstileEnabled) {
        resetTurnstile();
      }
      setIsSubmitting(false);
    }
  };

  const handleSendResetCode = async () => {
    const email = forgotEmail.trim();
    if (!emailPattern.test(email)) {
      toast.error("请输入有效邮箱地址");
      return;
    }
    if (turnstileEnabled && !turnstileToken) {
      toast.error("请先完成 Cloudflare 验证");
      return;
    }
    if (!passwordRecoveryEnabled) {
      toast.error("当前未启用找回密码，请联系管理员配置 SMTP");
      return;
    }
    setIsSendingResetCode(true);
    try {
      await sendPasswordResetCode(email, turnstileToken);
      setResetCooldown(60);
      toast.success("重置验证码已发送，请查收邮箱");
    } catch (error) {
      const message = error instanceof Error ? error.message : "发送验证码失败";
      toast.error(message);
    } finally {
      setIsSendingResetCode(false);
      if (turnstileEnabled) {
        resetTurnstile();
      }
    }
  };

  const handleResetPassword = async () => {
    const email = forgotEmail.trim();
    if (!emailPattern.test(email)) {
      toast.error("请输入有效邮箱地址");
      return;
    }
    if (forgotCode.trim() === "") {
      toast.error("请输入邮箱验证码");
      return;
    }
    if (forgotPassword.trim().length < 8) {
      toast.error("新密码至少 8 位");
      return;
    }
    if (turnstileEnabled && !turnstileToken) {
      toast.error("请先完成 Cloudflare 验证");
      return;
    }
    setIsResettingPassword(true);
    try {
      await resetPasswordByEmail(email, forgotPassword, forgotCode.trim(), turnstileToken);
      toast.success("密码重置成功，请使用新密码登录");
      setUsername(email);
      setPassword("");
      setIsForgotDialogOpen(false);
      setForgotCode("");
      setForgotPassword("");
      setLoginMethod("password");
      setIsRegisterMode(false);
    } catch (error) {
      const message = error instanceof Error ? error.message : "重置密码失败";
      toast.error(message);
    } finally {
      setIsResettingPassword(false);
      if (turnstileEnabled) {
        resetTurnstile();
      }
    }
  };

  const handleThemeToggle = () => {
    const nextTheme = theme === "dark" ? "light" : "dark";
    const rect = themeToggleRef.current?.getBoundingClientRect();
    applyColorTheme(nextTheme, rect ? {
      origin: {
        x: rect.left + rect.width / 2,
        y: rect.top + rect.height / 2,
      },
    } : undefined);
    saveColorTheme(nextTheme);
    setTheme(nextTheme);
  };

  if (isCheckingAuth) {
    return (
      <div
        className={`${loginBackgroundClass} fixed inset-0 z-50 grid min-h-svh w-screen place-items-center overflow-hidden px-4 py-6`}
      >
        <LoaderCircle className="size-5 animate-spin text-[#45515e] dark:text-white/60" />
      </div>
    );
  }

  const identifierLabel = isRegisterMode ? "邮箱" : "账号 / 邮箱";

  return (
    <div
      className={`${loginBackgroundClass} fixed inset-0 z-50 flex min-h-svh w-screen items-center justify-center overflow-y-auto px-4 py-6 font-login [align-items:safe_center] sm:px-6 lg:px-8`}
    >
      <div className="fixed right-4 top-4 z-50 flex items-center gap-2 sm:right-6 sm:top-6">
        <AnnouncementNotifications target="login" className="size-9" />
        <Button
          ref={themeToggleRef}
          type="button"
          variant="outline"
          size="icon"
          className="relative rounded-full border-border/60 bg-background/80 shadow-sm backdrop-blur"
          onClick={handleThemeToggle}
          aria-label={theme === "dark" ? "切换到浅色模式" : "切换到深色模式"}
          title={theme === "dark" ? "浅色模式" : "深色模式"}
        >
          <Sun className="scale-100 rotate-0 transition-all dark:scale-0 dark:-rotate-90" />
          <MoonStar className="absolute scale-0 rotate-90 transition-all dark:scale-100 dark:rotate-0" />
          <span className="sr-only">切换界面主题</span>
        </Button>
      </div>

      <div className="relative z-10 grid w-full max-w-[58rem] overflow-hidden rounded-[32px] border border-white/80 bg-white/95 shadow-[0_28px_80px_rgba(15,23,42,0.12),0_10px_28px_rgba(44,30,116,0.08)] backdrop-blur transition-[min-height] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:transition-none dark:border-white/10 dark:bg-[#111827]/92 dark:shadow-[0_30px_90px_rgba(2,6,23,0.58),0_12px_32px_rgba(2,6,23,0.32)] lg:min-h-[39rem] lg:grid-cols-[minmax(0,28rem)_minmax(0,1fr)]">
        <section className="flex min-h-[500px] flex-col justify-center px-6 py-8 sm:px-10 lg:px-12">
          <div className="flex flex-col gap-7">
            <div className="flex items-center gap-3">
              <img
                src={brandLogoURL}
                alt=""
                aria-hidden="true"
                className="size-11 rounded-[16px] shadow-[0_12px_16px_-4px_rgba(36,36,36,0.12)]"
              />
              <div className="grid min-w-0 leading-none">
                <div className="truncate text-sm font-semibold tracking-[-0.02em] text-[#222222] dark:text-white">
                  {appMeta.app_title || "chatgpt2api"}
                </div>
                <div className="truncate text-[10px] font-medium tracking-[0.28em] text-[#8e8e93] uppercase dark:text-white/50">
                  {appMeta.project_name && appMeta.project_name !== appMeta.app_title ? appMeta.project_name : "Control Center"}
                </div>
              </div>
            </div>

            <div className="flex flex-col gap-4">
              <div className="inline-flex w-fit items-center gap-2 rounded-full border border-[#dfe7f1] bg-white/80 px-3 py-1 text-[11px] font-semibold tracking-[0.2em] text-[#45515e] uppercase shadow-[0_4px_12px_rgba(24,40,72,0.05)] dark:border-white/10 dark:bg-white/8 dark:text-white/70 dark:shadow-[0_10px_26px_rgba(2,6,23,0.22)]">
                <ShieldCheck className="size-3.5 text-[#1456f0] dark:text-sky-300" />
                Secure Access
              </div>
              <h1 className="text-[2.1rem] leading-[1.12] font-semibold tracking-[-0.04em] text-[#222222] dark:text-white sm:text-[2.5rem]">
                {isRegisterMode ? "邮箱注册" : "欢迎回来"}
              </h1>
            </div>

            {!isRegisterMode && keyLoginEnabled ? (
              <div className="grid grid-cols-2 gap-2 rounded-2xl border border-border/60 bg-muted/20 p-1">
                <Button
                  type="button"
                  variant={loginMethod === "password" ? "default" : "ghost"}
                  className="h-9 rounded-xl"
                  onClick={() => setLoginMethod("password")}
                  disabled={isSubmitting}
                >
                  密码登录
                </Button>
                <Button
                  type="button"
                  variant={loginMethod === "key" ? "default" : "ghost"}
                  className="h-9 rounded-xl"
                  onClick={() => setLoginMethod("key")}
                  disabled={isSubmitting}
                >
                  密钥登录
                </Button>
              </div>
            ) : null}

            <form
              className="flex flex-col gap-4"
              onSubmit={(event) => {
                event.preventDefault();
                void handleSubmit();
              }}
            >
              {(!isRegisterMode && loginMethod === "key") ? (
                <div className="flex flex-col gap-2">
                  <label htmlFor="login-key" className="block text-sm font-semibold text-[#222222] dark:text-white/88">
                    密钥
                  </label>
                  <div className="relative">
                    <KeyRound className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-[#8e8e93] dark:text-white/42" />
                    <Input
                      id="login-key"
                      type="text"
                      value={accessKey}
                      onChange={(event) => setAccessKey(event.target.value)}
                      placeholder="sk-..."
                      className="h-12 rounded-[16px] bg-white/90 pl-10 shadow-[0_6px_18px_rgba(24,40,72,0.05)] dark:border-white/12 dark:bg-white/8 dark:text-white dark:placeholder:text-white/38"
                    />
                  </div>
                </div>
              ) : (
                <>
                  <div className="flex flex-col gap-2">
                    <label htmlFor="login-username" className="block text-sm font-semibold text-[#222222] dark:text-white/88">
                      {identifierLabel}
                    </label>
                    <div className="relative">
                      <UserRound className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-[#8e8e93] dark:text-white/42" />
                      <Input
                        id="login-username"
                        type="text"
                        autoComplete={isRegisterMode ? "email" : "username"}
                        value={username}
                        onChange={(event) => setUsername(event.target.value)}
                        placeholder={isRegisterMode ? "name@company.com" : "admin 或 name@company.com"}
                        className="h-12 rounded-[16px] bg-white/90 pl-10 shadow-[0_6px_18px_rgba(24,40,72,0.05)] dark:border-white/12 dark:bg-white/8 dark:text-white dark:placeholder:text-white/38"
                      />
                    </div>
                  </div>

                  {isRegisterMode ? (
                    <div className="flex flex-col gap-2">
                      <label htmlFor="login-display-name" className="block text-sm font-semibold text-[#222222] dark:text-white/88">
                        昵称（可选）
                      </label>
                      <div className="relative">
                        <UserPlus className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-[#8e8e93] dark:text-white/42" />
                        <Input
                          id="login-display-name"
                          type="text"
                          autoComplete="nickname"
                          value={displayName}
                          onChange={(event) => setDisplayName(event.target.value)}
                          placeholder="显示名称"
                          className="h-12 rounded-[16px] bg-white/90 pl-10 shadow-[0_6px_18px_rgba(24,40,72,0.05)] dark:border-white/12 dark:bg-white/8 dark:text-white dark:placeholder:text-white/38"
                        />
                      </div>
                    </div>
                  ) : null}

                  {isRegisterMode ? (
                    <div className="flex flex-col gap-2">
                      <label htmlFor="register-code" className="block text-sm font-semibold text-[#222222] dark:text-white/88">
                        邮箱验证码
                      </label>
                      <div className="flex gap-2">
                        <Input
                          id="register-code"
                          type="text"
                          inputMode="numeric"
                          value={registerCode}
                          onChange={(event) => setRegisterCode(event.target.value)}
                          placeholder="请输入 6 位验证码"
                          className="h-12 rounded-[16px] bg-white/90 shadow-[0_6px_18px_rgba(24,40,72,0.05)] dark:border-white/12 dark:bg-white/8 dark:text-white dark:placeholder:text-white/38"
                        />
                        <Button
                          type="button"
                          variant="outline"
                          className="h-12 rounded-[16px] px-4"
                          disabled={isSubmitting || isSendingCode || sendCooldown > 0}
                          onClick={() => {
                            void handleSendCode();
                          }}
                        >
                          {isSendingCode ? "发送中..." : sendCooldown > 0 ? `${sendCooldown}s` : "发送验证码"}
                        </Button>
                      </div>
                    </div>
                  ) : null}

                  {isRegisterMode ? (
                    <div className="flex flex-col gap-2">
                      <label htmlFor="register-invite-code" className="block text-sm font-semibold text-[#222222] dark:text-white/88">
                        邀请码（可选）
                      </label>
                      <Input
                        id="register-invite-code"
                        type="text"
                        value={inviteCode}
                        onChange={(event) => setInviteCode(event.target.value)}
                        placeholder="输入邀请码可获赠奖励"
                        className="h-12 rounded-[16px] bg-white/90 shadow-[0_6px_18px_rgba(24,40,72,0.05)] dark:border-white/12 dark:bg-white/8 dark:text-white dark:placeholder:text-white/38"
                      />
                    </div>
                  ) : null}

                  <div className="flex flex-col gap-2">
                    <label htmlFor="login-password" className="block text-sm font-semibold text-[#222222] dark:text-white/88">
                      密码
                    </label>
                    <div className="relative">
                      <KeyRound className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-[#8e8e93] dark:text-white/42" />
                      <Input
                        id="login-password"
                        type="password"
                        autoComplete={isRegisterMode ? "new-password" : "current-password"}
                        value={password}
                        onChange={(event) => setPassword(event.target.value)}
                        placeholder={isRegisterMode ? "至少 8 位" : "请输入密码"}
                        className="h-12 rounded-[16px] bg-white/90 pl-10 shadow-[0_6px_18px_rgba(24,40,72,0.05)] dark:border-white/12 dark:bg-white/8 dark:text-white dark:placeholder:text-white/38"
                      />
                    </div>
                  </div>
                </>
              )}

              {turnstileEnabled && turnstileSiteKey ? (
                <div className="rounded-2xl border border-border/60 bg-muted/15 px-3 py-3">
                  <div ref={turnstileRef} className="min-h-[65px]" />
                </div>
              ) : null}

              <div className="flex flex-col gap-3 pt-1">
                <Button
                  type="submit"
                  variant="outline"
                  className="relative mx-auto h-12 w-[88%] overflow-hidden rounded-[1.45rem] border-slate-300/85 bg-white/72 text-[#18181b]"
                  disabled={isSubmitting}
                >
                  <span className="relative z-10 flex items-center gap-2 font-semibold tracking-[-0.01em]">
                    {isSubmitting ? (
                      <LoaderCircle className="size-4 animate-spin" />
                    ) : (
                      <ArrowRight className="size-4" />
                    )}
                    {isRegisterMode ? "注册并进入" : "登录控制台"}
                  </span>
                </Button>

                {registrationEnabled ? (
                  <Button
                    type="button"
                    variant="ghost"
                    className="mx-auto h-10 w-[88%] rounded-[1.2rem] text-[#45515e] hover:bg-black/5 hover:text-[#18181b] dark:text-white/62 dark:hover:bg-white/8 dark:hover:text-white"
                    onClick={() => {
                      setIsRegisterMode((value) => {
                        const next = !value;
                        if (next) {
                          setLoginMethod("password");
                        }
                        return next;
                      });
                    }}
                    disabled={isSubmitting}
                  >
                    <span>{isRegisterMode ? "已有账号，返回登录" : "没有账号，去邮箱注册"}</span>
                  </Button>
                ) : null}
                {!isRegisterMode && loginMethod === "password" ? (
                  <Button
                    type="button"
                    variant="ghost"
                    className="mx-auto h-8 w-[88%] rounded-[1rem] text-xs text-[#45515e] hover:bg-black/5 hover:text-[#18181b] dark:text-white/62 dark:hover:bg-white/8 dark:hover:text-white"
                    onClick={() => {
                      setForgotEmail(username.includes("@") ? username : "");
                      setForgotCode("");
                      setForgotPassword("");
                      setIsForgotDialogOpen(true);
                    }}
                    disabled={isSubmitting}
                  >
                    找回密码
                  </Button>
                ) : null}
              </div>
            </form>
          </div>
        </section>

        <section className="relative hidden overflow-hidden border-l border-[#e5e7eb] bg-[#f8fafc] dark:border-white/10 dark:bg-[#0c1320] lg:flex">
          <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(255,255,255,0.48),transparent_38%)] dark:bg-[linear-gradient(180deg,rgba(255,255,255,0.04),transparent_38%)]" />
          <div className="relative flex flex-1 items-stretch justify-stretch">
            <LoginPageImageStage
              src={appMeta.login_page_image_url}
              mode={appMeta.login_page_image_mode}
              zoom={appMeta.login_page_image_zoom}
              positionX={appMeta.login_page_image_position_x}
              positionY={appMeta.login_page_image_position_y}
              fillParent
              frameClassName="rounded-none"
              imageClassName="rounded-none"
            />
          </div>
        </section>
      </div>

      <Dialog open={isForgotDialogOpen} onOpenChange={setIsForgotDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>找回密码</DialogTitle>
            <DialogDescription>输入注册邮箱、验证码和新密码即可重置账号密码。</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1.5">
              <label htmlFor="forgot-email" className="text-sm font-medium">注册邮箱</label>
              <Input
                id="forgot-email"
                type="email"
                value={forgotEmail}
                onChange={(event) => setForgotEmail(event.target.value)}
                placeholder="name@company.com"
              />
            </div>
            <div className="space-y-1.5">
              <label htmlFor="forgot-code" className="text-sm font-medium">邮箱验证码</label>
              <div className="flex gap-2">
                <Input
                  id="forgot-code"
                  type="text"
                  inputMode="numeric"
                  value={forgotCode}
                  onChange={(event) => setForgotCode(event.target.value)}
                  placeholder="请输入 6 位验证码"
                />
                <Button
                  type="button"
                  variant="outline"
                  className="shrink-0"
                  onClick={() => void handleSendResetCode()}
                  disabled={isSendingResetCode || resetCooldown > 0 || isResettingPassword}
                >
                  {isSendingResetCode ? "发送中..." : resetCooldown > 0 ? `${resetCooldown}s` : "发送验证码"}
                </Button>
              </div>
            </div>
            <div className="space-y-1.5">
              <label htmlFor="forgot-password" className="text-sm font-medium">新密码</label>
              <Input
                id="forgot-password"
                type="password"
                value={forgotPassword}
                onChange={(event) => setForgotPassword(event.target.value)}
                placeholder="至少 8 位"
              />
            </div>
            <Button
              type="button"
              className="w-full"
              onClick={() => void handleResetPassword()}
              disabled={isResettingPassword}
            >
              {isResettingPassword ? <LoaderCircle className="size-4 animate-spin" /> : null}
              重置密码
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
