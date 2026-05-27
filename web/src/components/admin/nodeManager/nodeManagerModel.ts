import type { Group, ManagedNode } from '@app-types/api';
import type { NodeRow } from '@app-types/admin';
import type { TranslationKey } from '@i18n';

type Translate = (key: TranslationKey, vars?: Record<string, string | number>) => string;

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
    version: {
      version: node.version?.version ?? '',
      is_outdated: node.version?.is_outdated ?? false,
      supports_auto_update: node.version?.supports_auto_update ?? false,
    },
    guestVisible: node.is_guest_visible,
    trafficP95Enabled: node.traffic_p95_enabled,
    trafficCycleMode: node.traffic_cycle_mode,
    trafficBillingStartDay: node.traffic_billing_start_day,
    trafficBillingAnchorDate: node.traffic_billing_anchor_date,
    trafficBillingTimezone: node.traffic_billing_timezone,
    trafficDirectionMode: node.traffic_direction_mode,
    displayOrder: node.display_order ?? 0,
  };
};

export const nodeRowsFromManaged = (
  nodes: ManagedNode[],
  groupLookup: Record<number, string>,
): NodeRow[] => {
  return nodes
    .slice()
    .sort((a, b) => (b.display_order ?? 0) - (a.display_order ?? 0))
    .map((node) => nodeRowFromManaged(node, groupLookup));
};

export const nodeTrafficSettingsLabel = (node: NodeRow, t: Translate): string => {
  const cycle =
    node.trafficCycleMode === 'default'
      ? t('admin_node_cycle_mode_inherited')
      : t(`traffic_cycle_${node.trafficCycleMode}` as TranslationKey);

  if (node.trafficDirectionMode === 'default') return cycle;
  return `${cycle} / ${t(`traffic_direction_${node.trafficDirectionMode}` as TranslationKey)}`;
};
