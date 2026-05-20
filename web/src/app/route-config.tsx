import { createElement, lazy, type ComponentType, type ReactNode } from "react";

function lazyPage(loader: () => Promise<{ default: ComponentType }>) {
  return createElement(lazy(loader));
}

export type AppRouteConfig = {
  path: string;
  element: ReactNode;
  requiredPath?: string;
};

export const appRoutes: AppRouteConfig[] = [
  { path: "/", element: lazyPage(() => import("@/app/page")) },
  { path: "/login", element: lazyPage(() => import("@/app/login/page")) },
  {
    path: "/accounts",
    element: lazyPage(() => import("@/app/accounts/page")),
    requiredPath: "/accounts",
  },
  {
    path: "/register",
    element: lazyPage(() => import("@/app/register/page")),
    requiredPath: "/register",
  },
  {
    path: "/image-manager",
    element: lazyPage(() => import("@/app/image-manager/page")),
    requiredPath: "/image-manager",
  },
  {
    path: "/users",
    element: lazyPage(() => import("@/app/users/page")),
    requiredPath: "/users",
  },
  {
    path: "/profile",
    element: lazyPage(() => import("@/app/profile/page")),
    requiredPath: "/profile",
  },
  {
    path: "/wallet",
    element: lazyPage(() => import("@/app/wallet/page")),
    requiredPath: "/wallet",
  },
  {
    path: "/subscription",
    element: lazyPage(() => import("@/app/subscription/page")),
    requiredPath: "/subscription",
  },
  {
    path: "/agency",
    element: lazyPage(() => import("@/app/agency/page")),
    requiredPath: "/agency",
  },
  {
    path: "/agency-commission",
    element: lazyPage(() => import("@/app/agency-commission/page")),
    requiredPath: "/agency-commission",
  },
  {
    path: "/rbac",
    element: lazyPage(() => import("@/app/rbac/page")),
    requiredPath: "/rbac",
  },
  {
    path: "/logs",
    element: lazyPage(() => import("@/app/logs/page")),
    requiredPath: "/logs",
  },
  {
    path: "/settings",
    element: lazyPage(() => import("@/app/settings/page")),
    requiredPath: "/settings",
  },
  {
    path: "/image",
    element: lazyPage(() => import("@/app/image/page")),
    requiredPath: "/image",
  },
  { path: "*", element: lazyPage(() => import("@/app/page")) },
];
