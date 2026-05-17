import type { Group, ManagedNode } from '@app-types/api';
import type { NodeRow } from '@app-types/admin';

export const reorderVisible = (
  all: NodeRow[],
  visible: NodeRow[],
  sourceId: number,
  targetId: number,
): NodeRow[] => {
  const sourceIdx = visible.findIndex((item) => item.id === sourceId);
  const targetIdx = visible.findIndex((item) => item.id === targetId);
  if (sourceIdx === -1 || targetIdx === -1 || sourceIdx === targetIdx) return all;

  const reorderedVisible = [...visible];
  const [moved] = reorderedVisible.splice(sourceIdx, 1);
  reorderedVisible.splice(targetIdx, 0, moved);

  const visibleQueue = reorderedVisible.slice();
  const visibleIds = new Set(visibleQueue.map((item) => item.id));

  return all.map((item) => (visibleIds.has(item.id) ? visibleQueue.shift()! : item));
};

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
    id: node.id,
    name: node.name,
    hostname: resolvedHostname,
    ip: node.ip ?? '',
    groupIds: node.group_ids,
    groupNames,
    secret: node.secret,
    tags: node.tags,
    version: node.version || { version: '', is_outdated: false },
    guestVisible: node.is_guest_visible,
    trafficP95Enabled: node.traffic_p95_enabled,
    trafficCycleMode: node.traffic_cycle_mode || 'default',
    trafficBillingStartDay: node.traffic_billing_start_day || 1,
    trafficBillingAnchorDate: node.traffic_billing_anchor_date || '',
    trafficBillingTimezone: node.traffic_billing_timezone || '',
    displayOrder: node.display_order ?? 0,
  };
};

export const nodeRowsFromManaged = (
  nodes: ManagedNode[],
  groupLookup: Record<number, string>,
): NodeRow[] =>
  nodes
    .slice()
    .sort((a, b) => (b.display_order ?? 0) - (a.display_order ?? 0))
    .map((node) => nodeRowFromManaged(node, groupLookup));
