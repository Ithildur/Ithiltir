export type MetricHistoryRange = '30m' | '1h' | '12h' | '24h' | '1w' | '15d' | '30d';

export type MetricHistoryAggregation = 'avg' | 'max' | 'min' | 'last';

export type MetricHistoryPoint = {
  ts: string;
  value: number | null;
};

export type MetricHistory = {
  server_id: number;
  metric: string;
  range: MetricHistoryRange;
  agg: MetricHistoryAggregation;
  step_sec: number;
  points: MetricHistoryPoint[];
};
