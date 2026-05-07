"use client";

import { useEffect, useState } from "react";

export type AppLanguage = "zh" | "en";

const STORAGE_KEY = "chatgpt2api:lang";
const LANGUAGE_UPDATED_EVENT = "chatgpt2api:language-updated";

function normalizeLanguage(value: unknown): AppLanguage {
  return value === "en" ? "en" : "zh";
}

export function getPreferredLanguage(): AppLanguage {
  if (typeof window === "undefined") {
    return "zh";
  }
  const saved = window.localStorage.getItem(STORAGE_KEY);
  if (saved) {
    return normalizeLanguage(saved);
  }
  return "zh";
}

export function savePreferredLanguage(language: AppLanguage) {
  if (typeof window === "undefined") {
    return;
  }
  const normalized = normalizeLanguage(language);
  window.localStorage.setItem(STORAGE_KEY, normalized);
  window.dispatchEvent(new CustomEvent(LANGUAGE_UPDATED_EVENT, { detail: { language: normalized } }));
}

export function usePreferredLanguage() {
  const [language, setLanguage] = useState<AppLanguage>(getPreferredLanguage);

  useEffect(() => {
    const sync = () => {
      setLanguage(getPreferredLanguage());
    };
    const handleUpdated = (event: Event) => {
      if (!(event instanceof CustomEvent)) {
        sync();
        return;
      }
      setLanguage(normalizeLanguage(event.detail?.language));
    };
    window.addEventListener("storage", sync);
    window.addEventListener(LANGUAGE_UPDATED_EVENT, handleUpdated);
    return () => {
      window.removeEventListener("storage", sync);
      window.removeEventListener(LANGUAGE_UPDATED_EVENT, handleUpdated);
    };
  }, []);

  return {
    language,
    setLanguage: (next: AppLanguage) => savePreferredLanguage(next),
  };
}
