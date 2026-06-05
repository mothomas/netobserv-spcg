import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "SPCG — Secure Packet Capture Gateway",
  description: "Namespace-scoped netobserv capture with zero-trust impersonation",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  const authMethods = (process.env.SPCG_AUTH_METHODS || "").trim();
  const apiBase = (process.env.SPCG_PUBLIC_API_BASE || process.env.NEXT_PUBLIC_SPCG_API_BASE || "").trim();
  let boot = authMethods ? `window.__SPCG_AUTH_METHODS__=${JSON.stringify(authMethods)};` : "";
  if (apiBase) {
    boot += `window.__SPCG_API_BASE__=${JSON.stringify(apiBase.replace(/\/$/, ""))};`;
  }
  return (
    <html lang="en">
      <head>
        {authMethods ? <meta name="spcg-auth-methods" content={authMethods} /> : null}
        {boot ? <script dangerouslySetInnerHTML={{ __html: boot }} /> : null}
      </head>
      <body>{children}</body>
    </html>
  );
}
