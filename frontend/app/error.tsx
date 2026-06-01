"use client";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <main className="min-h-screen flex items-center justify-center p-6">
      <div className="w-full max-w-lg bg-spcg-panel rounded-xl border border-slate-700 p-8">
        <h1 className="text-xl font-semibold text-spcg-err mb-2">Something went wrong</h1>
        <p className="text-sm text-slate-400 mb-4 whitespace-pre-wrap">{error.message}</p>
        <button className="px-4 py-2 rounded-lg bg-spcg-accent" onClick={() => reset()}>
          Try again
        </button>
      </div>
    </main>
  );
}
