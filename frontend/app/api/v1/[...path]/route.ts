import { NextRequest } from "next/server";

function backendBase(): string {
  return (process.env.SPCG_API_URL || "http://localhost:8080").replace(/\/$/, "");
}

function targetUrl(req: NextRequest, path: string[]): string {
  const suffix = path.join("/");
  return `${backendBase()}/api/v1/${suffix}${req.nextUrl.search}`;
}

async function proxy(req: NextRequest, ctx: { params: { path: string[] } }) {
  const url = targetUrl(req, ctx.params.path);
  const headers = new Headers();
  req.headers.forEach((value, key) => {
    if (key.toLowerCase() === "host") return;
    headers.set(key, value);
  });

  const init: RequestInit & { duplex?: "half" } = {
    method: req.method,
    headers,
    redirect: "manual",
    signal: req.signal,
  };

  if (req.method !== "GET" && req.method !== "HEAD") {
    init.body = req.body;
    init.duplex = "half";
  }

  const res = await fetch(url, init);
  const outHeaders = new Headers(res.headers);
  // Next.js may strip hop-by-hop headers; preserve SSE content-type.
  return new Response(res.body, {
    status: res.status,
    statusText: res.statusText,
    headers: outHeaders,
  });
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const DELETE = proxy;
export const PATCH = proxy;
