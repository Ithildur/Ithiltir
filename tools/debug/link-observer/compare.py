#!/usr/bin/env python3
import argparse
import bisect
from collections import defaultdict
from dataclasses import dataclass
import gzip
import json
from pathlib import Path
import re
import tarfile


PACKET = re.compile(
    r"^(\S+)\s+(\S+)\s+IP (\d+\.\d+\.\d+\.\d+)\.(\d+) > "
    r"(\d+\.\d+\.\d+\.\d+)\.(\d+): (.*)$"
)
SEQ = re.compile(r"\bseq (\d+):(\d+)")
ACK = re.compile(r"\back (\d+)\b")


@dataclass(frozen=True)
class Segment:
    at: float
    port: int
    start: int
    size: int


@dataclass(frozen=True)
class Acknowledgement:
    at: float
    port: int
    sequence: int


def records(path, start, end):
    with tarfile.open(path, "r:") as archive:
        chunks = sorted(archive.getmembers(), key=lambda entry: entry.name)
        times = [int(entry.name.split(".", 1)[0].removeprefix("capture-")) / 1e9 for entry in chunks]
        first = max(0, bisect.bisect_right(times, start) - 1)
        for chunk in chunks[first:bisect.bisect_right(times, end)]:
            with gzip.open(archive.extractfile(chunk), "rt") as source:
                for line in source:
                    record = json.loads(line)
                    if start <= record["at"] <= end:
                        yield record


def trace(samples, peer, node_ports, physical=None):
    flows = defaultdict(list)
    acknowledgements = defaultdict(list)
    zero_windows = defaultdict(int)
    for sample in samples:
        if sample["kind"] != "tcp":
            continue
        packet = PACKET.match(sample["data"])
        if packet is None:
            continue
        device, direction, src, sport, dst, dport, header = packet.groups()
        if peer not in (src, dst) or direction not in ("In", "Out"):
            continue
        if physical is not None and device != physical:
            continue
        upload = src == peer
        port = int(sport if upload else dport)
        if port in node_ports:
            continue
        if re.search(r"\bwin 0\b", header):
            zero_windows[upload, direction] += 1
        ack = ACK.search(header)
        if ack:
            acknowledgements[upload, direction].append(Acknowledgement(sample["at"], port, int(ack[1])))
        sequence = SEQ.search(header)
        if sequence:
            start, end = map(int, sequence.groups())
            flows[upload, direction].append(Segment(sample["at"], port, start, (end - start) % 2**32))
    return flows, acknowledgements, zero_windows


def acknowledgements_by_port(entries):
    by_port = defaultdict(list)
    for entry in sorted(entries, key=lambda item: item.at):
        by_port[entry.port].append(entry)
    return {port: ([item.at for item in group], group) for port, group in by_port.items()}


def covering_ack(index, segment, before):
    times, entries = index.get(segment.port, ([], []))
    first = bisect.bisect_left(times, segment.at - 0.05)
    last = bisect.bisect_right(times, before)
    end_sequence = segment.start + segment.size
    return next((item for item in entries[first:last] if (item.sequence - end_sequence) % 2**32 < 2**31), None)


def directional_evidence(outbound, inbound, start, end, acks_sent=(), acks_received=()):
    receipts = defaultdict(list)
    for segment in sorted(inbound, key=lambda item: item.at):
        receipts[segment.port].append(segment)
    arrival_times = {port: [item.at for item in entries] for port, entries in receipts.items()}

    def reaches(segment, before):
        times = arrival_times.get(segment.port, [])
        first = bisect.bisect_left(times, segment.at - 0.05)
        last = bisect.bisect_right(times, before)
        return any((segment.start - item.start) % 2**32 < item.size
                   for item in receipts[segment.port][first:last])

    sent = {}
    late = []
    reverse = []
    ack_out = acknowledgements_by_port(acks_sent)
    ack_in = acknowledgements_by_port(acks_received)
    retry_count = 0
    for segment in sorted(outbound, key=lambda item: item.at):
        key = segment.port, segment.start
        previous = sent.get(key)
        if previous is not None and segment.at - previous.at >= 0.25 and start <= segment.at <= end:
            retry_count += 1
            if not reaches(previous, segment.at) and reaches(segment, segment.at + 1):
                late.append({"port": segment.port, "sequence": segment.start, "first_sent": previous.at,
                             "resent": segment.at})
            ack = covering_ack(ack_out, previous, segment.at)
            if ack is not None and covering_ack(ack_in, previous, segment.at) is None:
                reverse.append({"port": segment.port, "sequence": segment.start, "ack_sent": ack.at,
                                "ack": ack.sequence, "data_resent": segment.at})
        sent[key] = segment
    return {"sent_with_payload": sum(start <= item.at <= end for item in outbound),
            "received_with_payload": sum(start <= item.at <= end for item in inbound),
            "repeated_sequence_starts": retry_count,
            "seen_at_receiver_only_after_repeat_count": len(late),
            "examples_seen_only_after_repeat": late[:12],
            "reverse_ack_unseen_before_retry_count": len(reverse),
            "examples_reverse_ack_unseen_before_retry": reverse[:12]}


def compare(directory):
    incident = json.loads((directory / "incident.json").read_text(encoding="utf-8"))
    peer = incident["peer"]
    start, end = incident["last"], incident["next"]
    node = list(records(directory / f"{directory.name}.tar", start - 8, end + 8))
    dash = list(records(directory / "dash.tar", start - 8, end + 8))
    physical = next((next(iter(item["data"]["driver"])) for item in node
                     if item["kind"] == "host" and item["data"]["driver"]), None)
    if physical is None:
        raise ValueError("node physical interface is missing from host samples")
    probes = [item for item in node if item["kind"] == "probe"]
    probe_ports = {item["data"].get("local_port") for item in probes}
    node_flows, node_acks, node_windows = trace(node, peer, probe_ports, physical)
    dash_flows, dash_acks, dash_windows = trace(dash, peer, probe_ports)
    hosts = [item["data"] for item in node if item["kind"] == "host"]
    interface_delta = {}
    if len(hosts) > 1:
        first, last = hosts[0]["links"][physical], hosts[-1]["links"][physical]
        interface_delta = {key: int(last[key]) - int(value) for key, value in first.items()
                           if value.isdigit() and last[key].isdigit()}
    return {
        "incident": incident,
        "node_physical_interface": physical,
        "node_to_dash": directional_evidence(node_flows[True, "Out"], dash_flows[True, "In"], start, end,
                                              dash_acks[False, "Out"], node_acks[False, "In"]),
        "dash_to_node": directional_evidence(dash_flows[False, "Out"], node_flows[False, "In"], start, end,
                                              node_acks[True, "Out"], dash_acks[True, "In"]),
        "tcp_zero_window_packets": {"node": sum(node_windows.values()), "dash": sum(dash_windows.values())},
        "fresh_http2_probes": [item for item in probes if start <= item["at"] <= end],
        "node_interface_counter_changes": interface_delta,
        "node_processes": sorted({item["pid"] for item in hosts}),
        "dash_samples_with_database_lock_wait": [
            item for item in dash if item["kind"] in ("database", "database_activity")
            and item["data"].get("waits") and start <= item["at"] <= end
        ],
        "capture_interruptions": [item for item in node + dash if item["kind"] == "source_stopped"],
    }


def main():
    parser = argparse.ArgumentParser(description="Compare both sides of an observed TCP reporting gap.")
    parser.add_argument("directory", nargs="+", type=Path, help="Per-node directories with saved incidents.")
    parser.add_argument("--write", action="store_true", help="Save comparison.json beside each available incident.")
    args = parser.parse_args()
    for directory in args.directory:
        if not args.write:
            print(json.dumps(compare(directory), indent=2))
            continue
        path = directory / "comparison.json"
        if path.exists() or not (directory / "incident.json").is_file():
            continue
        temporary = path.with_suffix(".json.tmp")
        temporary.write_text(json.dumps(compare(directory), indent=2) + "\n", encoding="utf-8")
        temporary.replace(path)


if __name__ == "__main__":
    main()
