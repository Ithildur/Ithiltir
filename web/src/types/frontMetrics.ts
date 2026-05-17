// /api/front/metrics response shape.

export interface GroupView {
  id: number;
  name: string;
  node_ids: number[];
}

export type DiskSmart = {
  status: string;
  updated_at?: string;
  ttl_seconds?: number;
  devices: Array<{
    ref?: string;
    name: string;
    device_path?: string;
    device_type?: string;
    protocol?: string;
    model?: string;
    serial?: string;
    wwn?: string;
    source: string;
    status: string;
    exit_status?: number;
    health?: string;
    temp_c?: number;
    power_on_hours?: number;
    lifetime_used_percent?: number;
    critical_warning?: number;
    failing_attrs?: Array<{
      id?: number;
      name: string;
      when_failed: string;
    }>;
  }>;
};

export type Thermal = {
  status: string;
  updated_at?: string;
  sensors: Array<{
    kind: string;
    name: string;
    sensor_key: string;
    source: string;
    status: string;
    temp_c?: number;
    high_c?: number;
    critical_c?: number;
  }>;
};

export type NodeView = {
  node: {
    id: string;
    order?: number;
    title: string;
    search_text?: string[];
    tags?: string[];
  };

  observation: {
    received_at?: string;
    observed_at: string;
    sent_at?: string;
    stale_after_sec: number;
  };

  system: {
    os_family: string;
    platform: string;
    platform_version: string;
    kernel_version: string;
    arch?: string;
    uptime_text: string;
  };

  cpu: {
    usage_ratio: number;
    load: {
      l1: number;
      l5: number;
      l15: number;
    };
    model_name?: string;
    cores_physical?: number;
    cores_logical?: number;
    sockets?: number;
  };

  memory: {
    total_bytes: number;
    used_bytes: number;
    available_bytes: number;
    buffers_bytes: number;
    cached_bytes: number;
    used_ratio: number;
    swap_total_bytes: number;
    swap_used_bytes: number;
  };

  disk: {
    mounts: Array<{
      mountpoint: string;
      fs_type: string;
      total_bytes: number;
      used_bytes: number;
      used_ratio: number;
    }>;
    io: {
      total: {
        read_bps: number;
        write_bps: number;
        read_iops: number;
        write_iops: number;
        iops: number;
      };
      by_device?: Record<
        string,
        {
          read_bps: number;
          write_bps: number;
          read_iops: number;
          write_iops: number;
          iops: number;
        }
      >;
    };
    temperature_devices?: string[];
    smart?: DiskSmart;
  };

  network: {
    total: {
      bytes_recv: number;
      bytes_sent: number;
      recv_bps: number;
      sent_bps: number;
    };
    by_interface?: Array<{
      name: string;
      bytes_recv: number;
      bytes_sent: number;
      recv_bps: number;
      sent_bps: number;
    }>;
  };

  processes: {
    count: number;
  };

  connections: {
    tcp: number;
    udp: number;
  };

  raid?: {
    supported: boolean;
    available: boolean;
    arrays: Array<{
      name: string;
      status: string;
      active: number;
      working: number;
      failed: number;
      health: string;
      members: Array<{
        name: string;
        state: string;
      }>;
      sync_status?: string;
      sync_progress?: string;
    }>;
  };

  thermal?: Thermal;
};
