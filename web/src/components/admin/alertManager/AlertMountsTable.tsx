import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import Checkbox from '@components/ui/Checkbox';
import IOSSwitch from '@components/ui/IOSSwitch';
import type { AlertMountNode, AlertMountRule } from '@app-types/admin';
import { useI18n } from '@i18n';
import type { AlertMountsBatchMode } from './useAlertMountsPanel';

type RuleLabel = (rule: AlertMountRule) => string;
type MountedLookup = (nodeId: number, ruleId: number) => boolean;

interface Props {
  nodes: AlertMountNode[];
  visibleBuiltinRules: AlertMountRule[];
  showCustomRules: boolean;
  allRuleIds: number[];
  selectedNodeIds: number[];
  selectedNodes: ReadonlySet<number>;
  allVisibleSelected: boolean;
  someVisibleSelected: boolean;
  tableColumnCount: number;
  saving: boolean;
  canOpenBatch: boolean;
  onToggleVisibleNodes: () => void;
  onToggleSelectedNode: (id: number) => void;
  onOpenCustomNode: (id: number) => void;
  onStartBatch: (mode: AlertMountsBatchMode) => void;
  onSetMounts: (ruleIds: number[], serverIds: number[], mounted: boolean) => Promise<boolean>;
  ruleName: RuleLabel;
  mounted: MountedLookup;
  customMountedCount: (nodeId: number) => number;
}

export const AlertMountsTable = ({
  nodes,
  visibleBuiltinRules,
  showCustomRules,
  allRuleIds,
  selectedNodeIds,
  selectedNodes,
  allVisibleSelected,
  someVisibleSelected,
  tableColumnCount,
  saving,
  canOpenBatch,
  onToggleVisibleNodes,
  onToggleSelectedNode,
  onOpenCustomNode,
  onStartBatch,
  onSetMounts,
  ruleName,
  mounted,
  customMountedCount,
}: Props) => {
  const { t } = useI18n();

  return (
    <Card className="overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
          <thead className="border-b border-(--theme-border-subtle) bg-(--theme-bg-muted) text-xs font-semibold text-(--theme-fg-default) dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle) dark:text-(--theme-fg-default)">
            <tr>
              <th className="w-10 px-3 py-2.5 align-middle">
                <div className="flex h-5 items-center">
                  <Checkbox
                    checked={allVisibleSelected}
                    indeterminate={!allVisibleSelected && someVisibleSelected}
                    onChange={onToggleVisibleNodes}
                    aria-label={t('admin_alerts_mounts_select_visible')}
                  />
                </div>
              </th>
              <th className="min-w-48 px-3 py-2.5 align-middle">
                <div className="flex h-5 items-center">{t('admin_nodes_column_node')}</div>
              </th>
              {visibleBuiltinRules.map((rule) => (
                <th key={rule.id} className="min-w-36 px-3 py-2.5 align-middle">
                  <div className="flex h-5 items-center">{ruleName(rule)}</div>
                </th>
              ))}
              {showCustomRules && (
                <th className="min-w-40 px-3 py-2.5 align-middle">
                  <div className="flex h-5 items-center">
                    {t('admin_alerts_mounts_custom_rules')}
                  </div>
                </th>
              )}
              <th className="w-36 px-3 py-2.5 text-right" aria-label={t('common_actions')} />
            </tr>
          </thead>
          <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
            {nodes.length === 0 ? (
              <tr>
                <td
                  colSpan={tableColumnCount}
                  className="px-4 py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)"
                >
                  {t('no_data')}
                </td>
              </tr>
            ) : (
              nodes.map((node) => (
                <tr
                  key={node.id}
                  className="transition-colors hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)"
                >
                  <td className="px-3 py-2.5 align-middle">
                    <Checkbox
                      checked={selectedNodes.has(node.id)}
                      onChange={() => onToggleSelectedNode(node.id)}
                      aria-label={t('admin_alerts_mounts_select_node', { name: node.name })}
                    />
                  </td>
                  <td className="px-3 py-2.5">
                    <div className="flex flex-col">
                      <span className="font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-default)">
                        {node.name}
                      </span>
                      <span className="font-mono text-[11px] text-(--theme-fg-muted-alt)">
                        {node.ip || node.hostname || `ID: ${node.id}`}
                      </span>
                    </div>
                  </td>
                  {visibleBuiltinRules.map((rule) => {
                    const isMounted = mounted(node.id, rule.id);
                    return (
                      <td key={rule.id} className="px-3 py-2.5">
                        <IOSSwitch
                          size="sm"
                          checked={isMounted}
                          disabled={saving}
                          onChange={() => {
                            void onSetMounts([rule.id], [node.id], !isMounted);
                          }}
                        />
                      </td>
                    );
                  })}
                  {showCustomRules && (
                    <td className="px-3 py-2.5">
                      <button
                        type="button"
                        onClick={() => onOpenCustomNode(node.id)}
                        className="inline-flex items-center rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-muted) px-2 py-1 text-xs font-semibold text-(--theme-fg-muted) hover:text-(--theme-fg-default) dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle)"
                      >
                        {t('admin_alerts_mounts_custom_enabled_count', {
                          count: String(customMountedCount(node.id)),
                        })}
                      </button>
                    </td>
                  )}
                  <td className="px-3 py-2.5 text-right">
                    <button
                      type="button"
                      disabled={saving || allRuleIds.length === 0}
                      onClick={() => {
                        void onSetMounts(allRuleIds, [node.id], false);
                      }}
                      className="rounded-md px-2 py-1 text-xs font-semibold text-(--theme-bg-danger-emphasis) hover:bg-(--theme-bg-danger-muted) disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-(--theme-bg-danger-subtle)"
                    >
                      {t('admin_alerts_mounts_disable_all')}
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
      {selectedNodeIds.length > 0 && (
        <div className="flex flex-col gap-2 border-t border-(--theme-border-subtle) bg-(--theme-bg-muted) px-4 py-3 text-xs text-(--theme-fg-muted) sm:flex-row sm:items-center sm:justify-between dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle) dark:text-(--theme-fg-muted)">
          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="secondary"
              disabled={!canOpenBatch}
              onClick={() => onStartBatch('mount')}
            >
              {t('admin_alerts_mounts_apply')}
            </Button>
            <Button
              variant="secondary"
              disabled={!canOpenBatch}
              onClick={() => onStartBatch('unmount')}
            >
              {t('admin_alerts_mounts_cancel')}
            </Button>
          </div>
          <div className="flex flex-wrap items-center gap-3 sm:justify-end">
            <span>
              {t('admin_alerts_mounts_selected_nodes', {
                count: String(selectedNodeIds.length),
              })}
            </span>
          </div>
        </div>
      )}
    </Card>
  );
};
