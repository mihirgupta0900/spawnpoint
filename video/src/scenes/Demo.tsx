import React from "react";
import { AbsoluteFill, useCurrentFrame } from "remotion";
import { c, fonts, repoColors } from "../theme";
import { Eyebrow, FadeUp, Window } from "../ui";

// A frame-accurate re-creation of `sp create` in the Go CLI. Everything is
// driven by the timeline below, which also feeds the sound effects
// (DEMO_SFX), so key clicks and ticks land exactly on what's drawn.

const REPOS = ["api", "billing-docs", "design-system", "infra", "mobile", "web", "worker"];

const T = {
  cmd: 4, // "sp create" starts typing
  found: 24,
  picker: 28,
  pickerDone: 86,
  branch: 90,
  branchDone: 122,
  fetch: 125,
  fetchDone: 145,
  base: 149,
  baseDone: 165,
  plan: 169,
  proceed: 184,
  proceedDone: 196,
  build: 200,
  npmDone: 232,
  done: 252,
  claude: 262,
};
export const DEMO_LENGTH = 300;

// Picker keystrokes: text is typed, "tab" toggles, "enter" confirms.
const PICKER_KEYS: { f: number; k: string }[] = [
  { f: 34, k: "a" },
  { f: 37, k: "p" },
  { f: 43, k: "tab" },
  { f: 50, k: "w" },
  { f: 53, k: "o" },
  { f: 56, k: "r" },
  { f: 62, k: "tab" },
  { f: 69, k: "w" },
  { f: 72, k: "e" },
  { f: 78, k: "tab" },
  { f: T.pickerDone, k: "enter" },
];
const BRANCH = "feat/billing";
const typedAt = (start: number, text: string, every: number) =>
  text.split("").map((_, i) => start + i * every);
const CMD_KEYS = typedAt(T.cmd, "sp create", 2);
const BRANCH_KEYS = typedAt(T.branch + 4, BRANCH, 2);
const CLAUDE_KEYS = typedAt(T.claude + 4, "claude", 3);

// Per-repo build steps: [frame, text, isSpinnerUntil?]
const BUILD: { f: number; repo?: string; line?: string; spinUntil?: number }[] = [
  { f: T.build + 2, repo: "api" },
  { f: T.build + 6, line: "worktree" },
  { f: T.build + 10, line: "copied .env, CLAUDE.md" },
  { f: T.build + 15, repo: "web" },
  { f: T.build + 19, line: "worktree" },
  { f: T.build + 23, line: "copied .env.local, CLAUDE.md" },
  { f: T.build + 27, line: "npm install", spinUntil: T.npmDone },
  { f: T.npmDone + 4, repo: "worker" },
  { f: T.npmDone + 8, line: "worktree" },
  { f: T.npmDone + 12, line: "copied .env" },
];

export type Sfx = { f: number; kind: "click" | "enter" | "tick" | "pop" };

export const DEMO_SFX: Sfx[] = [
  ...CMD_KEYS.map((f) => ({ f, kind: "click" as const })),
  { f: T.found - 4, kind: "enter" },
  ...PICKER_KEYS.map(({ f, k }) => ({ f, kind: (k.length === 1 ? "click" : k === "enter" ? "enter" : "pop") as Sfx["kind"] })),
  { f: T.pickerDone + 1, kind: "tick" },
  ...BRANCH_KEYS.map((f) => ({ f, kind: "click" as const })),
  { f: T.branchDone, kind: "enter" },
  { f: T.branchDone + 1, kind: "tick" },
  { f: T.fetchDone, kind: "tick" },
  { f: T.baseDone, kind: "enter" },
  { f: T.baseDone + 1, kind: "tick" },
  { f: T.proceedDone, kind: "enter" },
  { f: T.proceedDone + 1, kind: "tick" },
  ...BUILD.filter((b) => b.line).map((b) => ({ f: b.spinUntil ?? b.f, kind: "tick" as const })),
  { f: T.done, kind: "pop" },
  ...CLAUDE_KEYS.map((f) => ({ f, kind: "click" as const })),
];

// --- picker model (mirrors internal/ui/picker.go) ---------------------------

function fuzzy(query: string, s: string): number[] | null {
  const hits: number[] = [];
  let j = 0;
  for (let i = 0; i < s.length && j < query.length; i++) {
    if (s[i] === query[j]) {
      hits.push(i);
      j++;
    }
  }
  return j === query.length ? hits : null;
}

function pickerState(frame: number) {
  let query = "";
  let cursor = 0;
  const selected = new Set<number>();
  const matches = () =>
    REPOS.map((r, i) => ({ i, hits: query ? fuzzy(query, r) : [] })).filter((m) => m.hits !== null) as {
      i: number;
      hits: number[];
    }[];
  for (const { f, k } of PICKER_KEYS) {
    if (f > frame) break;
    if (k === "tab") {
      const ms = matches();
      const idx = ms[cursor]?.i;
      if (idx !== undefined) {
        if (selected.has(idx)) selected.delete(idx);
        else selected.add(idx);
        query = "";
        cursor = (idx + 1) % REPOS.length;
      }
    } else if (k.length === 1) {
      query += k;
      cursor = 0;
    }
  }
  return { query, cursor, selected, matches: matches() };
}

// --- line primitives -------------------------------------------------------

const Line: React.FC<{ children?: React.ReactNode }> = ({ children }) => (
  <div style={{ height: 40, lineHeight: "40px", whiteSpace: "pre" }}>{children}</div>
);
const Tick = () => <span style={{ color: c.green }}>✓ </span>;
const Bar = () => <span style={{ color: "#3a4150" }}>┃ </span>;
const SPIN = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
const Spinner: React.FC<{ frame: number }> = ({ frame }) => (
  <span style={{ color: c.purple }}>{SPIN[Math.floor(frame / 2) % SPIN.length]} </span>
);
const Caret = () => <span style={{ color: c.purple }}>▏</span>;
const Answer: React.FC<{ q: string; a: string }> = ({ q, a }) => (
  <Line>
    <Tick />
    <span style={{ fontWeight: 700 }}>{q}</span> <span style={{ color: c.blue }}>{a}</span>
  </Line>
);

function typed(keys: number[], text: string, frame: number) {
  return text.slice(0, keys.filter((f) => f <= frame).length);
}

const Prompt: React.FC<{ dir: string; text: string; caret?: boolean }> = ({ dir, text, caret }) => (
  <Line>
    <span style={{ color: c.blue }}>{dir}</span> <span style={{ color: c.purple }}>❯</span> {text}
    {caret && <Caret />}
  </Line>
);

function renderPicker(frame: number) {
  const { query, cursor, selected, matches } = pickerState(frame);
  const names = REPOS.filter((_, i) => selected.has(i));
  const rows: React.ReactNode[] = [];
  for (let r = 0; r < REPOS.length; r++) {
    const m = matches[r];
    if (!m) {
      rows.push(<Line key={`p${r}`} />);
      continue;
    }
    const here = r === cursor;
    const on = selected.has(m.i);
    const label = REPOS[m.i];
    rows.push(
      <Line key={`p${r}`}>
        <span style={{ color: c.purple, fontWeight: 700 }}>{here ? "❯ " : "  "}</span>
        <span style={{ color: on ? c.green : c.faint }}>{on ? "◉ " : "○ "}</span>
        {label.split("").map((ch, i) => (
          <span
            key={i}
            style={{
              color: m.hits.includes(i) ? c.blue : here || on ? c.text : c.dim,
              fontWeight: m.hits.includes(i) ? 700 : 400,
            }}
          >
            {ch}
          </span>
        ))}
      </Line>,
    );
  }
  return [
    <Line key="ph">
      <span style={{ color: c.blue, fontWeight: 700 }}>Select repositories</span>
      <span style={{ color: c.faint }}>{"  type to search · tab/space toggle · enter confirm"}</span>
    </Line>,
    <Line key="pq">
      <span style={{ color: c.purple }}>› </span>
      {query}
      <Caret />
      <span style={{ color: c.faint }}>{`  ${matches.length}/${REPOS.length}`}</span>
      {names.length > 0 && (
        <>
          <span style={{ color: c.faint }}>{"  ·  "}</span>
          <span style={{ color: c.green }}>{`selected (${names.length}): `}</span>
          {names.join(", ")}
        </>
      )}
    </Line>,
    ...rows,
  ];
}

const Button: React.FC<{ on: boolean; children: React.ReactNode }> = ({ on, children }) => (
  <span
    style={{
      background: on ? c.blue : "#2a3140",
      color: on ? "#0a0d14" : c.dim,
      padding: "3px 18px",
      marginRight: 12,
      borderRadius: 4,
    }}
  >
    {children}
  </span>
);

function terminalLines(frame: number): React.ReactNode[] {
  const L: React.ReactNode[] = [];
  L.push(<Prompt key="cmd" dir="~/code" text={typed(CMD_KEYS, "sp create", frame)} caret={frame < T.found} />);
  if (frame < T.found) return L;
  L.push(
    <Line key="found">
      <span style={{ color: c.dim }}>Found 7 repos in ~/code</span>
    </Line>,
  );
  if (frame < T.picker) return L;

  if (frame < T.pickerDone) return [...L, ...renderPicker(frame)];
  L.push(<Answer key="a1" q="Select repositories" a="api, web, worker" />);
  if (frame < T.branch) return L;

  if (frame < T.branchDone) {
    return [
      ...L,
      <Line key="bq">
        <Bar />
        <span style={{ color: c.blue, fontWeight: 700 }}>Branch name:</span>
      </Line>,
      <Line key="bi">
        <Bar />
        <span style={{ color: c.purple }}>{"> "}</span>
        {typed(BRANCH_KEYS, BRANCH, frame)}
        <Caret />
      </Line>,
    ];
  }
  L.push(<Answer key="a2" q="Branch name" a={BRANCH} />);
  if (frame < T.fetch) return L;

  if (frame < T.fetchDone) {
    return [
      ...L,
      <Line key="fs">
        <Spinner frame={frame} />
        <span style={{ color: c.dim }}>Fetching 3 repos…</span>
      </Line>,
    ];
  }
  L.push(
    <Line key="fd">
      <Tick />
      Fetched api, web, worker
    </Line>,
  );
  if (frame < T.base) return L;

  const baseQ = "'feat/billing' is new in api, web, worker. Create it from:";
  if (frame < T.baseDone) {
    return [
      ...L,
      <Line key="sq">
        <Bar />
        <span style={{ color: c.blue, fontWeight: 700 }}>{baseQ}</span>
      </Line>,
      <Line key="s1">
        <Bar />
        <span style={{ color: c.purple }}>{"> "}</span>
        <span style={{ color: c.green }}>main</span>
      </Line>,
      <Line key="s2">
        <Bar />
        {"  Other (manual input)"}
      </Line>,
    ];
  }
  L.push(<Answer key="a3" q={baseQ.slice(0, -1)} a="main" />);
  if (frame < T.plan) return L;

  L.push(<Line key="pb" />);
  L.push(
    <Line key="ph">
      <span style={{ color: c.blue, fontWeight: 700 }}>Plan</span>
    </Line>,
  );
  ["api", "web", "worker"].forEach((r, i) => {
    if (frame >= T.plan + 4 + i * 4) {
      L.push(
        <Line key={`plan-${r}`}>
          {"  "}
          <span style={{ fontWeight: 700 }}>{r.padEnd(8)}</span>
          <span style={{ color: c.purple }}>create branch</span> <span style={{ color: c.blue }}>{BRANCH}</span> from{" "}
          <span style={{ color: c.blue }}>main</span>
        </Line>,
      );
    }
  });
  if (frame < T.proceed) return L;
  L.push(<Line key="pb2" />);

  const confirm = (key: string, q: string, yes: boolean) => [
    <Line key={`${key}q`}>
      <Bar />
      <span style={{ color: c.blue, fontWeight: 700 }}>{q}</span>
    </Line>,
    <Line key={`${key}b`}>
      <Bar />
      <Button on={yes}>Yes</Button>
      <Button on={!yes}>No</Button>
    </Line>,
  ];
  if (frame < T.proceedDone) return [...L, ...confirm("pr", "Proceed?", true)];
  L.push(<Answer key="a4" q="Proceed?" a="Yes" />);
  if (frame < T.build) return L;
  L.push(<Line key="bb" />);

  for (const [i, b] of BUILD.entries()) {
    if (frame < b.f) break;
    if (b.repo) {
      L.push(
        <Line key={`b${i}`}>
          <span style={{ fontWeight: 700, color: repoColors[b.repo] }}>{b.repo}</span>{" "}
          <span style={{ color: c.dim }}>~/.spawnpoint/workspaces/feat-billing/{b.repo}</span>
        </Line>,
      );
    } else if (b.spinUntil && frame < b.spinUntil) {
      L.push(
        <Line key={`b${i}`}>
          {"  "}
          <Spinner frame={frame} />
          <span style={{ color: c.dim }}>installing deps · {b.line}</span>
        </Line>,
      );
      break;
    } else {
      L.push(
        <Line key={`b${i}`}>
          {"  "}
          <Tick />
          {b.line === "worktree" ? (
            <>
              worktree on <span style={{ color: c.blue }}>{BRANCH}</span>
              <span style={{ color: c.dim }}> (from main)</span>
            </>
          ) : b.line?.startsWith("copied") ? (
            <>
              copied <span style={{ color: c.dim }}>{b.line.slice(7)}</span>
            </>
          ) : (
            b.line
          )}
        </Line>,
      );
    }
  }
  if (frame < T.done) return L;

  L.push(<Line key="db" />);
  L.push(
    <Line key="done">
      <span style={{ color: c.green, fontWeight: 700 }}>✓ Done!</span> Workspace:{" "}
      <span style={{ color: c.blue }}>~/.spawnpoint/workspaces/feat-billing</span>
    </Line>,
  );
  if (frame < T.claude) return L;
  L.push(<Prompt key="cl" dir="~/.spawnpoint/workspaces/feat-billing" text={typed(CLAUDE_KEYS, "claude", frame)} caret />);
  return L;
}

const TERM_H = 680;

export const Demo: React.FC = () => {
  const frame = useCurrentFrame();
  const lines = terminalLines(frame);
  // Top-anchored until the screen fills, then scroll like a real terminal.
  const full = lines.length * 40 > TERM_H;
  return (
    <AbsoluteFill style={{ alignItems: "center", justifyContent: "center", gap: 36 }}>
      <FadeUp>
        <Eyebrow>sp create</Eyebrow>
      </FadeUp>
      <Window title="~/code — zsh" style={{ width: 1560 }}>
        <div
          style={{
            height: TERM_H,
            overflow: "hidden",
            display: "flex",
            flexDirection: "column",
            justifyContent: full ? "flex-end" : "flex-start",
            fontFamily: fonts.mono,
            fontSize: 25,
            color: c.text,
          }}
        >
          {lines}
        </div>
      </Window>
    </AbsoluteFill>
  );
};
