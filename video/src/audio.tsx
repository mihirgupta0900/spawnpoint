import React from "react";
import { Html5Audio, Sequence, staticFile } from "remotion";
import { DEMO_SFX } from "./scenes/Demo";

// Audio is synthesized by scripts/make_audio.py into public/audio/.
// Frames below are relative to each scene's start, and mirror the scene's
// own animation timings.

type Kind = "click" | "enter" | "tick" | "pop" | "whoosh" | "impact";
type Cue = { f: number; kind: Kind; vol?: number };

const VOLUME: Record<Kind, number> = {
  click: 0.3,
  enter: 0.45,
  tick: 0.35,
  pop: 0.3,
  whoosh: 0.35,
  impact: 0.85,
};

const range = (n: number, at: (i: number) => number, kind: Kind, vol?: number): Cue[] =>
  Array.from({ length: n }, (_, i) => ({ f: at(i), kind, vol }));

const SCENE_SFX: Record<string, Cue[]> = {
  hook: range(3, (i) => 14 + i * 7, "pop"),
  // One keystroke per chore line, then the headline lands.
  chore: [...range(13, (i) => 10 + (i + 1) * 5, "click", 0.25), { f: 81, kind: "pop" }],
  reveal: [{ f: 0, kind: "impact" }],
  demo: DEMO_SFX,
  agents: [...range(3, (i) => 20 + i * 10, "pop"), ...range(9, (i) => 20 + (i % 3) * 10 + 66 + Math.floor(i / 3) * 12, "tick", 0.18)],
  outro: [{ f: 22, kind: "tick" }, ...range(3, (i) => 30 + i * 3, "pop", 0.2)],
};

const Cue: React.FC<{ from: number; kind: Kind; vol?: number }> = ({ from, kind, vol }) => (
  <Sequence from={from} durationInFrames={kind === "impact" ? 80 : 20} layout="none">
    <Html5Audio src={staticFile(`audio/${kind}.mp3`)} volume={vol ?? VOLUME[kind]} />
  </Sequence>
);

/** `starts` maps scene id to its absolute start frame. */
export const Soundtrack: React.FC<{ starts: { id: string; from: number }[] }> = ({ starts }) => (
  <>
    <Html5Audio src={staticFile("audio/music.mp3")} volume={0.5} />
    {starts.flatMap(({ id, from }, i) => [
      // Transitions start where the next scene starts; a whoosh rides each one.
      ...(i > 0 && id !== "reveal" ? [<Cue key={`${id}-whoosh`} from={Math.max(0, from - 4)} kind="whoosh" />] : []),
      ...(SCENE_SFX[id] ?? []).map((cue, j) => <Cue key={`${id}-${j}`} from={from + cue.f} kind={cue.kind} vol={cue.vol} />),
    ])}
  </>
);
