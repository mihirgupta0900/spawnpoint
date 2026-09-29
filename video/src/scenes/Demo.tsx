import React from "react";
import { AbsoluteFill, interpolate, useCurrentFrame } from "remotion";
import { c, fonts, repoColors } from "../theme";
import { Check, clamp, Cursor, Eyebrow, FadeUp, useSpring, useTyped, Window } from "../ui";

const REPOS = ["api", "billing-docs", "infra", "web", "worker"];
const PICK: Record<string, number> = { api: 44, web: 62, worker: 78 };
const CURSOR_AT: [number, number][] = [
  [0, 0],
  [40, 0],
  [52, 3],
  [70, 4],
];

const STEPS: Record<string, string[]> = {
  api: ["worktree", ".env · CLAUDE.md", "uv sync"],
  web: ["worktree", ".env.local · CLAUDE.md", "pnpm install"],
  worker: ["worktree + submodules", ".env", "npm install"],
};

const T = {
  cmd: 4,
  picker: 30,
  confirm: 92,
  branch: 100,
  run: 140,
  perRepo: 32,
  done: 245,
};

const Prompt: React.FC<{ dir: string; children?: React.ReactNode }> = ({ dir, children }) => (
  <div style={{ fontSize: 28, lineHeight: "46px" }}>
    <span style={{ color: c.blue }}>{dir}</span> <span style={{ color: c.purple }}>❯</span> {children}
  </div>
);

const RepoRow: React.FC<{ name: string; start: number }> = ({ name, start }) => {
  const frame = useCurrentFrame();
  const s = useSpring(start);
  const steps = STEPS[name];
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 22, fontSize: 26, lineHeight: "50px", opacity: s, transform: `translateX(${(1 - s) * -20}px)` }}>
      <span style={{ width: 130, color: repoColors[name], fontWeight: 700 }}>{name}</span>
      {steps.map((st, i) => {
        const p = interpolate(frame, [start + 4 + i * 8, start + 12 + i * 8], [0, 1], clamp);
        return (
          <span key={st} style={{ display: "inline-flex", alignItems: "center", gap: 10, opacity: 0.3 + p * 0.7, marginRight: 14 }}>
            <Check size={26} progress={p} />
            <span style={{ color: p > 0.5 ? c.text : c.dim }}>{st}</span>
          </span>
        );
      })}
    </div>
  );
};

export const Demo: React.FC = () => {
  const frame = useCurrentFrame();
  const cmd = useTyped("sp create", T.cmd, 14);
  const branch = useTyped("feat/billing", T.branch, 24);
  const agentCmd = useTyped("claude", T.done + 22, 12);
  const cursorIdx = CURSOR_AT.reduce((acc, [f, i]) => (frame >= f ? i : acc), 0);
  const inPicker = frame >= T.picker && frame < T.confirm;
  const selected = Object.entries(PICK)
    .filter(([, f]) => frame >= f)
    .map(([r]) => r);

  return (
    <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", gap: 40 }}>
      <FadeUp>
        <Eyebrow>sp create</Eyebrow>
      </FadeUp>
      <Window title="~/code" style={{ width: 1500 }}>
        <div style={{ height: 560 }}>
          <Prompt dir="~/code">
            {cmd}
            {frame < T.picker && <Cursor />}
          </Prompt>

          {frame >= T.picker && (
            <div style={{ fontSize: 26, lineHeight: "42px", marginTop: 6 }}>
              <div>
                <span style={{ color: c.green }}>?</span> Select repositories <span style={{ color: c.dim }}>(type to search)</span>
                {frame >= T.confirm && <span style={{ color: c.blue }}> {selected.join(", ")}</span>}
              </div>
              {inPicker &&
                REPOS.map((r, i) => {
                  const on = selected.includes(r);
                  const here = i === cursorIdx;
                  return (
                    <div key={r} style={{ display: "flex", gap: 14, background: here ? `${c.blue}14` : "transparent", borderRadius: 6, paddingLeft: 8 }}>
                      <span style={{ color: here ? c.blue : "transparent" }}>❯</span>
                      <span style={{ color: on ? c.green : c.faint }}>{on ? "◉" : "○"}</span>
                      <span style={{ color: on ? repoColors[r] ?? c.text : c.dim }}>{r}</span>
                    </div>
                  );
                })}
            </div>
          )}

          {frame >= T.branch && (
            <div style={{ fontSize: 26, lineHeight: "42px" }}>
              <span style={{ color: c.green }}>?</span> Branch name: <span style={{ color: c.blue }}>{branch}</span>
              {frame < T.run - 10 && <Cursor />}
            </div>
          )}

          {frame >= T.run && (
            <div style={{ marginTop: 20 }}>
              {["api", "web", "worker"].map((r, i) => (
                <RepoRow key={r} name={r} start={T.run + i * T.perRepo} />
              ))}
            </div>
          )}

          {frame >= T.done && (
            <FadeUp delay={T.done} distance={12}>
              <div style={{ fontSize: 28, lineHeight: "46px", marginTop: 22 }}>
                <span style={{ color: c.green, fontWeight: 700 }}>Done!</span>{" "}
                <span style={{ color: c.dim }}>Workspace:</span>{" "}
                <span style={{ color: c.blue, fontWeight: 700 }}>~/.spawnpoint/workspaces/feat-billing</span>
              </div>
              <Prompt dir="~/.spawnpoint/workspaces/feat-billing">
                <span style={{ color: c.text }}>{agentCmd}</span>
                <Cursor />
              </Prompt>
            </FadeUp>
          )}
        </div>
      </Window>
    </AbsoluteFill>
  );
};

export const DEMO_LENGTH = T.done + 70;
export { fonts };
