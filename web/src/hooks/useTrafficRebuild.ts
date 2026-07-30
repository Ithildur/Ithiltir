import React from 'react';
import type { NodeTrafficRebuildStatus } from '@lib/adminApi';
import {
  runningRebuildNodeId,
  startTrafficRebuild,
  useTrafficRebuildStore,
  type TrafficRebuildStartOutcome,
} from '@stores/trafficRebuildStore';

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
  const [finishedKey, setFinishedKey] = React.useState<string | null>(null);
  const observedNodeRef = React.useRef<number | null>(null);

  React.useEffect(() => {
    if (nodeId === null) {
      observedNodeRef.current = null;
      setFinishedKey(null);
      return;
    }

    const nodePending =
      startingNodeId === nodeId || (status.running && status.server_id === nodeId);
    if (nodePending) {
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
  }, [nodeId, startingNodeId, status]);

  const rebuildingNodeId = runningRebuildNodeId(status) ?? startingNodeId;
  const busy = status.running || startingNodeId !== null;
  const nodeRebuildActive = nodeId !== null && status.running && status.server_id === nodeId;
  const nodeRebuildBusy = nodeId !== null && (nodeRebuildActive || startingNodeId === nodeId);
  const startBusy = nodeId === null ? busy : nodeRebuildBusy;

  return {
    rebuildingNodeId,
    busy,
    nodeRebuildActive,
    startBusy,
    finishedKey,
    start: startTrafficRebuild,
  };
};
