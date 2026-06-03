import React from 'react';
import type { NodeDeploy, NodeDeployPlatform, NodeTrafficPatch } from '@app-types/api';
import type { NodeRow } from '@app-types/admin';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import type { ConfirmRequest } from '@hooks/useConfirmDialog';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { useI18n } from '@i18n';
import {
  addAdminNode,
  removeAdminNode,
  renameAdminNode,
  requestAdminNodeUpgrade,
  saveAdminNodeSettings,
  saveAdminNodeTrafficSettings,
  setAdminNodeGuestVisible,
  setAdminNodesTrafficP95,
  setAdminNodeTrafficP95,
  type AdminNodeSettingsInput,
} from '@stores/adminNodesStore';
import type { TrafficRebuildStartOutcome } from '@stores/trafficRebuildStore';
import { copyTextToClipboardWithFeedback } from '@utils/clipboard';

export const useNodeManagerActions = ({
  token,
  deploy,
  nodes,
  selectedP95NodeIdSet,
  bundledNodeVersion,
  trafficRebuildBusy,
  requestConfirm,
  startTrafficRebuild,
  showTrafficRebuildOutcome,
}: {
  token: string | null;
  deploy: NodeDeploy | null;
  nodes: NodeRow[];
  selectedP95NodeIdSet: ReadonlySet<number>;
  bundledNodeVersion: string;
  trafficRebuildBusy: boolean;
  requestConfirm: ConfirmRequest;
  startTrafficRebuild: (id: number) => Promise<TrafficRebuildStartOutcome>;
  showTrafficRebuildOutcome: (outcome: TrafficRebuildStartOutcome) => void;
}) => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();

  const copyToClipboard = React.useCallback(
    async (text: string, successMessage = t('admin_secret_copied')) => {
      await copyTextToClipboardWithFeedback(text, {
        pushBanner: pushTopBanner,
        successMessage,
        httpsRequiredMessage: t('admin_clipboard_https_required'),
        failureMessage: t('admin_copy_failed_manual'),
      });
    },
    [t],
  );

  const copyDeploy = React.useCallback(
    (platform: NodeDeployPlatform, secret: string) => {
      const prefix = deploy?.scripts?.[platform]?.command_prefix;
      if (!prefix) {
        pushTopBanner(t('admin_deploy_command_unavailable'), { tone: 'error' });
        return;
      }
      void copyToClipboard(`${prefix}${secret}`, t('admin_deploy_command_copied'));
    },
    [copyToClipboard, deploy, t],
  );

  const addNode = React.useCallback(async () => {
    if (!token) return;
    try {
      const didCreate = await addAdminNode();
      if (!didCreate) return;
      pushTopBanner(t('admin_node_created'), { tone: 'info' });
    } catch (error) {
      apiError(error, t('admin_create_node_failed'));
    }
  }, [apiError, t, token]);

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
        const didRename = await renameAdminNode(node.id, trimmed);
        if (!didRename) return;
        pushTopBanner(t('admin_node_name_updated'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_update_node_name_failed'));
      }
    },
    [apiError, requestConfirm, t, token],
  );

  const toggleGuestVisible = React.useCallback(
    async (node: NodeRow) => {
      if (!token) return;
      try {
        const didSave = await setAdminNodeGuestVisible(node.id, !node.guestVisible);
        if (!didSave) return;
        pushTopBanner(t('admin_guest_visible_updated'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_update_guest_visible_failed'));
      }
    },
    [apiError, t, token],
  );

  const toggleTrafficP95 = React.useCallback(
    async (node: NodeRow) => {
      if (!token) return;
      try {
        const didSave = await setAdminNodeTrafficP95(node.id, !node.trafficP95Enabled);
        if (!didSave) return;
        pushTopBanner(t('admin_traffic_p95_updated'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_traffic_p95_update_failed'));
      }
    },
    [apiError, t, token],
  );

  const setTrafficP95ForSelected = React.useCallback(
    async (enabled: boolean) => {
      if (!token || selectedP95NodeIdSet.size === 0) return;
      const selectedIds = nodes
        .filter((node) => selectedP95NodeIdSet.has(node.id))
        .map((node) => node.id);
      if (selectedIds.length === 0) return;

      try {
        const didSave = await setAdminNodesTrafficP95(selectedIds, enabled);
        if (!didSave) return;
        pushTopBanner(
          t(enabled ? 'admin_traffic_p95_batch_enabled' : 'admin_traffic_p95_batch_disabled'),
          { tone: 'info' },
        );
      } catch (error) {
        apiError(error, t('admin_traffic_p95_update_failed'));
      }
    },
    [apiError, nodes, selectedP95NodeIdSet, t, token],
  );

  const saveNodeTrafficSettings = React.useCallback(
    async (nodeId: number, patch: NodeTrafficPatch): Promise<boolean> => {
      if (!token) return false;
      try {
        const didSave = await saveAdminNodeTrafficSettings(nodeId, patch);
        if (!didSave) return false;
        pushTopBanner(t('admin_node_traffic_settings_saved'), { tone: 'info' });
        return true;
      } catch (error) {
        apiError(error, t('admin_node_traffic_settings_save_failed'));
        return false;
      }
    },
    [apiError, t, token],
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
        const didRemove = await removeAdminNode(node.id);
        if (!didRemove) return;
        pushTopBanner(t('admin_node_deleted'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_delete_node_failed'));
      }
    },
    [apiError, requestConfirm, t, token],
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
        const didRequest = await requestAdminNodeUpgrade(node.id);
        if (!didRequest) return;
        pushTopBanner(t('admin_node_upgrade_requested'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_request_node_upgrade_failed'));
      }
    },
    [apiError, bundledNodeVersion, requestConfirm, t, token],
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
      if (!ok) return;

      const outcome = await startTrafficRebuild(node.id);
      showTrafficRebuildOutcome(outcome);
    },
    [requestConfirm, showTrafficRebuildOutcome, startTrafficRebuild, t, token, trafficRebuildBusy],
  );

  const saveSettings = React.useCallback(
    async (nodeId: number, input: AdminNodeSettingsInput): Promise<boolean> => {
      if (!token) return false;
      try {
        const didSave = await saveAdminNodeSettings(nodeId, input);
        if (!didSave) return false;
        pushTopBanner(t('admin_node_settings_updated'), { tone: 'info' });
        return true;
      } catch (error) {
        apiError(error, t('admin_save_node_settings_failed'));
        return false;
      }
    },
    [apiError, t, token],
  );

  return {
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
  };
};
