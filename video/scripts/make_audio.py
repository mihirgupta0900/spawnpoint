"""Synthesize the video's music bed and sound effects into public/audio/.

Everything is generated from scratch (no samples), so the audio is ours to
ship. Timings come from src/Video.tsx; rerun after changing scene lengths:

    python scripts/make_audio.py --fps 30 --total 900 --reveal 185 --outro 750

Needs numpy and ffmpeg.
"""

import argparse
import subprocess
import tempfile
import wave
from pathlib import Path

import numpy as np

SR = 44100
OUT = Path(__file__).resolve().parent.parent / "public" / "audio"
rng = np.random.default_rng(7)


def t(n: int) -> np.ndarray:
    return np.arange(n) / SR


def env(n: int, attack: float, decay: float) -> np.ndarray:
    """Exponential-decay envelope with a short linear attack (seconds)."""
    x = t(n)
    a = np.clip(x / max(attack, 1e-4), 0, 1)
    return a * np.exp(-x / decay)


def lowpass(x: np.ndarray, cutoff: np.ndarray | float) -> np.ndarray:
    """One-pole lowpass; cutoff may vary per sample."""
    cutoff = np.broadcast_to(np.asarray(cutoff, dtype=float), x.shape)
    a = 1 - np.exp(-2 * np.pi * cutoff / SR)
    y = np.empty_like(x)
    acc = 0.0
    for i in range(len(x)):
        acc += a[i] * (x[i] - acc)
        y[i] = acc
    return y


def highpass(x: np.ndarray, cutoff: float) -> np.ndarray:
    return x - lowpass(x, cutoff)


def saw(freq: float, n: int, phase: float = 0.0) -> np.ndarray:
    p = (t(n) * freq + phase) % 1.0
    return 2 * p - 1


def note(midi: float) -> float:
    return 440.0 * 2 ** ((midi - 69) / 12)


def write(name: str, x: np.ndarray, peak: float = 0.9) -> None:
    """Write mono or stereo float audio as mp3 via a temp wav."""
    if x.ndim == 1:
        x = np.stack([x, x], axis=1)
    x = x / (np.max(np.abs(x)) + 1e-9) * peak
    pcm = (x * 32767).astype("<i2")
    with tempfile.TemporaryDirectory() as d:
        wav = Path(d) / "a.wav"
        with wave.open(str(wav), "wb") as w:
            w.setnchannels(2)
            w.setsampwidth(2)
            w.setframerate(SR)
            w.writeframes(pcm.tobytes())
        subprocess.run(
            ["ffmpeg", "-y", "-loglevel", "error", "-i", str(wav), "-codec:a", "libmp3lame", "-q:a", "2", str(OUT / f"{name}.mp3")],
            check=True,
        )


# --- sound effects -------------------------------------------------------


def sfx_click() -> np.ndarray:
    """Soft mechanical key tap."""
    n = int(0.05 * SR)
    noise = highpass(rng.standard_normal(n), 2500) * env(n, 0.0005, 0.006)
    body = np.sin(2 * np.pi * 1800 * t(n)) * env(n, 0.0005, 0.004) * 0.4
    return noise + body


def sfx_enter() -> np.ndarray:
    """Heavier key: a thock plus a click."""
    n = int(0.12 * SR)
    thock = np.sin(2 * np.pi * (180 + 200 * np.exp(-t(n) / 0.01)) * t(n)) * env(n, 0.001, 0.03)
    return thock + sfx_click_padded(n) * 0.8


def sfx_click_padded(n: int) -> np.ndarray:
    c = sfx_click()
    return np.pad(c, (0, n - len(c)))


def sfx_tick() -> np.ndarray:
    """Bright two-note confirmation chime."""
    n = int(0.4 * SR)
    out = np.zeros(n)
    for i, (m, delay) in enumerate([(84, 0.0), (91, 0.06)]):
        k = n - int(delay * SR)
        f = note(m)
        tone = (np.sin(2 * np.pi * f * t(k)) + 0.3 * np.sin(2 * np.pi * 2 * f * t(k))) * env(k, 0.002, 0.12)
        out[n - k :] += tone * (0.8 if i else 1.0)
    return out


def sfx_pop() -> np.ndarray:
    """Bubbly pitch-up pop for elements appearing."""
    n = int(0.12 * SR)
    freq = 380 + 900 * (1 - np.exp(-t(n) / 0.02))
    phase = 2 * np.pi * np.cumsum(freq) / SR
    return np.sin(phase) * env(n, 0.002, 0.035)


def sfx_whoosh() -> np.ndarray:
    """Filtered-noise sweep for scene transitions (~0.5 s)."""
    n = int(0.55 * SR)
    x = t(n) / (n / SR)
    shape = np.sin(np.pi * x) ** 2
    cutoff = 300 + 5000 * np.sin(np.pi * x) ** 1.5
    noise = lowpass(rng.standard_normal(n), cutoff)
    left = noise * shape * (1 - 0.6 * x)
    right = noise * shape * (0.4 + 0.6 * x)
    return np.stack([left, right], axis=1)


def sfx_impact() -> np.ndarray:
    """Deep hit with a shimmering tail, for the logo reveal."""
    n = int(2.5 * SR)
    boom = np.sin(2 * np.pi * (45 + 120 * np.exp(-t(n) / 0.05)) * t(n)) * env(n, 0.002, 0.5)
    crack = lowpass(rng.standard_normal(n), 3000) * env(n, 0.001, 0.08) * 0.5
    shimmer = sum(np.sin(2 * np.pi * note(m) * t(n)) for m in (69, 76, 81, 88)) * env(n, 0.02, 0.9) * 0.12
    return boom + crack + shimmer


# --- music ---------------------------------------------------------------

# A minor: Am – F – C – G, one chord per bar.
CHORDS = [[57, 60, 64], [53, 57, 60], [48, 52, 55], [55, 59, 62]]
ROOTS = [45, 41, 36, 43]


def music(total_s: float, reveal_s: float, outro_s: float, bpm: float) -> np.ndarray:
    n = int(total_s * SR)
    beat = 60 / bpm
    bar = beat * 4
    pad = np.zeros(n)
    drums = np.zeros(n)
    bass = np.zeros(n)
    arp = np.zeros(n)

    def add(buf: np.ndarray, start_s: float, x: np.ndarray) -> None:
        i = int(start_s * SR)
        if i < 0:
            x, i = x[-i:], 0
        if i >= n or len(x) == 0:
            return
        j = min(n, i + len(x))
        buf[i:j] += x[: j - i]

    # Bars are counted from the reveal so the drop lands on a downbeat.
    first_bar = -int(np.ceil(reveal_s / bar))
    last_bar = int(np.ceil((total_s - reveal_s) / bar))
    for b in range(first_bar, last_bar):
        start = reveal_s + b * bar
        chord = CHORDS[b % 4]
        # Pad: detuned saws, lowpassed, one bar long, plays throughout.
        bn = int(bar * SR) + int(0.3 * SR)
        voice = sum(saw(note(m) * d, bn, rng.random()) for m in chord for d in (0.997, 1.003))
        a = np.clip(t(bn) / 0.3, 0, 1) * np.clip((bn / SR - t(bn)) / 0.3, 0, 1)
        add(pad, start, lowpass(voice, 900) * a)

        if start < reveal_s:
            # Intro: a quiet high pulse on each beat to keep time.
            for k in range(4):
                pn = int(0.15 * SR)
                add(arp, start + k * beat, np.sin(2 * np.pi * note(chord[2] + 12) * t(pn)) * env(pn, 0.003, 0.05) * 0.4)
            continue
        if start >= outro_s:
            continue

        for k in range(4):
            s = start + k * beat
            # Kick on every beat.
            kn = int(0.35 * SR)
            add(drums, s, np.sin(2 * np.pi * (50 + 110 * np.exp(-t(kn) / 0.03)) * t(kn)) * env(kn, 0.001, 0.16))
            # Off-beat hats.
            hn = int(0.06 * SR)
            add(drums, s + beat / 2, highpass(rng.standard_normal(hn), 7000) * env(hn, 0.001, 0.015) * 0.25)
            # Clap on 2 and 4.
            if k in (1, 3):
                cn = int(0.2 * SR)
                add(drums, s, lowpass(highpass(rng.standard_normal(cn), 900), 5000) * env(cn, 0.001, 0.05) * 0.35)
            # Bass: root on the off-beat eighths.
            bn2 = int(beat / 2 * SR)
            add(bass, s + beat / 2, lowpass(saw(note(ROOTS[b % 4]), bn2), 400) * env(bn2, 0.005, 0.12))
        # Arpeggio: 16ths through the chord, an octave up.
        seq = chord + [chord[0] + 12]
        for k in range(16):
            an = int(0.2 * SR)
            f = note(seq[k % 4] + 12)
            tone = (np.sign(np.sin(2 * np.pi * f * t(an))) * 0.3 + np.sin(2 * np.pi * f * t(an))) * env(an, 0.002, 0.07)
            add(arp, start + k * beat / 4, lowpass(tone, 2500) * 0.35)

    # Sidechain: duck the pad and bass under each kick after the reveal.
    x = t(n)
    since_beat = ((x - reveal_s) % beat)
    duck = np.where((x >= reveal_s) & (x < outro_s), 1 - 0.6 * np.exp(-since_beat / 0.08), 1.0)

    # Riser into the reveal: noise sweep plus rising pitch over the last 2 bars.
    rise_s = 2 * bar
    rn = int(rise_s * SR)
    rx = t(rn) / rise_s
    riser = lowpass(rng.standard_normal(rn), 200 + 6000 * rx**2) * rx**2 * 0.5
    riser += np.sin(2 * np.pi * np.cumsum(200 + 600 * rx**2) / SR) * rx**3 * 0.15
    riser_buf = np.zeros(n)
    add(riser_buf, reveal_s - rise_s, riser)

    # Intro is quieter; everything fades over the outro.
    level = np.interp(x, [0, reveal_s - 0.01, reveal_s, outro_s, total_s - 0.2, total_s], [0.55, 0.7, 1, 1, 0, 0])
    level *= np.clip(x / 0.5, 0, 1)
    mix = (pad * 0.12 * duck + bass * 0.35 * duck + drums * 0.55 + arp * 0.2) * level + riser_buf * 0.6

    # A little stereo width: delay the arp and pad in the right channel.
    d = int(0.012 * SR)
    right = mix.copy()
    right[d:] = (pad * 0.12 * duck + arp * 0.2)[:-d] * level[d:] + (bass * 0.35 * duck + drums * 0.55)[d:] * level[d:] + riser_buf[d:] * 0.6
    return np.stack([mix, right], axis=1)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--fps", type=float, default=30)
    ap.add_argument("--total", type=int, required=True, help="total frames")
    ap.add_argument("--reveal", type=int, required=True, help="frame where the logo reveal starts")
    ap.add_argument("--outro", type=int, required=True, help="frame where the outro starts")
    ap.add_argument("--bpm", type=float, default=112)
    args = ap.parse_args()

    OUT.mkdir(parents=True, exist_ok=True)
    for name, fn in [
        ("click", sfx_click),
        ("enter", sfx_enter),
        ("tick", sfx_tick),
        ("pop", sfx_pop),
        ("whoosh", sfx_whoosh),
        ("impact", sfx_impact),
    ]:
        write(name, fn())
    write("music", music(args.total / args.fps, args.reveal / args.fps, args.outro / args.fps, args.bpm), peak=0.8)
    print(f"wrote {OUT}")


if __name__ == "__main__":
    main()
