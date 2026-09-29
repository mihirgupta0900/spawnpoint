import React from "react";
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { c, fonts } from "../theme";
import { clamp, FadeUp, useSpring } from "../ui";

export const Logo: React.FC<{ size?: number; progress?: number }> = ({ size = 120, progress = 1 }) => {
  // A spawn point: a core with three branches radiating out.
  const len = 40;
  return (
    <svg width={size} height={size} viewBox="0 0 100 100">
      {[
        { a: -90, col: c.blue },
        { a: 30, col: c.purple },
        { a: 150, col: c.orange },
      ].map(({ a, col }) => {
        const r = (a * Math.PI) / 180;
        const x = 50 + Math.cos(r) * len * progress;
        const y = 50 + Math.sin(r) * len * progress;
        return (
          <g key={a}>
            <line x1={50} y1={50} x2={x} y2={y} stroke={col} strokeWidth={6} strokeLinecap="round" />
            <circle cx={x} cy={y} r={8 * progress} fill={c.bg} stroke={col} strokeWidth={5} />
          </g>
        );
      })}
      <circle cx={50} cy={50} r={13} fill={c.text} />
    </svg>
  );
};

export const TAGLINE = "One branch. Every repo. Ready to code.";
export const SUBTAGLINE = "Isolated multi-repo workspaces for you and your coding agents.";

export const Reveal: React.FC = () => {
  const frame = useCurrentFrame();
  const burst = useSpring(0, { damping: 12, stiffness: 90 });
  const word = useSpring(8);
  return (
    <AbsoluteFill style={{ alignItems: "center", justifyContent: "center" }}>
      {[0, 1, 2].map((i) => {
        const t = interpolate(frame, [i * 6, i * 6 + 45], [0, 1], clamp);
        return (
          <div
            key={i}
            style={{
              position: "absolute",
              width: 200 + t * 1100,
              height: 200 + t * 1100,
              borderRadius: "50%",
              border: `2px solid ${[c.blue, c.purple, c.orange][i]}`,
              opacity: (1 - t) * 0.5,
            }}
          />
        );
      })}
      <div style={{ display: "flex", alignItems: "center", gap: 36, transform: `scale(${0.8 + burst * 0.2})` }}>
        <Logo size={150} progress={burst} />
        <div
          style={{
            fontSize: 150,
            fontWeight: 800,
            letterSpacing: -5,
            opacity: word,
            transform: `translateX(${(1 - word) * -40}px)`,
          }}
        >
          Spawnpoint
        </div>
      </div>
      <FadeUp delay={22} style={{ marginTop: 40 }}>
        <div style={{ fontSize: 50, fontWeight: 600, color: c.text, textAlign: "center" }}>{TAGLINE}</div>
      </FadeUp>
      <FadeUp delay={32} style={{ marginTop: 18 }}>
        <div style={{ fontSize: 32, color: c.dim, textAlign: "center", fontFamily: fonts.sans }}>{SUBTAGLINE}</div>
      </FadeUp>
    </AbsoluteFill>
  );
};
