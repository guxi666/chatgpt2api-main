"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { ChevronDown, LoaderCircle } from "lucide-react";

import { useAuthGuard } from "@/lib/use-auth-guard";

import { AnnouncementsCard } from "./components/announcements-card";
import { BillingAdminCard } from "./components/billing-admin-card";
import { ConfigCard } from "./components/config-card";
import { CPAPoolDialog } from "./components/cpa-pool-dialog";
import { CPAPoolsCard } from "./components/cpa-pools-card";
import { ImportBrowserDialog } from "./components/import-browser-dialog";
import { LogGovernanceCard } from "./components/log-governance-card";
import { LoginPageImageCard } from "./components/login-page-image-card";
import { PaymentSettingsCard } from "./components/payment-settings-card";
import { ProxySettingsCard } from "./components/proxy-settings-card";
import { R2StorageCard } from "./components/r2-storage-card";
import { SiteBrandCard } from "./components/site-brand-card";
import { SettingsHeader } from "./components/settings-header";
import { Sub2APIConnections } from "./components/sub2api-connections";
import { useSettingsStore } from "./store";

function SettingsDataController() {
  const didLoadRef = useRef(false);
  const initialize = useSettingsStore((state) => state.initialize);

  useEffect(() => {
    if (didLoadRef.current) {
      return;
    }
    didLoadRef.current = true;
    void initialize();
  }, [initialize]);

  return null;
}

function SettingsMasonryItem({ children }: { children: ReactNode }) {
  return <div>{children}</div>;
}

function DeferredSettingsSection({
  title,
  onOpen,
  children,
}: {
  title: string;
  onOpen?: () => void;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="mb-5 break-inside-avoid overflow-hidden rounded-[20px] border border-border/80 bg-white">
      <button
        type="button"
        className="flex w-full items-center justify-between px-5 py-4 text-left"
        onClick={() => {
          const next = !open;
          setOpen(next);
          if (next) {
            onOpen?.();
          }
        }}
      >
        <span className="text-base font-semibold text-foreground">{title}</span>
        <ChevronDown className={open ? "size-4 rotate-180 text-muted-foreground transition" : "size-4 text-muted-foreground transition"} />
      </button>
      {open ? <div className="border-t border-border/70">{children}</div> : null}
    </div>
  );
}

function AdminSettingsPageContent() {
  const loadPools = useSettingsStore((state) => state.loadPools);
  const pools = useSettingsStore((state) => state.pools);
  const loadLogGovernance = useSettingsStore((state) => state.loadLogGovernance);

  useEffect(() => {
    const hasRunningJobs = pools.some((pool) => {
      const status = pool.import_job?.status;
      return status === "pending" || status === "running";
    });
    if (!hasRunningJobs) {
      return;
    }

    const timer = window.setInterval(() => {
      void loadPools(true);
    }, 1500);
    return () => window.clearInterval(timer);
  }, [loadPools, pools]);

  return (
    <div className="mx-auto flex w-full max-w-[1680px] flex-col gap-5 pb-8">
      <SettingsDataController />
      <SettingsHeader />
      <section className="grid grid-cols-1 gap-5 xl:grid-cols-2">
        <SettingsMasonryItem>
          <ConfigCard />
        </SettingsMasonryItem>
        <SettingsMasonryItem>
          <ProxySettingsCard />
        </SettingsMasonryItem>
        <SettingsMasonryItem>
          <SiteBrandCard />
        </SettingsMasonryItem>
        <SettingsMasonryItem>
          <PaymentSettingsCard />
        </SettingsMasonryItem>
        <SettingsMasonryItem>
          <R2StorageCard />
        </SettingsMasonryItem>
        <SettingsMasonryItem>
          <LoginPageImageCard />
        </SettingsMasonryItem>
        <SettingsMasonryItem>
          <AnnouncementsCard />
        </SettingsMasonryItem>
      </section>
      <section className="grid grid-cols-1 gap-5 xl:grid-cols-3">
        <DeferredSettingsSection title="日志数据治理" onOpen={() => void loadLogGovernance()}>
          <LogGovernanceCard />
        </DeferredSettingsSection>
        <DeferredSettingsSection title="CPA 连接管理" onOpen={() => void loadPools()}>
          <CPAPoolsCard />
        </DeferredSettingsSection>
        <DeferredSettingsSection title="Sub2API 连接管理">
          <Sub2APIConnections />
        </DeferredSettingsSection>
      </section>
      <section>
        <BillingAdminCard />
      </section>
      <CPAPoolDialog />
      <ImportBrowserDialog />
    </div>
  );
}

export default function SettingsPage() {
  const { isCheckingAuth, session } = useAuthGuard(undefined, "/settings");

  if (isCheckingAuth || !session) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <LoaderCircle className="size-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return <AdminSettingsPageContent />;
}
