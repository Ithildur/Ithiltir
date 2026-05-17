import { apiFetch } from '@lib/api';
import type {
  MetricHistoryAggregation,
  MetricHistoryRange,
  MetricHistory,
} from '@app-types/metricsHistory';

export interface MetricHistoryParams {
  serverId: number;
  metric: string;
  range: MetricHistoryRange;
  aggregation?: MetricHistoryAggregation;
  device?: string;
  signal?: AbortSignal;
}

export const fetchMetricHistory = (params: MetricHistoryParams) => {
  const search = new URLSearchParams({
    server_id: String(params.serverId),
    metric: params.metric,
    range: params.range,
  });

  if (params.aggregation) search.set('agg', params.aggregation);
  if (params.device) search.set('device', params.device);

  return apiFetch<MetricHistory>(`/metrics/history?${search.toString()}`, {
    method: 'GET',
    signal: params.signal,
  });
};
