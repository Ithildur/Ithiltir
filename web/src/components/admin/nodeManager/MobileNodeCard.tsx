import React from 'react';
import ArrowUpFromLine from 'lucide-react/dist/esm/icons/arrow-up-from-line';
import Copy from 'lucide-react/dist/esm/icons/copy';
import Globe from 'lucide-react/dist/esm/icons/globe';
import Settings from 'lucide-react/dist/esm/icons/settings';
import { Link } from 'react-router';
import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import IOSSwitch from '@components/ui/IOSSwitch';
import { Tooltip } from '@components/ui/Tooltip';
import { PlatformLogo } from '@components/system/SystemLogo';
import type { NodeDeployPlatform } from '@app-types/api';
import type { AlertEventSummary, NodeRow } from '@app-types/admin';
import { useI18n } from '@i18n';
import { alertSummaryMetricName } from '@components/admin/alertManager/alertLabels';
import { nodeAlertEventsPath } from './nodeManagerModel';

interface Props {
  node: NodeRow;
  alertSummary: AlertEventSummary | null;
  alertSummaryLoaded: boolean;
  bundledNodeVersion: string;
  canRequestUpgrade: boolean;
  needsManualUpdate: boolean;
  savingGuestVisible: boolean;
  upgrading: boolean;
  onOpenSettings: (node: NodeRow) => void;
  onToggleGuestVisible: (node: NodeRow) => void;
  onCopySecret: (secret: string) => void;
  onDeployCopy: (platform: NodeDeployPlatform, secret: string) => void;
  onRequestUpgrade: (node: NodeRow) => void;
}

const deployButtonClass =
  'bg-(--theme-bg-muted) text-(--theme-fg-muted) dark:bg-(--theme-canvas-subtle) dark:text-(--theme-fg-muted) p-2 text-xs rounded-md border border-(--theme-border-subtle) dark:border-(--theme-border-default) hover:border-(--theme-border-interactive-hover) hover:bg-(--theme-bg-interactive-muted) dark:hover:bg-(--theme-bg-interactive-soft) active:scale-95 transition-transform shadow-sm';

const MobileNodeCard: React.FC<Props> = ({
  node,
  alertSummary,
  alertSummaryLoaded,
  bundledNodeVersion,
  canRequestUpgrade,
  needsManualUpdate,
  savingGuestVisible,
  upgrading,
  onOpenSettings,
  onToggleGuestVisible,
  onCopySecret,
  onDeployCopy,
  onRequestUpgrade,
}) => {
  const { t } = useI18n();
  const hostname = node.hostname || t('admin_nodes_hostname_unknown');
  const version = node.version.version || t('common_unknown');

  return (
    <Card className="p-4 space-y-3 transition-all motion-reduce:transition-none">
      <div className="flex justify-between items-start">
        <div className="flex items-center gap-3">
          <div>
            <div className="font-bold text-(--theme-fg-default) dark:text-(--theme-fg-strong) flex items-center gap-2">
              <button
                type="button"
                className="text-left hover:text-(--theme-fg-interactive-hover)"
                onClick={() => onOpenSettings(node)}
                aria-label={t('common_edit')}
              >
                {node.name}
              </button>
            </div>
            <div className="text-xs text-(--theme-fg-muted) font-mono mt-0.5">
              {node.ip || t('admin_nodes_unconfigured')}
            </div>
          </div>
        </div>
        <button
          className="text-(--theme-fg-subtle) hover:text-(--theme-fg-interactive-hover) p-1"
          type="button"
          onClick={() => onOpenSettings(node)}
          aria-label={t('settings')}
        >
          <Settings size={18} />
        </button>
      </div>

      <div className="space-y-2 text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) border-t border-(--theme-border-muted) dark:border-(--theme-border-default) pt-3">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <div className="flex items-center gap-1.5">
            <Globe size={12} className="text-(--theme-fg-interactive)" />
            {t('admin_nodes_id', { id: node.id })}
          </div>
          <div className="flex min-w-0 items-center gap-1.5">
            <span className="shrink-0 uppercase text-(--theme-fg-subtle)">
              {t('admin_nodes_group_label')}
            </span>
            <span
              className="min-w-0 truncate px-1.5 py-0.5 bg-(--theme-surface-control-hover) dark:bg-(--theme-canvas-subtle) rounded border border-(--theme-border-subtle) dark:border-(--theme-border-default) text-(--theme-fg-default) dark:text-(--theme-fg-control-hover)"
              title={(node.groupNames.length > 0
                ? node.groupNames
                : [t('admin_nodes_ungrouped')]
              ).join(', ')}
            >
              {(node.groupNames.length > 0 ? node.groupNames : [t('admin_nodes_ungrouped')]).join(
                ', ',
              )}
            </span>
          </div>
        </div>

        <div className="grid gap-2">
          <div className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2">
            <span className="uppercase text-(--theme-fg-subtle)">
              {t('admin_nodes_guest_label')}
            </span>
            <IOSSwitch
              checked={node.guestVisible}
              disabled={savingGuestVisible}
              ariaLabel={t('admin_nodes_guest_visibility_toggle', { name: node.name })}
              onChange={() => onToggleGuestVisible(node)}
            />
          </div>
          <div className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2 text-(--theme-fg-default) dark:text-(--theme-fg-default)">
            <span className="uppercase text-(--theme-fg-subtle)">
              {t('admin_nodes_column_hostname')}
            </span>
            <span className="min-w-0 truncate font-mono" title={hostname}>
              {hostname}
            </span>
          </div>
          <div className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2">
            <span className="uppercase text-(--theme-fg-subtle)">
              {t('admin_nodes_alerts_label')}
            </span>
            {alertSummary ? (
              <Link
                to={nodeAlertEventsPath(node.id)}
                className="inline-flex min-w-0 items-center gap-1.5 rounded-md border border-(--theme-border-danger-muted) bg-(--theme-bg-danger-subtle) px-2 py-1 text-xs font-semibold text-(--theme-fg-danger)"
              >
                <span className="shrink-0">
                  {t('admin_nodes_alerts_open_count', {
                    count: String(alertSummary.open_count),
                  })}
                </span>
                <span className="truncate font-normal">
                  {alertSummaryMetricName(alertSummary.metric, t)}
                </span>
              </Link>
            ) : (
              <span className="text-(--theme-fg-muted)">
                {alertSummaryLoaded ? t('admin_nodes_alerts_normal') : t('common_unknown')}
              </span>
            )}
          </div>
          <div className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2">
            <span className="uppercase text-(--theme-fg-subtle)">
              {t('admin_nodes_column_secret')}
            </span>
            <button
              type="button"
              className="inline-flex size-7 shrink-0 items-center justify-center rounded-md text-(--theme-fg-subtle) transition-colors hover:bg-(--theme-bg-interactive-soft) hover:text-(--theme-fg-interactive-hover)"
              onClick={() => onCopySecret(node.secret)}
              title={t('admin_nodes_copy_secret')}
            >
              <Copy size={18} />
            </button>
          </div>
          <div className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2">
            <span className="uppercase text-(--theme-fg-subtle)">
              {t('admin_nodes_column_version')}
            </span>
            <div className="flex min-w-0 items-center gap-1.5">
              {node.version.is_outdated ? (
                <span
                  className="min-w-0 truncate text-xs font-medium text-(--theme-fg-danger-muted)"
                  title={t('admin_nodes_version_outdated')}
                >
                  {t('admin_nodes_version_outdated')}
                </span>
              ) : (
                <>
                  <span className="min-w-0 truncate font-mono text-xs" title={version}>
                    {version}
                  </span>
                  {canRequestUpgrade && bundledNodeVersion ? (
                    <button
                      type="button"
                      className="inline-flex size-6 shrink-0 items-center justify-center rounded-md text-(--theme-fg-warning-strong) transition-colors hover:bg-(--theme-bg-warning-muted) disabled:cursor-not-allowed disabled:opacity-50 dark:text-(--theme-fg-warning-strong) dark:hover:bg-(--theme-bg-warning-soft)"
                      onClick={() => onRequestUpgrade(node)}
                      disabled={upgrading}
                      title={t('admin_nodes_version_update_target', {
                        target: bundledNodeVersion,
                      })}
                      aria-label={t('admin_nodes_version_update_target', {
                        target: bundledNodeVersion,
                      })}
                    >
                      <ArrowUpFromLine size={12} aria-hidden="true" />
                    </button>
                  ) : needsManualUpdate ? (
                    <Tooltip
                      content={t('admin_nodes_auto_update_requires_manual')}
                      className="inline-flex cursor-not-allowed"
                    >
                      <button
                        type="button"
                        className="inline-flex size-6 shrink-0 cursor-not-allowed items-center justify-center rounded-md text-(--theme-fg-warning-strong) opacity-50 transition-colors dark:text-(--theme-fg-warning-strong)"
                        disabled
                        title={t('admin_nodes_auto_update_requires_manual')}
                        aria-label={t('admin_nodes_auto_update_requires_manual')}
                      >
                        <ArrowUpFromLine size={12} aria-hidden="true" />
                      </button>
                    </Tooltip>
                  ) : null}
                </>
              )}
            </div>
          </div>
        </div>

        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          {node.tags.length === 0 ? (
            <span className="text-xs text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted)">
              {t('admin_nodes_tags_none')}
            </span>
          ) : (
            node.tags.map((tag) => (
              <span
                key={tag}
                className="max-w-28 truncate px-1 py-0.5 text-[10px] bg-(--theme-surface-control-hover) dark:bg-(--theme-canvas-subtle) rounded border border-(--theme-border-subtle) dark:border-(--theme-border-default) text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)"
                title={tag}
              >
                #{tag}
              </span>
            ))
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="plain"
            className={deployButtonClass}
            title="Linux"
            size="none"
            onClick={() => onDeployCopy('linux', node.secret)}
          >
            <PlatformLogo platform="linux" size={16} />
            <span className="sr-only">Linux</span>
          </Button>
          <Button
            variant="plain"
            className={deployButtonClass}
            title="Windows"
            size="none"
            onClick={() => onDeployCopy('windows', node.secret)}
          >
            <PlatformLogo platform="windows" size={16} />
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
      </div>
    </Card>
  );
};

export default MobileNodeCard;
