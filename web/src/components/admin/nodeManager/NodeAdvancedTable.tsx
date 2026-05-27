import React from 'react';
import Checkbox from '@components/ui/Checkbox';
import type { NodeRow } from '@app-types/admin';
import { useI18n } from '@i18n';
import { NodeP95Switch, TrafficRebuildButton, TrafficSettingsButton } from './NodeAdvancedActions';

export interface Props {
  nodes: NodeRow[];
  selectedP95NodeIds: Set<number>;
  allVisibleP95Selected: boolean;
  someVisibleP95Selected: boolean;
  savingP95NodeIds: Set<number>;
  savingTrafficSettingsNodeIds: Set<number>;
  rebuildingTrafficNodeId: number | null;
  trafficRebuildBusy: boolean;
  onToggleVisibleNodes: () => void;
  onToggleP95Node: (id: number) => void;
  onToggleTrafficP95: (node: NodeRow) => void;
  onOpenTrafficSettings: (node: NodeRow) => void;
  onRebuildTraffic: (node: NodeRow) => void;
}

const NodeAdvancedTable: React.FC<Props> = ({
  nodes,
  selectedP95NodeIds,
  allVisibleP95Selected,
  someVisibleP95Selected,
  savingP95NodeIds,
  savingTrafficSettingsNodeIds,
  rebuildingTrafficNodeId,
  trafficRebuildBusy,
  onToggleVisibleNodes,
  onToggleP95Node,
  onToggleTrafficP95,
  onOpenTrafficSettings,
  onRebuildTraffic,
}) => {
  const { t } = useI18n();

  return (
    <table className="w-full text-sm text-left bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
      <thead className="bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-default) text-xs font-semibold whitespace-nowrap border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
        <tr>
          <th className="px-3 py-2.5 w-10">
            <div className="flex h-5 items-center">
              <Checkbox
                checked={allVisibleP95Selected}
                indeterminate={!allVisibleP95Selected && someVisibleP95Selected}
                onChange={onToggleVisibleNodes}
                aria-label={t('admin_nodes_select_visible')}
              />
            </div>
          </th>
          <th className="px-3 py-2.5">{t('admin_nodes_column_node')}</th>
          <th className="px-3 py-2.5 w-32">{t('admin_nodes_column_ip')}</th>
          <th className="px-3 py-2.5 w-44">{t('admin_nodes_column_traffic_settings')}</th>
          <th className="px-3 py-2.5 w-20">{t('admin_nodes_column_traffic_p95')}</th>
          <th className="px-3 py-2.5 w-16">{t('admin_nodes_column_traffic_rebuild')}</th>
        </tr>
      </thead>
      <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
        {nodes.length === 0 ? (
          <tr>
            <td
              colSpan={6}
              className="px-4 py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-action-muted)"
            >
              {t('no_data')}
            </td>
          </tr>
        ) : (
          nodes.map((node) => {
            const rebuilding = rebuildingTrafficNodeId === node.id;
            return (
              <tr
                key={node.id}
                className="transition-colors duration-150 group hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)"
              >
                <td className="px-3 py-2">
                  <div className="flex h-7 items-center">
                    <Checkbox
                      checked={selectedP95NodeIds.has(node.id)}
                      onChange={() => onToggleP95Node(node.id)}
                      aria-label={t('admin_nodes_select_node', { name: node.name })}
                    />
                  </div>
                </td>
                <td className="px-3 py-2">
                  <div className="font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-default)">
                    {node.name}
                  </div>
                </td>
                <td className="px-3 py-2 text-xs font-mono w-32">
                  {node.ip || t('admin_nodes_unconfigured')}
                </td>
                <td className="px-3 py-2 text-xs w-44">
                  <TrafficSettingsButton
                    node={node}
                    disabled={savingTrafficSettingsNodeIds.has(node.id)}
                    onOpen={onOpenTrafficSettings}
                  />
                </td>
                <td className="px-3 py-2 text-xs w-20">
                  <NodeP95Switch
                    node={node}
                    disabled={savingP95NodeIds.has(node.id)}
                    onToggle={onToggleTrafficP95}
                  />
                </td>
                <td className="px-3 py-2 text-xs w-16">
                  <TrafficRebuildButton
                    node={node}
                    disabled={trafficRebuildBusy}
                    rebuilding={rebuilding}
                    onRebuild={onRebuildTraffic}
                  />
                </td>
              </tr>
            );
          })
        )}
      </tbody>
    </table>
  );
};

export default NodeAdvancedTable;
