/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  async rewrites() {
    // Dev-only fallback; production uses app/api/v1/[...path]/route.ts (runtime proxy).
    if (process.env.NODE_ENV !== "production") {
      const api = process.env.SPCG_API_URL || "http://localhost:8080";
      return [{ source: "/api/:path*", destination: `${api}/api/:path*` }];
    }
    return [];
  },
};

export default nextConfig;
