import React from 'react';
import Checkbox from '@components/ui/Checkbox';
import IOSSwitch from '@components/ui/IOSSwitch';
import type { NodeRow } from '@app-types/admin';
import type { TrafficCycleMode } from '@app-types/traffic';
import { useI18n, type TranslationKey } from '@i18n';

export interface Props {
  nodes: NodeRow[];
  selectedNodeIds: Set<number>;
  allVisibleSelected: boolean;
  someVisibleSelected: boolean;
  savingNodeIds: Set<number>;
  savingCycleNodeIds: Set<number>;
  onToggleVisibleNodes: () => void;
  onToggleNode: (id: number) => void;
  onToggleTrafficP95: (node: NodeRow) => void;
  onOpenCycleSettings: (node: NodeRow) => void;
}

const NodeAdvancedTable: React.FC<Props> = ({
  nodes,
  selectedNodeIds,
  allVisibleSelected,
  someVisibleSelected,
  savingNodeIds,
  savingCycleNodeIds,
  onToggleVisibleNodes,
  onToggleNode,
  onToggleTrafficP95,
  onOpenCycleSettings,
}) => {
  const { t } = useI18n();
  const cycleLabel = (node: NodeRow) => {
    if (node.trafficCycleMode === 'default') return t('admin_node_cycle_mode_inherited');
    return t(`traffic_cycle_${node.trafficCycleMode as TrafficCycleMode}` as TranslationKey);
  };

  return (
    <table className="w-full text-sm text-left bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
      <thead className="bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-default) text-xs font-semibold whitespace-nowrap border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
        <tr>
          <th className="px-3 py-2.5 w-10">
            <div className="flex h-5 items-center">
              <Checkbox
                checked={allVisibleSelected}
                indeterminate={!allVisibleSelected && someVisibleSelected}
                onChange={onToggleVisibleNodes}
                aria-label={t('admin_nodes_select_visible')}
              />
            </div>
          </th>
          <th className="px-3 py-2.5">{t('admin_nodes_column_node')}</th>
          <th className="px-3 py-2.5 w-32">{t('admin_nodes_column_ip')}</th>
          <th className="px-3 py-2.5 w-36">{t('admin_nodes_column_cycle_mode')}</th>
          <th className="px-3 py-2.5 w-20">{t('admin_nodes_column_traffic_p95')}</th>
        </tr>
      </thead>
      <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
        {nodes.length === 0 ? (
          <tr>
            <td
              colSpan={5}
              className="px-4 py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-action-muted)"
            >
              {t('no_data')}
            </td>
          </tr>
        ) : (
          nodes.map((node) => {
            return (
              <tr
                key={node.id}
                className="transition-colors duration-150 group hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)"
              >
                <td className="px-3 py-2">
                  <div className="flex h-7 items-center">
                    <Checkbox
                      checked={selectedNodeIds.has(node.id)}
                      onChange={() => onToggleNode(node.id)}
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
                <td className="px-3 py-2 text-xs w-36">
                  <button
                    type="button"
                    disabled={savingCycleNodeIds.has(node.id)}
                    onClick={() => onOpenCycleSettings(node)}
                    className="inline-flex max-w-full items-center rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-muted) px-2 py-1 text-xs font-semibold text-(--theme-fg-muted) hover:text-(--theme-fg-default) disabled:cursor-not-allowed disabled:opacity-60 dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle)"
                    aria-label={t('admin_node_cycle_settings_button', { name: node.name })}
                  >
                    <span className="truncate">{cycleLabel(node)}</span>
                  </button>
                </td>
                <td className="px-3 py-2 text-xs w-20">
                  <IOSSwitch
                    size="sm"
                    checked={node.trafficP95Enabled}
                    disabled={savingNodeIds.has(node.id)}
                    ariaLabel={t('admin_node_traffic_p95_toggle', { name: node.name })}
                    onChange={() => onToggleTrafficP95(node)}
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
