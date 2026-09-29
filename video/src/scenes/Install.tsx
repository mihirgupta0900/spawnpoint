import React from "react";
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { c, fonts } from "../theme";
import { clamp, Eyebrow, FadeUp, useSpring, Window } from "../ui";
import type { Sfx } from "./Demo";

const METHODS = [
  { label: "Homebrew", cmd: "brew install mihirgupta0900/tap/spawnpoint", color: c.orange },
  { label: "Script", cmd: "curl -fsSL …/install.sh | sh", color: c.blue },
  { label: "pipx · uv", cmd: "pipx install spawnpoint", color: c.purple },
];

const UPDATE = [
  { f: 70, node: "cmd" },
  { f: 92, node: "installed" },
  { f: 98, node: "latest" },
  { f: 108, node: "running" },
  { f: 150, node: "done" },
] as const;

export const INSTALL_LENGTH = 220;

export const INSTALL_SFX: Sfx[] = [
  ...METHODS.map((_, i) => ({ f: 14 + i * 8, kind: "pop" as const })),
  ..."sp update".split("").map((_, i) => ({ f: 72 + i * 2, kind: "click" as const })),
  { f: 90, kind: "enter" },
  { f: 150, kind: "tick" },
  { f: 170, kind: "pop" },
];

const Card: React.FC<{ m: (typeof METHODS)[number]; i: number }> = ({ m, i }) => {
  const s = useSpring(14 + i * 8, { damping: 15 });
  return (
    <div
      style={{
        width: 740,
        padding: "24px 30px",
        borderRadius: 18,
        background: c.panel,
        border: `1px solid ${c.border}`,
        borderLeft: `4px solid ${m.color}`,
        opacity: s,
        transform: `translateX(${(1 - s) * -50}px)`,
      }}
    >
      <div style={{ fontSize: 22, color: m.color, fontWeight: 700, letterSpacing: 1 }}>{m.label}</div>
      <div style={{ fontFamily: fonts.mono, fontSize: 24, marginTop: 10, whiteSpace: "nowrap" }}>
        <span style={{ color: c.purple }}>$ </span>
        {m.cmd}
      </div>
    </div>
  );
};

const SPIN = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];

export const Install: React.FC = () => {
  const frame = useCurrentFrame();
  const typed = "sp update".slice(0, Math.max(0, Math.floor((frame - 72) / 2) + 1));
  const at = (n: (typeof UPDATE)[number]["node"]) => frame >= UPDATE.find((u) => u.node === n)!.f;
  const chips = interpolate(frame, [170, 188], [0, 1], clamp);
  const line = { height: 42, lineHeight: "42px", whiteSpace: "pre" as const };

  return (
    <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", gap: 56 }}>
      <FadeUp>
        <div style={{ textAlign: "center" }}>
          <Eyebrow color={c.green}>Install &amp; update</Eyebrow>
          <div style={{ fontSize: 62, fontWeight: 800, letterSpacing: -1.5, marginTop: 14 }}>
            One binary. <span style={{ color: c.dim }}>Updates itself, however you installed it.</span>
          </div>
        </div>
      </FadeUp>
      <div style={{ display: "flex", gap: 44, alignItems: "center" }}>
        <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
          {METHODS.map((m, i) => (
            <Card key={m.label} m={m} i={i} />
          ))}
        </div>
        <FadeUp delay={50}>
          <Window title="zsh" style={{ width: 700 }}>
            <div style={{ height: 300, fontFamily: fonts.mono, fontSize: 25, color: c.text }}>
              {at("cmd") && (
                <div style={line}>
                  <span style={{ color: c.purple }}>❯ </span>
                  {typed}
                  {!at("installed") && <span style={{ color: c.purple }}>▏</span>}
                </div>
              )}
              {at("installed") && (
                <div style={line}>
                  <span style={{ color: c.dim }}>installed </span>1.0.0<span style={{ color: c.dim }}>  via </span>homebrew
                </div>
              )}
              {at("latest") && (
                <div style={line}>
                  <span style={{ color: c.dim }}>latest    </span>1.1.0
                </div>
              )}
              {at("running") && !at("done") && (
                <div style={line}>
                  <span style={{ color: c.purple }}>{SPIN[Math.floor(frame / 2) % SPIN.length]} </span>
                  <span style={{ color: c.dim }}>running </span>
                  <span style={{ fontWeight: 700 }}>brew upgrade spawnpoint</span>
                </div>
              )}
              {at("done") && (
                <>
                  <div style={line}>
                    <span style={{ color: c.dim }}>running </span>
                    <span style={{ fontWeight: 700 }}>brew upgrade spawnpoint</span>
                  </div>
                  <div style={line}>
                    <span style={{ color: c.green }}>✓ </span>Updated to 1.1.0
                  </div>
                </>
              )}
            </div>
          </Window>
        </FadeUp>
      </div>
      <div style={{ display: "flex", gap: 22, opacity: chips, transform: `translateY(${(1 - chips) * 16}px)` }}>
        {["~20 ms startup", "no Python required", "macOS · Linux · Windows"].map((t) => (
          <span
            key={t}
            style={{
              fontFamily: fonts.mono,
              fontSize: 24,
              padding: "10px 22px",
              borderRadius: 999,
              border: `1px solid ${c.green}55`,
              background: `${c.green}14`,
              color: c.green,
            }}
          >
            {t}
          </span>
        ))}
      </div>
    </AbsoluteFill>
  );
};
