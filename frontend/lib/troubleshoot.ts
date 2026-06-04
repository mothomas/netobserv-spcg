export type TroubleshootEntry = {
  at: string;
  step: string;
  detail?: string;
};

export function isTroubleshootMode(): boolean {
  if (typeof window === "undefined") return false;
  return (window as Window & { __SPCG_TROUBLESHOOT__?: boolean }).__SPCG_TROUBLESHOOT__ === true;
}

export function ts(): string {
  return new Date().toISOString();
}
