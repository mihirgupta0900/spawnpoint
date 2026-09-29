import React from "react";
import { linearTiming, TransitionSeries } from "@remotion/transitions";
import { fade } from "@remotion/transitions/fade";
import { slide } from "@remotion/transitions/slide";
import { Background } from "./ui";
import { Chore, Hook } from "./scenes/Problem";
import { Reveal } from "./scenes/Reveal";
import { Demo, DEMO_LENGTH } from "./scenes/Demo";
import { Tree } from "./scenes/Tree";
import { Agents } from "./scenes/Agents";
import { Features } from "./scenes/Features";
import { Outro } from "./scenes/Outro";

const T = 15;

export const SCENES = [
  { id: "hook", C: Hook, len: 85 },
  { id: "chore", C: Chore, len: 190 },
  { id: "reveal", C: Reveal, len: 120 },
  { id: "demo", C: Demo, len: DEMO_LENGTH },
  { id: "tree", C: Tree, len: 190 },
  { id: "agents", C: Agents, len: 290 },
  { id: "features", C: Features, len: 170 },
  { id: "outro", C: Outro, len: 140 },
];

export const TOTAL = SCENES.reduce((a, s) => a + s.len, 0) - T * (SCENES.length - 1);

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
  </Background>
);
