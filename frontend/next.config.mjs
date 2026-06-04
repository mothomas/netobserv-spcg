/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  async rewrites() {
    const api = (process.env.SPCG_API_URL || "http://localhost:8080").replace(/\/$/, "");
    return [{ source: "/api/:path*", destination: `${api}/api/:path*` }];
  },
};

export default nextConfig;
