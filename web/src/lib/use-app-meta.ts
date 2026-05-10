"use client";

import { useEffect, useState } from "react";

import {
  APP_META_UPDATED_EVENT,
  defaultAppMeta,
  fetchAppMeta,
  normalizeAppMeta,
  resolveAppAssetSrc,
  type AppMeta,
} from "@/lib/app-meta";

export function useAppMeta() {
  const [appMeta, setAppMeta] = useState<AppMeta>(defaultAppMeta);

  useEffect(() => {
    let active = true;

    const load = async () => {
      try {
        const data = await fetchAppMeta();
        if (active) {
          setAppMeta(data);
        }
      } catch {
        if (active) {
          setAppMeta(defaultAppMeta);
        }
      }
    };

    const handleUpdated = (event: Event) => {
      const detail = event instanceof CustomEvent ? event.detail : {};
      setAppMeta((current) => normalizeAppMeta({ ...current, ...detail }));
    };

    void load();
    window.addEventListener(APP_META_UPDATED_EVENT, handleUpdated);
    return () => {
      active = false;
      window.removeEventListener(APP_META_UPDATED_EVENT, handleUpdated);
    };
  }, []);

  useEffect(() => {
    document.title = appMeta.project_name || appMeta.app_title || "chatgpt2api";
    const iconHref = resolveAppAssetSrc(appMeta.site_icon_url);
    let link = document.querySelector("link[data-chatgpt2api-site-icon]") as HTMLLinkElement | null;
    if (!link) {
      link = document.createElement("link");
      link.rel = "icon";
      link.setAttribute("data-chatgpt2api-site-icon", "true");
      document.head.appendChild(link);
    }
    link.href = iconHref;
  }, [appMeta.app_title, appMeta.project_name, appMeta.site_icon_url]);

  return appMeta;
}
