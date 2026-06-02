"use client";

import type { ReactNode } from "react";

type Props = {
  sidebar: ReactNode;
  children: ReactNode;
  topbar?: ReactNode;
};

export function AppShell({ sidebar, children, topbar }: Props) {
  return (
    <div className="min-h-screen flex bg-siem-bg">
      <aside className="w-56 shrink-0 border-r border-siem-border bg-siem-panel flex flex-col">
        {sidebar}
      </aside>
      <div className="flex-1 flex flex-col min-w-0">
        {topbar && (
          <header className="shrink-0 border-b border-siem-border bg-siem-panel/80 backdrop-blur px-6 py-3">
            {topbar}
          </header>
        )}
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  );
}
