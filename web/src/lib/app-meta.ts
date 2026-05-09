import webConfig from "@/constants/common-env";
import { httpRequest } from "@/lib/request";

import {
  LOGIN_PAGE_IMAGE_DEFAULT_TRANSFORM,
  normalizeLoginPageImageMode,
  normalizeLoginPageImageTransform,
  type LoginPageImageMode,
} from "./login-page-image-layout";

export const APP_META_UPDATED_EVENT = "chatgpt2api:app-meta-updated";
export const DEFAULT_LOGIN_PAGE_IMAGE = "/login-panel-illustration.svg";

export type AppMeta = {
  app_title: string;
  project_name: string;
  top_left_logo_url: string;
  site_logo_url: string;
  image_single_count_limit: number;
  login_page_image_url: string;
  login_page_image_mode: LoginPageImageMode;
  login_page_image_zoom: number;
  login_page_image_position_x: number;
  login_page_image_position_y: number;
};

export const defaultAppMeta: AppMeta = {
  app_title: "chatgpt2api",
  project_name: "chatgpt2api",
  top_left_logo_url: "/logo-mark.svg",
  site_logo_url: "/logo-mark.svg",
  image_single_count_limit: 10,
  login_page_image_url: "",
  login_page_image_mode: "contain",
  login_page_image_zoom: LOGIN_PAGE_IMAGE_DEFAULT_TRANSFORM.zoom,
  login_page_image_position_x: LOGIN_PAGE_IMAGE_DEFAULT_TRANSFORM.positionX,
  login_page_image_position_y: LOGIN_PAGE_IMAGE_DEFAULT_TRANSFORM.positionY,
};

export async function fetchAppMeta() {
  const data = await httpRequest<Partial<AppMeta>>("/api/app-meta", {
    redirectOnUnauthorized: false,
  });
  return normalizeAppMeta(data);
}

export function normalizeAppMeta(data: Partial<AppMeta> = {}): AppMeta {
  const transform = normalizeLoginPageImageTransform({
    zoom: Number(data.login_page_image_zoom),
    positionX: Number(data.login_page_image_position_x),
    positionY: Number(data.login_page_image_position_y),
  });
  return {
    ...defaultAppMeta,
    ...data,
    app_title: typeof data.app_title === "string" && data.app_title.trim() ? data.app_title.trim() : defaultAppMeta.app_title,
    project_name:
      typeof data.project_name === "string" && data.project_name.trim() ? data.project_name.trim() : defaultAppMeta.project_name,
    top_left_logo_url:
      typeof data.top_left_logo_url === "string" && data.top_left_logo_url.trim()
        ? data.top_left_logo_url.trim()
        : defaultAppMeta.top_left_logo_url,
    site_logo_url:
      typeof data.site_logo_url === "string" && data.site_logo_url.trim()
        ? data.site_logo_url.trim()
        : defaultAppMeta.site_logo_url,
    image_single_count_limit: Math.max(1, Math.min(10, Number(data.image_single_count_limit) || 10)),
    login_page_image_url: typeof data.login_page_image_url === "string" ? data.login_page_image_url : "",
    login_page_image_mode: normalizeLoginPageImageMode(data.login_page_image_mode),
    login_page_image_zoom: transform.zoom,
    login_page_image_position_x: transform.positionX,
    login_page_image_position_y: transform.positionY,
  };
}

export function resolveBrandAssetURL(src?: string) {
  const value = String(src || "").trim();
  if (!value) {
    return "";
  }
  if (
    value.startsWith("blob:") ||
    value.startsWith("data:") ||
    value.startsWith("http://") ||
    value.startsWith("https://")
  ) {
    return value;
  }
  if (value.startsWith("/")) {
    const base = webConfig.apiUrl.replace(/\/$/, "");
    return `${base}${value}`;
  }
  return value;
}

export function dispatchAppMetaUpdated(payload: Partial<AppMeta> = {}) {
  window.dispatchEvent(new CustomEvent(APP_META_UPDATED_EVENT, { detail: payload }));
}

export function resolveLoginPageImageSrc(src?: string) {
  const value = String(src || "").trim();
  if (!value) {
    return DEFAULT_LOGIN_PAGE_IMAGE;
  }
  if (
    value.startsWith("blob:") ||
    value.startsWith("data:") ||
    value.startsWith("http://") ||
    value.startsWith("https://")
  ) {
    return value;
  }
  if (value.startsWith("/login-page-images/")) {
    const base = webConfig.apiUrl.replace(/\/$/, "");
    return `${base}${value}`;
  }
  return value;
}
