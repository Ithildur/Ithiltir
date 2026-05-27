import React from 'react';
import type { NodeTrafficRebuildStatus } from '@lib/adminApi';
import {
  runningRebuildNodeId,
  useTrafficRebuildStore,
  type TrafficRebuildStartOutcome,
} from '../stores/trafficRebuildStore';

interface UseTrafficRebuildOptions {
  nodeId?: number | null;
}

export type { TrafficRebuildStartOutcome };

const normalizedNodeId = (id: number | null | undefined): number | null =>
  id !== null && id !== undefined && Number.isFinite(id) && id > 0 ? id : null;

const watchedFinishedKey = (status: NodeTrafficRebuildStatus): string =>
  status.status === 'idle'
    ? `idle:${status.finished_at ?? ''}`
    : `${status.status}:${status.server_id}:${status.finished_at ?? status.started_at ?? ''}`;

export const useTrafficRebuild = (options: UseTrafficRebuildOptions = {}) => {
  const nodeId = normalizedNodeId(options.nodeId);
  const status = useTrafficRebuildStore((state) => state.status);
  const startingNodeId = useTrafficRebuildStore((state) => state.startingNodeId);
  const startedNodeId = useTrafficRebuildStore((state) => state.startedNodeId);
  const start = useTrafficRebuildStore((state) => state.start);
  const [finishedKey, setFinishedKey] = React.useState<string | null>(null);
  const observedNodeRef = React.useRef<number | null>(null);

  React.useEffect(() => {
    if (nodeId === null) {
      observedNodeRef.current = null;
      setFinishedKey(null);
      return;
    }

    const nodeRunning = status.running && status.server_id === nodeId;
    if (nodeRunning) {
      observedNodeRef.current = nodeId;
      setFinishedKey((current) => (current === null ? current : null));
      return;
    }

    if (observedNodeRef.current !== nodeId) {
      observedNodeRef.current = null;
      return;
    }
    observedNodeRef.current = null;
    const nextKey = watchedFinishedKey(status);
    setFinishedKey((current) => (current === nextKey ? current : nextKey));
  }, [status, nodeId]);

  const rebuildingNodeId = runningRebuildNodeId(status) ?? startingNodeId ?? startedNodeId;
  const rebuildRunning = status.running;
  const busy = rebuildRunning || startingNodeId !== null || startedNodeId !== null;
  const nodeRebuildActive = nodeId !== null && status.running && status.server_id === nodeId;
  const nodeRebuildBusy =
    nodeId !== null && (nodeRebuildActive || startingNodeId === nodeId || startedNodeId === nodeId);
  const actionBusy = nodeId === null ? busy : nodeRebuildBusy;

  return {
    rebuildingNodeId,
    startingNodeId,
    startedNodeId,
    rebuildRunning,
    busy,
    nodeRebuildActive,
    nodeRebuildBusy,
    actionBusy,
    finishedKey,
    start,
  };
};
