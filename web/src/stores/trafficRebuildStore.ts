import { create } from 'zustand';
import { ApiError, isApiAuthStaleError } from '@lib/api';
import {
  fetchTrafficRebuild,
  rebuildNodeTraffic,
  type NodeTrafficRebuildStatus,
} from '@lib/adminApi';
import { isAbortError } from '@utils/errors';
import { createSeqGate } from '@utils/seqGate';

const idleStatus: NodeTrafficRebuildStatus = {
  server_id: 0,
  status: 'idle',
  running: false,
};

export type TrafficRebuildStartOutcome =
  | { status: 'started'; state: NodeTrafficRebuildStatus }
  | { status: 'running_other'; state: NodeTrafficRebuildStatus }
  | { status: 'sync_failed'; error: unknown }
  | { status: 'failed'; error: unknown }
  | { status: 'busy' | 'stale' | 'noop' };

interface TrafficRebuildState {
  status: NodeTrafficRebuildStatus;
  startingNodeId: number | null;
}

const startTimeoutMs = 15000;
const busyOutcome = { status: 'busy' } satisfies TrafficRebuildStartOutcome;
const staleOutcome = { status: 'stale' } satisfies TrafficRebuildStartOutcome;
const noopOutcome = { status: 'noop' } satisfies TrafficRebuildStartOutcome;

const initialState = (): TrafficRebuildState => ({
  status: idleStatus,
  startingNodeId: null,
});

export const useTrafficRebuildStore = create<TrafficRebuildState>()(initialState);

const statusGate = createSeqGate();
let startSeq = 0;

const getState = (): TrafficRebuildState => useTrafficRebuildStore.getState();

export const runningRebuildNodeId = (status: NodeTrafficRebuildStatus): number | null =>
  status.running && status.server_id > 0 ? status.server_id : null;

export const isTrafficRebuildBusy = (): boolean => {
  const state = getState();
  return state.status.running || state.startingNodeId !== null;
};

const isRebuildRunningError = (error: unknown): boolean =>
  error instanceof ApiError && error.status === 409 && error.code === 'traffic_rebuild_running';

const rebuildNodeTrafficWithTimeout = async (id: number): Promise<NodeTrafficRebuildStatus> => {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), startTimeoutMs);
  try {
    return await rebuildNodeTraffic(id, controller.signal);
  } finally {
    window.clearTimeout(timeout);
  }
};

const outcomeFromStatus = (
  id: number,
  state: NodeTrafficRebuildStatus,
): TrafficRebuildStartOutcome => {
  if (!state.running) return noopOutcome;
  if (state.server_id === id) return { status: 'started', state };
  return { status: 'running_other', state };
};

const finishStart = (id: number, seq: number, status: NodeTrafficRebuildStatus): boolean => {
  const state = getState();
  if (startSeq !== seq || state.startingNodeId !== id) return false;

  // A POST response is newer than every status GET started before it.
  statusGate.invalidate();
  useTrafficRebuildStore.setState({ status, startingNodeId: null });
  return true;
};

const rollbackStart = (id: number, seq: number): boolean => {
  const state = getState();
  if (startSeq !== seq || state.startingNodeId !== id) return false;
  useTrafficRebuildStore.setState({ startingNodeId: null });
  return true;
};

const syncStartStatus = async (
  id: number,
  seq: number,
  startError: unknown,
): Promise<TrafficRebuildStartOutcome> => {
  try {
    const status = await fetchTrafficRebuild();
    if (!finishStart(id, seq, status)) return staleOutcome;
    const outcome = outcomeFromStatus(id, status);
    if (outcome.status !== 'noop') return outcome;
    if (
      isRebuildRunningError(startError) ||
      (status.server_id === id && (status.status === 'completed' || status.status === 'failed'))
    ) {
      return noopOutcome;
    }
    return { status: 'failed', error: startError };
  } catch (error) {
    if (!rollbackStart(id, seq)) return staleOutcome;
    return { status: 'sync_failed', error };
  }
};

export const resetTrafficRebuild = (): void => {
  startSeq += 1;
  statusGate.invalidate();
  useTrafficRebuildStore.setState(initialState());
};

export const syncTrafficRebuildStatus = async (
  signal?: AbortSignal,
): Promise<NodeTrafficRebuildStatus> => {
  const seq = statusGate.next();
  const status = await fetchTrafficRebuild(signal);
  if (statusGate.isCurrent(seq)) useTrafficRebuildStore.setState({ status });
  return status;
};

export const startTrafficRebuild = async (id: number): Promise<TrafficRebuildStartOutcome> => {
  const current = getState();
  if (current.status.running) return outcomeFromStatus(id, current.status);
  if (current.startingNodeId !== null) return busyOutcome;

  const seq = startSeq + 1;
  startSeq = seq;
  useTrafficRebuildStore.setState({ startingNodeId: id });

  try {
    const status = await rebuildNodeTrafficWithTimeout(id);
    if (!finishStart(id, seq, status)) return staleOutcome;
    return outcomeFromStatus(id, status);
  } catch (error) {
    if (isApiAuthStaleError(error)) {
      rollbackStart(id, seq);
      return staleOutcome;
    }
    if (isRebuildRunningError(error) || isAbortError(error)) {
      return syncStartStatus(id, seq, error);
    }
    if (!rollbackStart(id, seq)) return staleOutcome;
    return { status: 'failed', error };
  }
};
