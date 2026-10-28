import type { Group, ManagedNode } from '@app-types/api';
import type { NodeRow } from '@app-types/admin';
import { isPVESystem } from '@components/system/SystemLogo';

export const buildGroupLookup = (groups: Group[]): Record<number, string> =>
  groups.reduce(
    (acc, group) => {
      acc[group.id] = group.name;
      return acc;
    },
    {} as Record<number, string>,
  );

const nodeRowFromManaged = (node: ManagedNode, groupLookup: Record<number, string>): NodeRow => {
  const groupNames = node.group_ids.map((groupId) => groupLookup[groupId] ?? `#${groupId}`);
  const resolvedHostname =
    typeof node.hostname === 'string' && node.hostname.trim() ? node.hostname.trim() : '';

  return {
    pve: isPVESystem({
      os: node.os ?? undefined,
      platform: node.platform ?? undefined,
      kernelVersion: node.kernel_version ?? undefined,
    }),
    vmCPUThreshold: node.vm_cpu_threshold,
    vmMemoryThreshold: node.vm_memory_threshold,
    vmMemoryEnabled: node.vm_memory_enabled,
    id: node.id,
    name: node.name,
    hostname: resolvedHostname,
    ip: node.ip ?? '',
    groupIds: node.group_ids,
    groupNames,
    secret: node.secret,
    tags: node.tags,
    version: node.version,
    guestVisible: node.is_guest_visible,
    trafficP95Enabled: node.traffic_p95_enabled,
    trafficCycleMode: node.traffic_cycle_mode,
    trafficBillingStartDay: node.traffic_billing_start_day,
    trafficBillingAnchorDate: node.traffic_billing_anchor_date,
    trafficBillingTimezone: node.traffic_billing_timezone,
    trafficDirectionMode: node.traffic_direction_mode,
    displayOrder: node.display_order,
  };
};

export const nodeRowsFromManaged = (
  nodes: ManagedNode[],
  groupLookup: Record<number, string>,
): NodeRow[] => {
  return nodes
    .slice()
    .sort((a, b) => b.display_order - a.display_order)
    .map((node) => nodeRowFromManaged(node, groupLookup));
};
