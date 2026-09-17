export interface UptimeDaily {
  enabled: boolean;
  timezone: string;
  generated_at: string;
  warning_sla: number;
  error_sla: number;
  nodes: Array<{
    server_id: string;
    days: Array<{ date: string; percent: number | null; observed_ms: number }>;
  }>;
}

export interface UptimeHours {
  date: string;
  hours: Array<number | null>;
  observed_ms: number[];
}
