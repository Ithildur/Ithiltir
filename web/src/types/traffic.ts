export type TrafficGuestAccessMode = 'disabled' | 'by_node';
export type TrafficUsageMode = 'lite' | 'billing';
export type TrafficCycleMode = 'calendar_month' | 'whmcs_compatible' | 'clamp_to_month_end';
export type TrafficDirectionMode = 'out' | 'both' | 'max';
export type TrafficPeriod = 'current' | 'previous';
export type TrafficSelectedDirection = '' | 'in' | 'out' | 'total';
export type TrafficP95Status =
  | 'available'
  | 'disabled'
  | 'lite_mode'
  | 'insufficient_samples'
  | 'snapshot_without_p95';

export interface StatisticsAccess {
  history_guest_access_mode: 'disabled' | 'by_node';
  traffic_guest_access_mode: TrafficGuestAccessMode;
}

export interface TrafficSettings {
  guest_access_mode: TrafficGuestAccessMode;
  usage_mode: TrafficUsageMode;
  cycle_mode: TrafficCycleMode;
  billing_start_day: number;
  billing_anchor_date: string;
  billing_timezone: string;
  direction_mode: TrafficDirectionMode;
}

export interface TrafficIface {
  name: string;
}

export interface TrafficCycle {
  mode: TrafficCycleMode;
  billing_start_day: number;
  billing_anchor_date?: string;
  timezone: string;
  start: string;
  end: string;
}

export interface TrafficStats {
  in_bytes: number;
  out_bytes: number;
  p95_enabled: boolean;
  p95_status: TrafficP95Status;
  p95_unavailable_reason?: string;
  in_p95_bytes_per_sec: number | null;
  out_p95_bytes_per_sec: number | null;
  in_peak_bytes_per_sec: number;
  out_peak_bytes_per_sec: number;
  selected_bytes: number;
  selected_p95_bytes_per_sec: number | null;
  selected_peak_bytes_per_sec: number;
  selected_bytes_direction: TrafficSelectedDirection;
  selected_p95_direction?: TrafficSelectedDirection;
  selected_peak_direction: TrafficSelectedDirection;
  sample_count: number;
  expected_sample_count: number;
  effective_start: string;
  effective_end: string;
  coverage_ratio: number;
  covered_until: string;
  gap_count: number;
  reset_count: number;
  cycle_complete: boolean;
  data_complete: boolean;
  status: 'provisional' | 'grace' | 'sealed' | 'stale';
  /** @deprecated Use coverage_ratio for display decisions. */
  partial: boolean;
}

export interface TrafficSummary {
  server_id: number;
  server_name?: string;
  iface: string;
  usage_mode: TrafficUsageMode;
  direction_mode: TrafficDirectionMode;
  cycle: TrafficCycle;
  stats: TrafficStats;
}

export interface TrafficMonthly {
  includes_current: boolean;
  items: TrafficSummary[];
}

export interface TrafficDailyItem {
  server_id: number;
  iface: string;
  usage_mode: TrafficUsageMode;
  direction_mode: TrafficDirectionMode;
  start: string;
  end: string;
  stats: TrafficStats;
}

export interface TrafficDaily {
  items: TrafficDailyItem[];
}
