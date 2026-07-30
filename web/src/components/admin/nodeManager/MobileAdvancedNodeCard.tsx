import React from 'react';
import Card from '@components/ui/Card';
import Checkbox from '@components/ui/Checkbox';
import type { NodeRow } from '@app-types/admin';
import { useI18n } from '@i18n';
import { NodeP95Switch, TrafficRebuildButton, TrafficSettingsButton } from './NodeAdvancedActions';

interface Props {
  node: NodeRow;
  p95Selected: boolean;
  savingP95: boolean;
  savingTrafficSettings: boolean;
  billingEnabled: boolean;
  rebuilding: boolean;
  trafficRebuildBusy: boolean;
  onToggleP95Node: (id: number) => void;
  onToggleTrafficP95: (node: NodeRow) => void;
  onOpenTrafficSettings: (node: NodeRow) => void;
  onRebuildTraffic: (node: NodeRow) => void;
}

const MobileAdvancedNodeCard: React.FC<Props> = ({
  node,
  p95Selected,
  savingP95,
  savingTrafficSettings,
  billingEnabled,
  rebuilding,
  trafficRebuildBusy,
  onToggleP95Node,
  onToggleTrafficP95,
  onOpenTrafficSettings,
  onRebuildTraffic,
}) => {
  const { t } = useI18n();

  return (
    <Card className="p-4">
      <div className="flex items-center justify-between gap-4">
        <div className="flex min-w-0 items-center gap-3">
          <Checkbox
            checked={p95Selected}
            onChange={() => onToggleP95Node(node.id)}
            aria-label={t('admin_nodes_select_node', { name: node.name })}
          />
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold text-(--theme-fg-default)">
              {node.name}
            </div>
            <div className="mt-1 truncate font-mono text-xs text-(--theme-fg-muted)">
              {node.ip || `ID: ${node.id}`}
            </div>
          </div>
        </div>
        <NodeP95Switch node={node} disabled={savingP95} onToggle={onToggleTrafficP95} />
      </div>
      <div className="mt-3 flex items-center justify-between gap-3">
        <span className="text-[11px] font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
          {t('admin_nodes_column_traffic_settings')}
        </span>
        <TrafficSettingsButton
          node={node}
          disabled={savingTrafficSettings}
          className="max-w-48"
          onOpen={onOpenTrafficSettings}
        />
      </div>
      <div className="mt-3 flex justify-end">
        <TrafficRebuildButton
          node={node}
          billingEnabled={billingEnabled}
          disabled={trafficRebuildBusy}
          rebuilding={rebuilding}
          onRebuild={onRebuildTraffic}
        />
      </div>
    </Card>
  );
};

export default MobileAdvancedNodeCard;
