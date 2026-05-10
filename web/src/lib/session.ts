"use client";

import { login } from "@/lib/api";
import {
  clearStoredAuthSession,
  getStoredAuthSession,
  setStoredAuthSession,
  type StoredAuthSession,
} from "@/store/auth";

let cachedAuthSession: StoredAuthSession | null | undefined;
let verifyAuthSessionPromise: Promise<StoredAuthSession | null> | null = null;
let authSessionVersion = 0;

export function getCachedAuthSession() {
  return cachedAuthSession;
}

export async function getVerifiedAuthSession(): Promise<StoredAuthSession | null> {
  if (cachedAuthSession !== undefined) {
    return cachedAuthSession;
  }

  const verifyStartedAtVersion = authSessionVersion;
  verifyAuthSessionPromise ??= verifyStoredAuthSession();
  try {
    const verifiedSession = await verifyAuthSessionPromise;
    if (verifyStartedAtVersion === authSessionVersion) {
      cachedAuthSession = verifiedSession;
      if (verifiedSession) {
        await setStoredAuthSession(verifiedSession);
      } else {
        await clearStoredAuthSession();
      }
      return verifiedSession;
    }
    return cachedAuthSession ?? null;
  } finally {
    if (verifyStartedAtVersion === authSessionVersion) {
      verifyAuthSessionPromise = null;
    }
  }
}

export async function setVerifiedAuthSession(session: StoredAuthSession) {
  authSessionVersion += 1;
  cachedAuthSession = session;
  verifyAuthSessionPromise = null;
  await setStoredAuthSession(session);
}

export async function clearVerifiedAuthSession() {
  authSessionVersion += 1;
  cachedAuthSession = null;
  verifyAuthSessionPromise = null;
  await clearStoredAuthSession();
}

async function verifyStoredAuthSession(): Promise<StoredAuthSession | null> {
  const storedSession = await getStoredAuthSession();
  if (!storedSession) {
    return null;
  }

  try {
    const data = await login(storedSession.key);
    const verifiedSession: StoredAuthSession = {
      key: storedSession.key,
      role: data.role,
      subjectId: data.subject_id,
      name: data.name,
      provider: data.provider,
    };
    return verifiedSession;
  } catch {
    return null;
  }
}
