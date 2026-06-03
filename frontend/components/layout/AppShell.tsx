import type { ReactNode } from "react";

type Props = {
  sidebar: ReactNode;
  children: ReactNode;
  topbar?: ReactNode;
};

export function AppShell({ sidebar, children, topbar }: Props) {
  return (
    <div className="min-h-screen flex app-shell-root">
      <aside className="app-shell-sidebar w-56 shrink-0 flex flex-col">{sidebar}</aside>
      <div className="flex-1 flex flex-col min-w-0">
        {topbar && <header className="app-shell-header shrink-0 px-6 py-3">{topbar}</header>}
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  );
}
