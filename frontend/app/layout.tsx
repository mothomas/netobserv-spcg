import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "SPCG — Secure Packet Capture Gateway",
  description: "Namespace-scoped netobserv capture with zero-trust impersonation",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  const apiBase = (process.env.SPCG_PUBLIC_API_BASE || process.env.NEXT_PUBLIC_SPCG_API_BASE || "").replace(
    /\/$/,
    ""
  );
  return (
    <html lang="en">
      <head>
        {apiBase ? (
          <script
            dangerouslySetInnerHTML={{
              __html: `window.__SPCG_API_BASE__=${JSON.stringify(apiBase)};`,
            }}
          />
        ) : null}
      </head>
      <body>{children}</body>
    </html>
  );
}
