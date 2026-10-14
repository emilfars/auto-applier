import type { CSSProperties } from "react";

export type IllustrationName = "jobs" | "search" | "documents" | "tracker" | "secure" | "error";

const paper: CSSProperties = { fill: "var(--illo-paper)", stroke: "var(--illo-line)", strokeWidth: 1.5 };
const line: CSSProperties = { fill: "var(--illo-line)", opacity: 0.55 };
const ink: CSSProperties = { fill: "var(--illo-ink)" };
const inkStroke: CSSProperties = { fill: "none", stroke: "var(--illo-ink)", strokeWidth: 6, strokeLinecap: "round" };
const success: CSSProperties = { fill: "var(--brand-success)" };
const warning: CSSProperties = { fill: "var(--brand-warning)" };
const danger: CSSProperties = { fill: "var(--brand-danger)" };
const glow: CSSProperties = { fill: "var(--illo-ink)", opacity: 0.12 };

function Bars({ x, y, widths }: { x: number; y: number; widths: number[] }) {
  return (
    <>
      {widths.map((w, i) => (
        <rect key={i} x={x} y={y + i * 12} width={w} height={6} rx={3} style={line} />
      ))}
    </>
  );
}

const ART: Record<IllustrationName, JSX.Element> = {
  jobs: (
    <>
      <circle cx="130" cy="92" r="78" style={glow} />
      <rect x="40" y="34" width="150" height="112" rx="12" style={paper} />
      <rect x="40" y="34" width="150" height="20" rx="10" style={{ ...line, opacity: 0.25 }} />
      <circle cx="54" cy="44" r="3" style={danger} />
      <circle cx="64" cy="44" r="3" style={warning} />
      <circle cx="74" cy="44" r="3" style={success} />
      <rect x="54" y="66" width="22" height="22" rx="6" style={ink} />
      <Bars x={84} y={68} widths={[72, 48]} />
      <rect x="54" y="100" width="22" height="22" rx="6" style={success} />
      <Bars x={84} y={102} widths={[60, 80]} />
      <g className="float">
        <path d="M196 30 L236 14 L224 58 L212 46 L200 56 L204 40 Z" style={ink} />
        <path d="M212 46 L236 14 L204 40" style={{ fill: "none", stroke: "var(--illo-paper)", strokeWidth: 2 }} />
      </g>
    </>
  ),
  search: (
    <>
      <circle cx="120" cy="90" r="70" style={glow} />
      <rect x="54" y="44" width="110" height="34" rx="8" style={paper} />
      <Bars x={66} y={54} widths={[60, 40]} />
      <rect x="54" y="88" width="110" height="34" rx="8" style={{ ...paper, opacity: 0.6 }} />
      <circle cx="160" cy="112" r="26" style={{ fill: "none", stroke: "var(--illo-ink)", strokeWidth: 7 }} />
      <path d="M179 131 L200 152" style={inkStroke} />
    </>
  ),
  documents: (
    <>
      <circle cx="120" cy="92" r="72" style={glow} />
      <rect
        x="62"
        y="40"
        width="90"
        height="116"
        rx="10"
        transform="rotate(-8 107 98)"
        style={{ ...paper, opacity: 0.6 }}
      />
      <rect x="84" y="30" width="90" height="116" rx="10" style={paper} />
      <circle cx="106" cy="56" r="10" style={ink} />
      <Bars x={98} y={78} widths={[62, 48, 56, 36]} />
      <circle cx="170" cy="138" r="18" className="pop" style={success} />
      <path
        d="M162 138 L168 144 L179 132"
        style={{
          fill: "none",
          stroke: "var(--illo-paper)",
          strokeWidth: 3.5,
          strokeLinecap: "round",
          strokeLinejoin: "round",
        }}
      />
    </>
  ),
  tracker: (
    <>
      <circle cx="120" cy="90" r="72" style={glow} />
      <rect x="56" y="32" width="128" height="120" rx="12" style={paper} />
      {[0, 1, 2].map((i) => (
        <g key={i}>
          <rect
            x="70"
            y={50 + i * 32}
            width="16"
            height="16"
            rx="4"
            style={i < 2 ? success : { ...line, opacity: 0.4 }}
          />
          <Bars x={96} y={52 + i * 32} widths={[70 - i * 12]} />
        </g>
      ))}
      <rect x="150" y="118" width="54" height="40" rx="10" className="float" style={ink} />
      <path
        d="M162 140 L172 130 L182 136 L194 126"
        style={{ fill: "none", stroke: "var(--illo-paper)", strokeWidth: 3, strokeLinecap: "round" }}
      />
    </>
  ),
  secure: (
    <>
      <circle cx="120" cy="90" r="74" style={glow} />
      <path d="M120 26 L170 44 V92 C170 124 148 146 120 158 C92 146 70 124 70 92 V44 Z" style={paper} />
      <path d="M120 42 L156 55 V92 C156 115 141 132 120 142 C99 132 84 115 84 92 V55 Z" style={ink} />
      <path
        d="M104 92 L116 104 L138 80"
        style={{
          fill: "none",
          stroke: "var(--illo-paper)",
          strokeWidth: 7,
          strokeLinecap: "round",
          strokeLinejoin: "round",
        }}
      />
      <circle cx="186" cy="50" r="8" className="float" style={success} />
      <circle cx="56" cy="130" r="6" className="float" style={warning} />
    </>
  ),
  error: (
    <>
      <circle cx="120" cy="90" r="72" style={glow} />
      <rect x="56" y="40" width="128" height="96" rx="12" style={paper} />
      <path d="M120 62 L146 110 H94 Z" style={warning} />
      <rect x="118" y="78" width="4" height="16" rx="2" style={{ fill: "var(--brand-on-state)" }} />
      <circle cx="120" cy="101" r="2.5" style={{ fill: "var(--brand-on-state)" }} />
    </>
  ),
};

export function Illustration({ name, width = 240 }: { name: IllustrationName; width?: number }) {
  return (
    <svg className="illustration" viewBox="0 0 240 180" width={width} aria-hidden="true" focusable="false">
      {ART[name]}
    </svg>
  );
}
