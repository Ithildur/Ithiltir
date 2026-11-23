import React from 'react';
import {
  runningRebuildNodeId,
  startTrafficRebuild,
  useTrafficRebuildStore,
} from '@stores/trafficRebuildStore';

interface UseTrafficRebuildOptions {
  nodeId?: number | null;
}

const normalizedNodeId = (id: number | null | undefined): number | null =>
  id !== null && id !== undefined && Number.isFinite(id) && id > 0 ? id : null;

export const useTrafficRebuild = (options: UseTrafficRebuildOptions = {}) => {
  const nodeId = normalizedNodeId(options.nodeId);
  const status = useTrafficRebuildStore((state) => state.status);
  const startingNodeId = useTrafficRebuildStore((state) => state.startingNodeId);
  const [completionCount, setCompletionCount] = React.useState(0);
  const observedNodeRef = React.useRef<number | null>(null);

  React.useEffect(() => {
    if (nodeId === null) {
      observedNodeRef.current = null;
      return;
    }

    const nodePending =
      startingNodeId === nodeId || (status.running && status.server_id === nodeId);
    if (nodePending) {
      observedNodeRef.current = nodeId;
      return;
    }

    if (observedNodeRef.current !== nodeId) {
      observedNodeRef.current = null;
      return;
    }
    observedNodeRef.current = null;
    setCompletionCount((count) => count + 1);
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
    completionCount,
    start: startTrafficRebuild,
  };
};
