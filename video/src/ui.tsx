import React from "react";
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from "remotion";
import { c, fonts } from "./theme";

export const Background: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const frame = useCurrentFrame();
  const drift = frame * 0.15;
  return (
    <AbsoluteFill style={{ backgroundColor: c.bg, fontFamily: fonts.sans, color: c.text }}>
      <AbsoluteFill
        style={{
          backgroundImage: `radial-gradient(${c.border} 1.2px, transparent 1.2px)`,
          backgroundSize: "36px 36px",
          backgroundPosition: `${drift}px ${drift}px`,
          opacity: 0.45,
          maskImage: "radial-gradient(ellipse at center, black 30%, transparent 80%)",
        }}
      />
      <AbsoluteFill
        style={{
          background: `radial-gradient(circle at 20% 15%, ${c.blue}14, transparent 45%), radial-gradient(circle at 85% 90%, ${c.purple}12, transparent 45%)`,
        }}
      />
      {children}
    </AbsoluteFill>
  );
};

export const useSpring = (delay = 0, config: { damping?: number; mass?: number; stiffness?: number } = {}) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  return spring({ frame: frame - delay, fps, config: { damping: 18, stiffness: 120, ...config } });
};

export const FadeUp: React.FC<{
  delay?: number;
  distance?: number;
  children: React.ReactNode;
  style?: React.CSSProperties;
}> = ({ delay = 0, distance = 30, children, style }) => {
  const s = useSpring(delay);
  return (
    <div style={{ opacity: s, transform: `translateY(${(1 - s) * distance}px)`, ...style }}>{children}</div>
  );
};

export const Eyebrow: React.FC<{ children: React.ReactNode; color?: string }> = ({ children, color = c.blue }) => (
  <div
    style={{
      fontFamily: fonts.mono,
      fontSize: 22,
      letterSpacing: 4,
      textTransform: "uppercase",
      color,
      fontWeight: 500,
    }}
  >
    {children}
  </div>
);

export const Window: React.FC<{
  title?: string;
  children: React.ReactNode;
  style?: React.CSSProperties;
  accent?: string;
}> = ({ title, children, style, accent }) => (
  <div
    style={{
      background: c.panel,
      border: `1px solid ${accent ?? c.border}`,
      borderRadius: 16,
      boxShadow: `0 30px 80px rgba(0,0,0,0.55)${accent ? `, 0 0 0 1px ${accent}33, 0 0 60px ${accent}22` : ""}`,
      overflow: "hidden",
      ...style,
    }}
  >
    <div
      style={{
        height: 44,
        display: "flex",
        alignItems: "center",
        padding: "0 18px",
        gap: 9,
        borderBottom: `1px solid ${c.border}`,
        background: c.bg2,
      }}
    >
      {["#ff5f57", "#febc2e", "#28c840"].map((col) => (
        <div key={col} style={{ width: 13, height: 13, borderRadius: 7, background: col }} />
      ))}
      {title && (
        <div style={{ marginLeft: 14, fontFamily: fonts.mono, fontSize: 17, color: c.dim }}>{title}</div>
      )}
    </div>
    <div style={{ padding: 26, fontFamily: fonts.mono }}>{children}</div>
  </div>
);

/** Types `text` from `start`, `cps` chars per second. */
export const useTyped = (text: string, start: number, cps = 28) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const n = Math.max(0, Math.floor(((frame - start) / fps) * cps));
  return text.slice(0, Math.min(n, text.length));
};

export const Cursor: React.FC<{ color?: string }> = ({ color = c.text }) => {
  const frame = useCurrentFrame();
  const on = Math.floor(frame / 15) % 2 === 0;
  return (
    <span
      style={{
        display: "inline-block",
        width: "0.55em",
        height: "1.1em",
        background: color,
        opacity: on ? 0.9 : 0,
        verticalAlign: "text-bottom",
        marginLeft: 2,
      }}
    />
  );
};

export const Check: React.FC<{ size?: number; color?: string; progress?: number }> = ({
  size = 26,
  color = c.green,
  progress = 1,
}) => (
  <svg width={size} height={size} viewBox="0 0 24 24" style={{ flexShrink: 0 }}>
    <circle cx="12" cy="12" r="11" fill={`${color}22`} stroke={color} strokeWidth="1.5" />
    <path
      d="M7 12.5l3.2 3.2L17 9"
      fill="none"
      stroke={color}
      strokeWidth="2.2"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeDasharray={16}
      strokeDashoffset={16 * (1 - progress)}
    />
  </svg>
);

export const Chip: React.FC<{ color: string; children: React.ReactNode; style?: React.CSSProperties }> = ({
  color,
  children,
  style,
}) => (
  <span
    style={{
      display: "inline-flex",
      alignItems: "center",
      gap: 8,
      padding: "6px 14px",
      borderRadius: 999,
      background: `${color}1f`,
      border: `1px solid ${color}55`,
      color,
      fontFamily: fonts.mono,
      fontSize: 20,
      fontWeight: 500,
      ...style,
    }}
  >
    {children}
  </span>
);

export const BranchIcon: React.FC<{ size?: number; color?: string }> = ({ size = 20, color = c.dim }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth="2" strokeLinecap="round">
    <circle cx="6" cy="5" r="2.5" />
    <circle cx="6" cy="19" r="2.5" />
    <circle cx="18" cy="8" r="2.5" />
    <path d="M6 7.5v9M18 10.5c0 4-6 3-11.2 6.5" />
  </svg>
);

export const FolderIcon: React.FC<{ size?: number; color?: string }> = ({ size = 28, color = c.blue }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill={`${color}33`} stroke={color} strokeWidth="1.6">
    <path d="M3 6.5A1.5 1.5 0 0 1 4.5 5h4.2l2 2.2h8.8A1.5 1.5 0 0 1 21 8.7v9.8a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 18.5z" />
  </svg>
);

export const clamp = { extrapolateLeft: "clamp", extrapolateRight: "clamp" } as const;
export const fadeOut = (frame: number, end: number, len = 12) => interpolate(frame, [end - len, end], [1, 0], clamp);
