import { useEffect } from "react";

import { AnimatedRoutes } from "@/app/animated-routes";
import { TopNav } from "@/components/top-nav";
import { resolveBrandAssetURL } from "@/lib/app-meta";
import { useAppMeta } from "@/lib/use-app-meta";

export function AppShell() {
  const appMeta = useAppMeta();

  useEffect(() => {
    const siteName = (appMeta.project_name || appMeta.app_title || "chatgpt2api").trim() || "chatgpt2api";
    document.title = siteName;

    const iconURL = resolveBrandAssetURL(appMeta.site_logo_url || "/logo-mark.svg") || "/logo-mark.svg";
    let icon = document.querySelector("link[rel='icon']") as HTMLLinkElement | null;
    if (!icon) {
      icon = document.createElement("link");
      icon.setAttribute("rel", "icon");
      document.head.appendChild(icon);
    }
    icon.setAttribute("href", iconURL);
  }, [appMeta.app_title, appMeta.project_name, appMeta.site_logo_url]);

  return (
    <main className="min-h-screen bg-background text-foreground">
      <div className="mx-auto flex min-h-screen w-full max-w-[1660px] flex-col gap-2 px-3 py-3 sm:px-4 lg:px-4">
        <TopNav />
        <AnimatedRoutes />
      </div>
    </main>
  );
}
