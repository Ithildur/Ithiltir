export type ScheduleHistory = (load: (serverId: number) => Promise<void>) => () => void;

export const createHistoryQueue = (serverId: number): ScheduleHistory => {
  const pending = new Set<() => void>();
  let active = 0;

  return (load) => {
    const start = async () => {
      active++;
      try {
        await load(serverId);
      } finally {
        active--;
        const next = pending.values().next().value;
        if (next) {
          pending.delete(next);
          next();
        }
      }
    };

    if (active < 4) void start();
    else pending.add(start);
    return () => pending.delete(start);
  };
};
