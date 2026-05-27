import { create } from 'zustand';
import { ApiError } from '@lib/api';
import {
  fetchTrafficRebuild,
  rebuildNodeTraffic,
  type NodeTrafficRebuildStatus,
} from '@lib/adminApi';

const idleRebuildStatus: NodeTrafficRebuildStatus = {
  server_id: 0,
  status: 'idle',
  running: false,
};

const startTimeoutMs = 15000;

type TrafficRebuildEventPayload =
  | { type: 'completed' }
  | { type: 'failed' }
  | { type: 'status_sync_failed'; error: unknown };

export type TrafficRebuildEvent = TrafficRebuildEventPayload & { id: number };

export type TrafficRebuildStartOutcome =
  | { status: 'started'; state: NodeTrafficRebuildStatus }
  | { status: 'running_other'; state: NodeTrafficRebuildStatus }
  | { status: 'sync_failed'; error: unknown }
  | { status: 'failed'; error: unknown };

interface TrafficRebuildState {
  status: NodeTrafficRebuildStatus;
  event: TrafficRebuildEvent | null;
  startingNodeId: number | null;
  startedNodeId: number | null;
  syncErrorShown: boolean;
  eventSeq: number;
  startSeq: number;
  reset: () => void;
  refresh: (signal?: AbortSignal) => Promise<NodeTrafficRebuildStatus>;
  markSyncError: (error: unknown) => void;
  takeEvent: (id: number) => TrafficRebuildEvent | null;
  start: (id: number) => Promise<TrafficRebuildStartOutcome | null>;
}

export const isAbortError = (error: unknown): boolean =>
  error instanceof DOMException && error.name === 'AbortError';

export const runningRebuildNodeId = (status: NodeTrafficRebuildStatus): number | null =>
  status.running && status.server_id > 0 ? status.server_id : null;

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
): TrafficRebuildStartOutcome | null => {
  if (!state.running) return null;
  if (state.server_id === id) return { status: 'started', state };
  return { status: 'running_other', state };
};

const eventPatch = (
  state: TrafficRebuildState,
  payload: TrafficRebuildEventPayload,
): Pick<TrafficRebuildState, 'event' | 'eventSeq'> => {
  const id = state.eventSeq + 1;
  return {
    eventSeq: id,
    event: { ...payload, id },
  };
};

const statusPatch = (
  state: TrafficRebuildState,
  next: NodeTrafficRebuildStatus,
): Partial<TrafficRebuildState> => {
  const patch: Partial<TrafficRebuildState> = {
    status: next,
    syncErrorShown: false,
  };
  const startedNodeId = state.startedNodeId;
  if (startedNodeId === null) return patch;

  if (next.running) {
    if (next.server_id !== startedNodeId) {
      patch.startedNodeId = null;
    }
    return patch;
  }

  patch.startedNodeId = null;
  if (next.server_id !== startedNodeId) return patch;

  if (next.status === 'completed') {
    Object.assign(patch, eventPatch(state, { type: 'completed' }));
  } else if (next.status === 'failed') {
    Object.assign(patch, eventPatch(state, { type: 'failed' }));
  }
  return patch;
};

export const useTrafficRebuildStore = create<TrafficRebuildState>((set, get) => ({
  status: idleRebuildStatus,
  event: null,
  startingNodeId: null,
  startedNodeId: null,
  syncErrorShown: false,
  eventSeq: 0,
  startSeq: 0,
  reset: () => {
    set((state) => ({
      status: idleRebuildStatus,
      event: null,
      startingNodeId: null,
      startedNodeId: null,
      syncErrorShown: false,
      startSeq: state.startSeq + 1,
    }));
  },
  refresh: async (signal) => {
    const next = await fetchTrafficRebuild(signal);
    set((state) => statusPatch(state, next));
    return next;
  },
  markSyncError: (error) => {
    set((state) => {
      if (state.syncErrorShown) return {};
      return {
        ...eventPatch(state, { type: 'status_sync_failed', error }),
        syncErrorShown: true,
      };
    });
  },
  takeEvent: (id) => {
    const event = get().event;
    if (!event || event.id !== id) return null;
    set({ event: null });
    return event;
  },
  start: async (id) => {
    const current = get();
    const currentStatus = current.status;
    if (currentStatus.running) {
      if (currentStatus.server_id === id) {
        set({ startedNodeId: id });
        return { status: 'started', state: currentStatus };
      }
      return { status: 'running_other', state: currentStatus };
    }
    if (current.startingNodeId !== null || current.startedNodeId !== null) return null;

    const lastStatus = currentStatus;
    const seq = get().startSeq + 1;
    set((state) => ({
      ...statusPatch(
        {
          ...state,
          startingNodeId: id,
          startedNodeId: id,
          syncErrorShown: false,
        },
        { server_id: id, status: 'running', running: true },
      ),
      startingNodeId: id,
      startedNodeId: id,
      startSeq: seq,
    }));

    try {
      const started = await rebuildNodeTrafficWithTimeout(id);
      set((state) => {
        if (state.startSeq !== seq) return {};
        return {
          ...statusPatch({ ...state, startingNodeId: null }, started),
          startingNodeId: null,
        };
      });
      return startOutcomeFromStatus(id, started);
    } catch (error) {
      if (isRebuildRunningError(error) || isAbortError(error)) {
        try {
          const next = await fetchTrafficRebuild();
          set((state) => {
            if (state.startSeq !== seq) return {};
            return {
              ...statusPatch({ ...state, startingNodeId: null }, next),
              startingNodeId: null,
            };
          });
          const result = startOutcomeFromStatus(id, next);
          if (result) return result;
          if (isRebuildRunningError(error)) return { status: 'running_other', state: next };
          if (next.server_id === id && (next.status === 'completed' || next.status === 'failed')) {
            return null;
          }
          return { status: 'failed', error };
        } catch (syncError) {
          set((state) => {
            if (state.startSeq !== seq) return {};
            return {
              status: lastStatus,
              startingNodeId: null,
              startedNodeId: null,
              syncErrorShown: true,
            };
          });
          return { status: 'sync_failed', error: syncError };
        }
      }

      set((state) => {
        if (state.startSeq !== seq) return {};
        return {
          status: lastStatus,
          startingNodeId: null,
          startedNodeId: null,
          syncErrorShown: false,
        };
      });
      return { status: 'failed', error };
    }
  },
}));

export const isTrafficRebuildBusy = (): boolean => {
  const state = useTrafficRebuildStore.getState();
  return state.status.running || state.startingNodeId !== null || state.startedNodeId !== null;
};
