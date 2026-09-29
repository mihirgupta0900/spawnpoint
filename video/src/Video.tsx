import React from "react";
import { linearTiming, TransitionSeries } from "@remotion/transitions";
import { fade } from "@remotion/transitions/fade";
import { slide } from "@remotion/transitions/slide";
import { Background } from "./ui";
import { Soundtrack } from "./audio";
import { Chore, Hook } from "./scenes/Problem";
import { Reveal } from "./scenes/Reveal";
import { Demo, DEMO_LENGTH } from "./scenes/Demo";
import { Agents } from "./scenes/Agents";
import { Outro } from "./scenes/Outro";

const T = 15;

export const SCENES = [
  { id: "hook", C: Hook, len: 75 },
  { id: "chore", C: Chore, len: 140 },
  { id: "reveal", C: Reveal, len: 100 },
  { id: "demo", C: Demo, len: DEMO_LENGTH },
  { id: "agents", C: Agents, len: 210 },
  { id: "outro", C: Outro, len: 150 },
];

export const TOTAL = SCENES.reduce((a, s) => a + s.len, 0) - T * (SCENES.length - 1);

// Absolute start frame of each scene (transitions overlap by T).
const STARTS = SCENES.map((s, i) => ({
  id: s.id,
  from: SCENES.slice(0, i).reduce((a, p) => a + p.len - T, 0),
}));

export const SpawnpointVideo: React.FC = () => (
  <Background>
    <TransitionSeries>
      {SCENES.flatMap(({ id, C, len }, i) => {
        const seq = (
          <TransitionSeries.Sequence key={id} durationInFrames={len}>
            <C />
          </TransitionSeries.Sequence>
        );
        if (i === 0) return [seq];
        const presentation = i % 2 === 0 ? fade() : slide({ direction: "from-right" });
        return [
          <TransitionSeries.Transition key={`${id}-t`} presentation={presentation} timing={linearTiming({ durationInFrames: T })} />,
          seq,
        ];
      })}
    </TransitionSeries>
    <Soundtrack starts={STARTS} />
  </Background>
);
