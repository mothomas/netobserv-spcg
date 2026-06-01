import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        spcg: {
          bg: "#ffffff",
          panel: "#f8fafc",
          accent: "#2563eb",
          ok: "#16a34a",
          warn: "#d97706",
          err: "#dc2626",
          muted: "#64748b",
          border: "#e2e8f0",
        },
      },
      fontFamily: { mono: ["ui-monospace", "SFMono-Regular", "Menlo", "monospace"] },
    },
  },
  plugins: [],
};
export default config;
