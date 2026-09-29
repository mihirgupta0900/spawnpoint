import React from "react";
import { AbsoluteFill } from "remotion";
import { c, fonts } from "../theme";
import { FadeUp, useSpring } from "../ui";
import { Logo, TAGLINE } from "./Reveal";

export const Outro: React.FC = () => {
  const s = useSpring(0, { damping: 13 });
  return (
    <AbsoluteFill style={{ alignItems: "center", justifyContent: "center" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 28, transform: `scale(${0.85 + s * 0.15})`, opacity: s }}>
        <Logo size={110} progress={s} />
        <div style={{ fontSize: 110, fontWeight: 800, letterSpacing: -4 }}>Spawnpoint</div>
      </div>
      <FadeUp delay={10} style={{ marginTop: 26 }}>
        <div style={{ fontSize: 40, color: c.dim, fontWeight: 500 }}>{TAGLINE}</div>
      </FadeUp>
      <FadeUp delay={22} style={{ marginTop: 60 }}>
        <div
          style={{
            fontFamily: fonts.mono,
            fontSize: 38,
            padding: "22px 44px",
            borderRadius: 16,
            background: c.panel,
            border: `1px solid ${c.blue}66`,
            boxShadow: `0 0 60px ${c.blue}22`,
          }}
        >
          <span style={{ color: c.purple }}>$</span> brew install mihirgupta0900/tap/spawnpoint
        </div>
      </FadeUp>
      <FadeUp delay={30} style={{ marginTop: 30 }}>
        <div style={{ display: "flex", gap: 18 }}>
          {["single native binary", "~20 ms startup", "sp update keeps it current"].map((t) => (
            <span
              key={t}
              style={{
                fontFamily: fonts.mono,
                fontSize: 22,
                padding: "8px 20px",
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
      </FadeUp>
      <FadeUp delay={40} style={{ marginTop: 44 }}>
        <div style={{ fontFamily: fonts.mono, fontSize: 28, color: c.dim }}>
          github.com/<span style={{ color: c.text }}>mihirgupta0900/spawnpoint</span>
        </div>
      </FadeUp>
    </AbsoluteFill>
  );
};
