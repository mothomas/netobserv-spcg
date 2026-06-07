type Action = {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  primary?: boolean;
};

type Props = {
  title: string;
  description: string;
  steps?: string[];
  primaryAction?: Action;
  secondaryAction?: Action;
};

export function SectionEmptyState({
  title,
  description,
  steps,
  primaryAction,
  secondaryAction,
}: Props) {
  return (
    <div
      className="siem-card p-8 max-w-xl mx-auto text-center"
      role="status"
      aria-live="polite"
    >
      <h2 className="text-base font-semibold text-siem-text mb-2">{title}</h2>
      <p className="text-sm text-siem-muted mb-5">{description}</p>
      {steps && steps.length > 0 && (
        <ol className="text-left text-sm text-siem-muted space-y-2 mb-6 mx-auto max-w-md list-decimal list-inside">
          {steps.map((step) => (
            <li key={step}>{step}</li>
          ))}
        </ol>
      )}
      <div className="flex flex-wrap justify-center gap-2">
        {primaryAction && (
          <button
            type="button"
            className="siem-btn-primary"
            disabled={primaryAction.disabled}
            onClick={primaryAction.onClick}
          >
            {primaryAction.label}
          </button>
        )}
        {secondaryAction && (
          <button
            type="button"
            className="siem-btn-ghost"
            disabled={secondaryAction.disabled}
            onClick={secondaryAction.onClick}
          >
            {secondaryAction.label}
          </button>
        )}
      </div>
    </div>
  );
}
