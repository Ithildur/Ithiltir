import type { NodeRow } from '@app-types/admin';
import type { TranslationKey } from '@i18n';
import { isVersionUpdateTarget } from '@utils/version';
import { nodeTrafficCycleLabelKey, nodeTrafficDirectionLabelKey } from '@lib/trafficSettingsModel';

type Translate = (key: TranslationKey, vars?: Record<string, string | number>) => string;

export const nodeHasBundledUpdate = (node: NodeRow, bundledNodeVersion: string): boolean => {
  if (!bundledNodeVersion) return false;
  return isVersionUpdateTarget(node.version.version, bundledNodeVersion);
};

export const nodeCanRequestUpgrade = (node: NodeRow, bundledNodeVersion: string): boolean =>
  !node.version.is_outdated &&
  node.version.supports_auto_update &&
  nodeHasBundledUpdate(node, bundledNodeVersion);

export const nodeNeedsManualUpdate = (node: NodeRow, bundledNodeVersion: string): boolean =>
  !node.version.is_outdated &&
  !node.version.supports_auto_update &&
  nodeHasBundledUpdate(node, bundledNodeVersion);

export const nodeAlertEventsPath = (nodeID: number): string => {
  const params = new URLSearchParams({
    tab: 'alerts',
    alerts_tab: 'events',
    alert_server_id: String(nodeID),
    alert_status: 'open',
    alert_range: 'all',
  });
  return `/admin?${params.toString()}`;
};

const nodeNeedsUpdate = (node: NodeRow, bundledNodeVersion: string): boolean =>
  node.version.is_outdated ||
  nodeCanRequestUpgrade(node, bundledNodeVersion) ||
  nodeNeedsManualUpdate(node, bundledNodeVersion);

export const filterNodeManagerNodes = ({
  nodes,
  search,
  selectedGroupIds,
  updatableOnly,
  bundledNodeVersion,
}: {
  nodes: NodeRow[];
  search: string;
  selectedGroupIds: number[];
  updatableOnly: boolean;
  bundledNodeVersion: string;
}): NodeRow[] => {
  const keyword = search.trim().toLowerCase();
  const selectedGroupSet = new Set(selectedGroupIds);

  return nodes.filter((node) => {
    if (keyword) {
      const haystack = [
        node.name,
        node.ip ?? '',
        String(node.id),
        node.groupNames.join(' '),
        node.tags.join(' '),
      ]
        .join(' ')
        .toLowerCase();

      if (!haystack.includes(keyword)) return false;
    }

    if (
      selectedGroupSet.size > 0 &&
      !node.groupIds.some((groupId) => selectedGroupSet.has(groupId))
    ) {
      return false;
    }

    return !updatableOnly || nodeNeedsUpdate(node, bundledNodeVersion);
  });
};

export const nodeTrafficSettingsLabel = (node: NodeRow, t: Translate): string => {
  const cycle =
    node.trafficCycleMode === 'default'
      ? t('admin_node_cycle_mode_inherited')
      : t(nodeTrafficCycleLabelKey[node.trafficCycleMode]);

  if (node.trafficDirectionMode === 'default') return cycle;
  return `${cycle} / ${t(nodeTrafficDirectionLabelKey[node.trafficDirectionMode])}`;
};
