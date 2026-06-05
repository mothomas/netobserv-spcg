import { NextRequest, NextResponse } from "next/server";

/** All /api traffic is handled by Node route handlers (Edge cannot see K8s runtime env). */
export async function middleware(request: NextRequest) {
  if (!request.nextUrl.pathname.startsWith("/api/")) {
    return NextResponse.next();
  }
  return NextResponse.next();
}

export const config = {
  matcher: "/api/:path*",
};
