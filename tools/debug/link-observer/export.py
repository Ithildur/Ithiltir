#!/usr/bin/env python3
import argparse
import ipaddress
import json
from pathlib import Path
import re
import subprocess
import tarfile
import time


REMOTE_ARCHIVE = """
import bisect
from pathlib import Path
import sys
import tarfile

directory = Path("/var/log/ithiltir-link-observer")
chunks = sorted(directory.glob("capture-*.log.gz"))
if len(sys.argv) == 3:
    start, end = map(float, sys.argv[1:])
    times = [int(p.name.split(".", 1)[0].removeprefix("capture-")) / 1e9 for p in chunks]
    chunks = chunks[max(0, bisect.bisect_right(times, start)-1):bisect.bisect_right(times, end)]
if not chunks:
    raise SystemExit("no closed capture chunks")
with tarfile.open(fileobj=sys.stdout.buffer, mode="w|") as archive:
    for chunk in chunks:
        if chunk.is_file():
            archive.add(chunk, arcname=chunk.name, recursive=False)
"""


def ssh(address, identity):
    return ["ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "IdentitiesOnly=yes",
            "-o", "StrictHostKeyChecking=yes", "-i", str(identity), f"root@{address}"]


def export(label, address, directory, identity, window=()):
    path = directory / f"{label}.tar"
    if path.exists():
        return
    temporary = path.with_suffix(".tar.part")
    command = ssh(address, identity) + ["python3 -", *map(str, window)]
    try:
        with temporary.open("wb") as output:
            subprocess.run(command, input=REMOTE_ARCHIVE.encode(), stdout=output, timeout=1800, check=True)
        with tarfile.open(temporary, "r:") as archive:
            names = archive.getnames()
            if not names or not all(re.fullmatch(r"capture-[0-9]+\.log\.gz", name) for name in names):
                raise ValueError("missing or unexpected capture entries")
        temporary.replace(path)
        print(f"{path}: {len(names)} chunks", flush=True)
    finally:
        temporary.unlink(missing_ok=True)


def export_hosts(targets, directory, identity, window=()):
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    ok = True
    for label, address in targets.items():
        try:
            export(label, address, directory, identity, window)
        except (OSError, subprocess.SubprocessError, tarfile.TarError, ValueError) as error:
            print(f"{label}: {error}", flush=True)
            ok = False
    return ok


def next_gaps(targets, arguments):
    status = ssh(arguments.watch_dash, arguments.identity) + [
        "journalctl", "-q", "-o", "cat", "--no-pager", "-u", "ithiltir-link-observer.service",
        f"--since=@{arguments.since}", "--grep='^(gap|database_lock) '",
    ]
    result = subprocess.run(status, capture_output=True, text=True, timeout=25, check=False)
    if result.returncode and (result.returncode != 1 or result.stderr.strip()):
        raise RuntimeError(result.stderr.strip() or f"journalctl exit {result.returncode}")
    missing = {str(address): label for label, address in targets.items() if address != arguments.watch_dash
               and not (arguments.directory / label / "incident.json").exists()}
    database_missing = not (arguments.directory / "database" / "incident.json").exists()
    now = time.time()
    ok = True
    for line in result.stdout.splitlines():
        kind, _, payload = line.partition(" ")
        if kind not in ("gap", "database_lock"):
            continue
        incident = json.loads(payload)
        if kind == "database_lock":
            if not database_missing:
                continue
            database_missing = False
            label = "database"
            start, end = incident["started_at"], incident["ended_at"]
            scope = {name: address for name, address in targets.items() if address == arguments.watch_dash}
        else:
            label = missing.pop(incident["peer"], None)
            start, end = incident["last"], incident["next"]
            scope = targets
        if label is None or now - end < 340:
            continue
        directory = arguments.directory / label
        if export_hosts(scope, directory, arguments.identity, (start - 60, end + 30)):
            marker = directory / "incident.json.tmp"
            marker.write_text(json.dumps(incident, indent=2) + "\n", encoding="utf-8")
            marker.replace(directory / "incident.json")
            print(f"{directory}: {kind} for {end - start:.3f}s", flush=True)
        else:
            ok = False
    return ok


def main():
    parser = argparse.ArgumentParser(description="Export closed diagnostic chunks over SSH.")
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--identity", type=Path, required=True)
    parser.add_argument("--target", action="append", required=True, metavar="LABEL=IP")
    parser.add_argument("--watch-dash", type=ipaddress.IPv4Address, help="Export the first gap for each node.")
    parser.add_argument("--since", type=int, help="Unix time from which to check for gaps.")
    args = parser.parse_args()
    targets = {}
    for target in args.target:
        label, _, address = target.partition("=")
        if not re.fullmatch(r"[a-z0-9-]+", label):
            parser.error(f"invalid target label: {label!r}")
        targets[label] = ipaddress.IPv4Address(address)
    if args.watch_dash is not None:
        if args.since is None:
            parser.error("--watch-dash requires --since")
        if args.watch_dash not in targets.values():
            parser.error("--watch-dash must match a target address")
        ok = next_gaps(targets, args)
    else:
        ok = export_hosts(targets, args.directory, args.identity)
    raise SystemExit(int(not ok))


if __name__ == "__main__":
    main()
