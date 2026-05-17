export interface ErrorResponse {
  code: string;
  message: string;
}

export interface LoginResponse {
  access_token: string;
  expires_at: string;
  csrf_token: string;
}

export interface Group {
  id: number;
  name: string;
  remark: string;
  server_count: number;
}

export interface ManagedNode {
  id: number;
  name: string;
  hostname?: string | null;
  ip?: string | null;
  is_guest_visible: boolean;
  traffic_p95_enabled: boolean;
  traffic_cycle_mode: 'default' | 'calendar_month' | 'whmcs_compatible' | 'clamp_to_month_end';
  traffic_billing_start_day: number;
  traffic_billing_anchor_date: string;
  traffic_billing_timezone: string;
  secret: string;
  tags: string[];
  display_order: number;
  group_ids: number[];
  version?: NodeVersion | null;
}

export interface NodeVersion {
  version: string;
  is_outdated: boolean;
}

export interface AppVersion {
  version: string;
  node_version: string;
}

export type NodeDeployPlatform = 'linux' | 'macos' | 'windows';

export interface DeployScript {
  url: string;
  command_prefix: string;
}

export interface NodeDeploy {
  scripts: Record<NodeDeployPlatform, DeployScript>;
}
