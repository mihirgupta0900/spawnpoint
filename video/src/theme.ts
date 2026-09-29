import { loadFont as loadInter } from "@remotion/google-fonts/Inter";
import { loadFont as loadMono } from "@remotion/google-fonts/JetBrainsMono";

const inter = loadInter("normal", { weights: ["400", "500", "600", "700", "800"], subsets: ["latin"] });
const mono = loadMono("normal", { weights: ["400", "500", "700"], subsets: ["latin"] });

export const fonts = {
  sans: inter.fontFamily,
  mono: mono.fontFamily,
};

export const c = {
  bg: "#0a0d14",
  bg2: "#0f141d",
  panel: "#131a26",
  border: "#232c3b",
  text: "#e6edf3",
  dim: "#8b95a5",
  faint: "#4a5568",
  blue: "#58a6ff",
  green: "#3fb950",
  purple: "#bc8cff",
  orange: "#f0883e",
  pink: "#ff7eb6",
  yellow: "#e3b341",
  red: "#f85149",
};

export const repoColors: Record<string, string> = {
  api: c.blue,
  web: c.purple,
  worker: c.orange,
};
