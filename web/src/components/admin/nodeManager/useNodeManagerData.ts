import React from 'react';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import type { Group, NodeDeploy } from '@app-types/api';
import type { NodeRow } from '@app-types/admin';
import type { TrafficSettings } from '@app-types/traffic';
import {
  clearAdminNodeDeploy,
  clearBundledNodeVersion,
  loadAdminNodeDeploy,
  loadAdminNodeOverview,
  loadBundledNodeVersion,
  refreshAdminNodes,
  useAdminNodesStore,
} from '@stores/adminNodesStore';
import { useAdminGroupsStore } from '@stores/adminGroupsStore';
import { loadTrafficSettings, useTrafficSettingsStore } from '@stores/trafficSettingsStore';
import { isCanceledRequestError } from '@utils/errors';

export const useNodeManagerData = (
  token: string | null,
): {
  nodes: NodeRow[];
  groups: Group[];
  deploy: NodeDeploy | null;
  trafficSettings: TrafficSettings;
  trafficSettingsLoading: boolean;
  trafficSettingsLoaded: boolean;
  bundledNodeVersion: string;
  isLoading: boolean;
  refreshNodes: () => Promise<void>;
} => {
  const apiError = useApiErrorHandler();
  const nodes = useAdminNodesStore((state) => state.nodes);
  const groups = useAdminGroupsStore((state) => state.groups);
  const deploy = useAdminNodesStore((state) => state.deploy);
  const trafficSettings = useTrafficSettingsStore((state) => state.settings);
  const trafficSettingsLoading = useTrafficSettingsStore((state) => state.loading);
  const trafficSettingsLoaded = useTrafficSettingsStore((state) => state.loaded);
  const bundledNodeVersion = useAdminNodesStore((state) => state.bundledNodeVersion);
  const isLoading = useAdminNodesStore((state) => state.loading);

  React.useEffect(() => {
    if (!token) return;
    const controller = new AbortController();

    const load = async () => {
      try {
        await loadAdminNodeOverview({ signal: controller.signal });
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_fetch_nodes_data_failed' });
      }
    };

    void load();
    return () => {
      controller.abort();
    };
  }, [apiError, token]);

  React.useEffect(() => {
    if (!token) return;
    const controller = new AbortController();

    void loadTrafficSettings({ signal: controller.signal }).catch((error) => {
      if (isCanceledRequestError(error)) return;
      apiError(error, { key: 'admin_traffic_fetch_failed' });
    });

    return () => {
      controller.abort();
    };
  }, [apiError, token]);

  React.useEffect(() => {
    if (!token) return;
    const controller = new AbortController();

    loadBundledNodeVersion({ signal: controller.signal }).catch((error) => {
      if (isCanceledRequestError(error)) return;
      clearBundledNodeVersion();
      apiError(error, { key: 'admin_fetch_nodes_data_failed' });
    });

    return () => {
      controller.abort();
    };
  }, [apiError, token]);

  React.useEffect(() => {
    if (!token) return;
    const controller = new AbortController();

    const loadDeploy = async () => {
      try {
        await loadAdminNodeDeploy({ signal: controller.signal });
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_deploy_command_unavailable' });
        clearAdminNodeDeploy();
      }
    };

    void loadDeploy();
    return () => {
      controller.abort();
    };
  }, [apiError, token]);

  const refreshNodes = React.useCallback(async () => {
    if (!token) return;
    await refreshAdminNodes();
  }, [token]);

  return {
    nodes,
    groups,
    deploy,
    trafficSettings,
    trafficSettingsLoading,
    trafficSettingsLoaded,
    bundledNodeVersion,
    isLoading,
    refreshNodes,
  };
};
