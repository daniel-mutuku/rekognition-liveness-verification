// Always paired with text that says the same, so hidden from screen readers.

interface IconProps {
  className?: string;
}

const common = {
  width: "1em",
  height: "1em",
  viewBox: "0 0 24 24",
  fill: "none",
  "aria-hidden": true,
  focusable: false,
} as const;

export function TickIcon({ className }: IconProps) {
  return (
    <svg {...common} className={className}>
      <path d="M5 12.5l4.5 4.5L19 7.5" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function HalfCircleIcon({ className }: IconProps) {
  return (
    <svg {...common} className={className}>
      <circle cx="12" cy="12" r="8.5" stroke="currentColor" strokeWidth="2.5" />
      <path d="M12 3.5a8.5 8.5 0 0 1 0 17z" fill="currentColor" />
    </svg>
  );
}

export function CrossIcon({ className }: IconProps) {
  return (
    <svg {...common} className={className}>
      <path d="M7 7l10 10M17 7L7 17" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}
