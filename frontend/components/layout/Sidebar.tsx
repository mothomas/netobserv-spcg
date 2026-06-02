"use client";

type Props = {
  product: string;
  cluster?: string;
  sessionActive?: boolean;
  captureActive?: boolean;
  onSignOut?: () => void;
};

export function Sidebar({ product, cluster, sessionActive, captureActive, onSignOut }: Props) {
  return (
    <>
      <div className="px-4 py-5 border-b border-siem-border">
        <div className="flex items-center gap-2">
          <span
            className="h-8 w-8 rounded-full border border-blue-300/35 flex items-center justify-center text-xs font-bold text-white shadow-[0_8px_20px_rgba(37,99,235,0.35)]"
            style={{ background: "linear-gradient(180deg, #2d66ff 0%, #1f4ed8 100%)" }}
          >
            SPCG
          </span>
          <div>
            <p className="text-sm font-semibold text-siem-text">{product}</p>
            <p className="text-[10px] text-siem-muted uppercase tracking-wide">Packet observability</p>
          </div>
        </div>
      </div>
      <nav className="flex-1 px-3 py-4 space-y-1 text-sm">
        <NavItem label="Workspace" active />
        <NavItem label="Capture" hint={captureActive ? "Live" : "Idle"} hintTone={captureActive ? "ok" : "muted"} />
        <NavItem label="Flow graph" />
        <NavItem label="AI analyst" />
      </nav>
      <div className="px-4 py-4 border-t border-siem-border space-y-2 text-xs">
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
}: {
  label: string;
  active?: boolean;
  hint?: string;
  hintTone?: "ok" | "muted";
}) {
  return (
    <div
      className={`flex items-center justify-between px-3 py-2 rounded-md ${
        active
          ? "text-white border border-blue-300/30 shadow-[0_8px_20px_rgba(37,99,235,0.3)]"
          : "text-siem-muted hover:text-siem-text hover:bg-siem-card"
      }`}
      style={active ? { background: "linear-gradient(180deg, #2d66ff 0%, #1f4ed8 100%)" } : undefined}
    >
      <span>{label}</span>
      {hint && (
        <span
          className={`text-[10px] px-1.5 py-0.5 rounded ${
            hintTone === "ok" ? "bg-siem-ok/20 text-siem-ok" : "bg-siem-border text-siem-muted"
          }`}
        >
          {hint}
        </span>
      )}
    </div>
  );
}

function StatusDot({ ok }: { ok?: boolean }) {
  return (
    <span
      className={`inline-block h-1.5 w-1.5 rounded-full mr-1 ${ok ? "bg-siem-ok" : "bg-siem-muted"}`}
    />
  );
}
