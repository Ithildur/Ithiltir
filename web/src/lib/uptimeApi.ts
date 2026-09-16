import { apiFetch } from './api';
import type { UptimeDaily, UptimeHours } from '@app-types/uptime';

export const fetchUptime = (signal: AbortSignal) =>
  apiFetch<UptimeDaily>('/metrics/uptime', { signal, timeoutMs: 10_000 });

export const fetchUptimeDay = (serverID: string, date: string, signal: AbortSignal) =>
  apiFetch<UptimeHours>(
    `/metrics/uptime/day?${new URLSearchParams({ server_id: serverID, date })}`,
    { signal, timeoutMs: 10_000 },
  );
