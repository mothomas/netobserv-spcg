import { NextRequest, NextResponse } from "next/server";

function portalBase(): string {
  return (process.env.SPCG_API_URL || "http://spcg-ui-portal.pcap-frontend.svc.cluster.local:80").replace(
    /\/$/,
    ""
  );
}

export async function middleware(request: NextRequest) {
  const pathname = request.nextUrl.pathname;
  if (!pathname.startsWith("/api/")) {
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
  const res = await fetch(target, init);
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
