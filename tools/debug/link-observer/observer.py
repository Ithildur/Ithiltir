#!/usr/bin/env python3
import argparse
import concurrent.futures
import gzip
import ipaddress
import json
import os
from pathlib import Path
import re
import selectors
import shutil
import signal
import socket
import subprocess
import time


ROLL_SECONDS = 300
ROLL_BYTES = 16 * 1024**2
MAX_BYTES = 768 * 1024**2
KEEP_SECONDS = 7 * 24 * 60 * 60
OFFLINE_SECONDS = 14
PACKET = re.compile(r"^\d+\.\d+ .*\bFlags \[")
IDENTIFIER = r'(?:"[A-Za-z_][A-Za-z_0-9$]*"|[A-Za-z_][A-Za-z_0-9$]*)'
SQL_TARGET = re.compile(
    rf"^\s*(INSERT\s+INTO|UPDATE|DELETE\s+FROM|ALTER\s+TABLE|LOCK\s+TABLE|TRUNCATE(?:\s+TABLE)?)"
    rf"\s+(?:ONLY\s+)?({IDENTIFIER}(?:\s*\.\s*{IDENTIFIER})?)(?=[\s(;,*]|$)", re.IGNORECASE,
)
SQL_FUNCTION = re.compile(r"^\s*SELECT\s+((?:pg_catalog\.)?pg_[a-z_0-9]+)\s*\(", re.IGNORECASE)
SQL_COMMANDS = {
    "ALTER", "ANALYZE", "BEGIN", "CALL", "CHECKPOINT", "COMMIT", "COPY", "CREATE",
    "DELETE", "DROP", "INSERT", "LOCK", "MERGE", "REINDEX", "ROLLBACK", "SELECT",
    "SET", "TRUNCATE", "UPDATE", "VACUUM", "WITH",
}
DISK_FIELDS = (
    "read_ios", "read_merges", "read_sectors", "read_ms", "write_ios", "write_merges",
    "write_sectors", "write_ms", "in_flight", "io_ms", "weighted_io_ms", "discard_ios",
    "discard_merges", "discard_sectors", "discard_ms", "flush_ios", "flush_ms",
)


class Log:
    def __init__(self, directory):
        self.directory = directory
        for path in sorted(directory.glob("capture-*.log")):
            self.compress(path)
        self.prune()
        self.open()

    def open(self):
        self.path = self.directory / f"capture-{time.time_ns()}.log"
        self.file = self.path.open("x", encoding="utf-8", buffering=1)
        self.roll_at = time.monotonic() + ROLL_SECONDS

    def write(self, kind, data, at=None):
        record = {"at": time.time() if at is None else at, "kind": kind, "data": data}
        self.file.write(json.dumps(record, separators=(",", ":")) + "\n")

    def roll(self):
        if time.monotonic() >= self.roll_at or self.file.tell() >= ROLL_BYTES:
            self.close()
            self.prune()
            self.open()

    def close(self):
        self.file.close()
        self.compress(self.path)

    def compress(self, path):
        archive = path.with_suffix(".log.gz")
        temporary = archive.with_suffix(".gz.tmp")
        if not archive.exists():
            with path.open("rb") as source, gzip.open(temporary, "wb", compresslevel=3) as target:
                shutil.copyfileobj(source, target)
            temporary.replace(archive)
        path.unlink()

    def prune(self):
        cutoff = time.time_ns() - KEEP_SECONDS * 10**9
        archives = []
        for path in sorted(self.directory.glob("capture-*.log.gz")):
            stamp = int(path.name.removeprefix("capture-").split(".")[0])
            if stamp < cutoff:
                path.unlink()
            else:
                archives.append((path, path.stat().st_size))
        used = sum(size for _, size in archives)
        for path, size in archives:
            if used <= MAX_BYTES - ROLL_BYTES:
                break
            path.unlink()
            used -= size


def read(path):
    try:
        return Path(path).read_text(encoding="utf-8").strip()
    except (FileNotFoundError, ProcessLookupError, PermissionError):
        return ""


def command(*args):
    try:
        result = subprocess.run(args, text=True, capture_output=True, timeout=1, check=False)
    except subprocess.TimeoutExpired:
        return f"{args[0]} timed out"
    return result.stdout.strip() if result.returncode == 0 else f"{args[0]} exit {result.returncode}"


def host_sample(args, pid):
    counters = ("rx_bytes", "tx_bytes", "rx_dropped", "tx_dropped", "rx_errors",
                "tx_errors", "rx_missed_errors", "rx_fifo_errors", "tx_carrier_errors")
    links = {}
    for link in args.link:
        base = Path("/sys/class/net") / link
        links[link] = {counter: read(base / "statistics" / counter) for counter in counters}
        links[link]["carrier_changes"] = read(base / "carrier_changes")
        links[link]["qdisc"] = command("tc", "-s", "-j", "qdisc", "show", "dev", link)
    softnet = [row.split()[:3] for row in read("/proc/net/softnet_stat").splitlines()]
    proc = Path("/proc") / pid
    return {
        "cpu": read("/proc/stat").partition("\n")[0],
        "pressure": {name: read(f"/proc/pressure/{name}") for name in ("cpu", "io", "memory")},
        "softnet": [sum(int(value, 16) for value in column) for column in zip(*softnet)],
        "links": links,
        "pid": pid,
        "process": {name: read(proc / name) for name in ("stat", "schedstat", "io")} if pid else {},
        "driver": {link: [
            row.strip() for row in command("ethtool", "-S", link).splitlines()
            if re.search(r"drop|miss|err|pause|reset|stall|timeout|no.?buf", row, re.I)
        ] for link in args.driver},
    }


def probe(address, port):
    started = time.monotonic()
    data = {"peer": address, "stage": "connect"}
    try:
        with socket.create_connection((address, port), timeout=2) as connection:
            connected = time.monotonic()
            data.update(stage="http2_ping", connect_ms=round(1000 * (connected - started), 2),
                        local_port=connection.getsockname()[1])
            token = os.urandom(8)
            connection.sendall(
                b"PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
                b"\x00\x00\x00\x04\x00\x00\x00\x00\x00"
                b"\x00\x00\x08\x06\x00\x00\x00\x00\x00" + token
            )
            contents = bytearray()
            deadline = connected + 2
            while time.monotonic() < deadline:
                connection.settimeout(max(0.001, deadline - time.monotonic()))
                part = connection.recv(4096)
                if not part:
                    raise EOFError
                contents.extend(part)
                while len(contents) >= 9:
                    length = int.from_bytes(contents[:3], "big")
                    if length > 65536:
                        raise ValueError("unexpected frame size")
                    if len(contents) < length + 9:
                        break
                    if contents[3:5] == b"\x06\x01" and contents[9:9 + length] == token:
                        data.update(stage="ok", ping_ms=round(1000 * (time.monotonic() - connected), 2))
                        return data
                    del contents[:9 + length]
            raise TimeoutError
    except (OSError, EOFError, ValueError) as error:
        data["error"] = type(error).__name__
    return data


def database_sql(node_ips):
    addresses = ", ".join(f"'{address}'" for address in node_ips)
    return f"""
SET statement_timeout='1500ms';
SET default_transaction_read_only=on;
SET application_name='ithiltir-link-observer-reports';
SELECT json_build_object(
  'at', extract(epoch FROM clock_timestamp()),
  'received', (SELECT json_object_agg(n.ip, extract(epoch FROM m.collected_at))
               FROM servers n LEFT JOIN server_current_metrics m ON n.id=m.server_id
               WHERE NOT n.is_deleted AND n.ip IN ({addresses})),
  'stale', (SELECT count(*) FROM server_current_metrics
            WHERE clock_timestamp()-collected_at > interval '9 seconds')
)::jsonb;
\\watch 1
"""


ACTIVITY_SQL = r"""
SET statement_timeout='1500ms';
SET default_transaction_read_only=on;
SET application_name='ithiltir-link-observer-activity';
WITH waiting AS MATERIALIZED (
  SELECT pid, wait_event AS lock, pg_blocking_pids(pid) AS blockers
  FROM pg_stat_activity
  WHERE datname=current_database() AND wait_event_type='Lock'
), blockers AS (
  SELECT unnest(blockers) AS pid FROM waiting
), participants AS (
  SELECT pid FROM waiting UNION SELECT pid FROM blockers
), sessions AS (
  SELECT a.pid, a.application_name, a.backend_type, a.state, a.wait_event_type, a.wait_event,
         a.backend_xid::text AS xid, a.xact_start, a.query_start, a.state_change, a.query
  FROM pg_stat_activity a
  WHERE a.pid <> pg_backend_pid()
    AND (a.pid IN (SELECT pid FROM participants)
      OR ((a.datname=current_database() OR a.backend_type IN ('checkpointer','background writer','walwriter'))
          AND a.wait_event_type IN ('IO','LWLock','BufferPin')))
)
SELECT json_build_object(
  'at', extract(epoch FROM clock_timestamp()),
  'waits', (SELECT json_agg(waiting ORDER BY pid) FROM waiting),
  'sessions', (SELECT json_agg(sessions ORDER BY pid) FROM sessions),
  'locks', CASE WHEN EXISTS (SELECT 1 FROM waiting) THEN (
      SELECT json_agg(json_build_object('pid', l.pid, 'type', l.locktype, 'mode', l.mode,
                        'granted', l.granted, 'relation', c.relname,
                        'xid', l.transactionid::text, 'wait_since', l.waitstart) ORDER BY l.pid, l.granted)
      FROM pg_locks l LEFT JOIN pg_class c ON c.oid=l.relation
        AND l.database=(SELECT oid FROM pg_database WHERE datname=current_database())
      WHERE l.pid IN (SELECT pid FROM participants)
        AND (NOT l.granted OR (l.pid IN (SELECT pid FROM blockers)
          AND (l.locktype='transactionid' OR (l.locktype='relation' AND l.mode<>'AccessShareLock'))))
  ) END
)::jsonb;
\watch 1
"""


def statement_name(query):
    if not query:
        return None
    if target := SQL_TARGET.match(query):
        action = " ".join(target[1].upper().split())
        relation = "".join(target[2].replace('"', "").split())
        return f"{action} {relation}"
    if function := SQL_FUNCTION.match(query):
        return "SELECT " + function[1] + "(...)"
    command = re.match(r"^\s*([A-Za-z]+)\b", query)
    if command and command[1].upper() in SQL_COMMANDS:
        return command[1].upper()
    return None


def record_activity(log, sample, previous):
    at = sample.pop("at")
    sessions = {}
    for session in sample.get("sessions") or ():
        session["statement"] = statement_name(session.pop("query"))
        sessions[session["pid"]] = session["statement"]
    log.write("database_activity", sample, at)

    metrics_waiters = 0
    for wait in sample.get("waits") or ():
        statement = sessions.get(wait["pid"])
        if statement and statement.startswith(("INSERT INTO ", "UPDATE ")):
            if statement.split()[-1].rsplit(".", 1)[-1] == "server_current_metrics":
                metrics_waiters += 1
    if metrics_waiters >= 3:
        if previous is None:
            previous = {"started_at": at, "peak_waiters": metrics_waiters}
        else:
            previous["peak_waiters"] = max(previous["peak_waiters"], metrics_waiters)
        return previous
    if previous is not None:
        previous["ended_at"] = at
        log.write("database_lock", previous, at)
        print("database_lock " + json.dumps(previous), flush=True)
    return None


def disk_sample(disks):
    return {
        "devices": {name: dict(zip(DISK_FIELDS, map(int, read(f"/sys/class/block/{name}/stat").split())))
                    for name in disks},
        "pressure": read("/proc/pressure/io"),
    }


def collect(args, log):
    peers = [str(peer) for peer in args.peer]
    endpoints = " or ".join(f"host {peer}" for peer in peers)
    capture = ["tcpdump", "-p", "-i", args.interface, "-nn", "-tt", "-S", "-l", "-s", "96",
               f"tcp port {args.port} and ({endpoints})"]
    socket_filter = ("( sport = :" if args.database else "( dport = :") + str(args.port) + " )"
    sources = {"tcp": (capture, None)}
    if args.database:
        postgres = ["runuser", "-u", "postgres", "--", "psql", "-X", "-Atq",
                    "-v", "ON_ERROR_STOP=1", "-d", args.database]
        sources["database"] = (postgres, database_sql(peers))
        sources["activity"] = (postgres, ACTIVITY_SQL)
    jobs = {}
    buffer = {}
    retry_at = {}
    running = True

    def stop(_signal, _frame):
        nonlocal running
        running = False

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    watch = selectors.DefaultSelector()

    def spawn(kind, argv, sql):
        process = subprocess.Popen(argv, stdin=subprocess.PIPE if sql else subprocess.DEVNULL,
                                   stdout=subprocess.PIPE, bufsize=0)
        jobs[kind] = process
        buffer[kind] = b""
        os.set_blocking(process.stdout.fileno(), False)
        watch.register(process.stdout, selectors.EVENT_READ, kind)
        if sql:
            with process.stdin as writer:
                writer.write(sql.encode())

    try:
        for kind, source in sources.items():
            spawn(kind, *source)
        incidents = {}
        last_report = {}
        lock_episode = None
        unit = "dash" if args.database else "ithiltir-node"
        next_sockets = next_host = next_probe = next_clock = next_disk = 0
        outstanding = None
        with concurrent.futures.ThreadPoolExecutor(max_workers=1) as worker:
            while running:
                for key, _ in watch.select(timeout=0.5):
                    piece = os.read(key.fd, 65536)
                    kind = key.data
                    if not piece:
                        watch.unregister(key.fileobj)
                        key.fileobj.close()
                        log.write("source_stopped", {"source": kind, "status": jobs[kind].poll()})
                        retry_at[kind] = time.monotonic() + 2
                        continue
                    lines = (buffer[kind] + piece).split(b"\n")
                    buffer[kind] = lines.pop()
                    for raw in lines:
                        line = raw.decode(errors="replace")
                        if kind == "tcp" and PACKET.match(line):
                            stamp, packet = line.split(maxsplit=1)
                            log.write("tcp", packet, float(stamp))
                        if kind in ("activity", "database") and line.startswith("{"):
                            try:
                                sample = json.loads(line)
                            except json.JSONDecodeError:
                                log.write("source_invalid_sample", {"source": kind})
                                continue
                            if kind == "activity":
                                lock_episode = record_activity(log, sample, lock_episode)
                                continue
                            at = sample.pop("at")
                            log.write("database", sample, at)
                            for peer, received in (sample.get("received") or {}).items():
                                if received is None:
                                    continue
                                if received != last_report.get(peer):
                                    previous = last_report.get(peer)
                                    last_report[peer] = received
                                    if previous is not None and received - previous > OFFLINE_SECONDS:
                                        incident = {"peer": peer, "last": previous, "next": received,
                                                    "gap": round(received - previous, 3)}
                                        log.write("gap", incident, at)
                                        print("gap " + json.dumps(incident), flush=True)
                                if at - received > OFFLINE_SECONDS and peer not in incidents:
                                    incidents[peer] = received
                                    print(f"offline peer={peer} last_receive={received:.6f}", flush=True)
                                elif peer in incidents and received != incidents[peer]:
                                    del incidents[peer]
                now = time.monotonic()
                for kind, at in list(retry_at.items()):
                    if now >= at and jobs[kind].poll() is not None:
                        spawn(kind, *sources[kind])
                        del retry_at[kind]
                if now >= next_sockets:
                    sockets = command("ss", "-tinH", "state", "established", socket_filter).splitlines()
                    matched = False
                    for i, line in enumerate(sockets):
                        if any(re.search(rf"{re.escape(peer)}\]?:\d+\b", line) for peer in peers):
                            stats = sockets[i + 1] if i + 1 < len(sockets) else ""
                            log.write("socket", [line.strip(), stats.strip() if stats[:1].isspace() else ""])
                            matched = True
                    if not matched:
                        log.write("socket", sockets)
                    next_sockets = now + 2
                if now >= next_host:
                    pid = command("systemctl", "show", f"{unit}.service", "-p", "MainPID", "--value")
                    log.write("host", host_sample(args, pid if pid.isdigit() and int(pid) else ""))
                    next_host = now + 5
                if args.disk and now >= next_disk:
                    log.write("disk", disk_sample(args.disk))
                    next_disk = now + 1
                if now >= next_clock:
                    if shutil.which("chronyc"):
                        log.write("clock", command("chronyc", "-c", "tracking"))
                    else:
                        log.write("clock", command("timedatectl", "show", "-p", "NTPSynchronized", "--value"))
                    next_clock = now + 60
                if outstanding is not None and outstanding.done():
                    log.write("probe", outstanding.result())
                    outstanding = None
                if not args.database and outstanding is None and now >= next_probe:
                    outstanding = worker.submit(probe, peers[0], args.port)
                    next_probe = now + 10
                log.roll()
    finally:
        watch.close()
        for process in jobs.values():
            process.terminate()
            try:
                process.wait(timeout=2)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()


def main():
    parser = argparse.ArgumentParser(description="Observe Ithiltir node transport without storing packet payloads.")
    parser.add_argument("--interface", required=True)
    parser.add_argument("--peer", type=ipaddress.IPv4Address, action="append", required=True)
    parser.add_argument("--link", action="append", default=[])
    parser.add_argument("--driver", action="append", default=[])
    parser.add_argument("--disk", action="append", default=[], help="Block device name for I/O sampling.")
    parser.add_argument("--database", help="Dash database, authenticated through the local postgres OS account.")
    parser.add_argument("--port", type=int, default=28080)
    parser.add_argument("--directory", type=Path, default=Path("/var/log/ithiltir-link-observer"))
    args = parser.parse_args()
    log = Log(args.directory)
    try:
        collect(args, log)
    finally:
        log.close()


if __name__ == "__main__":
    main()
