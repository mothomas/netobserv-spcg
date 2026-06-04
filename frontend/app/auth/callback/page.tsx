import { Suspense } from "react";
import { AuthCallbackClient } from "./AuthCallbackClient";

export const dynamic = "force-dynamic";

export default function AuthCallbackPage() {
  return (
    <Suspense
      fallback={
        <main className="min-h-screen flex items-center justify-center p-6 bg-siem-bg">
          <p className="text-siem-muted text-sm">Completing OpenShift sign-in…</p>
        </main>
      }
    >
      <AuthCallbackClient />
    </Suspense>
  );
}
