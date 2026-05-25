import React from 'react';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useI18n } from '@i18n';
import { ApiError } from '@lib/api';
import {
  fetchTrafficRebuild,
  rebuildNodeTraffic,
  type NodeTrafficRebuildStatus,
} from '@lib/adminApi';

const activePollMs = 3000;
const idlePollMs = 15000;
const idleStatus: NodeTrafficRebuildStatus = {
  server_id: 0,
  status: 'idle',
  running: false,
};

interface Options {
  token: string | null;
  sync: boolean;
}

const isAbortError = (error: unknown): boolean =>
  error instanceof DOMException && error.name === 'AbortError';

const runningNode = (status: NodeTrafficRebuildStatus): number | null =>
  status.running && status.server_id > 0 ? status.server_id : null;

const isRebuildRunningError = (error: unknown): boolean =>
  error instanceof ApiError && error.status === 409 && error.code === 'traffic_rebuild_running';

export const useTrafficRebuild = ({ token, sync }: Options) => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const pushBanner = useTopBanner();
  const [status, setStatus] = React.useState<NodeTrafficRebuildStatus>(idleStatus);
  const statusRef = React.useRef<NodeTrafficRebuildStatus>(idleStatus);
  const watchNodeIdRef = React.useRef<number | null>(null);
  const syncErrorShownRef = React.useRef(false);
  const rebuildingNodeId = runningNode(status);
  const rebuildActive = status.running;

  const setRebuildStatus = React.useCallback((next: NodeTrafficRebuildStatus) => {
    statusRef.current = next;
    setStatus(next);
  }, []);

  const showStatusSyncError = React.useCallback(() => {
    if (syncErrorShownRef.current) return;
    pushBanner(t('admin_node_traffic_rebuild_status_failed'), { tone: 'error' });
    syncErrorShownRef.current = true;
  }, [pushBanner, t]);

  const applyStatus = React.useCallback(
    (next: NodeTrafficRebuildStatus) => {
      const watchNodeId = watchNodeIdRef.current;
      const current = statusRef.current;
      if (
        next.status === 'idle' &&
        watchNodeId !== null &&
        current.running &&
        current.server_id === watchNodeId
      ) {
        watchNodeIdRef.current = null;
      }

      syncErrorShownRef.current = false;
      setRebuildStatus(next);

      if (watchNodeId === null) return;
      if (watchNodeId !== next.server_id) {
        if (!next.running) {
          watchNodeIdRef.current = null;
        }
        return;
      }

      if (next.status === 'completed') {
        pushBanner(t('admin_node_traffic_rebuild_completed'), { tone: 'info' });
        watchNodeIdRef.current = null;
      } else if (next.status === 'failed') {
        pushBanner(t('admin_node_traffic_rebuild_failed'), { tone: 'error' });
        watchNodeIdRef.current = null;
      } else if (next.status === 'idle') {
        watchNodeIdRef.current = null;
      }
    },
    [pushBanner, setRebuildStatus, t],
  );

  React.useEffect(() => {
    if (token) return;
    watchNodeIdRef.current = null;
    syncErrorShownRef.current = false;
    setRebuildStatus(idleStatus);
  }, [setRebuildStatus, token]);

  React.useEffect(() => {
    if (!token) return;
    if (!sync && !statusRef.current.running && watchNodeIdRef.current === null) return;

    const controller = new AbortController();
    let timer: number | undefined;

    const nextPollMs = (): number =>
      statusRef.current.running || watchNodeIdRef.current !== null ? activePollMs : idlePollMs;

    const refresh = async () => {
      let delay = idlePollMs;
      try {
        const next = await fetchTrafficRebuild(controller.signal);
        if (controller.signal.aborted) return;
        applyStatus(next);
        delay = nextPollMs();
      } catch (error) {
        if (isAbortError(error)) {
          return;
        }
        if (statusRef.current.running || watchNodeIdRef.current !== null) {
          showStatusSyncError();
          delay = activePollMs;
        } else {
          delay = idlePollMs;
        }
      } finally {
        if (
          !controller.signal.aborted &&
          (sync || statusRef.current.running || watchNodeIdRef.current !== null)
        ) {
          timer = window.setTimeout(() => void refresh(), delay);
        }
      }
    };

    void refresh();
    return () => {
      controller.abort();
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [applyStatus, showStatusSyncError, sync, token]);

  const start = React.useCallback(
    async (id: number) => {
      if (!token || statusRef.current.running) return;

      const lastStatus = statusRef.current;
      watchNodeIdRef.current = id;
      syncErrorShownRef.current = false;
      setRebuildStatus({ server_id: id, status: 'running', running: true });
      try {
        const started = await rebuildNodeTraffic(id);
        applyStatus(started);
        if (started.running) {
          pushBanner(t('admin_node_traffic_rebuild_started'), { tone: 'info' });
        }
      } catch (error) {
        if (isRebuildRunningError(error)) {
          watchNodeIdRef.current = null;
          try {
            applyStatus(await fetchTrafficRebuild());
          } catch {
            showStatusSyncError();
            setRebuildStatus(lastStatus);
          }
          return;
        }
        watchNodeIdRef.current = null;
        syncErrorShownRef.current = false;
        setRebuildStatus(lastStatus);
        apiError(error, t('admin_node_traffic_rebuild_failed'));
      }
    },
    [apiError, applyStatus, pushBanner, setRebuildStatus, showStatusSyncError, t, token],
  );

  return {
    rebuildingNodeId,
    rebuildActive,
    start,
  };
};
