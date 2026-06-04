import { NextRequest, NextResponse } from "next/server";

function troubleshoot(): boolean {
  return process.env.SPCG_TROUBLESHOOT === "true" || process.env.SPCG_TROUBLESHOOT === "1";
}

function log(msg: string) {
  console.log(`[spcg-frontend] ${new Date().toISOString()} ${msg}`);
}

function portalBase(): string {
  return (process.env.SPCG_API_URL || "http://spcg-ui-portal.pcap-frontend.svc.cluster.local:80").replace(
    /\/$/,
    ""
  );
}

function runtimeAuthMethods(): string[] {
  const raw = (process.env.SPCG_AUTH_METHODS || "").trim();
  if (!raw) return [];
  return raw.split(",").map((m) => m.trim().toLowerCase()).filter(Boolean);
}

/** When ui-portal is on an old image (404), still expose OpenShift login UI from frontend env. */
function authConfigFallback(): NextResponse | null {
  const methods = runtimeAuthMethods();
  if (!methods.includes("openshift")) return null;
  return NextResponse.json(
    {
      methods,
      openshift: {
        authorize_path: "/api/v1/auth/openshift/authorize",
        error:
          "spcg-ui-portal returned 404 (image too old or rollout stuck). Use tag small-20260622+, delete stale portal pods, verify with: oc get pods -l app=spcg-ui-portal -o custom-columns=IMAGE:.spec.containers[0].image",
      },
    },
    { status: 200, headers: { "Content-Type": "application/json" } }
  );
}

export async function middleware(request: NextRequest) {
  const pathname = request.nextUrl.pathname;
  if (!pathname.startsWith("/api/")) {
    return NextResponse.next();
  }
  const target = portalBase() + pathname + request.nextUrl.search;
  const t0 = Date.now();
  if (troubleshoot()) {
    log(`proxy ${request.method} ${pathname} -> ${target}`);
  }
  const headers = new Headers();
  request.headers.forEach((value, key) => {
    if (key.toLowerCase() === "host") return;
    headers.set(key, value);
  });
  const init: RequestInit & { duplex?: "half" } = {
    method: request.method,
    headers,
    redirect: "manual",
  };
  if (request.method !== "GET" && request.method !== "HEAD") {
    init.body = request.body;
    init.duplex = "half";
  }
  let res: Response;
  try {
    res = await fetch(target, init);
  } catch (err) {
    if (troubleshoot()) {
      log(`proxy ${request.method} ${pathname} ERROR ${err instanceof Error ? err.message : String(err)}`);
    }
    throw err;
  }
  if (troubleshoot()) {
    const snippet = res.status >= 400 ? ` body=${(await res.clone().text()).slice(0, 200)}` : "";
    log(`proxy ${request.method} ${pathname} <- ${res.status} ${Date.now() - t0}ms${snippet}`);
  }
  if (
    request.method === "GET" &&
    pathname === "/api/v1/auth/config" &&
    res.status === 404
  ) {
    const fb = authConfigFallback();
    if (fb) {
      if (troubleshoot()) log("auth/config portal 404 — returning frontend fallback from SPCG_AUTH_METHODS");
      return fb;
    }
  }
  const out = new NextResponse(res.body, {
    status: res.status,
    statusText: res.statusText,
    headers: res.headers,
  });
  return out;
}

export const config = {
  matcher: "/api/:path*",
};
