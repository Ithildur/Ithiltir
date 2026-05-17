import React from 'react';
import ArrowUpFromLine from 'lucide-react/dist/esm/icons/arrow-up-from-line';
import Copy from 'lucide-react/dist/esm/icons/copy';
import Globe from 'lucide-react/dist/esm/icons/globe';
import Settings from 'lucide-react/dist/esm/icons/settings';
import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import IOSSwitch from '@components/ui/IOSSwitch';
import { PlatformLogo } from '@components/system/SystemLogo';
import type { NodeDeployPlatform } from '@app-types/api';
import type { NodeRow } from '@app-types/admin';
import { useI18n } from '@i18n';

export interface Props {
  node: NodeRow;
  bundledNodeVersion: string;
  canRequestUpgrade: boolean;
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
  bundledNodeVersion,
  canRequestUpgrade,
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
    <Card className="p-4 space-y-3 transition-all">
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

      <div className="grid grid-cols-2 gap-2 text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) border-t border-(--theme-border-muted) dark:border-(--theme-border-default) pt-2">
        <div className="flex items-center gap-1.5">
          <Globe size={12} className="text-(--theme-fg-interactive)" />
          {t('admin_nodes_id', { id: node.id })}
        </div>
        <div className="flex items-center gap-1.5">
          <span className="uppercase text-(--theme-fg-subtle)">{t('admin_nodes_group_label')}</span>
          <span className="px-1.5 py-0.5 bg-(--theme-surface-control-hover) dark:bg-(--theme-canvas-subtle) rounded border border-(--theme-border-subtle) dark:border-(--theme-border-default) text-(--theme-fg-default) dark:text-(--theme-fg-control-hover)">
            {(node.groupNames.length > 0 ? node.groupNames : [t('admin_nodes_ungrouped')]).join(
              ', ',
            )}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <span className="uppercase text-(--theme-fg-subtle)">{t('admin_nodes_guest_label')}</span>
          <IOSSwitch checked={node.guestVisible} onChange={() => onToggleGuestVisible(node)} />
        </div>
        <div className="flex items-center gap-1.5 text-(--theme-fg-default) dark:text-(--theme-fg-default)">
          <span className="uppercase text-(--theme-fg-subtle)">
            {t('admin_nodes_column_hostname')}
          </span>
          <span className="min-w-0 truncate font-mono">{hostname}</span>
        </div>
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="shrink-0 uppercase text-(--theme-fg-subtle)">
            {t('admin_nodes_column_secret')}
          </span>
          <button
            type="button"
            className="inline-flex size-7 shrink-0 items-center justify-center rounded-md text-(--theme-fg-subtle) transition-colors hover:bg-(--theme-bg-interactive-soft) hover:text-(--theme-fg-interactive-hover)"
            onClick={() => onCopySecret(node.secret)}
            title={t('admin_nodes_copy_secret')}
          >
            <Copy size={14} />
          </button>
        </div>
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="shrink-0 uppercase text-(--theme-fg-subtle)">
            {t('admin_nodes_column_version')}
          </span>
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
                  className="inline-flex size-6 shrink-0 items-center justify-center rounded-md text-(--theme-fg-warning-strong) transition-colors hover:bg-(--theme-bg-warning-muted) dark:text-(--theme-fg-warning-strong) dark:hover:bg-(--theme-bg-warning-soft)"
                  onClick={() => onRequestUpgrade(node)}
                  title={t('admin_nodes_version_update_target', {
                    target: bundledNodeVersion,
                  })}
                  aria-label={t('admin_nodes_version_update_target', {
                    target: bundledNodeVersion,
                  })}
                >
                  <ArrowUpFromLine size={12} aria-hidden="true" />
                </button>
              ) : null}
            </>
          )}
        </div>
        <div className="flex items-center gap-1.5 col-span-2">
          {node.tags.length === 0 ? (
            <span className="text-xs text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted)">
              {t('admin_nodes_tags_none')}
            </span>
          ) : (
            node.tags.map((tag) => (
              <span
                key={tag}
                className="px-1 py-0.5 text-[10px] bg-(--theme-surface-control-hover) dark:bg-(--theme-canvas-subtle) rounded border border-(--theme-border-subtle) dark:border-(--theme-border-default) text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)"
              >
                #{tag}
              </span>
            ))
          )}
        </div>
        <div className="flex items-center gap-2 col-span-2">
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
