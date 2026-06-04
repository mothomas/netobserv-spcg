"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import type { LoginResponse } from "@/lib/api";

export function AuthCallbackClient() {
  const router = useRouter();
  const params = useSearchParams();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const sessionId = params.get("session_id")?.trim();
    if (!sessionId) {
      setError("Missing session_id from OpenShift login.");
      return;
    }
    const auth: LoginResponse = {
      session_id: sessionId,
      mode: "openshift",
      cluster: params.get("cluster") || "OpenShift",
    };
    try {
      sessionStorage.setItem("spcg_pending_auth", JSON.stringify(auth));
    } catch {
      setError("Could not store session.");
      return;
    }
    router.replace("/");
  }, [params, router]);

  if (error) {
    return (
      <main className="min-h-screen flex items-center justify-center p-6 bg-siem-bg">
        <div className="siem-card p-6 max-w-md">
          <p className="text-siem-err text-sm">{error}</p>
          <button type="button" className="siem-btn-primary mt-4 w-full" onClick={() => router.replace("/")}>
            Back to sign in
          </button>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen flex items-center justify-center p-6 bg-siem-bg">
      <p className="text-siem-muted text-sm">Completing OpenShift sign-in…</p>
    </main>
  );
}
