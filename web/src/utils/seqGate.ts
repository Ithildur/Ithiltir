export class StaleLoadError extends Error {
  constructor() {
    super('stale load');
    this.name = 'StaleLoadError';
  }
}

export const isStaleLoadError = (error: unknown): error is StaleLoadError =>
  error instanceof StaleLoadError;

export const createSeqGate = () => {
  let seq = 0;

  const next = (): number => {
    seq += 1;
    return seq;
  };

  return {
    next,
    invalidate: next,
    isCurrent: (value: number): boolean => value === seq,
  };
};

export type SeqGate = ReturnType<typeof createSeqGate>;

type LoadingSetter = (loading: boolean) => void;

const invalidateLatestLoad = (gate: SeqGate, setLoading?: LoadingSetter): number => {
  const seq = gate.invalidate();
  setLoading?.(false);
  return seq;
};

export const runLatestLoad = async <T>(
  gate: SeqGate,
  load: () => Promise<T>,
  apply: (value: T) => void,
  setLoading?: LoadingSetter,
): Promise<T> => {
  const seq = gate.next();
  setLoading?.(true);
  try {
    const value = await load();
    if (!gate.isCurrent(seq)) throw new StaleLoadError();
    apply(value);
    return value;
  } catch (error) {
    if (!gate.isCurrent(seq)) throw new StaleLoadError();
    throw error;
  } finally {
    if (gate.isCurrent(seq)) setLoading?.(false);
  }
};

export const reloadLatestLoad = async <T>(
  gate: SeqGate,
  load: () => Promise<T>,
  apply: (value: T) => void,
  setLoading?: LoadingSetter,
): Promise<void> => {
  const seq = invalidateLatestLoad(gate, setLoading);
  try {
    const value = await load();
    if (!gate.isCurrent(seq)) return;
    apply(value);
  } catch (error) {
    if (!gate.isCurrent(seq)) return;
    throw error;
  }
};
