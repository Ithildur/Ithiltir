export interface PVEVM {
  collected_at?: string;
  ttl_seconds?: number;
  id: number;
  name: string;
  status: string;
  qmp_status?: string;
  template: boolean;
  cpus?: number;
  cpu_ratio?: number;
  memory_bytes?: number;
  memory_limit_bytes?: number;
  disk_capacity_bytes?: number;
  disk_read_bytes?: number;
  disk_write_bytes?: number;
  net_in_bytes?: number;
  net_out_bytes?: number;
  net_rate_limits_mbps?: Record<string, number>;
  uptime_seconds?: number;
  ips?: string[];
  ips_collected_at?: string;
  ips_ttl_seconds?: number;
}

export interface PVESnapshot {
  schema: number;
  provider: string;
  host: string;
  collected_at: string;
  last_success_at?: string;
  ttl_seconds: number;
  status: string;
  error?: string;
  vms: PVEVM[] | null;
}

export interface PVEView {
  vm_cpu_threshold: number;
  vm_memory_threshold: number;
  vm_memory_enabled: boolean;
  snapshot: PVESnapshot | null;
  received_at: string | null;
  stale: boolean;
}

export type PVETimeframe = 'hour' | 'day' | 'week' | 'month' | 'year';
export type PVEConsolidation = 'AVERAGE' | 'MAX';

export interface PVEHistoryPoint {
  timestamp: string;
  cpu_ratio?: number;
  memory_bytes?: number;
  memory_limit_bytes?: number;
  disk_read_bytes_per_second?: number;
  disk_write_bytes_per_second?: number;
  net_in_bytes_per_second?: number;
  net_out_bytes_per_second?: number;
}

export interface PVEHistory {
  source: 'pve_rrd';
  vm_id: number;
  timeframe: PVETimeframe;
  consolidation: PVEConsolidation;
  collected_at: string;
  points: PVEHistoryPoint[];
}
