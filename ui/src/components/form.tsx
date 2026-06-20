import type { ReactNode } from "react";

// Small controlled form primitives sharing edp's .field/.lbl styles.

export function Accordion({ title, children, open }: { title: string; children: ReactNode; open?: boolean }) {
  return (
    <details className="adv" open={open}>
      <summary>
        <span>{title}</span>
        <span className="chev">›</span>
      </summary>
      <div className="adv-body">{children}</div>
    </details>
  );
}

export function Text({
  label,
  hint,
  value,
  onChange,
  placeholder,
  type = "text",
  required,
  autoFocus,
  mono,
}: {
  label: string;
  hint?: ReactNode;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  type?: string;
  required?: boolean;
  autoFocus?: boolean;
  mono?: boolean;
}) {
  return (
    <label className="lbl">
      {label} {hint && <span className="text-faint">{hint}</span>}
      <input
        type={type}
        value={value}
        required={required}
        autoFocus={autoFocus}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className={"field" + (mono ? " font-mono" : "")}
      />
    </label>
  );
}

export function Area({
  label,
  hint,
  value,
  onChange,
  placeholder,
  rows = 4,
}: {
  label: string;
  hint?: ReactNode;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  rows?: number;
}) {
  return (
    <label className="lbl">
      {label} {hint && <span className="text-faint">{hint}</span>}
      <textarea
        rows={rows}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="field font-mono text-[13px]"
      />
    </label>
  );
}

export function Select({
  label,
  hint,
  value,
  onChange,
  options,
}: {
  label: string;
  hint?: ReactNode;
  value: string;
  onChange: (v: string) => void;
  options: string[];
}) {
  return (
    <label className="lbl">
      {label} {hint && <span className="text-faint">{hint}</span>}
      <select value={value} onChange={(e) => onChange(e.target.value)} className="field">
        {options.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </select>
    </label>
  );
}

export function Check({
  label,
  checked,
  onChange,
}: {
  label: ReactNode;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label className="flex items-center gap-2.5 text-sm text-dim">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="h-4 w-4 rounded border-line bg-ink text-go focus:ring-go/40"
      />{" "}
      {label}
    </label>
  );
}
