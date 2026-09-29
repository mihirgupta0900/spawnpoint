import React from "react";
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { c, fonts, repoColors } from "../theme";
import { BranchIcon, clamp, FadeUp, useSpring, Window } from "../ui";

const RepoCard: React.FC<{ name: string; delay: number }> = ({ name, delay }) => {
  const s = useSpring(delay, { damping: 14 });
  const col = repoColors[name];
  return (
    <div
      style={{
        width: 300,
        padding: "30px 32px",
        borderRadius: 20,
        background: c.panel,
        border: `1px solid ${col}66`,
        boxShadow: `0 0 50px ${col}22`,
        transform: `scale(${0.6 + s * 0.4}) translateY(${(1 - s) * 60}px)`,
        opacity: s,
      }}
    >
      <div style={{ fontFamily: fonts.mono, fontSize: 40, fontWeight: 700, color: col }}>{name}</div>
      <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14, color: c.dim, fontSize: 22, fontFamily: fonts.mono }}>
        <BranchIcon /> main
      </div>
    </div>
  );
};

/** "Your feature touches 3 repos." */
export const Hook: React.FC = () => (
  <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", gap: 70 }}>
    <FadeUp>
      <div style={{ fontSize: 84, fontWeight: 800, letterSpacing: -2, textAlign: "center" }}>
        One feature. <span style={{ color: c.dim }}>Three repos.</span>
      </div>
    </FadeUp>
    <div style={{ display: "flex", gap: 44 }}>
      {["api", "web", "worker"].map((r, i) => (
        <RepoCard key={r} name={r} delay={14 + i * 7} />
      ))}
    </div>
  </AbsoluteFill>
);

const CHORE = [
  { cmd: "cd ~/code/api && git fetch", tag: "api" },
  { cmd: "git worktree add ../api-billing -b feat/billing origin/main", tag: "api" },
  { cmd: "cp .env .env.local CLAUDE.md ../api-billing/", tag: "api" },
  { cmd: "cd ../api-billing && uv sync", tag: "api" },
  { cmd: "cd ~/code/web && git fetch", tag: "web" },
  { cmd: "git worktree add ../web-billing -b feat/billing origin/main", tag: "web" },
  { cmd: "cp .env* CLAUDE.md ../web-billing/", tag: "web" },
  { cmd: "cd ../web-billing && pnpm install", tag: "web" },
  { cmd: "cd ~/code/worker && git fetch", tag: "worker" },
  { cmd: "git worktree add ../worker-billing -b feat/billing …", tag: "worker" },
  { cmd: "git submodule update --init --recursive", tag: "worker" },
  { cmd: "cp .env ../worker-billing/  # wait, which .env?", tag: "worker" },
  { cmd: "cd ../worker-billing && npm install", tag: "worker" },
];

/** The manual chore, cascading. */
export const Chore: React.FC = () => {
  const frame = useCurrentFrame();
  const perLine = 5;
  const visible = Math.floor((frame - 10) / perLine);
  const minutes = interpolate(frame, [10, 10 + CHORE.length * perLine], [0, 14], clamp);
  const headline = useSpring(10 + CHORE.length * perLine + 6);
  const dimTerm = interpolate(headline, [0, 1], [1, 0.25]);

  return (
    <AbsoluteFill style={{ alignItems: "center", justifyContent: "center" }}>
      <div style={{ position: "relative", opacity: dimTerm, filter: `blur(${(1 - dimTerm) * 6}px)`, transform: `scale(${1 - (1 - dimTerm) * 0.05})` }}>
        <Window title="zsh — setting up feat/billing" style={{ width: 1400 }}>
          <div style={{ height: 600, overflow: "hidden", display: "flex", flexDirection: "column", justifyContent: "flex-end" }}>
            {CHORE.slice(0, Math.max(0, visible)).map((l, i) => (
              <div key={i} style={{ fontSize: 27, lineHeight: "44px", whiteSpace: "nowrap" }}>
                <span style={{ color: repoColors[l.tag] }}>❯ </span>
                <span style={{ color: c.text }}>{l.cmd}</span>
              </div>
            ))}
          </div>
        </Window>
        <div
          style={{
            position: "absolute",
            right: -30,
            top: -40,
            background: c.red,
            color: "#fff",
            borderRadius: 14,
            padding: "12px 22px",
            fontFamily: fonts.mono,
            fontSize: 30,
            fontWeight: 700,
            transform: "rotate(3deg)",
            boxShadow: `0 10px 40px ${c.red}55`,
          }}
        >
          ⏱ {Math.floor(minutes).toString().padStart(2, "0")}:{Math.floor((minutes % 1) * 60).toString().padStart(2, "0")} of setup
        </div>
      </div>
      <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", opacity: headline }}>
        <div style={{ textAlign: "center", transform: `translateY(${(1 - headline) * 30}px)` }}>
          <div style={{ fontSize: 88, fontWeight: 800, letterSpacing: -2 }}>Every feature.</div>
          <div style={{ fontSize: 88, fontWeight: 800, letterSpacing: -2, color: c.dim }}>Every agent. Every time.</div>
        </div>
      </AbsoluteFill>
    </AbsoluteFill>
  );
};
