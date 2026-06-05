"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

/** Legacy /trace URLs redirect into the main app shell. */
export default function TraceRedirectPage() {
  const router = useRouter();

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const traceId = params.get("trace_id")?.trim();
    const next = traceId
      ? `/?section=trace&trace_id=${encodeURIComponent(traceId)}`
      : "/?section=trace";
    router.replace(next);
  }, [router]);

  return (
    <main className="min-h-screen flex items-center justify-center bg-siem-bg text-siem-muted text-sm">
      Opening packet trace…
    </main>
  );
}
