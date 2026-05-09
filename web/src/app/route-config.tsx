import type { ReactNode } from "react";

import AccountsPage from "@/app/accounts/page";
import AgencyPage from "@/app/agency/page";
import AgencyCommissionPage from "@/app/agency-commission/page";
import ImagePage from "@/app/image/page";
import ImageManagerPage from "@/app/image-manager/page";
import HomePage from "@/app/page";
import LoginPage from "@/app/login/page";
import LogsPage from "@/app/logs/page";
import ProfilePage from "@/app/profile/page";
import RBACPage from "@/app/rbac/page";
import RegisterPage from "@/app/register/page";
import SettingsPage from "@/app/settings/page";
import UsersPage from "@/app/users/page";
import WalletPage from "@/app/wallet/page";

export type AppRouteConfig = {
  path: string;
  element: ReactNode;
  requiredPath?: string;
};

export const appRoutes: AppRouteConfig[] = [
  { path: "/", element: <HomePage /> },
  { path: "/login", element: <LoginPage /> },
  { path: "/accounts", element: <AccountsPage />, requiredPath: "/accounts" },
  { path: "/register", element: <RegisterPage />, requiredPath: "/register" },
  { path: "/image-manager", element: <ImageManagerPage />, requiredPath: "/image-manager" },
  { path: "/users", element: <UsersPage />, requiredPath: "/users" },
  { path: "/profile", element: <ProfilePage />, requiredPath: "/profile" },
  { path: "/wallet", element: <WalletPage />, requiredPath: "/wallet" },
  { path: "/agency", element: <AgencyPage />, requiredPath: "/agency" },
  { path: "/agency-commission", element: <AgencyCommissionPage />, requiredPath: "/agency-commission" },
  { path: "/rbac", element: <RBACPage />, requiredPath: "/rbac" },
  { path: "/logs", element: <LogsPage />, requiredPath: "/logs" },
  { path: "/settings", element: <SettingsPage />, requiredPath: "/settings" },
  { path: "/image", element: <ImagePage />, requiredPath: "/image" },
  { path: "*", element: <HomePage /> },
];
