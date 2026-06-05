import { NextRequest, NextResponse } from "next/server";
import { apiProxyDisabled, buildAuthConfigBody, portalBase } from "./lib/authConfigFallback";

async function proxyAuthConfig(request: NextRequest): Promise<NextResponse> {
  const target = portalBase() + request.nextUrl.pathname + request.nextUrl.search;
  try {
    const res = await fetch(target, { method: "GET", redirect: "manual" });
    if (res.status === 404 || res.status >= 500) {
      const fb = buildAuthConfigBody(`portal HTTP ${res.status}`);
      if (fb) return NextResponse.json(fb, { status: 200 });
    }
    if (!res.ok) {
      return new NextResponse(await res.text(), { status: res.status });
    }
    return new NextResponse(res.body, { status: res.status, headers: res.headers });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    const fb = buildAuthConfigBody(`portal unreachable (${msg})`);
    if (fb) return NextResponse.json(fb, { status: 200 });
    return NextResponse.json({ methods: [], error: msg }, { status: 502 });
  }
}

export async function middleware(request: NextRequest) {
  const pathname = request.nextUrl.pathname;
  if (!pathname.startsWith("/api/")) {
    return NextResponse.next();
  }

  // Auth config: always handle here. Secure layout has no egress — env fallback when proxy disabled.
  if (request.method === "GET" && pathname === "/api/v1/auth/config") {
    if (apiProxyDisabled()) {
      const fb = buildAuthConfigBody("portal proxy disabled; using SPCG_AUTH_METHODS");
      if (fb) return NextResponse.json(fb, { status: 200 });
      return NextResponse.json({ methods: [], error: "SPCG_AUTH_METHODS not set" }, { status: 502 });
    }
    return proxyAuthConfig(request);
  }

  if (apiProxyDisabled()) {
    return NextResponse.next();
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
