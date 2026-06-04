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
  const authMethods = (process.env.SPCG_AUTH_METHODS || "").trim();
  const troubleshoot =
    process.env.SPCG_TROUBLESHOOT === "true" || process.env.SPCG_TROUBLESHOOT === "1";
  const boot: string[] = [];
  if (apiBase) boot.push(`window.__SPCG_API_BASE__=${JSON.stringify(apiBase)};`);
  if (authMethods) boot.push(`window.__SPCG_AUTH_METHODS__=${JSON.stringify(authMethods)};`);
  if (troubleshoot) boot.push("window.__SPCG_TROUBLESHOOT__=true;");
  return (
    <html lang="en">
      <head>
        {authMethods ? <meta name="spcg-auth-methods" content={authMethods} /> : null}
        {boot.length > 0 ? (
          <script dangerouslySetInnerHTML={{ __html: boot.join("") }} />
        ) : null}
      </head>
      <body>{children}</body>
    </html>
  );
}
