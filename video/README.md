# Spawnpoint video

[Remotion](https://www.remotion.dev) source for the overview video (`assets/spawnpoint.mp4`, `assets/spawnpoint.gif`) and the GitHub social preview (`assets/social-preview.png`).

```sh
npm install
npm run studio   # live preview and scrub the timeline
npm run build    # render mp4 + gif + social preview and copy them into ../assets
```

Scenes live in `src/scenes/`, and their order and lengths are set in `src/Video.tsx`. The tagline is `TAGLINE` in `src/scenes/Reveal.tsx`, which the outro and social preview also use. `npm run gif` needs `ffmpeg`.
