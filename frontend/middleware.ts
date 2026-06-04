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
