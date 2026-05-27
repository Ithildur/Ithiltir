import React from 'react';
import Plus from 'lucide-react/dist/esm/icons/plus';
import Search from 'lucide-react/dist/esm/icons/search';
import Settings2 from 'lucide-react/dist/esm/icons/settings-2';
import SlidersHorizontal from 'lucide-react/dist/esm/icons/sliders-horizontal';
import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import Input from '@components/ui/Input';
import { useTopBanner } from '@components/ui/TopBannerStack';
import NodeTrafficSettingsModal from './NodeTrafficSettingsModal';
import NodeSettingsModal from './NodeSettingsModal';
import { useAuth } from '@context/AuthContext';
import {
  createNode,
  deleteNode,
  requestNodeUpgrade,
  updateNode,
  updateNodesTrafficP95,
} from '@lib/adminApi';
import type { NodeDeployPlatform, NodeTrafficPatch } from '@app-types/api';
import type { NodeRow } from '@app-types/admin';
import { useI18n } from '@i18n';
import { copyTextToClipboardWithFeedback } from '@utils/clipboard';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useConfirmDialog } from '@hooks/useConfirmDialog';
import { isVersionOlder } from '@utils/version';
import MobileAdvancedNodeCard from '@components/admin/nodeManager/MobileAdvancedNodeCard';
import MobileNodeCard from '@components/admin/nodeManager/MobileNodeCard';
import NodeAdvancedTable from '@components/admin/nodeManager/NodeAdvancedTable';
import NodeFilterMenu from '@components/admin/nodeManager/NodeFilterMenu';
import NodeTable from '@components/admin/nodeManager/NodeTable';
import { useNodes } from '@components/admin/nodeManager/useNodes';
import { useReorder } from '@components/admin/nodeManager/useReorder';
import { useTrafficRebuildBanner } from '@hooks/useTrafficRebuildBanner';
import { useTrafficRebuild } from '@hooks/useTrafficRebuild';

type SubTab = 'basic' | 'advanced';

const tabClass = (active: boolean) =>
  `px-1 py-2 text-sm font-semibold border-b-2 -mb-px transition-colors ${
    active
      ? 'border-(--theme-border-underline-nav-active) text-(--theme-fg-default)'
      : 'border-transparent text-(--theme-fg-muted) dark:text-(--theme-fg-muted) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default)'
  }`;

const NodeManager: React.FC = () => {
  const { token } = useAuth();
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const pushBanner = useTopBanner();
  const { dialogProps: confirmDialogProps, request: requestConfirm } = useConfirmDialog();

  const {
    nodes,
    setNodes,
    groups,
    groupLookup,
    deploy,
    trafficSettings,
    bundledNodeVersion,
    isLoading,
    refreshNodes,
  } = useNodes(token);

  const [activeTab, setActiveTab] = React.useState<SubTab>('basic');
  const [search, setSearch] = React.useState('');
  const [selectedGroupIds, setSelectedGroupIds] = React.useState<number[]>([]);
  const [updateableOnly, setUpdateableOnly] = React.useState(false);
  const [basicSettingsNode, setBasicSettingsNode] = React.useState<NodeRow | null>(null);
  const [trafficSettingsNode, setTrafficSettingsNode] = React.useState<NodeRow | null>(null);
  const [isCreating, setIsCreating] = React.useState(false);
  const [savingP95NodeIds, setSavingP95NodeIds] = React.useState<Set<number>>(() => new Set());
  const [savingTrafficSettingsNodeIds, setSavingTrafficSettingsNodeIds] = React.useState<
    Set<number>
  >(() => new Set());
  const [selectedP95NodeIds, setSelectedP95NodeIds] = React.useState<Set<number>>(() => new Set());

  const {
    rebuildingNodeId: rebuildingTrafficNodeId,
    busy: trafficRebuildBusy,
    start: startTrafficRebuild,
  } = useTrafficRebuild();
  const showTrafficRebuildOutcome = useTrafficRebuildBanner();

  const nodeHasNewerBundledVersion = React.useCallback(
    (node: NodeRow): boolean => {
      if (!bundledNodeVersion) return false;
      return isVersionOlder(node.version.version, bundledNodeVersion);
    },
    [bundledNodeVersion],
  );
  const nodeCanRequestUpgrade = React.useCallback(
    (node: NodeRow): boolean =>
      !node.version.is_outdated &&
      node.version.supports_auto_update &&
      nodeHasNewerBundledVersion(node),
    [nodeHasNewerBundledVersion],
  );
  const nodeNeedsManualUpdate = React.useCallback(
    (node: NodeRow): boolean =>
      !node.version.is_outdated &&
      !node.version.supports_auto_update &&
      nodeHasNewerBundledVersion(node),
    [nodeHasNewerBundledVersion],
  );
  const nodeNeedsUpdate = React.useCallback(
    (node: NodeRow): boolean =>
      node.version.is_outdated || nodeCanRequestUpgrade(node) || nodeNeedsManualUpdate(node),
    [nodeCanRequestUpgrade, nodeNeedsManualUpdate],
  );

  const selectedGroupSet = React.useMemo(() => new Set(selectedGroupIds), [selectedGroupIds]);

  const filteredNodes = React.useMemo(() => {
    const keyword = search.trim().toLowerCase();

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

      return activeTab !== 'basic' || !updateableOnly || nodeNeedsUpdate(node);
    });
  }, [activeTab, nodeNeedsUpdate, nodes, search, selectedGroupSet, updateableOnly]);

  const filteredNodeIds = React.useMemo(
    () => filteredNodes.map((node) => node.id),
    [filteredNodes],
  );

  React.useEffect(() => {
    setSelectedP95NodeIds((current) => {
      if (current.size === 0) return current;

      const nodeIds = new Set(nodes.map((node) => node.id));
      let changed = false;
      const next = new Set<number>();
      for (const id of current) {
        if (nodeIds.has(id)) {
          next.add(id);
        } else {
          changed = true;
        }
      }

      return changed ? next : current;
    });
  }, [nodes]);

  const selectedP95Ids = React.useMemo(() => Array.from(selectedP95NodeIds), [selectedP95NodeIds]);
  const allVisibleP95Selected =
    filteredNodeIds.length > 0 && filteredNodeIds.every((id) => selectedP95NodeIds.has(id));
  const someVisibleP95Selected = filteredNodeIds.some((id) => selectedP95NodeIds.has(id));
  const canBatchP95 =
    selectedP95Ids.length > 0 &&
    !isLoading &&
    !selectedP95Ids.some((id) => savingP95NodeIds.has(id));

  const updatableNodeIds = React.useMemo(() => {
    return new Set(nodes.filter((node) => nodeCanRequestUpgrade(node)).map((node) => node.id));
  }, [nodeCanRequestUpgrade, nodes]);
  const manualUpdateNodeIds = React.useMemo(() => {
    return new Set(nodes.filter((node) => nodeNeedsManualUpdate(node)).map((node) => node.id));
  }, [nodeNeedsManualUpdate, nodes]);

  const copyToClipboard = React.useCallback(
    async (text: string, successMessage = t('admin_secret_copied')) => {
      await copyTextToClipboardWithFeedback(text, {
        pushBanner,
        successMessage,
        httpsRequiredMessage: t('admin_clipboard_https_required'),
        failureMessage: t('admin_copy_failed_manual'),
      });
    },
    [pushBanner, t],
  );

  const copyDeploy = React.useCallback(
    (platform: NodeDeployPlatform, secret: string) => {
      const prefix = deploy?.scripts?.[platform]?.command_prefix;
      if (!prefix) {
        pushBanner(t('admin_deploy_command_unavailable'), { tone: 'error' });
        return;
      }
      void copyToClipboard(`${prefix}${secret}`, t('admin_deploy_command_copied'));
    },
    [copyToClipboard, deploy, pushBanner, t],
  );

  const addNode = React.useCallback(async () => {
    if (!token) return;
    setIsCreating(true);
    try {
      await createNode();
      pushBanner(t('admin_node_created'), { tone: 'info' });
    } catch (error) {
      apiError(error, t('admin_create_node_failed'));
      setIsCreating(false);
      return;
    }
    try {
      await refreshNodes();
    } catch (error) {
      apiError(error, t('admin_fetch_nodes_failed'));
    } finally {
      setIsCreating(false);
    }
  }, [apiError, pushBanner, refreshNodes, t, token]);

  const { draggingId, dragOverId, dragStart, dragOver, drop, dragEnd } = useReorder({
    token,
    nodes,
    setNodes,
    filteredNodeIds,
    refreshNodes,
  });

  const rename = React.useCallback(
    async (node: NodeRow, nextName: string) => {
      const trimmed = nextName.trim();
      if (!trimmed || trimmed === node.name.trim() || !token) return;
      const ok = await requestConfirm({
        title: t('common_confirm'),
        message: t('admin_confirm_save_node_name', { name: node.name, next: trimmed }),
        confirmLabel: t('common_save_changes'),
        cancelLabel: t('common_cancel'),
        tone: 'default',
      });
      if (!ok) return;

      try {
        await updateNode(node.id, { name: trimmed });
        setNodes((prev) =>
          prev.map((item) => (item.id === node.id ? { ...item, name: trimmed } : item)),
        );
        pushBanner(t('admin_node_name_updated'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_update_node_name_failed'));
      }
    },
    [apiError, pushBanner, requestConfirm, setNodes, t, token],
  );

  const toggleGuestVisible = React.useCallback(
    async (node: NodeRow) => {
      if (!token) return;
      const nextVisible = !node.guestVisible;
      try {
        await updateNode(node.id, { is_guest_visible: nextVisible });
        setNodes((prev) =>
          prev.map((item) => (item.id === node.id ? { ...item, guestVisible: nextVisible } : item)),
        );
        pushBanner(t('admin_guest_visible_updated'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_update_guest_visible_failed'));
      }
    },
    [apiError, pushBanner, setNodes, t, token],
  );

  const toggleTrafficP95 = React.useCallback(
    async (node: NodeRow) => {
      if (!token || savingP95NodeIds.has(node.id)) return;
      const nextEnabled = !node.trafficP95Enabled;
      setSavingP95NodeIds((current) => new Set(current).add(node.id));
      setNodes((prev) =>
        prev.map((item) =>
          item.id === node.id ? { ...item, trafficP95Enabled: nextEnabled } : item,
        ),
      );
      try {
        await updateNode(node.id, { traffic_p95_enabled: nextEnabled });
        pushBanner(t('admin_traffic_p95_updated'), { tone: 'info' });
      } catch (error) {
        setNodes((prev) =>
          prev.map((item) =>
            item.id === node.id ? { ...item, trafficP95Enabled: node.trafficP95Enabled } : item,
          ),
        );
        apiError(error, t('admin_traffic_p95_update_failed'));
      } finally {
        setSavingP95NodeIds((current) => {
          const next = new Set(current);
          next.delete(node.id);
          return next;
        });
      }
    },
    [apiError, pushBanner, savingP95NodeIds, setNodes, t, token],
  );

  const toggleP95NodeSelection = React.useCallback((id: number) => {
    setSelectedP95NodeIds((current) => {
      const next = new Set(current);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const toggleVisibleP95Nodes = React.useCallback(() => {
    setSelectedP95NodeIds((current) => {
      const next = new Set(current);
      if (filteredNodeIds.length > 0 && filteredNodeIds.every((id) => next.has(id))) {
        for (const id of filteredNodeIds) next.delete(id);
      } else {
        for (const id of filteredNodeIds) next.add(id);
      }
      return next;
    });
  }, [filteredNodeIds]);

  const setTrafficP95ForSelected = React.useCallback(
    async (enabled: boolean) => {
      if (!token || selectedP95Ids.length === 0) return;
      const selected = nodes.filter((node) => selectedP95NodeIds.has(node.id));
      if (selected.length === 0) return;
      const previous = new Map(selected.map((node) => [node.id, node.trafficP95Enabled]));
      const selectedIds = selected.map((node) => node.id);
      setSavingP95NodeIds((current) => new Set([...current, ...selectedIds]));
      setNodes((prev) =>
        prev.map((node) =>
          previous.has(node.id) ? { ...node, trafficP95Enabled: enabled } : node,
        ),
      );
      try {
        await updateNodesTrafficP95(selectedIds, enabled);
        pushBanner(
          t(enabled ? 'admin_traffic_p95_batch_enabled' : 'admin_traffic_p95_batch_disabled'),
          { tone: 'info' },
        );
      } catch (error) {
        setNodes((prev) =>
          prev.map((node) =>
            previous.has(node.id)
              ? { ...node, trafficP95Enabled: previous.get(node.id) ?? node.trafficP95Enabled }
              : node,
          ),
        );
        apiError(error, t('admin_traffic_p95_update_failed'));
      } finally {
        setSavingP95NodeIds((current) => {
          const next = new Set(current);
          for (const id of selectedIds) next.delete(id);
          return next;
        });
      }
    },
    [apiError, nodes, pushBanner, selectedP95Ids.length, selectedP95NodeIds, setNodes, t, token],
  );

  const saveNodeTrafficSettings = React.useCallback(
    async (node: NodeRow, patch: NodeTrafficPatch): Promise<boolean> => {
      if (!token || savingTrafficSettingsNodeIds.has(node.id)) return false;
      setSavingTrafficSettingsNodeIds((current) => new Set(current).add(node.id));
      let failureMessage = t('admin_node_traffic_settings_save_failed');
      try {
        await updateNode(node.id, patch);
        failureMessage = t('admin_fetch_nodes_failed');
        await refreshNodes();
        pushBanner(t('admin_node_traffic_settings_saved'), { tone: 'info' });
        return true;
      } catch (error) {
        apiError(error, failureMessage);
        return false;
      } finally {
        setSavingTrafficSettingsNodeIds((current) => {
          const next = new Set(current);
          next.delete(node.id);
          return next;
        });
      }
    },
    [apiError, pushBanner, refreshNodes, savingTrafficSettingsNodeIds, t, token],
  );

  const confirmAndDelete = React.useCallback(
    async (node: NodeRow) => {
      const ok = await requestConfirm({
        title: t('common_confirm'),
        message: t('admin_confirm_delete_node', { name: node.name }),
        confirmLabel: t('common_delete'),
        cancelLabel: t('common_cancel'),
        tone: 'danger',
      });
      if (!ok || !token) return;
      try {
        await deleteNode(node.id);
        setNodes((prev) => prev.filter((item) => item.id !== node.id));
        pushBanner(t('admin_node_deleted'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_delete_node_failed'));
      }
    },
    [apiError, pushBanner, requestConfirm, setNodes, t, token],
  );

  const confirmUpgrade = React.useCallback(
    async (node: NodeRow) => {
      if (!token || !bundledNodeVersion) return;
      const ok = await requestConfirm({
        title: t('common_confirm'),
        message: t('admin_confirm_upgrade_node', {
          name: node.name,
          version: bundledNodeVersion,
        }),
        confirmLabel: t('admin_nodes_confirm_upgrade'),
        cancelLabel: t('common_cancel'),
        tone: 'default',
      });
      if (!ok) return;

      try {
        await requestNodeUpgrade(node.id);
        pushBanner(t('admin_node_upgrade_requested'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_request_node_upgrade_failed'));
        return;
      }
      try {
        await refreshNodes();
      } catch (error) {
        apiError(error, t('admin_fetch_nodes_failed'));
      }
    },
    [apiError, bundledNodeVersion, pushBanner, refreshNodes, requestConfirm, t, token],
  );

  const rebuildNodeTraffic = React.useCallback(
    async (node: NodeRow) => {
      if (!token || trafficRebuildBusy) return;
      const ok = await requestConfirm({
        title: t('common_confirm'),
        message: t('admin_confirm_rebuild_node_traffic', { name: node.name }),
        confirmLabel: t('admin_node_traffic_rebuild'),
        cancelLabel: t('common_cancel'),
        tone: 'default',
      });
      if (ok) {
        const outcome = await startTrafficRebuild(node.id);
        showTrafficRebuildOutcome(outcome);
      }
    },
    [requestConfirm, showTrafficRebuildOutcome, startTrafficRebuild, t, token, trafficRebuildBusy],
  );

  const saveSettings = React.useCallback(
    async (
      nodeId: number,
      input: {
        name: string;
        secret: string;
        guestVisible: boolean;
        groupIds: number[];
        tags?: string[];
      },
    ) => {
      if (!token) return;
      try {
        const payload = {
          name: input.name,
          secret: input.secret,
          is_guest_visible: input.guestVisible,
          group_ids: input.groupIds,
          ...(input.tags !== undefined ? { tags: input.tags } : {}),
        };
        await updateNode(nodeId, payload);
        const groupNames =
          input.groupIds.length > 0
            ? input.groupIds.map((gid) => groupLookup[gid] ?? `#${gid}`)
            : [];
        setNodes((prev) =>
          prev.map((node) =>
            node.id === nodeId
              ? {
                  ...node,
                  name: input.name,
                  secret: input.secret,
                  guestVisible: input.guestVisible,
                  groupIds: input.groupIds,
                  groupNames,
                  ...(input.tags !== undefined ? { tags: input.tags } : {}),
                }
              : node,
          ),
        );
        pushBanner(t('admin_node_settings_updated'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_save_node_settings_failed'));
      }
    },
    [groupLookup, apiError, pushBanner, setNodes, t, token],
  );

  return (
    <div className="space-y-4 md:space-y-6">
      <ConfirmDialog {...confirmDialogProps} />

      <div className="flex w-full md:w-auto md:flex-1">
        <div className="flex gap-4 border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
          <button
            type="button"
            onClick={() => setActiveTab('basic')}
            aria-current={activeTab === 'basic' ? 'page' : undefined}
            className={tabClass(activeTab === 'basic')}
          >
            <span className="inline-flex items-center gap-2">
              <Settings2 className="size-4" aria-hidden="true" />
              {t('admin_nodes_tab_basic')}
            </span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('advanced')}
            aria-current={activeTab === 'advanced' ? 'page' : undefined}
            className={tabClass(activeTab === 'advanced')}
          >
            <span className="inline-flex items-center gap-2">
              <SlidersHorizontal className="size-4" aria-hidden="true" />
              {t('admin_nodes_tab_advanced')}
            </span>
          </button>
        </div>
      </div>

      <div className="flex flex-col md:flex-row justify-between gap-3 md:gap-4">
        <div className="flex w-full md:w-auto md:flex-1 gap-2">
          <Input
            icon={Search}
            placeholder={t('admin_nodes_search_placeholder')}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            data-search-input="true"
            wrapperClassName="flex-1 max-w-md"
          />
          <NodeFilterMenu
            groups={groups}
            selectedGroupIds={selectedGroupIds}
            updateableOnly={updateableOnly}
            showVersionFilter={activeTab === 'basic'}
            onGroupChange={setSelectedGroupIds}
            onUpdateableOnlyChange={setUpdateableOnly}
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
            updatableNodeIds={updatableNodeIds}
            manualUpdateNodeIds={manualUpdateNodeIds}
            bundledNodeVersion={bundledNodeVersion}
            draggingId={draggingId}
            dragOverId={dragOverId}
            onRename={(node, nextName) => void rename(node, nextName)}
            onToggleGuestVisible={(node) => void toggleGuestVisible(node)}
            onCopySecret={(secret) => void copyToClipboard(secret)}
            onDeployCopy={copyDeploy}
            onRequestUpgrade={(node) => void confirmUpgrade(node)}
            onOpenSettings={setBasicSettingsNode}
            onDelete={(node) => void confirmAndDelete(node)}
            onDragStart={dragStart}
            onDragOver={dragOver}
            onDrop={drop}
            onDragEnd={dragEnd}
          />
        </Card>
      ) : (
        <Card className="hidden md:block overflow-hidden">
          <NodeAdvancedTable
            nodes={filteredNodes}
            selectedP95NodeIds={selectedP95NodeIds}
            allVisibleP95Selected={allVisibleP95Selected}
            someVisibleP95Selected={someVisibleP95Selected}
            savingP95NodeIds={savingP95NodeIds}
            savingTrafficSettingsNodeIds={savingTrafficSettingsNodeIds}
            rebuildingTrafficNodeId={rebuildingTrafficNodeId}
            trafficRebuildBusy={trafficRebuildBusy}
            onToggleVisibleNodes={toggleVisibleP95Nodes}
            onToggleP95Node={toggleP95NodeSelection}
            onToggleTrafficP95={(node) => void toggleTrafficP95(node)}
            onOpenTrafficSettings={setTrafficSettingsNode}
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
                <Button variant="ghost" onClick={() => setSelectedP95NodeIds(new Set())}>
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
              bundledNodeVersion={bundledNodeVersion}
              canRequestUpgrade={updatableNodeIds.has(node.id)}
              needsManualUpdate={manualUpdateNodeIds.has(node.id)}
              onOpenSettings={setBasicSettingsNode}
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
              p95Selected={selectedP95NodeIds.has(node.id)}
              savingP95={savingP95NodeIds.has(node.id)}
              savingTrafficSettings={savingTrafficSettingsNodeIds.has(node.id)}
              rebuilding={rebuildingTrafficNodeId === node.id}
              trafficRebuildBusy={trafficRebuildBusy}
              onToggleP95Node={toggleP95NodeSelection}
              onToggleTrafficP95={(target) => void toggleTrafficP95(target)}
              onOpenTrafficSettings={setTrafficSettingsNode}
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
                <Button variant="ghost" onClick={() => setSelectedP95NodeIds(new Set())}>
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
          onClose={() => setBasicSettingsNode(null)}
          onSave={(input) => void saveSettings(basicSettingsNode.id, input)}
        />
      )}

      {trafficSettingsNode && (
        <NodeTrafficSettingsModal
          isOpen={!!trafficSettingsNode}
          node={trafficSettingsNode}
          globalSettings={trafficSettings}
          saving={savingTrafficSettingsNodeIds.has(trafficSettingsNode.id)}
          onClose={() => setTrafficSettingsNode(null)}
          onSave={(patch) => saveNodeTrafficSettings(trafficSettingsNode, patch)}
        />
      )}
    </div>
  );
};

export default NodeManager;
