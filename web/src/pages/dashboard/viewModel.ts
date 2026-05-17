import type { NodeView } from '@app-types/frontMetrics';
import {
  DASHBOARD_OFFLINE_THRESHOLD_MS,
  DASHBOARD_USAGE_DANGER_THRESHOLD,
  DASHBOARD_USAGE_WARN_THRESHOLD,
} from '@config/dashboard';

export const formatBytes = (bytes: number | null | undefined, decimals: number = 1): string => {
  if (bytes == null || bytes === 0) return '0 B';

  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const;

  const i = Math.min(sizes.length - 1, Math.max(0, Math.floor(Math.log(bytes) / Math.log(k))));

  const value = bytes / Math.pow(k, i);
  return `${parseFloat(value.toFixed(dm))} ${sizes[i]}`;
};

export const getColorByPercent = (percent: number): string => {
  if (percent < DASHBOARD_USAGE_WARN_THRESHOLD)
    return 'text-(--theme-fg-success-muted) dark:text-(--theme-fg-success-muted)';
  if (percent < DASHBOARD_USAGE_DANGER_THRESHOLD)
    return 'text-(--theme-fg-warning-muted) dark:text-(--theme-fg-warning)';
  return 'text-(--theme-fg-danger-muted) dark:text-(--theme-fg-danger)';
};

export const ratioToPercent = (value: number | null | undefined): number => {
  if (value == null) return 0;
  if (!Number.isFinite(value)) return 0;
  return value * 100;
};

export const normalizeNodeViews = (nodes: NodeView[]): NodeView[] =>
  Array.isArray(nodes) ? [...nodes] : [];

export interface ServerViewModel {
  id: string;
  hostname: string;
  searchText: string;
  tags: string[];
  uptime: string;
  isAlive: boolean;

  system: {
    os: string;
    platform: string;
    platformVersion: string;
    kernelVersion: string;
    arch?: string;
  };

  cpu: {
    usagePercent: number;
    load1: number;
    load5: number;
    load15: number;
    modelName: string;
    hasDetails: boolean;
    coresLogical?: number;
  };

  memory: {
    used: number;
    total: number;
    available: number;
    buffers: number;
    cached: number;
    usedPercent: number;
    swapUsed: number;
    swapTotal: number;
  };

  disk: {
    path: string;
    fsType: string;
    used: number;
    total: number;
    usedPercent: number;
  };

  network: {
    rateIn: number;
    rateOut: number;
    totalIn: number;
    totalOut: number;
  };

  raid: {
    available: boolean;
    primaryArray?: {
      name: string;
      status: string;
      health: string;
      syncStatus?: string;
      syncProgress?: string;
    };
    showAlert: boolean;
  };

  diskIo: {
    deviceName: string;
    readRate: number;
    writeRate: number;
    readIops: number;
    writeIops: number;
  };

  connections: {
    tcpCount: number;
    udpCount: number;
  };

  processes: {
    count: number;
  };
}

const pickPrimaryMount = (node: NodeView): NodeView['disk']['mounts'][number] => {
  const mounts = node.disk?.mounts ?? [];
  const root = mounts.find((m) => m.mountpoint === '/');
  return (
    root ??
    mounts[0] ?? {
      mountpoint: '/',
      fs_type: '',
      total_bytes: 0,
      used_bytes: 0,
      used_ratio: 0,
    }
  );
};

const deriveSearchText = (node: NodeView): string => {
  const tokens = [node.node.title, ...(node.node.search_text ?? [])]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
  return tokens;
};

const resolveObservedAgeMs = (observedAt: string): number => {
  const observedAtMs = Date.parse(observedAt);
  if (Number.isNaN(observedAtMs)) return Number.POSITIVE_INFINITY;
  return Date.now() - observedAtMs;
};

const computeIsAlive = (node: NodeView): boolean => {
  const staleAfterSec = node.observation?.stale_after_sec;
  const thresholdMs = Number.isFinite(staleAfterSec)
    ? Math.max(0, staleAfterSec) * 1000
    : DASHBOARD_OFFLINE_THRESHOLD_MS;
  return (
    resolveObservedAgeMs(node.observation.received_at ?? node.observation.observed_at) <=
    thresholdMs
  );
};

const deriveRaid = (node: NodeView): ServerViewModel['raid'] => {
  const arrays = node.raid?.arrays ?? [];
  const primaryArray = arrays[0]
    ? {
        name: arrays[0].name,
        status: arrays[0].status,
        health: arrays[0].health,
        syncStatus: arrays[0].sync_status,
        syncProgress: arrays[0].sync_progress,
      }
    : undefined;
  const showAlert = arrays.some((array) => {
    const failed = Number.isFinite(array.failed) ? array.failed : 0;
    if (failed > 0) return true;
    return typeof array.health === 'string' ? array.health.toLowerCase() !== 'healthy' : false;
  });
  return {
    available: Boolean(node.raid?.available),
    primaryArray,
    showAlert,
  };
};

const deriveDiskIo = (node: NodeView): ServerViewModel['diskIo'] => {
  const total = node.disk?.io?.total ?? {
    read_bps: 0,
    write_bps: 0,
    read_iops: 0,
    write_iops: 0,
    iops: 0,
  };
  const entries = node.disk?.io?.by_device ? Object.entries(node.disk.io.by_device) : [];
  const deviceName = entries[0]?.[0] ?? '';
  const device = entries[0]?.[1];
  return {
    deviceName,
    readRate: device?.read_bps ?? total.read_bps,
    writeRate: device?.write_bps ?? total.write_bps,
    readIops: device?.read_iops ?? total.read_iops,
    writeIops: device?.write_iops ?? total.write_iops,
  };
};

export const buildServerViewModel = (node: NodeView): ServerViewModel => {
  const mount = pickPrimaryMount(node);
  const isAlive = computeIsAlive(node);
  const searchText = deriveSearchText(node);
  const raid = deriveRaid(node);
  const diskIo = deriveDiskIo(node);

  return {
    id: node.node.id,
    hostname: node.node.title,
    searchText,
    tags: node.node.tags ?? [],
    uptime: node.system.uptime_text,
    isAlive,
    system: {
      os: node.system.os_family,
      platform: node.system.platform,
      platformVersion: node.system.platform_version,
      kernelVersion: node.system.kernel_version,
      arch: node.system.arch,
    },
    cpu: {
      usagePercent: ratioToPercent(node.cpu.usage_ratio),
      load1: node.cpu.load.l1,
      load5: node.cpu.load.l5,
      load15: node.cpu.load.l15,
      modelName: node.cpu.model_name ?? '',
      hasDetails: Boolean(node.cpu.model_name || node.cpu.cores_logical || node.cpu.cores_physical),
      coresLogical: node.cpu.cores_logical,
    },
    memory: {
      used: node.memory.used_bytes,
      total: node.memory.total_bytes,
      available: node.memory.available_bytes,
      buffers: node.memory.buffers_bytes,
      cached: node.memory.cached_bytes,
      usedPercent: ratioToPercent(node.memory.used_ratio),
      swapUsed: node.memory.swap_used_bytes,
      swapTotal: node.memory.swap_total_bytes,
    },
    disk: {
      path: mount.mountpoint,
      fsType: mount.fs_type,
      used: mount.used_bytes,
      total: mount.total_bytes,
      usedPercent: ratioToPercent(mount.used_ratio),
    },
    network: {
      rateIn: node.network.total.recv_bps,
      rateOut: node.network.total.sent_bps,
      totalIn: node.network.total.bytes_recv,
      totalOut: node.network.total.bytes_sent,
    },
    raid,
    diskIo,
    connections: {
      tcpCount: node.connections.tcp,
      udpCount: node.connections.udp,
    },
    processes: {
      count: node.processes.count,
    },
  };
};
