import { NextRequest, NextResponse } from "next/server";

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

/** Minimal fallback when portal is down but OpenShift login should still render. */
function authConfigFallback(detail: string): NextResponse | null {
  if (!runtimeAuthMethods().includes("openshift")) return null;
  return NextResponse.json(
    {
      methods: runtimeAuthMethods(),
      openshift: {
        authorize_path: "/api/v1/auth/openshift/authorize",
        error: detail,
      },
    },
    { status: 200, headers: { "Content-Type": "application/json" } }
  );
}

async function proxyAuthConfig(request: NextRequest): Promise<NextResponse> {
  const target = portalBase() + request.nextUrl.pathname + request.nextUrl.search;
  try {
    const res = await fetch(target, { method: "GET", redirect: "manual" });
    if (res.status === 404 || res.status >= 500) {
      const fb = authConfigFallback(`portal HTTP ${res.status}`);
      if (fb) return fb;
    }
    if (!res.ok) {
      return new NextResponse(await res.text(), { status: res.status });
    }
    return new NextResponse(res.body, { status: res.status, headers: res.headers });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    const fb = authConfigFallback(`portal unreachable (${msg})`);
    if (fb) return fb;
    return NextResponse.json({ methods: [], error: msg }, { status: 502 });
  }
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
  try {
    const res = await fetch(target, init);
    return new NextResponse(res.body, { status: res.status, statusText: res.statusText, headers: res.headers });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return NextResponse.json({ error: `portal unreachable: ${msg}` }, { status: 502 });
  }
}

export const config = {
  matcher: "/api/:path*",
};
