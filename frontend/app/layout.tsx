import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "SPCG — Secure Packet Capture Gateway",
  description: "Namespace-scoped netobserv capture with zero-trust impersonation",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  const authMethods = (process.env.SPCG_AUTH_METHODS || "").trim();
  const boot = authMethods ? `window.__SPCG_AUTH_METHODS__=${JSON.stringify(authMethods)};` : "";
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
