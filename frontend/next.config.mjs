/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  async rewrites() {
    // Local dev: proxy /api to ui-portal. In cluster, OpenShift Route spcg-api serves /api.
    if (process.env.NODE_ENV !== "production") {
      const api = process.env.SPCG_API_URL || "http://localhost:8080";
      return [{ source: "/api/:path*", destination: `${api}/api/:path*` }];
    }
    return [];
  },
};

export default nextConfig;
