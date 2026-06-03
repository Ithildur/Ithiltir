import { create } from 'zustand';
import type { StatisticsAccess } from '@app-types/traffic';
import { fetchStatisticsAccess } from '@lib/statisticsApi';
import { actionNoop, actionOk, type ActionOutcome } from '@utils/actionOutcome';
import { isCanceledRequestError } from '@utils/errors';

export type StatisticsAccessLoad =
  | { status: 'idle'; access: null; error: null; requestId: null; fetchedAt: null }
  | {
      status: 'loading';
      access: StatisticsAccess | null;
      error: null;
      requestId: number;
      fetchedAt: number | null;
    }
  | { status: 'ready'; access: StatisticsAccess; error: null; requestId: null; fetchedAt: number }
  | {
      status: 'error';
      access: StatisticsAccess | null;
      error: unknown;
      requestId: null;
      fetchedAt: number | null;
    };

export interface StatisticsAccessState {
  load: StatisticsAccessLoad;
}

const initialStatisticsAccessLoad: StatisticsAccessLoad = {
  status: 'idle',
  access: null,
  error: null,
  requestId: null,
  fetchedAt: null,
};

const loadingLoad = (current: StatisticsAccessLoad, requestId: number): StatisticsAccessLoad => ({
  status: 'loading',
  access: current.access,
  error: null,
  requestId,
  fetchedAt: current.fetchedAt,
});

const readyLoad = (access: StatisticsAccess, fetchedAt: number): StatisticsAccessLoad => ({
  status: 'ready',
  access,
  error: null,
  requestId: null,
  fetchedAt,
});

const cachedLoad = (current: StatisticsAccessLoad): StatisticsAccessLoad =>
  current.access && current.fetchedAt !== null
    ? readyLoad(current.access, current.fetchedAt)
    : initialStatisticsAccessLoad;

const errorLoad = (current: StatisticsAccessLoad, error: unknown): StatisticsAccessLoad => ({
  status: 'error',
  access: current.access,
  error,
  requestId: null,
  fetchedAt: current.fetchedAt,
});

export const useStatisticsAccessStore = create<StatisticsAccessState>()(() => ({
  load: initialStatisticsAccessLoad,
}));

export const getStatisticsAccessState = (): StatisticsAccessState =>
  useStatisticsAccessStore.getState();

const startStatisticsAccessLoad = (requestId: number): void => {
  const current = getStatisticsAccessState().load;
  useStatisticsAccessStore.setState({
    load: loadingLoad(current, requestId),
  });
};

const setStatisticsAccess = (
  requestId: number,
  access: StatisticsAccess,
  fetchedAt: number,
): void => {
  if (getStatisticsAccessState().load.requestId !== requestId) return;
  useStatisticsAccessStore.setState({
    load: readyLoad(access, fetchedAt),
  });
};

const setStatisticsAccessError = (requestId: number, error: unknown): void => {
  const current = getStatisticsAccessState().load;
  if (current.requestId !== requestId) return;
  useStatisticsAccessStore.setState({
    load: errorLoad(current, error),
  });
};

const cancelStatisticsAccessLoad = (requestId: number): void => {
  const current = getStatisticsAccessState().load;
  if (current.requestId !== requestId) return;
  useStatisticsAccessStore.setState({ load: cachedLoad(current) });
};

const updateStatisticsAccessCache = (
  patch: Partial<StatisticsAccess>,
): ActionOutcome<StatisticsAccess> => {
  const access = getStatisticsAccessState().load.access;
  if (!access) return actionNoop;
  const nextAccess = { ...access, ...patch };
  useStatisticsAccessStore.setState({
    load: readyLoad(nextAccess, Date.now()),
  });
  return actionOk(nextAccess);
};

export const cacheHistoryGuestAccess = (
  mode: StatisticsAccess['history_guest_access_mode'],
): ActionOutcome<StatisticsAccess> =>
  updateStatisticsAccessCache({
    history_guest_access_mode: mode,
  });

export const cacheTrafficGuestAccess = (
  mode: StatisticsAccess['traffic_guest_access_mode'],
): ActionOutcome<StatisticsAccess> =>
  updateStatisticsAccessCache({
    traffic_guest_access_mode: mode,
  });

export const resetStatisticsAccessStore = (): void => {
  inFlight?.controller.abort();
  inFlight = null;
  nextRequestId += 1;
  useStatisticsAccessStore.setState({ load: initialStatisticsAccessLoad });
};

const accessCacheTtlMs = 30000;

type StatisticsAccessRequest = {
  id: number;
  controller: AbortController;
  promise: Promise<StatisticsAccess>;
};

let nextRequestId = 0;
let inFlight: StatisticsAccessRequest | null = null;

const abortSignalError = (): DOMException =>
  new DOMException('The operation was aborted.', 'AbortError');

const withAbortSignal = <T>(promise: Promise<T>, signal?: AbortSignal): Promise<T> => {
  if (!signal) return promise;
  if (signal.aborted) return Promise.reject(abortSignalError());

  return new Promise<T>((resolve, reject) => {
    const cleanup = () => {
      signal.removeEventListener('abort', abort);
    };
    const abort = () => {
      cleanup();
      reject(abortSignalError());
    };

    signal.addEventListener('abort', abort, { once: true });
    promise.then(
      (value) => {
        cleanup();
        resolve(value);
      },
      (error) => {
        cleanup();
        reject(error);
      },
    );
  });
};

const runRequest = async (requestId: number, signal: AbortSignal): Promise<StatisticsAccess> => {
  try {
    const next = await fetchStatisticsAccess({ signal });
    setStatisticsAccess(requestId, next, Date.now());
    return next;
  } catch (error) {
    if (isCanceledRequestError(error)) {
      cancelStatisticsAccessLoad(requestId);
    } else {
      setStatisticsAccessError(requestId, error);
    }
    throw error;
  } finally {
    if (inFlight?.id === requestId) {
      inFlight = null;
    }
  }
};

export const refreshStatisticsAccess = async (
  params: { signal?: AbortSignal } = {},
): Promise<StatisticsAccess> => {
  if (params.signal?.aborted) {
    return Promise.reject(abortSignalError());
  }
  if (inFlight) {
    return withAbortSignal(inFlight.promise, params.signal);
  }

  const requestId = ++nextRequestId;
  const controller = new AbortController();
  startStatisticsAccessLoad(requestId);
  const promise = runRequest(requestId, controller.signal);

  inFlight = { id: requestId, controller, promise };
  return withAbortSignal(promise, params.signal);
};

export const ensureStatisticsAccess = async (
  params: { maxAgeMs?: number; signal?: AbortSignal } = {},
): Promise<StatisticsAccess> => {
  const { load } = getStatisticsAccessState();
  if (load.status === 'ready') {
    const maxAgeMs = params.maxAgeMs ?? accessCacheTtlMs;
    if (Date.now() - load.fetchedAt <= maxAgeMs) {
      return load.access;
    }
  }
  if (load.status === 'loading' && inFlight?.id === load.requestId) {
    return withAbortSignal(inFlight.promise, params.signal);
  }
  return refreshStatisticsAccess({ signal: params.signal });
};
