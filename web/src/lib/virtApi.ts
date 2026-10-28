import { apiFetch } from './api';
import type { PVEConsolidation, PVEHistory, PVETimeframe, PVEView } from '@app-types/virt';

export const fetchPVE = (serverId: number, signal: AbortSignal) =>
  apiFetch<PVEView>(`/virt/${serverId}`, { signal, timeoutMs: 15_000 });

export const fetchPVEHistory = (
  serverId: number,
  vmId: number,
  timeframe: PVETimeframe,
  consolidation: PVEConsolidation,
  signal: AbortSignal,
) => {
  const query = new URLSearchParams({ timeframe, consolidation });
  return apiFetch<PVEHistory>(`/virt/${serverId}/vms/${vmId}/history?${query}`, {
    signal,
    timeoutMs: 15_000,
  });
};
