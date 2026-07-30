import React from 'react';
import LoaderCircle from 'lucide-react/dist/esm/icons/loader-circle';
import Wrench from 'lucide-react/dist/esm/icons/wrench';
import IOSSwitch from '@components/ui/IOSSwitch';
import type { NodeRow } from '@app-types/admin';
import { useI18n } from '@i18n';
import { nodeTrafficSettingsLabel } from './nodeManagerModel';

interface NodeActionProps {
  node: NodeRow;
}

interface NodeP95SwitchProps extends NodeActionProps {
  disabled: boolean;
  onToggle: (node: NodeRow) => void;
}

interface TrafficSettingsButtonProps extends NodeActionProps {
  disabled: boolean;
  className?: string;
  onOpen: (node: NodeRow) => void;
}

interface TrafficRebuildButtonProps extends NodeActionProps {
  billingEnabled: boolean;
  disabled: boolean;
  rebuilding: boolean;
  onRebuild: (node: NodeRow) => void;
}

const trafficSettingsButtonClass =
  'inline-flex items-center rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-muted) px-2 py-1 text-xs font-semibold text-(--theme-fg-muted) hover:text-(--theme-fg-default) disabled:cursor-not-allowed disabled:opacity-60 dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle)';

export const NodeP95Switch: React.FC<NodeP95SwitchProps> = ({ node, disabled, onToggle }) => {
  const { t } = useI18n();

  return (
    <IOSSwitch
      size="sm"
      checked={node.trafficP95Enabled}
      disabled={disabled}
      ariaLabel={t('admin_node_traffic_p95_toggle', { name: node.name })}
      onChange={() => onToggle(node)}
    />
  );
};

export const TrafficSettingsButton: React.FC<TrafficSettingsButtonProps> = ({
  node,
  disabled,
  className = 'max-w-full',
  onOpen,
}) => {
  const { t } = useI18n();

  return (
    <button
      type="button"
      disabled={disabled}
      onClick={() => onOpen(node)}
      className={`${trafficSettingsButtonClass} ${className}`}
      aria-label={t('admin_node_traffic_settings_button', { name: node.name })}
    >
      <span className="truncate">{nodeTrafficSettingsLabel(node, t)}</span>
    </button>
  );
};

export const TrafficRebuildButton: React.FC<TrafficRebuildButtonProps> = ({
  node,
  billingEnabled,
  disabled,
  rebuilding,
  onRebuild,
}) => {
  const { t } = useI18n();

  return (
    <button
      type="button"
      disabled={disabled || !billingEnabled}
      onClick={() => onRebuild(node)}
      className="ui-focus-ring inline-flex size-8 items-center justify-center rounded-md text-(--theme-fg-action-muted) transition-[background-color,border-color,color,transform] duration-150 hover:bg-(--theme-bg-interactive-hover) hover:text-(--theme-fg-interactive) active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-(--theme-bg-interactive-hover) dark:hover:text-(--theme-fg-interactive-hover)"
      aria-label={t('admin_node_traffic_rebuild_button', { name: node.name })}
      title={t(
        billingEnabled
          ? 'admin_node_traffic_rebuild'
          : 'admin_node_traffic_rebuild_requires_billing',
      )}
    >
      {rebuilding ? (
        <LoaderCircle size={18} className="animate-spin" aria-hidden="true" />
      ) : (
        <Wrench size={18} strokeWidth={2.25} aria-hidden="true" />
      )}
    </button>
  );
};
