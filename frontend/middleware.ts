import { NextRequest, NextResponse } from "next/server";
import { apiProxyDisabled, portalBase } from "./lib/authConfigFallback";

export async function middleware(request: NextRequest) {
  const pathname = request.nextUrl.pathname;
  if (!pathname.startsWith("/api/")) {
    return NextResponse.next();
  }

  // Auth config: Node route handler only (Edge middleware cannot see K8s runtime env vars).
  if (request.method === "GET" && pathname === "/api/v1/auth/config") {
    return NextResponse.next();
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
