import React from 'react';
import ArrowUpFromLine from 'lucide-react/dist/esm/icons/arrow-up-from-line';
import CircleAlert from 'lucide-react/dist/esm/icons/circle-alert';
import Copy from 'lucide-react/dist/esm/icons/copy';
import GripVertical from 'lucide-react/dist/esm/icons/grip-vertical';
import Settings from 'lucide-react/dist/esm/icons/settings';
import Trash2 from 'lucide-react/dist/esm/icons/trash-2';
import { Link } from 'react-router-dom';
import Button from '@components/ui/Button';
import IOSSwitch from '@components/ui/IOSSwitch';
import Input from '@components/ui/Input';
import { Tooltip } from '@components/ui/Tooltip';
import { PlatformLogo } from '@components/system/SystemLogo';
import type { NodeDeployPlatform } from '@app-types/api';
import type { AlertEventSummary, NodeRow } from '@app-types/admin';
import { useI18n } from '@i18n';
import { alertSummaryMetricName } from '@components/admin/alertManager/alertLabels';
import { nodeAlertEventsPath } from './nodeManagerModel';

export interface Props {
  nodes: NodeRow[];
  alertSummaryByServer: ReadonlyMap<number, AlertEventSummary>;
  updatableNodeIds: Set<number>;
  manualUpdateNodeIds: Set<number>;
  savingGuestVisibleNodeIds: Set<number>;
  upgradingNodeIds: Set<number>;
  bundledNodeVersion: string;
  draggingId: number | null;
  dragOverId: number | null;
  onRename: (node: NodeRow, nextName: string) => void;
  onToggleGuestVisible: (node: NodeRow) => void;
  onCopySecret: (secret: string) => void;
  onDeployCopy: (platform: NodeDeployPlatform, secret: string) => void;
  onRequestUpgrade: (node: NodeRow) => void;
  onOpenSettings: (node: NodeRow) => void;
  onDelete: (node: NodeRow) => void;
  onDragStart: (id: number) => (event: React.DragEvent) => void;
  onDragOver: (targetId: number) => (event: React.DragEvent) => void;
  onDrop: (targetId: number) => (event: React.DragEvent) => Promise<void>;
  onDragEnd: () => void;
}

const NodeTable: React.FC<Props> = ({
  nodes,
  alertSummaryByServer,
  updatableNodeIds,
  manualUpdateNodeIds,
  savingGuestVisibleNodeIds,
  upgradingNodeIds,
  bundledNodeVersion,
  draggingId,
  dragOverId,
  onRename,
  onToggleGuestVisible,
  onCopySecret,
  onDeployCopy,
  onRequestUpgrade,
  onOpenSettings,
  onDelete,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
}) => {
  const { lang, t } = useI18n();
  const [editingId, setEditingId] = React.useState<number | null>(null);
  const [draftName, setDraftName] = React.useState('');
  const deployButtonClass =
    'bg-transparent text-(--theme-fg-muted) dark:bg-(--theme-canvas-subtle) dark:text-(--theme-fg-muted) hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-default) dark:hover:bg-(--theme-canvas-muted) dark:hover:text-(--theme-fg-control-hover) p-1.5 text-xs rounded-md border border-(--theme-border-subtle) dark:border-(--theme-border-default) hover:border-(--theme-border-hover) dark:hover:border-(--theme-fg-muted) transition-colors';

  const startEditName = React.useCallback((node: NodeRow) => {
    setEditingId(node.id);
    setDraftName(node.name);
  }, []);

  const commitName = React.useCallback(
    (node: NodeRow) => {
      if (editingId !== node.id) return;

      const trimmed = draftName.trim();
      setEditingId(null);
      if (!trimmed || trimmed === node.name.trim()) {
        setDraftName(node.name);
        return;
      }

      onRename(node, trimmed);
    },
    [draftName, editingId, onRename],
  );

  const renderAlertSummary = (node: NodeRow) => {
    const summary = alertSummaryByServer.get(node.id);
    if (!summary) return null;
    const rawMetrics = summary.metrics && summary.metrics.length > 0 ? summary.metrics : [summary.metric];
    const metrics = rawMetrics.filter(Boolean);
    const label =
      metrics.length > 0
        ? metrics
            .map((metric) => alertSummaryMetricName(metric, t))
            .join(lang === 'zh' ? '，' : ', ')
        : t('admin_nodes_alerts_label');
    return (
      <Tooltip content={label} className="ml-auto inline-flex shrink-0">
        <Link
          to={nodeAlertEventsPath(node.id)}
          className="ui-focus-ring inline-flex size-7 items-center justify-center rounded-md border border-(--theme-border-danger-muted) bg-(--theme-bg-danger-subtle) text-(--theme-fg-danger) transition-colors hover:bg-(--theme-bg-danger-muted) dark:border-(--theme-border-danger-muted) dark:bg-(--theme-bg-danger-soft)"
          aria-label={label}
        >
          <CircleAlert size={15} aria-hidden="true" />
        </Link>
      </Tooltip>
    );
  };

  return (
    <table className="w-full text-sm text-left bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
      <thead className="bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-default) text-xs font-semibold whitespace-nowrap border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
        <tr>
          <th className="px-3 py-2.5 w-10"></th>
          <th className="px-3 py-2.5 min-w-72">{t('admin_nodes_column_node')}</th>
          <th className="px-3 py-2.5 w-32">{t('admin_nodes_column_group')}</th>
          <th className="px-3 py-2.5 w-20">{t('admin_nodes_column_guest_visible')}</th>
          <th className="px-3 py-2.5 w-14">{t('admin_nodes_column_secret')}</th>
          <th className="px-3 py-2.5 w-56">{t('admin_nodes_column_tags')}</th>
          <th className="px-3 py-2.5 w-36">{t('admin_nodes_column_deploy')}</th>
          <th className="px-3 py-2.5 w-44">{t('admin_nodes_column_version')}</th>
          <th className="px-3 py-2.5 text-right w-16"></th>
        </tr>
      </thead>
      <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
        {nodes.map((node) => (
          <tr
            key={node.id}
            className={`transition-colors duration-150 group ${
              dragOverId === node.id
                ? 'bg-(--theme-bg-interactive-muted)! dark:bg-(--theme-bg-interactive-hover)!'
                : 'hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)'
            } ${draggingId === node.id ? 'opacity-70 bg-(--theme-bg-interactive-muted)! dark:bg-(--theme-bg-interactive-soft)!' : ''}`}
            onDragOver={onDragOver(node.id)}
            onDrop={onDrop(node.id)}
          >
            <td className="px-3 py-2">
              <button
                type="button"
                className="ui-focus-ring p-1.5 text-(--theme-fg-muted) dark:text-(--theme-fg-muted) hover:text-(--theme-fg-interactive) dark:hover:text-(--theme-fg-interactive-hover) hover:bg-(--theme-surface-control-hover) dark:hover:bg-(--theme-canvas-muted) rounded transition-colors cursor-grab active:cursor-grabbing"
                draggable
                onDragStart={onDragStart(node.id)}
                onDragEnd={onDragEnd}
                aria-label={t('admin_nodes_drag_reorder')}
              >
                <GripVertical size={18} />
              </button>
            </td>
            <td className="px-3 py-2">
              <div className="flex min-w-0 items-center gap-3">
                <div className="min-w-0 flex-1">
                  {editingId === node.id ? (
                    <Input
                      autoFocus
                      enterKeyHint="done"
                      value={draftName}
                      onChange={(event) => setDraftName(event.target.value)}
                      onBlur={() => commitName(node)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' && !event.nativeEvent.isComposing) {
                          event.preventDefault();
                          event.currentTarget.blur();
                        }
                        if (event.key === 'Escape') {
                          setDraftName(node.name);
                          setEditingId(null);
                        }
                      }}
                    />
                  ) : (
                    <button
                      type="button"
                      className="ui-focus-ring block max-w-full truncate rounded-sm font-semibold text-(--theme-fg-default) transition-colors hover:text-(--theme-bg-accent-emphasis) hover:underline dark:text-(--theme-fg-default)"
                      onClick={() => startEditName(node)}
                    >
                      {node.name}
                    </button>
                  )}
                  <div className="mt-1 flex min-w-0 items-center gap-2 text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-muted)">
                    <span className="shrink-0 font-mono">
                      {node.ip || t('admin_nodes_unconfigured')}
                    </span>
                    <span
                      className="min-w-0 max-w-80 truncate font-mono"
                      title={node.hostname || t('admin_nodes_hostname_unknown')}
                    >
                      {node.hostname || t('admin_nodes_hostname_unknown')}
                    </span>
                  </div>
                </div>
                {renderAlertSummary(node)}
              </div>
            </td>
            <td className="px-3 py-2 text-(--theme-fg-muted) dark:text-(--theme-fg-muted) w-32">
              <div className="flex max-w-40 flex-wrap gap-1">
                {(node.groupNames.length > 0 ? node.groupNames : [t('admin_nodes_ungrouped')]).map(
                  (name) => (
                    <span
                      key={`${node.id}-${name}`}
                      className="inline-flex max-w-full px-1.5 py-0.5 text-xs font-medium rounded-md border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-muted) dark:bg-(--theme-canvas-muted) text-(--theme-fg-muted) dark:text-(--theme-fg-muted)"
                      title={name}
                    >
                      <span className="truncate">{name}</span>
                    </span>
                  ),
                )}
              </div>
            </td>
            <td className="px-3 py-2 text-xs">
              <IOSSwitch
                size="sm"
                checked={node.guestVisible}
                disabled={savingGuestVisibleNodeIds.has(node.id)}
                onChange={() => onToggleGuestVisible(node)}
              />
            </td>
            <td className="px-3 py-2 text-xs">
              <div className="flex items-center">
                <button
                  type="button"
                  className="ui-focus-ring p-1.5 text-(--theme-fg-muted) dark:text-(--theme-fg-muted) hover:text-(--theme-fg-interactive) dark:hover:text-(--theme-fg-interactive-hover) hover:bg-(--theme-surface-control-hover) dark:hover:bg-(--theme-canvas-muted) rounded transition-colors"
                  onClick={() => onCopySecret(node.secret)}
                  title={t('admin_nodes_copy_secret')}
                  aria-label={t('admin_nodes_copy_secret')}
                >
                  <Copy size={18} />
                </button>
              </div>
            </td>
            <td className="px-3 py-2">
              <div className="flex max-w-72 flex-wrap gap-1">
                {node.tags.length === 0 ? (
                  <span className="text-xs text-(--theme-fg-subtle) dark:text-(--theme-fg-subtle)">
                    {t('admin_nodes_tags_none')}
                  </span>
                ) : (
                  node.tags.map((tag) => (
                    <span
                      key={`${node.id}-${tag}`}
                      className="text-xs max-w-32 truncate px-1.5 py-0.5 bg-(--theme-bg-muted) dark:bg-(--theme-canvas-muted) text-(--theme-fg-muted) dark:text-(--theme-fg-muted) rounded-md border border-(--theme-border-subtle) dark:border-(--theme-border-default)"
                      title={tag}
                    >
                      {tag}
                    </span>
                  ))
                )}
              </div>
            </td>
            <td className="px-3 py-2 text-xs w-36">
              <div className="flex gap-1.5 items-center">
                <Button
                  variant="plain"
                  className={deployButtonClass}
                  title="Linux"
                  size="none"
                  onClick={() => onDeployCopy('linux', node.secret)}
                >
                  <PlatformLogo platform="linux" size={14} />
                  <span className="sr-only">Linux</span>
                </Button>
                <Button
                  variant="plain"
                  className={deployButtonClass}
                  title="Windows"
                  size="none"
                  onClick={() => onDeployCopy('windows', node.secret)}
                >
                  <PlatformLogo platform="windows" size={14} />
                  <span className="sr-only">Windows</span>
                </Button>
                <Button
                  variant="plain"
                  className={deployButtonClass}
                  title="macOS"
                  size="none"
                  onClick={() => onDeployCopy('macos', node.secret)}
                >
                  <PlatformLogo platform="macos" size={16} />
                  <span className="sr-only">macOS</span>
                </Button>
              </div>
            </td>
            <td className="px-3 py-2 text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-muted) w-44">
              {node.version.is_outdated ? (
                <span className="text-(--theme-fg-danger-muted) font-medium">
                  {t('admin_nodes_version_outdated')}
                </span>
              ) : updatableNodeIds.has(node.id) ? (
                <div className="inline-flex items-center gap-1.5">
                  <span className="font-mono text-xs">
                    {node.version.version || t('common_unknown')}
                  </span>
                  <button
                    type="button"
                    className="inline-flex size-5 shrink-0 items-center justify-center rounded-md border border-(--theme-border-warning-muted) bg-(--theme-bg-warning-muted) p-1 text-(--theme-fg-warning-strong) disabled:cursor-not-allowed disabled:opacity-50 dark:border-(--theme-border-warning-soft) dark:bg-(--theme-bg-warning-soft) dark:text-(--theme-fg-warning-strong)"
                    onClick={() => onRequestUpgrade(node)}
                    disabled={upgradingNodeIds.has(node.id)}
                    title={t('admin_nodes_version_update_target', {
                      target: bundledNodeVersion,
                    })}
                    aria-label={t('admin_nodes_version_update_target', {
                      target: bundledNodeVersion,
                    })}
                  >
                    <ArrowUpFromLine size={10} aria-hidden="true" />
                  </button>
                </div>
              ) : manualUpdateNodeIds.has(node.id) ? (
                <div className="inline-flex items-center gap-1.5">
                  <span className="font-mono text-xs">
                    {node.version.version || t('common_unknown')}
                  </span>
                  <Tooltip
                    content={t('admin_nodes_auto_update_requires_manual')}
                    className="inline-flex cursor-not-allowed"
                  >
                    <button
                      type="button"
                      className="inline-flex size-5 shrink-0 cursor-not-allowed items-center justify-center rounded-md border border-(--theme-border-warning-muted) bg-(--theme-bg-warning-muted) p-1 text-(--theme-fg-warning-strong) opacity-50 dark:border-(--theme-border-warning-soft) dark:bg-(--theme-bg-warning-soft) dark:text-(--theme-fg-warning-strong)"
                      disabled
                      title={t('admin_nodes_auto_update_requires_manual')}
                      aria-label={t('admin_nodes_auto_update_requires_manual')}
                    >
                      <ArrowUpFromLine size={10} aria-hidden="true" />
                    </button>
                  </Tooltip>
                </div>
              ) : (
                <span className="font-mono text-xs">
                  {node.version.version || t('common_unknown')}
                </span>
              )}
            </td>
            <td className="px-4 py-2 text-right">
              <div className="flex justify-end gap-1 opacity-100 transition-opacity">
                <button
                  type="button"
                  className="ui-focus-ring p-1.5 text-(--theme-fg-action-muted) hover:text-(--theme-fg-interactive) dark:hover:text-(--theme-fg-interactive-hover) hover:bg-(--theme-bg-interactive-hover) dark:hover:bg-(--theme-bg-interactive-hover) rounded transition-colors"
                  onClick={() => onOpenSettings(node)}
                  aria-label={t('common_edit')}
                >
                  <Settings size={18} />
                </button>
                <button
                  type="button"
                  className="ui-focus-ring p-1.5 text-(--theme-fg-danger-muted) hover:text-(--theme-fg-danger) dark:text-(--theme-fg-danger) dark:hover:text-(--theme-fg-danger-soft) hover:bg-(--theme-bg-danger-muted) dark:hover:bg-(--theme-bg-danger-subtle) rounded transition-colors"
                  onClick={() => onDelete(node)}
                  aria-label={t('common_delete')}
                >
                  <Trash2 size={18} />
                </button>
              </div>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
};

export default NodeTable;
