"use client";

import { useEffect, useState } from "react";

import {
  APP_META_UPDATED_EVENT,
  defaultAppMeta,
  fetchAppMeta,
  normalizeAppMeta,
  type AppMeta,
} from "@/lib/app-meta";

function initialAppMeta(): AppMeta {
  if (typeof document === "undefined") {
    return defaultAppMeta;
  }
  const initialTitle = document.title.trim();
  const initialIcon = document.querySelector<HTMLLinkElement>("link[rel='icon']")?.getAttribute("href")?.trim();
  return normalizeAppMeta({
    ...defaultAppMeta,
    app_title: initialTitle || defaultAppMeta.app_title,
    project_name: initialTitle || defaultAppMeta.project_name,
    site_logo_url: initialIcon || defaultAppMeta.site_logo_url,
  });
}

export function useAppMeta() {
  const [appMeta, setAppMeta] = useState<AppMeta>(() => initialAppMeta());

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
          setAppMeta((current) => current);
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

  return appMeta;
}
