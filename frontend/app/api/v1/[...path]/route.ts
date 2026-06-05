import { NextRequest } from "next/server";
import { apiProxyDisabled, buildAuthConfigBody, portalBase } from "@/lib/authConfigFallback";

function targetUrl(req: NextRequest, path: string[]): string {
  const suffix = path.join("/");
  return `${portalBase()}/api/v1/${suffix}${req.nextUrl.search}`;
}

function requestContext(req: NextRequest) {
  const host = req.headers.get("x-forwarded-host") || req.headers.get("host") || "";
  const proto = req.headers.get("x-forwarded-proto") || "https";
  return { host, proto };
}

async function proxy(req: NextRequest, ctx: { params: { path: string[] } }) {
  const path = ctx.params.path;
  const pathKey = path.join("/");
  const reqCtx = requestContext(req);

  // SPCG_DISABLE_API_PROXY skips Edge middleware only; Node proxies in-cluster to portal.
  if (apiProxyDisabled() && req.method === "GET" && pathKey === "auth/config") {
    const fb = buildAuthConfigBody("portal proxy disabled; using SPCG_AUTH_METHODS", reqCtx);
    if (fb) {
      return Response.json(fb, { status: 200, headers: { "Content-Type": "application/json" } });
    }
  }

  const url = targetUrl(req, path);
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

  try {
    const res = await fetch(url, init);
    if (req.method === "GET" && pathKey === "auth/config" && (res.status === 404 || res.status >= 500)) {
      const fb = buildAuthConfigBody(`portal HTTP ${res.status}`, reqCtx);
      if (fb) return Response.json(fb, { status: 200 });
    }
    const outHeaders = new Headers(res.headers);
    return new Response(res.body, {
      status: res.status,
      statusText: res.statusText,
      headers: outHeaders,
    });
  } catch (err) {
    if (req.method === "GET" && pathKey === "auth/config") {
      const msg = err instanceof Error ? err.message : String(err);
      const fb = buildAuthConfigBody(`portal unreachable (${msg})`, reqCtx);
      if (fb) return Response.json(fb, { status: 200 });
    }
    const msg = err instanceof Error ? err.message : String(err);
    return Response.json({ error: `portal unreachable: ${msg}` }, { status: 502 });
  }
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const DELETE = proxy;
export const PATCH = proxy;
