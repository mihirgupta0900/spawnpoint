#!/usr/bin/env python3
"""Pack GoReleaser's `pypi` builds into platform wheels for PyPI.

Each wheel installs the native binary as `spawnpoint` (plus an `sp` alias)
into the environment's bin/Scripts dir, the same way ruff and uv ship. That
keeps `pipx install spawnpoint` / `pipx upgrade spawnpoint` working for
everyone who installed the original Python release: the next upgrade simply
swaps in the Go binary.

Usage: python scripts/build_wheels.py [--dist dist] [--out dist/wheels]
Reads dist/artifacts.json and dist/metadata.json written by `goreleaser`.
"""

import argparse
import base64
import hashlib
import json
import os
import stat
import zipfile
from pathlib import Path

NAME = "spawnpoint"
SUMMARY = "One branch, every repo, ready to code: multi-repo git worktree workspaces for you and your coding agents"

# Go 1.26 binaries need macOS 12+. Static (CGO-free) Linux binaries run on
# both glibc and musl.
PLATFORM_TAGS = {
    ("darwin", "amd64"): ["macosx_12_0_x86_64"],
    ("darwin", "arm64"): ["macosx_12_0_arm64"],
    ("linux", "amd64"): ["manylinux_2_17_x86_64", "manylinux2014_x86_64", "musllinux_1_1_x86_64"],
    ("linux", "arm64"): ["manylinux_2_17_aarch64", "manylinux2014_aarch64", "musllinux_1_1_aarch64"],
    ("windows", "amd64"): ["win_amd64"],
}

# `sp` on unix: hand off to the spawnpoint binary installed next to it
# (pipx/uv link both into the same bin dir).
SP_SHIM = b"""#!/bin/sh
dir=$(dirname "$0")
if [ -x "$dir/spawnpoint" ]; then exec "$dir/spawnpoint" "$@"; fi
exec spawnpoint "$@"
"""


def record_line(path: str, data: bytes) -> str:
    digest = base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=").decode()
    return f"{path},sha256={digest},{len(data)}"


def metadata(version: str, readme: str) -> bytes:
    lines = [
        "Metadata-Version: 2.1",
        f"Name: {NAME}",
        f"Version: {version}",
        f"Summary: {SUMMARY}",
        "Home-page: https://github.com/mihirgupta0900/spawnpoint",
        "Author: Mihir Gupta",
        "License: MIT",
        "Project-URL: Repository, https://github.com/mihirgupta0900/spawnpoint",
        "Project-URL: Issues, https://github.com/mihirgupta0900/spawnpoint/issues",
        "Keywords: git,worktree,workspace,multi-repo,ai-agents,claude-code,codex",
        "Classifier: Environment :: Console",
        "Classifier: Intended Audience :: Developers",
        "Classifier: License :: OSI Approved :: MIT License",
        "Classifier: Programming Language :: Go",
        "Classifier: Topic :: Software Development :: Version Control :: Git",
        "Description-Content-Type: text/markdown",
        "",
        readme,
    ]
    return "\n".join(lines).encode()


def build_wheel(binary: Path, goos: str, goarch: str, version: str, readme: str, out: Path) -> Path:
    tags = PLATFORM_TAGS[(goos, goarch)]
    plat = ".".join(tags)
    wheel = out / f"{NAME}-{version}-py3-none-{plat}.whl"
    data_dir = f"{NAME}-{version}.data/scripts"
    info_dir = f"{NAME}-{version}.dist-info"
    exe = ".exe" if goos == "windows" else ""
    bin_bytes = binary.read_bytes()

    files = [(f"{data_dir}/{NAME}{exe}", bin_bytes, True)]
    if goos == "windows":
        files.append((f"{data_dir}/sp.exe", bin_bytes, True))
    else:
        files.append((f"{data_dir}/sp", SP_SHIM, True))
    files.append((f"{info_dir}/METADATA", metadata(version, readme), False))
    wheel_meta = ["Wheel-Version: 1.0", "Generator: spawnpoint build_wheels.py", "Root-Is-Purelib: false"]
    wheel_meta += [f"Tag: py3-none-{t}" for t in tags]
    files.append((f"{info_dir}/WHEEL", ("\n".join(wheel_meta) + "\n").encode(), False))
    license_path = Path("LICENSE")
    if license_path.exists():
        files.append((f"{info_dir}/licenses/LICENSE", license_path.read_bytes(), False))

    records = [record_line(p, d) for p, d, _ in files] + [f"{info_dir}/RECORD,,"]
    files.append((f"{info_dir}/RECORD", ("\n".join(records) + "\n").encode(), False))

    with zipfile.ZipFile(wheel, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        for path, data, executable in files:
            info = zipfile.ZipInfo(path, date_time=(2020, 1, 1, 0, 0, 0))
            mode = 0o755 if executable else 0o644
            info.external_attr = (stat.S_IFREG | mode) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            zf.writestr(info, data)
    return wheel


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--dist", default="dist")
    parser.add_argument("--out", default=None)
    parser.add_argument("--version", default=None, help="override the version (snapshot builds aren't PEP 440)")
    args = parser.parse_args()

    dist = Path(args.dist)
    out = Path(args.out or dist / "wheels")
    out.mkdir(parents=True, exist_ok=True)
    version = args.version or json.loads((dist / "metadata.json").read_text())["version"]
    readme = Path("README.md").read_text() if Path("README.md").exists() else ""

    built = []
    for art in json.loads((dist / "artifacts.json").read_text()):
        if art.get("type") != "Binary" or art.get("extra", {}).get("ID") != "pypi":
            continue
        key = (art["goos"], art["goarch"])
        if key not in PLATFORM_TAGS:
            continue
        built.append(build_wheel(Path(art["path"]), *key, version, readme, out))

    if len(built) != len(PLATFORM_TAGS):
        raise SystemExit(f"expected {len(PLATFORM_TAGS)} wheels, built {len(built)}")
    for w in sorted(built):
        print(f"{w}  ({os.path.getsize(w) / 1e6:.1f} MB)")


if __name__ == "__main__":
    main()
