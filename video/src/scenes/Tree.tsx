import React from "react";
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { c, fonts, repoColors } from "../theme";
import { BranchIcon, clamp, Eyebrow, FadeUp, FolderIcon, useSpring } from "../ui";

const REPOS = [
  { name: "api", files: [".env", "CLAUDE.md", ".venv"] },
  { name: "web", files: [".env.local", "CLAUDE.md", "node_modules"] },
  { name: "worker", files: [".env", "submodules", "node_modules"] },
];

const ROW_H = 150;
const TOP = 120;

const Source: React.FC<{ name: string; i: number }> = ({ name, i }) => {
  const s = useSpring(6 + i * 4);
  return (
    <div
      style={{
        position: "absolute",
        left: 0,
        top: TOP + i * ROW_H,
        width: 300,
        height: 100,
        borderRadius: 16,
        border: `1px solid ${c.border}`,
        background: c.panel,
        display: "flex",
        alignItems: "center",
        gap: 16,
        padding: "0 24px",
        opacity: s,
      }}
    >
      <FolderIcon color={c.dim} />
      <div>
        <div style={{ fontFamily: fonts.mono, fontSize: 30, fontWeight: 700, color: c.text }}>{name}</div>
        <div style={{ fontFamily: fonts.mono, fontSize: 19, color: c.dim, display: "flex", gap: 8, alignItems: "center" }}>
          <BranchIcon size={16} /> main
        </div>
      </div>
    </div>
  );
};

const Target: React.FC<{ name: string; files: string[]; i: number }> = ({ name, files, i }) => {
  const frame = useCurrentFrame();
  const start = 40 + i * 12;
  const s = useSpring(start, { damping: 14 });
  const col = repoColors[name];
  return (
    <div
      style={{
        position: "absolute",
        left: 800,
        top: TOP + i * ROW_H - 10,
        width: 740,
        height: 120,
        borderRadius: 18,
        border: `1px solid ${col}77`,
        background: c.panel,
        boxShadow: `0 0 50px ${col}1f`,
        display: "flex",
        alignItems: "center",
        gap: 20,
        padding: "0 28px",
        opacity: s,
        transform: `translateX(${(1 - s) * 60}px)`,
      }}
    >
      <FolderIcon color={col} size={36} />
      <div style={{ flex: 1 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
          <span style={{ fontFamily: fonts.mono, fontSize: 32, fontWeight: 700, color: col }}>{name}/</span>
          <span style={{ fontFamily: fonts.mono, fontSize: 19, color: c.green, display: "flex", gap: 8, alignItems: "center" }}>
            <BranchIcon size={16} color={c.green} /> feat/billing
          </span>
        </div>
        <div style={{ display: "flex", gap: 10, marginTop: 10 }}>
          {files.map((f, j) => {
            const p = interpolate(frame, [start + 14 + j * 6, start + 22 + j * 6], [0, 1], clamp);
            return (
              <span
                key={f}
                style={{
                  fontFamily: fonts.mono,
                  fontSize: 18,
                  padding: "3px 12px",
                  borderRadius: 8,
                  background: `${c.text}0d`,
                  border: `1px solid ${c.border}`,
                  color: c.dim,
                  opacity: p,
                  transform: `scale(${0.8 + p * 0.2})`,
                }}
              >
                ✓ {f}
              </span>
            );
          })}
        </div>
      </div>
    </div>
  );
};

export const Tree: React.FC = () => {
  const frame = useCurrentFrame();
  const header = useSpring(26);
  return (
    <AbsoluteFill style={{ alignItems: "center", justifyContent: "center" }}>
      <div style={{ position: "absolute", top: 90, width: "100%", textAlign: "center" }}>
        <FadeUp>
          <Eyebrow>What you get</Eyebrow>
          <div style={{ fontSize: 64, fontWeight: 800, letterSpacing: -1.5, marginTop: 14 }}>
            One folder per task. <span style={{ color: c.dim }}>Every repo on the same branch.</span>
          </div>
        </FadeUp>
      </div>
      <div style={{ position: "relative", width: 1540, height: 620, marginTop: 170 }}>
        <div style={{ position: "absolute", left: 0, top: 50, fontFamily: fonts.mono, fontSize: 24, color: c.dim }}>~/code</div>
        <div
          style={{
            position: "absolute",
            left: 800,
            top: 50,
            fontFamily: fonts.mono,
            fontSize: 24,
            color: c.blue,
            opacity: header,
          }}
        >
          ~/.spawnpoint/workspaces/<span style={{ fontWeight: 700 }}>feat-billing/</span>
        </div>
        <svg style={{ position: "absolute", left: 0, top: 0 }} width={1540} height={620}>
          {REPOS.map((r, i) => {
            const y1 = TOP + i * ROW_H + 50;
            const y2 = TOP + i * ROW_H + 50;
            const p = interpolate(frame, [28 + i * 12, 52 + i * 12], [0, 1], clamp);
            const d = `M 300 ${y1} C 550 ${y1}, 550 ${y2}, 800 ${y2}`;
            return (
              <path
                key={r.name}
                d={d}
                fill="none"
                stroke={repoColors[r.name]}
                strokeWidth={3}
                strokeDasharray="520"
                strokeDashoffset={520 * (1 - p)}
                opacity={0.8}
              />
            );
          })}
        </svg>
        {REPOS.map((r, i) => (
          <Source key={r.name} name={r.name} i={i} />
        ))}
        {REPOS.map((r, i) => (
          <Target key={r.name} name={r.name} files={r.files} i={i} />
        ))}
        <div style={{ position: "absolute", left: 0, top: TOP + 3 * ROW_H + 10, fontSize: 24, color: c.dim, width: 320, opacity: interpolate(frame, [90, 110], [0, 1], clamp) }}>
          Your checkouts stay untouched.
        </div>
        <div style={{ position: "absolute", left: 800, top: TOP + 3 * ROW_H + 10, fontSize: 24, color: c.dim, opacity: interpolate(frame, [96, 116], [0, 1], clamp) }}>
          Worktrees + env files + deps installed. Ready to run.
        </div>
      </div>
    </AbsoluteFill>
  );
};
