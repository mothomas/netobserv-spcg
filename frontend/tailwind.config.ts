import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        siem: {
          bg: "#0a101d",
          panel: "#0f172a",
          card: "#121c2f",
          border: "#223048",
          borderHi: "#334155",
          text: "#e5edf8",
          muted: "#90a4c2",
          accent: "#22d3ee",
          accentHi: "#67e8f9",
          ok: "#34d399",
          warn: "#fbbf24",
          err: "#fb7185",
          info: "#38bdf8",
        },
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "monospace"],
      },
      boxShadow: {
        panel: "0 1px 0 rgba(255,255,255,0.05) inset, 0 16px 42px rgba(0,0,0,0.35)",
      },
    },
  },
  plugins: [],
};
export default config;
