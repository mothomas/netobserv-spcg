import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        siem: {
          bg: "var(--siem-bg)",
          panel: "var(--siem-panel)",
          card: "var(--siem-card)",
          border: "var(--siem-border)",
          borderHi: "var(--siem-border-hi)",
          text: "var(--siem-text)",
          muted: "var(--siem-muted)",
          accent: "var(--siem-accent)",
          accentHi: "var(--siem-accent-hi)",
          ok: "var(--siem-ok)",
          warn: "var(--siem-warn)",
          err: "var(--siem-err)",
          info: "var(--siem-info)",
        },
      },
      fontFamily: {
        sans: ["var(--siem-font-sans)", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["ui-monospace", "SFMono-Regular", "Menlo", "Consolas", "monospace"],
      },
      boxShadow: {
        panel: "var(--siem-shadow-panel)",
      },
      borderRadius: {
        siem: "var(--siem-radius)",
        "siem-lg": "var(--siem-radius-lg)",
      },
    },
  },
  plugins: [],
};
export default config;
