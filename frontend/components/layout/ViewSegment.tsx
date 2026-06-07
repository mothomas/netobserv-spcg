type Option<T extends string> = {
  id: T;
  label: string;
  disabled?: boolean;
};

type Props<T extends string> = {
  value: T;
  options: Option<T>[];
  onChange: (value: T) => void;
  ariaLabel: string;
};

export function ViewSegment<T extends string>({ value, options, onChange, ariaLabel }: Props<T>) {
  return (
    <div
      className="flex rounded-siem border border-siem-border overflow-hidden text-xs shrink-0"
      role="tablist"
      aria-label={ariaLabel}
    >
      {options.map((opt) => {
        const active = value === opt.id;
        return (
          <button
            key={opt.id}
            type="button"
            role="tab"
            aria-selected={active}
            disabled={opt.disabled}
            className={`px-3 py-1.5 transition ${
              opt.disabled
                ? "opacity-40 cursor-not-allowed fluent-nav-idle"
                : active
                  ? "fluent-nav-active"
                  : "fluent-nav-idle hover:bg-siem-panel/60"
            }`}
            onClick={() => onChange(opt.id)}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}
