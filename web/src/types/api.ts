import type { NodeTrafficDirectionMode, TrafficCycleMode } from './traffic';

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
  traffic_cycle_mode: TrafficCycleMode;
  traffic_billing_start_day: number;
  traffic_billing_anchor_date: string;
  traffic_billing_timezone: string;
  traffic_direction_mode: NodeTrafficDirectionMode;
  secret: string;
  tags: string[];
  display_order: number;
  group_ids: number[];
  version: NodeVersion;
}

export interface UpdateNodeInput {
  name?: string;
  is_guest_visible?: boolean;
  traffic_p95_enabled?: boolean;
  traffic_cycle_mode?: TrafficCycleMode;
  traffic_billing_start_day?: number;
  traffic_billing_anchor_date?: string;
  traffic_billing_timezone?: string;
  traffic_direction_mode?: NodeTrafficDirectionMode;
  display_order?: number;
  tags?: string[];
  secret?: string;
  group_ids?: number[];
}

export type NodeTrafficPatch = Pick<
  UpdateNodeInput,
  | 'traffic_cycle_mode'
  | 'traffic_billing_start_day'
  | 'traffic_billing_anchor_date'
  | 'traffic_billing_timezone'
  | 'traffic_direction_mode'
>;

export interface NodeVersion {
  version: string;
  is_outdated: boolean;
  supports_auto_update: boolean;
}

export interface AppVersion {
  version: string;
  node_version: string;
}

export type NodeDeployPlatform = 'linux' | 'macos' | 'windows';

interface DeployScript {
  url: string;
  command_prefix: string;
}

export interface NodeDeploy {
  scripts: Record<NodeDeployPlatform, DeployScript>;
}
