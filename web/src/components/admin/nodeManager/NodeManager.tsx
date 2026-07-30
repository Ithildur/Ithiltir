import React from 'react';
import Plus from 'lucide-react/dist/esm/icons/plus';
import Search from 'lucide-react/dist/esm/icons/search';
import Settings2 from 'lucide-react/dist/esm/icons/settings-2';
import SlidersHorizontal from 'lucide-react/dist/esm/icons/sliders-horizontal';
import { AdminSectionTabs } from '@components/admin/AdminSectionTabs';
import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import SearchInput from '@components/ui/SearchInput';
import NodeTrafficSettingsModal from './NodeTrafficSettingsModal';
import NodeSettingsModal from './NodeSettingsModal';
import { useAuthStore } from '@stores/authStore';
import { useAdminNodesStore } from '@stores/adminNodesStore';
import { useI18n } from '@i18n';
import { useConfirmDialog } from '@hooks/useConfirmDialog';
import MobileAdvancedNodeCard from '@components/admin/nodeManager/MobileAdvancedNodeCard';
import MobileNodeCard from '@components/admin/nodeManager/MobileNodeCard';
import NodeAdvancedTable from '@components/admin/nodeManager/NodeAdvancedTable';
import NodeFilterMenu from '@components/admin/nodeManager/NodeFilterMenu';
import NodeTable from '@components/admin/nodeManager/NodeTable';
import {
  filterNodeManagerNodes,
  nodeCanRequestUpgrade,
  nodeNeedsManualUpdate,
} from '@components/admin/nodeManager/nodeManagerModel';
import { useNodeManagerData } from '@components/admin/nodeManager/useNodeManagerData';
import { useNodeManagerActions } from '@components/admin/nodeManager/useNodeManagerActions';
import { useNodeP95Selection } from '@components/admin/nodeManager/useNodeP95Selection';
import { useNodeOrder } from '@components/admin/nodeManager/useNodeOrder';
import { useTrafficRebuildBanner } from '@hooks/useTrafficRebuildBanner';
import { useTrafficRebuild } from '@hooks/useTrafficRebuild';
import { useIdSelection } from '@hooks/useIdSelection';
import { useOpenAlertSummary } from '@hooks/useOpenAlertSummary';

const tabs = [
  { key: 'basic', labelKey: 'admin_nodes_tab_basic', icon: Settings2 },
  { key: 'advanced', labelKey: 'admin_nodes_tab_advanced', icon: SlidersHorizontal },
] as const;

type NodeManagerTab = (typeof tabs)[number]['key'];

const NodeManager: React.FC = () => {
  const token = useAuthStore((state) => state.accessToken);
  const { t } = useI18n();
  const { dialogProps: confirmDialogProps, request: requestConfirm } = useConfirmDialog();

  const {
    nodes,
    groups,
    deploy,
    trafficSettings,
    trafficSettingsLoaded,
    bundledNodeVersion,
    isLoading,
    refreshNodes,
  } = useNodeManagerData(token);

  const [activeTab, setActiveTab] = React.useState<NodeManagerTab>('basic');
  const [search, setSearch] = React.useState('');
  const {
    selectedIds: selectedGroupIds,
    setSelectedIds: setSelectedGroupIds,
    prune: pruneSelectedGroupIds,
  } = useIdSelection();
  const [updatableOnly, setUpdatableOnly] = React.useState(false);
  const isCreating = useAdminNodesStore((state) => state.creating);
  const savingGuestVisibleNodeIds = useAdminNodesStore((state) => state.savingGuestVisibleNodeIds);
  const savingP95NodeIds = useAdminNodesStore((state) => state.savingP95NodeIds);
  const savingTrafficSettingsNodeIds = useAdminNodesStore(
    (state) => state.savingTrafficSettingsNodeIds,
  );
  const savingSettingsNodeIds = useAdminNodesStore((state) => state.savingSettingsNodeIds);
  const upgradingNodeIds = useAdminNodesStore((state) => state.upgradingNodeIds);
  const [basicSettingsNodeId, setBasicSettingsNodeId] = React.useState<number | null>(null);
  const [trafficSettingsNodeId, setTrafficSettingsNodeId] = React.useState<number | null>(null);

  const {
    rebuildingNodeId: rebuildingTrafficNodeId,
    busy: trafficRebuildBusy,
    start: startTrafficRebuild,
  } = useTrafficRebuild();
  const { summaryByServer: alertSummaryByServer, loaded: alertSummaryLoaded } = useOpenAlertSummary(
    activeTab === 'basic',
  );
  const showTrafficRebuildOutcome = useTrafficRebuildBanner();

  const savingGuestVisibleNodeIdSet = React.useMemo(
    () => new Set(savingGuestVisibleNodeIds),
    [savingGuestVisibleNodeIds],
  );
  const savingTrafficSettingsNodeIdSet = React.useMemo(
    () => new Set(savingTrafficSettingsNodeIds),
    [savingTrafficSettingsNodeIds],
  );
  const savingSettingsNodeIdSet = React.useMemo(
    () => new Set(savingSettingsNodeIds),
    [savingSettingsNodeIds],
  );
  const basicSettingsNode = React.useMemo(
    () => nodes.find((node) => node.id === basicSettingsNodeId) ?? null,
    [basicSettingsNodeId, nodes],
  );
  const trafficSettingsNode = React.useMemo(
    () => nodes.find((node) => node.id === trafficSettingsNodeId) ?? null,
    [nodes, trafficSettingsNodeId],
  );
  const billingEnabled = trafficSettingsLoaded && trafficSettings.usage_mode === 'billing';

  React.useEffect(() => {
    if (basicSettingsNodeId !== null && !basicSettingsNode && !isLoading) {
      setBasicSettingsNodeId(null);
    }
  }, [basicSettingsNode, basicSettingsNodeId, isLoading, setBasicSettingsNodeId]);

  React.useEffect(() => {
    if (trafficSettingsNodeId !== null && !trafficSettingsNode && !isLoading) {
      setTrafficSettingsNodeId(null);
    }
  }, [isLoading, setTrafficSettingsNodeId, trafficSettingsNode, trafficSettingsNodeId]);

  React.useEffect(() => {
    if (selectedGroupIds.length === 0) return;
    pruneSelectedGroupIds(groups.map((group) => group.id));
  }, [groups, pruneSelectedGroupIds, selectedGroupIds.length]);

  const filteredNodes = React.useMemo(
    () =>
      filterNodeManagerNodes({
        nodes,
        search,
        selectedGroupIds,
        updatableOnly: activeTab === 'basic' && updatableOnly,
        bundledNodeVersion,
      }),
    [activeTab, bundledNodeVersion, nodes, search, selectedGroupIds, updatableOnly],
  );

  const filteredNodeIds = React.useMemo(
    () => filteredNodes.map((node) => node.id),
    [filteredNodes],
  );

  const {
    selectedP95Ids,
    selectedP95NodeIdSet,
    savingP95NodeIdSet,
    allVisibleP95Selected,
    someVisibleP95Selected,
    canBatchP95,
    toggleP95NodeSelection,
    toggleVisibleP95Nodes,
    clearP95Selection,
  } = useNodeP95Selection({
    nodes,
    filteredNodeIds,
    savingP95NodeIds,
    isLoading,
  });

  const updatableNodeIds = React.useMemo(() => {
    return new Set(
      nodes
        .filter((node) => nodeCanRequestUpgrade(node, bundledNodeVersion))
        .map((node) => node.id),
    );
  }, [bundledNodeVersion, nodes]);
  const manualUpdateNodeIds = React.useMemo(() => {
    return new Set(
      nodes
        .filter((node) => nodeNeedsManualUpdate(node, bundledNodeVersion))
        .map((node) => node.id),
    );
  }, [bundledNodeVersion, nodes]);
  const upgradingNodeIdSet = React.useMemo(() => new Set(upgradingNodeIds), [upgradingNodeIds]);

  const { draggingId, dragOverId, dragStart, dragOver, drop, dragEnd, move } = useNodeOrder({
    token,
    nodes,
    filteredNodeIds,
    refreshNodes,
  });

  const {
    addNode,
    confirmAndDelete,
    confirmUpgrade,
    copyDeploy,
    copyToClipboard,
    rebuildNodeTraffic,
    rename,
    saveNodeTrafficSettings,
    saveSettings,
    setTrafficP95ForSelected,
    toggleGuestVisible,
    toggleTrafficP95,
  } = useNodeManagerActions({
    token,
    deploy,
    nodes,
    selectedP95NodeIdSet,
    bundledNodeVersion,
    billingEnabled,
    trafficRebuildBusy,
    requestConfirm,
    startTrafficRebuild,
    showTrafficRebuildOutcome,
  });

  return (
    <div className="space-y-4 md:space-y-6">
      <ConfirmDialog {...confirmDialogProps} />

      <div className="flex w-full md:w-auto md:flex-1">
        <AdminSectionTabs tabs={tabs} activeKey={activeTab} onChange={setActiveTab} />
      </div>

      <div className="flex flex-col md:flex-row justify-between gap-3 md:gap-4">
        <div className="flex w-full md:w-auto md:flex-1 gap-2">
          <SearchInput
            icon={Search}
            placeholder={t('admin_nodes_search_placeholder')}
            aria-label={t('admin_nodes_search_placeholder')}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            wrapperClassName="flex-1 max-w-md"
          />
          <NodeFilterMenu
            groups={groups}
            selectedGroupIds={selectedGroupIds}
            updatableOnly={updatableOnly}
            showVersionFilter={activeTab === 'basic'}
            onGroupChange={setSelectedGroupIds}
            onUpdatableOnlyChange={setUpdatableOnly}
          />
        </div>

        {activeTab === 'basic' && (
          <div className="flex w-full md:w-auto gap-2">
            <Button
              icon={Plus}
              className="w-full md:w-auto shadow-(color:--theme-shadow-interactive)"
              onClick={addNode}
              disabled={isCreating}
            >
              {isCreating ? t('admin_nodes_creating') : t('admin_nodes_add')}
            </Button>
          </div>
        )}
      </div>

      {isLoading ? (
        <Card className="hidden md:block p-8 text-center text-sm text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
          {t('admin_nodes_loading')}
        </Card>
      ) : activeTab === 'basic' ? (
        <Card className="hidden md:block overflow-hidden">
          <NodeTable
            nodes={filteredNodes}
            alertSummaryByServer={alertSummaryByServer}
            updatableNodeIds={updatableNodeIds}
            manualUpdateNodeIds={manualUpdateNodeIds}
            savingGuestVisibleNodeIds={savingGuestVisibleNodeIdSet}
            upgradingNodeIds={upgradingNodeIdSet}
            bundledNodeVersion={bundledNodeVersion}
            draggingId={draggingId}
            dragOverId={dragOverId}
            onRename={(node, nextName) => void rename(node, nextName)}
            onToggleGuestVisible={(node) => void toggleGuestVisible(node)}
            onCopySecret={(secret) => void copyToClipboard(secret)}
            onDeployCopy={copyDeploy}
            onRequestUpgrade={(node) => void confirmUpgrade(node)}
            onOpenSettings={(node) => setBasicSettingsNodeId(node.id)}
            onDelete={(node) => void confirmAndDelete(node)}
            onDragStart={dragStart}
            onDragOver={dragOver}
            onDrop={drop}
            onDragEnd={dragEnd}
            onMove={(id, offset) => void move(id, offset)}
          />
        </Card>
      ) : (
        <Card className="hidden md:block overflow-hidden">
          <NodeAdvancedTable
            nodes={filteredNodes}
            selectedP95NodeIds={selectedP95NodeIdSet}
            allVisibleP95Selected={allVisibleP95Selected}
            someVisibleP95Selected={someVisibleP95Selected}
            savingP95NodeIds={savingP95NodeIdSet}
            savingTrafficSettingsNodeIds={savingTrafficSettingsNodeIdSet}
            billingEnabled={billingEnabled}
            rebuildingTrafficNodeId={rebuildingTrafficNodeId}
            trafficRebuildBusy={trafficRebuildBusy}
            onToggleVisibleNodes={toggleVisibleP95Nodes}
            onToggleP95Node={toggleP95NodeSelection}
            onToggleTrafficP95={(node) => void toggleTrafficP95(node)}
            onOpenTrafficSettings={(node) => setTrafficSettingsNodeId(node.id)}
            onRebuildTraffic={(node) => void rebuildNodeTraffic(node)}
          />
          {selectedP95Ids.length > 0 && (
            <div className="flex flex-col gap-2 border-t border-(--theme-border-subtle) bg-(--theme-bg-muted) px-4 py-3 text-xs text-(--theme-fg-muted) sm:flex-row sm:items-center sm:justify-between dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle) dark:text-(--theme-fg-muted)">
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  variant="secondary"
                  disabled={!canBatchP95}
                  onClick={() => void setTrafficP95ForSelected(true)}
                >
                  {t('admin_nodes_p95_batch_enable')}
                </Button>
                <Button
                  variant="secondary"
                  disabled={!canBatchP95}
                  onClick={() => void setTrafficP95ForSelected(false)}
                >
                  {t('admin_nodes_p95_batch_disable')}
                </Button>
              </div>
              <div className="flex flex-wrap items-center gap-3 sm:justify-end">
                <span>
                  {t('admin_nodes_selected_count', {
                    count: String(selectedP95Ids.length),
                  })}
                </span>
                <Button variant="ghost" onClick={clearP95Selection}>
                  {t('common_clear')}
                </Button>
              </div>
            </div>
          )}
        </Card>
      )}

      {activeTab === 'basic' ? (
        <div className="md:hidden grid grid-cols-1 gap-3">
          {filteredNodes.map((node) => (
            <MobileNodeCard
              key={node.id}
              node={node}
              alertSummary={alertSummaryByServer.get(node.id) ?? null}
              alertSummaryLoaded={alertSummaryLoaded}
              bundledNodeVersion={bundledNodeVersion}
              canRequestUpgrade={updatableNodeIds.has(node.id)}
              needsManualUpdate={manualUpdateNodeIds.has(node.id)}
              savingGuestVisible={savingGuestVisibleNodeIdSet.has(node.id)}
              upgrading={upgradingNodeIdSet.has(node.id)}
              onOpenSettings={(node) => setBasicSettingsNodeId(node.id)}
              onToggleGuestVisible={(target) => void toggleGuestVisible(target)}
              onCopySecret={(secret) => void copyToClipboard(secret)}
              onDeployCopy={copyDeploy}
              onRequestUpgrade={(target) => void confirmUpgrade(target)}
            />
          ))}
        </div>
      ) : (
        <div className="md:hidden grid grid-cols-1 gap-3">
          {filteredNodes.map((node) => (
            <MobileAdvancedNodeCard
              key={node.id}
              node={node}
              p95Selected={selectedP95NodeIdSet.has(node.id)}
              savingP95={savingP95NodeIdSet.has(node.id)}
              savingTrafficSettings={savingTrafficSettingsNodeIdSet.has(node.id)}
              billingEnabled={billingEnabled}
              rebuilding={rebuildingTrafficNodeId === node.id}
              trafficRebuildBusy={trafficRebuildBusy}
              onToggleP95Node={toggleP95NodeSelection}
              onToggleTrafficP95={(target) => void toggleTrafficP95(target)}
              onOpenTrafficSettings={(target) => setTrafficSettingsNodeId(target.id)}
              onRebuildTraffic={(target) => void rebuildNodeTraffic(target)}
            />
          ))}
          {selectedP95Ids.length > 0 && (
            <Card className="p-3">
              <div className="flex flex-col gap-2 text-xs text-(--theme-fg-muted)">
                <div className="flex flex-wrap gap-2">
                  <Button
                    variant="secondary"
                    disabled={!canBatchP95}
                    onClick={() => void setTrafficP95ForSelected(true)}
                  >
                    {t('admin_nodes_p95_batch_enable')}
                  </Button>
                  <Button
                    variant="secondary"
                    disabled={!canBatchP95}
                    onClick={() => void setTrafficP95ForSelected(false)}
                  >
                    {t('admin_nodes_p95_batch_disable')}
                  </Button>
                </div>
                <span>
                  {t('admin_nodes_selected_count', {
                    count: String(selectedP95Ids.length),
                  })}
                </span>
                <Button variant="ghost" onClick={clearP95Selection}>
                  {t('common_clear')}
                </Button>
              </div>
            </Card>
          )}
        </div>
      )}

      {!isLoading && filteredNodes.length === 0 && (
        <Card className="p-8 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) border-dashed border-2 border-(--theme-border-subtle) dark:border-(--theme-border-default)">
          {t('admin_nodes_empty')}
        </Card>
      )}

      {basicSettingsNode && (
        <NodeSettingsModal
          isOpen={!!basicSettingsNode}
          node={basicSettingsNode}
          groups={groups}
          deploy={deploy}
          saving={savingSettingsNodeIdSet.has(basicSettingsNode.id)}
          onClose={() => setBasicSettingsNodeId(null)}
          onSave={(input) => saveSettings(basicSettingsNode.id, input)}
        />
      )}

      {trafficSettingsNode && (
        <NodeTrafficSettingsModal
          isOpen={!!trafficSettingsNode}
          node={trafficSettingsNode}
          saving={savingTrafficSettingsNodeIdSet.has(trafficSettingsNode.id)}
          onClose={() => setTrafficSettingsNodeId(null)}
          onSave={(patch) => saveNodeTrafficSettings(trafficSettingsNode.id, patch)}
        />
      )}
    </div>
  );
};

export default NodeManager;
