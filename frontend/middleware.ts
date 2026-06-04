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

/** When ui-portal is down/old, still expose OpenShift login UI from frontend env (never throw). */
function authConfigFallback(portalDetail?: string): NextResponse | null {
  const methods = runtimeAuthMethods();
  if (!methods.includes("openshift")) return null;
  const detail =
    portalDetail ||
    "spcg-ui-portal unavailable. Use image small-20260624+, delete stale portal pods.";
  return NextResponse.json(
    {
      methods,
      openshift: {
        authorize_path: "/api/v1/auth/openshift/authorize",
        error: `${detail} Verify: oc get pods -n pcap-frontend -l app=spcg-ui-portal -o custom-columns=IMAGE:.spec.containers[0].image`,
      },
    },
    { status: 200, headers: { "Content-Type": "application/json" } }
  );
}

async function proxyAuthConfig(request: NextRequest): Promise<NextResponse> {
  const target = portalBase() + request.nextUrl.pathname + request.nextUrl.search;
  const t0 = Date.now();
  if (troubleshoot()) log(`proxy GET /api/v1/auth/config -> ${target}`);
  let res: Response | null = null;
  try {
    res = await fetch(target, { method: "GET", redirect: "manual" });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (troubleshoot()) log(`proxy GET /api/v1/auth/config ERROR ${msg}`);
    const fb = authConfigFallback(`portal unreachable (${msg})`);
    if (fb) return fb;
    return NextResponse.json({ methods: [], error: msg }, { status: 502 });
  }
  if (troubleshoot()) {
    const snippet = res.status >= 400 ? ` body=${(await res.clone().text()).slice(0, 200)}` : "";
    log(`proxy GET /api/v1/auth/config <- ${res.status} ${Date.now() - t0}ms${snippet}`);
  }
  if (res.status === 404 || res.status >= 500) {
    const fb = authConfigFallback(`portal HTTP ${res.status}`);
    if (fb) {
      if (troubleshoot()) log("auth/config bad portal response — returning frontend fallback");
      return fb;
    }
  }
  if (!res.ok) {
    const text = await res.text();
    return new NextResponse(text || `HTTP ${res.status}`, { status: res.status });
  }
  return new NextResponse(res.body, {
    status: res.status,
    statusText: res.statusText,
    headers: res.headers,
  });
}

export async function middleware(request: NextRequest) {
  const pathname = request.nextUrl.pathname;
  if (!pathname.startsWith("/api/")) {
    return NextResponse.next();
  }
  if (request.method === "GET" && pathname === "/api/v1/auth/config") {
    return proxyAuthConfig(request);
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
    const msg = err instanceof Error ? err.message : String(err);
    if (troubleshoot()) log(`proxy ${request.method} ${pathname} ERROR ${msg}`);
    return NextResponse.json(
      { error: `portal unreachable: ${msg}` },
      { status: 502, headers: { "Content-Type": "application/json" } }
    );
  }
  if (troubleshoot()) {
    const snippet = res.status >= 400 ? ` body=${(await res.clone().text()).slice(0, 200)}` : "";
    log(`proxy ${request.method} ${pathname} <- ${res.status} ${Date.now() - t0}ms${snippet}`);
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
