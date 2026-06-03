import { create } from 'zustand';
import { ApiError, isApiAuthStaleError } from '@lib/api';
import {
  fetchTrafficRebuild,
  rebuildNodeTraffic,
  type NodeTrafficRebuildStatus,
} from '@lib/adminApi';
import { isAbortError } from '@utils/errors';
import { createSeqGate } from '@utils/seqGate';

export const idleRebuildStatus: NodeTrafficRebuildStatus = {
  server_id: 0,
  status: 'idle',
  running: false,
};

type TrafficRebuildEventPayload =
  | { kind: 'completed' }
  | { kind: 'failed' }
  | { kind: 'status_sync_failed' };

export type TrafficRebuildEvent = TrafficRebuildEventPayload & { id: number };

export type TrafficRebuildLocal =
  | { phase: 'idle'; syncErrorShown: boolean }
  | {
      phase: 'starting';
      nodeId: number;
      seq: number;
      syncErrorShown: boolean;
    }
  | { phase: 'watching'; nodeId: number; syncErrorShown: boolean };

export type TrafficRebuildStartOutcome =
  | { status: 'started'; state: NodeTrafficRebuildStatus }
  | { status: 'running_other'; state: NodeTrafficRebuildStatus }
  | { status: 'sync_failed'; error: unknown }
  | { status: 'failed'; error: unknown }
  | { status: 'busy' | 'stale' | 'noop' };

const trafficRebuildBusy = { status: 'busy' } satisfies TrafficRebuildStartOutcome;
const trafficRebuildStale = { status: 'stale' } satisfies TrafficRebuildStartOutcome;
const trafficRebuildNoop = { status: 'noop' } satisfies TrafficRebuildStartOutcome;

type StartClaim =
  | { status: 'claimed'; seq: number; lastStatus: NodeTrafficRebuildStatus }
  | TrafficRebuildStartOutcome;

export interface TrafficRebuildState {
  status: NodeTrafficRebuildStatus;
  local: TrafficRebuildLocal;
  event: TrafficRebuildEvent | null;
}

const startTimeoutMs = 15000;

export const runningRebuildNodeId = (status: NodeTrafficRebuildStatus): number | null =>
  status.running && status.server_id > 0 ? status.server_id : null;

export const startingRebuildNodeId = (local: TrafficRebuildLocal): number | null =>
  local.phase === 'starting' ? local.nodeId : null;

export const watchedRebuildNodeId = (local: TrafficRebuildLocal): number | null =>
  local.phase === 'watching' ? local.nodeId : null;

export const localRebuildNodeId = (local: TrafficRebuildLocal): number | null =>
  local.phase === 'idle' ? null : local.nodeId;

const idleLocal = (syncErrorShown = false): TrafficRebuildLocal => ({
  phase: 'idle',
  syncErrorShown,
});

let eventSeq = 0;
let startSeq = 0;

const eventPatch = (payload: TrafficRebuildEventPayload): Pick<TrafficRebuildState, 'event'> => {
  const id = eventSeq + 1;
  eventSeq = id;
  return {
    event: { ...payload, id },
  };
};

const isFinishedStatus = (id: number, status: NodeTrafficRebuildStatus): boolean =>
  status.server_id === id && (status.status === 'completed' || status.status === 'failed');

const statusPatch = (
  state: TrafficRebuildState,
  next: NodeTrafficRebuildStatus,
): Partial<TrafficRebuildState> => {
  const local: TrafficRebuildLocal = { ...state.local, syncErrorShown: false };
  if (state.local.phase !== 'watching') return { status: next, local };

  const nodeId = state.local.nodeId;
  if (next.running) {
    return {
      status: next,
      local: next.server_id === nodeId ? local : idleLocal(),
    };
  }

  return {
    status: next,
    local: idleLocal(),
    ...(next.server_id === nodeId && next.status === 'completed'
      ? eventPatch({ kind: 'completed' })
      : {}),
    ...(next.server_id === nodeId && next.status === 'failed'
      ? eventPatch({ kind: 'failed' })
      : {}),
  };
};

const initialState = (): TrafficRebuildState => ({
  status: idleRebuildStatus,
  local: idleLocal(),
  event: null,
});

export const useTrafficRebuildStore = create<TrafficRebuildState>()(initialState);

const getState = (): TrafficRebuildState => useTrafficRebuildStore.getState();
const syncGate = createSeqGate();

export const isTrafficRebuildBusy = (): boolean => {
  const state = getState();
  return state.status.running || localRebuildNodeId(state.local) !== null;
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

const startOutcomeFromStatus = (
  id: number,
  state: NodeTrafficRebuildStatus,
): TrafficRebuildStartOutcome => {
  if (!state.running) return trafficRebuildNoop;
  if (state.server_id === id) return { status: 'started', state };
  return { status: 'running_other', state };
};

const resetState = (): void => {
  startSeq += 1;
  useTrafficRebuildStore.setState(initialState());
};

const applyStatus = (next: NodeTrafficRebuildStatus): void => {
  useTrafficRebuildStore.setState((state) => statusPatch(state, next));
};

const markSyncError = (): void => {
  const state = getState();
  if (state.local.syncErrorShown) return;
  useTrafficRebuildStore.setState((current) => ({
    ...eventPatch({ kind: 'status_sync_failed' }),
    local: { ...current.local, syncErrorShown: true },
  }));
};

const claimStart = (id: number): StartClaim => {
  const current = getState();
  const currentStatus = current.status;
  if (currentStatus.running) {
    if (currentStatus.server_id === id) {
      useTrafficRebuildStore.setState({
        local: { phase: 'watching', nodeId: id, syncErrorShown: false },
      });
      return { status: 'started', state: currentStatus };
    }
    return { status: 'running_other', state: currentStatus };
  }
  if (current.local.phase !== 'idle') return trafficRebuildBusy;

  const lastStatus = currentStatus;
  const seq = startSeq + 1;
  startSeq = seq;
  const local: TrafficRebuildLocal = {
    phase: 'starting',
    nodeId: id,
    seq,
    syncErrorShown: false,
  };
  useTrafficRebuildStore.setState({
    status: { server_id: id, status: 'running', running: true },
    local,
  });
  return { status: 'claimed', seq, lastStatus };
};

const finishStart = (seq: number, next: NodeTrafficRebuildStatus): boolean => {
  const state = getState();
  if (state.local.phase !== 'starting' || state.local.seq !== seq) return false;
  useTrafficRebuildStore.setState(
    statusPatch(
      {
        ...state,
        local: { phase: 'watching', nodeId: state.local.nodeId, syncErrorShown: false },
      },
      next,
    ),
  );
  return true;
};

const rollbackStart = (
  seq: number,
  status: NodeTrafficRebuildStatus,
  syncErrorShown: boolean,
): boolean => {
  const current = getState();
  if (current.local.phase !== 'starting' || current.local.seq !== seq) return false;
  useTrafficRebuildStore.setState({
    status,
    local: idleLocal(syncErrorShown),
  });
  return true;
};

const syncStartStatus = async (
  id: number,
  seq: number,
  lastStatus: NodeTrafficRebuildStatus,
  error: unknown,
): Promise<TrafficRebuildStartOutcome> => {
  try {
    const next = await fetchTrafficRebuild();
    if (!finishStart(seq, next)) return trafficRebuildStale;
    const result = startOutcomeFromStatus(id, next);
    if (result.status !== 'noop') return result;
    if (isRebuildRunningError(error) || isFinishedStatus(id, next)) return trafficRebuildNoop;
    return { status: 'failed', error };
  } catch (syncError) {
    if (!rollbackStart(seq, lastStatus, true)) return trafficRebuildStale;
    return { status: 'sync_failed', error: syncError };
  }
};

export const resetTrafficRebuild = (): void => {
  syncGate.invalidate();
  resetState();
};

export const syncTrafficRebuildStatus = async (
  signal?: AbortSignal,
): Promise<NodeTrafficRebuildStatus> => {
  const seq = syncGate.next();
  const next = await fetchTrafficRebuild(signal);
  if (syncGate.isCurrent(seq)) applyStatus(next);
  return next;
};

export const reportTrafficRebuildSyncError = (): void => {
  markSyncError();
};

export const takeTrafficRebuildEvent = (id: number): TrafficRebuildEvent | null => {
  const event = getState().event;
  if (!event || event.id !== id) return null;
  useTrafficRebuildStore.setState({ event: null });
  return event;
};

export const startTrafficRebuild = async (id: number): Promise<TrafficRebuildStartOutcome> => {
  const claim = claimStart(id);
  if (claim.status !== 'claimed') return claim;

  try {
    const started = await rebuildNodeTrafficWithTimeout(id);
    if (!finishStart(claim.seq, started)) return trafficRebuildStale;
    return startOutcomeFromStatus(id, started);
  } catch (error) {
    if (isApiAuthStaleError(error)) {
      if (!rollbackStart(claim.seq, claim.lastStatus, false)) return trafficRebuildStale;
      return trafficRebuildStale;
    }
    if (isRebuildRunningError(error) || isAbortError(error)) {
      return syncStartStatus(id, claim.seq, claim.lastStatus, error);
    }
    if (!rollbackStart(claim.seq, claim.lastStatus, false)) return trafficRebuildStale;
    return { status: 'failed', error };
  }
};
