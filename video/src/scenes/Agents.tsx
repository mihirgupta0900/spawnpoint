import React from "react";
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { c, fonts } from "../theme";
import { Check, clamp, Eyebrow, FadeUp, useSpring, useTyped, Window } from "../ui";

const AGENTS = [
  {
    name: "Claude Code",
    color: "#d97757",
    branch: "feat/billing",
    repos: "api,web",
    edits: ["api/billing/invoice.py", "web/app/billing/page.tsx", "api/tests/test_invoice.py"],
  },
  {
    name: "Codex",
    color: "#10a37f",
    branch: "fix/auth-timeout",
    repos: "api,worker",
    edits: ["api/auth/session.py", "worker/jobs/refresh.ts", "api/auth/middleware.py"],
  },
  {
    name: "Gemini CLI",
    color: "#4f8cf7",
    branch: "feat/search",
    repos: "web,worker,infra",
    edits: ["web/components/Search.tsx", "worker/index/build.ts", "infra/search.tf"],
  },
];

const AgentPane: React.FC<{ a: (typeof AGENTS)[number]; i: number }> = ({ a, i }) => {
  const frame = useCurrentFrame();
  const delay = 20 + i * 10;
  const s = useSpring(delay);
  const cmd = useTyped(`spawnpoint create --no-input --json --repos ${a.repos} --branch ${a.branch}`, delay + 12, 60);
  const jsonAt = delay + 70;
  const editsAt = jsonAt + 24;
  const slug = a.branch.replace("/", "-");
  return (
    <div style={{ opacity: s, transform: `translateY(${(1 - s) * 50}px)` }}>
      <Window accent={a.color} style={{ width: 540, height: 560 }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: -8, marginBottom: 16 }}>
          <span style={{ fontFamily: fonts.sans, fontSize: 28, fontWeight: 700, color: a.color }}>{a.name}</span>
          <span style={{ fontSize: 18, color: c.green }}>⎇ {a.branch}</span>
        </div>
        <div style={{ fontSize: 18, lineHeight: "28px", color: c.text, minHeight: 84, wordBreak: "break-all" }}>
          <span style={{ color: a.color }}>$ </span>
          {cmd}
        </div>
        {frame >= jsonAt && (
          <div
            style={{
              fontSize: 17,
              lineHeight: "26px",
              marginTop: 10,
              padding: 14,
              borderRadius: 10,
              background: c.bg,
              color: c.dim,
              opacity: interpolate(frame, [jsonAt, jsonAt + 8], [0, 1], clamp),
            }}
          >
            {"{ "}
            <span style={{ color: c.blue }}>"workspace"</span>: <span style={{ color: c.green }}>"~/…/workspaces/{slug}"</span>,
            <br />
            {"  "}
            <span style={{ color: c.blue }}>"repos"</span>: [ … <span style={{ color: c.green }}>"created"</span> ] {"}"}
          </div>
        )}
        <div style={{ marginTop: 18 }}>
          {a.edits.map((e, j) => {
            const at = editsAt + j * 16;
            const p = interpolate(frame, [at, at + 10], [0, 1], clamp);
            return (
              <div key={e} style={{ display: "flex", alignItems: "center", gap: 12, fontSize: 19, lineHeight: "40px", opacity: p }}>
                <Check size={22} color={a.color} progress={p} />
                <span style={{ color: c.dim }}>edited</span>
                <span style={{ color: c.text }}>{e}</span>
              </div>
            );
          })}
        </div>
      </Window>
    </div>
  );
};

export const Agents: React.FC = () => {
  const frame = useCurrentFrame();
  const pills = interpolate(frame, [190, 210], [0, 1], clamp);
  return (
    <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", gap: 50 }}>
      <FadeUp>
        <div style={{ textAlign: "center" }}>
          <Eyebrow color={c.orange}>Built for coding agents</Eyebrow>
          <div style={{ fontSize: 68, fontWeight: 800, letterSpacing: -1.5, marginTop: 14 }}>
            Let your agents spawn their own workspaces.
          </div>
        </div>
      </FadeUp>
      <div style={{ display: "flex", gap: 36 }}>
        {AGENTS.map((a, i) => (
          <AgentPane key={a.name} a={a} i={i} />
        ))}
      </div>
      <div style={{ display: "flex", gap: 50, fontSize: 30, color: c.text, opacity: pills, transform: `translateY(${(1 - pills) * 20}px)` }}>
        <span><span style={{ color: c.green }}>●</span> Own branch per task</span>
        <span><span style={{ color: c.green }}>●</span> Own .env + deps</span>
        <span><span style={{ color: c.green }}>●</span> No stepping on each other</span>
      </div>
    </AbsoluteFill>
  );
};
