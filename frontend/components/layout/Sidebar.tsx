export type AppSection = "workspace" | "flow" | "trace" | "microservices" | "ai";

type Props = {
  product: string;
  cluster?: string;
  sessionActive?: boolean;
  captureActive?: boolean;
  traceActive?: boolean;
  microservicesAvailable?: boolean;
  microservicesActive?: boolean;
  active?: AppSection;
  flowAvailable?: boolean;
  traceAvailable?: boolean;
  aiAvailable?: boolean;
  onNavigate?: (section: AppSection) => void;
  onSignOut?: () => void;
};

export function Sidebar({
  product,
  cluster,
  sessionActive,
  captureActive,
  traceActive,
  microservicesAvailable,
  microservicesActive,
  active = "workspace",
  flowAvailable,
  traceAvailable,
  aiAvailable,
  onNavigate,
  onSignOut,
}: Props) {
  return (
    <>
      <div className="px-4 py-5 border-b border-siem-border">
        <div className="flex items-center gap-2">
          <span className="fluent-logo-mark h-9 w-9">SPCG</span>
          <div>
            <p className="text-sm font-semibold text-siem-text">{product}</p>
            <p className="text-[10px] text-siem-muted uppercase tracking-wide">Packet observability</p>
          </div>
        </div>
      </div>
      <nav className="flex-1 px-3 py-4 space-y-1 text-sm" aria-label="Primary">
        <NavItem
          label="Workspace"
          active={active === "workspace"}
          onClick={() => onNavigate?.("workspace")}
        />
        <NavItem
          label="Capture"
          hint={captureActive ? "Live" : "Idle"}
          hintTone={captureActive ? "ok" : "muted"}
          disabled
        />
        <NavItem
          label="Packet Trace"
          active={active === "trace"}
          hint={traceActive ? "Live" : traceAvailable ? (active === "trace" ? "Active" : "Ready") : "Setup"}
          hintTone={traceActive ? "ok" : "muted"}
          disabled={!traceAvailable}
          onClick={() => traceAvailable && onNavigate?.("trace")}
        />
        <NavItem
          label="L7 analysis"
          active={active === "microservices"}
          hint={microservicesActive ? "Live" : microservicesAvailable ? "Ready" : "Trace first"}
          hintTone={microservicesActive ? "ok" : "muted"}
          disabled={!microservicesAvailable}
          onClick={() => microservicesAvailable && onNavigate?.("microservices")}
        />
        <NavItem
          label="Flow graph"
          active={active === "flow"}
          disabled={!flowAvailable}
          onClick={() => flowAvailable && onNavigate?.("flow")}
        />
        <NavItem
          label="AI analyst"
          active={active === "ai"}
          disabled={!aiAvailable}
          onClick={() => aiAvailable && onNavigate?.("ai")}
        />
      </nav>
      <div className="px-4 py-4 border-t border-siem-border space-y-3 text-xs">
        {cluster && (
          <p className="text-siem-muted">
            Cluster <span className="font-mono text-siem-text">{cluster}</span>
          </p>
        )}
        <p className="text-siem-muted">
          Auth <StatusDot ok={sessionActive} /> {sessionActive ? "Active" : "—"}
        </p>
        <button type="button" className="siem-btn-ghost w-full text-left" onClick={onSignOut}>
          Sign out & purge session
        </button>
      </div>
    </>
  );
}

function NavItem({
  label,
  active,
  hint,
  hintTone,
  disabled,
  onClick,
}: {
  label: string;
  active?: boolean;
  hint?: string;
  hintTone?: "ok" | "muted";
  disabled?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      aria-current={active ? "page" : undefined}
      className={`w-full flex items-center justify-between px-3 py-2 rounded-siem text-left transition ${
        disabled
          ? "opacity-40 cursor-not-allowed"
          : active
            ? "fluent-nav-active"
            : "fluent-nav-idle hover:bg-siem-panel/60"
      }`}
    >
      <span>{label}</span>
      {hint && (
        <span
          className={`text-[10px] px-1.5 py-0.5 rounded-md ${
            hintTone === "ok"
              ? "text-siem-ok border border-siem-ok/30"
              : "text-siem-muted border border-siem-border"
          }`}
          style={
            hintTone === "ok"
              ? { background: "color-mix(in srgb, var(--siem-ok) 18%, transparent)" }
              : { background: "color-mix(in srgb, var(--siem-border) 80%, transparent)" }
          }
        >
          {hint}
        </span>
      )}
    </button>
  );
}

function StatusDot({ ok }: { ok?: boolean }) {
  return (
    <span
      className={`inline-block h-1.5 w-1.5 rounded-full mr-1 ${ok ? "bg-siem-ok" : "bg-siem-muted"}`}
    />
  );
}
