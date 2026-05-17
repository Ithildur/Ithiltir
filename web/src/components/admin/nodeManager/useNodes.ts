import React from 'react';
import { useI18n } from '@i18n';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { fetchGroupList, fetchNodeDeploy, fetchNodes } from '@lib/adminApi';
import { fetchTrafficSettings } from '@lib/statisticsApi';
import { fetchAppVersion } from '@lib/versionApi';
import type { Group, NodeDeploy } from '@app-types/api';
import type { NodeRow } from '@app-types/admin';
import type { TrafficSettings } from '@app-types/traffic';
import {
  buildGroupLookup,
  nodeRowsFromManaged,
} from '@components/admin/nodeManager/nodeManagerModel';

const defaultTrafficSettings: TrafficSettings = {
  guest_access_mode: 'disabled',
  usage_mode: 'lite',
  cycle_mode: 'calendar_month',
  billing_start_day: 1,
  billing_anchor_date: '',
  billing_timezone: '',
  direction_mode: 'out',
};

export const useNodes = (
  token: string | null,
): {
  nodes: NodeRow[];
  setNodes: React.Dispatch<React.SetStateAction<NodeRow[]>>;
  groups: Group[];
  groupLookup: Record<number, string>;
  deploy: NodeDeploy | null;
  trafficSettings: TrafficSettings;
  bundledNodeVersion: string;
  isLoading: boolean;
  refreshNodes: (nextLookup?: Record<number, string>) => Promise<void>;
} => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();

  const [nodes, setNodes] = React.useState<NodeRow[]>([]);
  const [groups, setGroups] = React.useState<Group[]>([]);
  const [groupLookup, setGroupLookup] = React.useState<Record<number, string>>({});
  const [deploy, setDeploy] = React.useState<NodeDeploy | null>(null);
  const [trafficSettings, setTrafficSettings] =
    React.useState<TrafficSettings>(defaultTrafficSettings);
  const [bundledNodeVersion, setBundledNodeVersion] = React.useState('');
  const [isLoading, setIsLoading] = React.useState(true);

  React.useEffect(() => {
    if (!token) return;
    let cancelled = false;

    const load = async () => {
      setIsLoading(true);
      try {
        const [groupRes, nodeRes, trafficRes] = await Promise.all([
          fetchGroupList(),
          fetchNodes(),
          fetchTrafficSettings(),
        ]);
        if (cancelled) return;
        const lookup = buildGroupLookup(groupRes);
        setGroups(groupRes);
        setGroupLookup(lookup);
        setTrafficSettings(trafficRes);
        setNodes(nodeRowsFromManaged(nodeRes, lookup));
      } catch (error) {
        if (!cancelled) {
          apiError(error, t('admin_fetch_nodes_data_failed'));
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [apiError, t, token]);

  React.useEffect(() => {
    if (!token) return;
    const controller = new AbortController();

    fetchAppVersion({ signal: controller.signal })
      .then((res) => setBundledNodeVersion(res.node_version?.trim() ?? ''))
      .catch((error) => {
        if (error instanceof DOMException && error.name === 'AbortError') return;
        console.warn('Failed to fetch app version', error);
        setBundledNodeVersion('');
      });

    return () => {
      controller.abort();
    };
  }, [token]);

  React.useEffect(() => {
    if (!token) return;
    let cancelled = false;

    const loadDeploy = async () => {
      try {
        const res = await fetchNodeDeploy();
        if (!cancelled) setDeploy(res);
      } catch (error) {
        if (cancelled) return;
        apiError(error, t('admin_deploy_command_unavailable'));
        console.warn('Failed to fetch deploy scripts', error);
        setDeploy(null);
      }
    };

    void loadDeploy();
    return () => {
      cancelled = true;
    };
  }, [apiError, t, token]);

  const refreshNodes = React.useCallback(
    async (nextLookup?: Record<number, string>) => {
      if (!token) return;
      const lookup = nextLookup ?? groupLookup;
      try {
        const nodeRes = await fetchNodes();
        setNodes(nodeRowsFromManaged(nodeRes, lookup));
      } catch (error) {
        apiError(error, t('admin_fetch_nodes_failed'));
      }
    },
    [groupLookup, apiError, t, token],
  );

  return {
    nodes,
    setNodes,
    groups,
    groupLookup,
    deploy,
    trafficSettings,
    bundledNodeVersion,
    isLoading,
    refreshNodes,
  };
};
