# Ithiltir link observer

Run the systemd unit on Dash and on each node in the investigation. Set
`OBSERVER_ARGS` in `/etc/default/ithiltir-link-observer`. On a node, pass the
Dash address with `--peer`. On Dash, pass each node address with `--peer` and
the local PostgreSQL database with `--database`. Use `--interface any` to
retain physical and bridge device names in TCP records. Pass relevant devices
with `--link` and physical devices with `--driver` for driver counters.

The JSON Lines records use Unix timestamps. TCP records contain tcpdump header
text only. `socket` describes the established connection and its TCP state.
`probe` records a separate TCP connection and HTTP/2 PING, including its
ephemeral port. `host` contains local process, pressure, and interface
counters. Dash's `database` records contain the latest committed report
timestamps. The independent `database_activity` sampler records PostgreSQL
blocker PIDs, transaction and query times, lock details, statement names
without literal values, and I/O or lock wait events. With `--disk DEVICE`,
Dash also records Linux block device counters and I/O pressure every second.
A receive gap over 14 seconds is
recorded as `gap` and reported in the observer's systemd journal.

Compare the physical interface on the node with Dash ingress for upload and
Dash egress with the node interface for download. Match sequence and ACK
ranges across hosts. Segmentation offload can combine or split ranges; bridge
and physical captures can report the same packet twice. If TCP data and ACKs
reach both sides during a gap, compare socket queues, the fresh HTTP/2 probe,
and the PostgreSQL samples before attributing the stall.

The observer closes a chunk every five minutes or at 16 MiB. It compresses closed
chunks and retains up to seven days within a 768 MiB per-host limit. From the
management host, run `export.py --directory DIR --identity KEY --target LABEL=IP`
with one target per host to export the closed chunks. The output is one tar
archive per target.

To export the first completed gap for each node, add `--watch-dash IP --since
EPOCH` and run the exporter periodically. It waits for the relevant chunks to
close, then saves a three-host capture window with an `incident.json` marker
under each affected node's label. The first completed episode with three or
more simultaneous blocked writes to `server_current_metrics` is exported from
Dash under `database`, including any episode shorter than the node offline
threshold.

Run `compare.py DIR` on one saved incident directory to compare directional
TCP sequence evidence, fresh HTTP/2 probes, and Dash lock waits. A segment seen
only after a repeat points to delivery delay or loss between capture points.
An ACK seen leaving the receiver but absent at the sender before retransmission
provides evidence about the reverse direction.
The absence of a matching repeat does not establish an HTTP/2 failure.
