"use client";

import { useEffect } from "react";
import { useNavigate } from "react-router-dom";

import { getVerifiedAuthSession } from "@/lib/session";
import { getDefaultRouteForRole } from "@/store/auth";

export default function HomePage() {
  const navigate = useNavigate();

  useEffect(() => {
    let active = true;

    const redirect = async () => {
      const session = await getVerifiedAuthSession();
      if (!active) {
        return;
      }
      navigate(session ? getDefaultRouteForRole(session.role) : "/login", { replace: true });
    };

    void redirect();
    return () => {
      active = false;
    };
  }, [navigate]);

  return null;
}
