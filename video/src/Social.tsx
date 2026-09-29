import React from "react";
import { AbsoluteFill } from "remotion";
import { c, fonts } from "./theme";
import { Background } from "./ui";
import { Logo, SUBTAGLINE, TAGLINE } from "./scenes/Reveal";

/** Static 1280x640 GitHub social preview. */
export const Social: React.FC = () => (
  <Background>
    <AbsoluteFill style={{ padding: "0 90px", justifyContent: "center" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 22 }}>
        <Logo size={92} />
        <div style={{ fontSize: 92, fontWeight: 800, letterSpacing: -3 }}>Spawnpoint</div>
      </div>
      <div style={{ fontSize: 44, fontWeight: 700, marginTop: 30 }}>{TAGLINE}</div>
      <div style={{ fontSize: 26, color: c.dim, marginTop: 12 }}>{SUBTAGLINE}</div>
      <div style={{ display: "flex", gap: 14, marginTop: 46, fontFamily: fonts.mono, fontSize: 22 }}>
        {[
          ["api", c.blue],
          ["web", c.purple],
          ["worker", c.orange],
        ].map(([r, col]) => (
          <span key={r} style={{ padding: "8px 18px", borderRadius: 10, border: `1px solid ${col}77`, color: col, background: `${col}14` }}>
            {r}/ ⎇ feat/billing
          </span>
        ))}
      </div>
      <div style={{ position: "absolute", left: 90, bottom: 50, fontFamily: fonts.mono, fontSize: 22, color: c.dim }}>
        <span style={{ color: c.purple }}>$</span> pipx install spawnpoint
      </div>
      <div style={{ position: "absolute", right: 90, bottom: 50, fontSize: 20, color: c.faint }}>
        Claude Code · Codex · Cursor · Gemini CLI
      </div>
    </AbsoluteFill>
  </Background>
);
