import React from "react";
import { Composition, Still } from "remotion";
import { Social } from "./Social";
import { SpawnpointVideo, TOTAL } from "./Video";

export const Root: React.FC = () => (
  <>
    <Composition id="Spawnpoint" component={SpawnpointVideo} durationInFrames={TOTAL} fps={30} width={1920} height={1080} />
    <Still id="Social" component={Social} width={1280} height={640} />
  </>
);
