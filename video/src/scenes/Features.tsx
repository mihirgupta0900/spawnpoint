import React from "react";
import { AbsoluteFill } from "remotion";
import { c, fonts } from "../theme";
import { Eyebrow, FadeUp, useSpring } from "../ui";

const FEATURES = [
  { cmd: "npx skills add mihirgupta0900/spawnpoint", title: "Agent skill", body: "Claude Code, Codex & co. learn the non-interactive flags.", color: c.orange },
  { cmd: "--no-input --json", title: "Scriptable", body: "Every command runs headless with machine-readable output.", color: c.blue },
  { cmd: "sp create -t billing", title: "Templates", body: "Save repo sets you always spawn together.", color: c.purple },
  { cmd: "sp add", title: "Grow a workspace", body: "Pull in another repo mid-task, same branch.", color: c.green },
  { cmd: "sp light-cleanup", title: "Reclaim disk", body: "Drop node_modules & .venv, keep the code.", color: c.yellow },
  { cmd: "sp cleanup", title: "Clean exits", body: "Remove workspaces and their branches in one go.", color: c.pink },
];

const Card: React.FC<{ f: (typeof FEATURES)[number]; i: number }> = ({ f, i }) => {
  const s = useSpring(12 + i * 6, { damping: 15 });
  return (
    <div
      style={{
        width: 500,
        height: 220,
        padding: 32,
        borderRadius: 20,
        background: c.panel,
        border: `1px solid ${c.border}`,
        borderTop: `3px solid ${f.color}`,
        opacity: s,
        transform: `translateY(${(1 - s) * 40}px) scale(${0.92 + s * 0.08})`,
      }}
    >
      <div style={{ fontFamily: fonts.mono, fontSize: 17, color: f.color, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{f.cmd}</div>
      <div style={{ fontSize: 38, fontWeight: 700, marginTop: 18 }}>{f.title}</div>
      <div style={{ fontSize: 24, color: c.dim, marginTop: 10, lineHeight: 1.35 }}>{f.body}</div>
    </div>
  );
};

export const Features: React.FC = () => (
  <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", gap: 56 }}>
    <FadeUp>
      <div style={{ textAlign: "center" }}>
        <Eyebrow color={c.purple}>And the rest</Eyebrow>
        <div style={{ fontSize: 64, fontWeight: 800, letterSpacing: -1.5, marginTop: 14 }}>
          Works with any agent, editor, or terminal.
        </div>
      </div>
    </FadeUp>
    <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 500px)", gap: 30 }}>
      {FEATURES.map((f, i) => (
        <Card key={f.title} f={f} i={i} />
      ))}
    </div>
  </AbsoluteFill>
);
