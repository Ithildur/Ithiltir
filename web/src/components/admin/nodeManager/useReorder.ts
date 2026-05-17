import React from 'react';
import { useI18n } from '@i18n';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { updateNodesDisplayOrder } from '@lib/adminApi';
import type { NodeRow } from '@app-types/admin';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';

export const useReorder = ({
  token,
  nodes,
  setNodes,
  filteredNodeIds,
  refreshNodes,
}: {
  token: string | null;
  nodes: NodeRow[];
  setNodes: React.Dispatch<React.SetStateAction<NodeRow[]>>;
  filteredNodeIds: number[];
  refreshNodes: () => void;
}): {
  draggingId: number | null;
  dragOverId: number | null;
  dragStart: (id: number) => (event: React.DragEvent) => void;
  dragOver: (targetId: number) => (event: React.DragEvent) => void;
  drop: (targetId: number) => (event: React.DragEvent) => Promise<void>;
  dragEnd: () => void;
} => {
  const { t } = useI18n();
  const pushBanner = useTopBanner();
  const apiError = useApiErrorHandler();

  const [draggingId, setDraggingId] = React.useState<number | null>(null);
  const [dragOverId, setDragOverId] = React.useState<number | null>(null);

  type DragSession = {
    sourceId: number;
    allIds: number[];
    visibleIds: number[];
    didChange: boolean;
  };

  const sessionRef = React.useRef<DragSession | null>(null);

  const resetDragState = React.useCallback(() => {
    sessionRef.current = null;
    setDraggingId(null);
    setDragOverId(null);
  }, []);

  const reorderWithinVisible = React.useCallback(
    (visibleIds: number[], sourceId: number, targetId: number): number[] => {
      const sourceIdx = visibleIds.indexOf(sourceId);
      const targetIdx = visibleIds.indexOf(targetId);
      if (sourceIdx === -1 || targetIdx === -1 || sourceIdx === targetIdx) return visibleIds;
      const next = visibleIds.slice();
      const [moved] = next.splice(sourceIdx, 1);
      next.splice(targetIdx, 0, moved);
      return next;
    },
    [],
  );

  const mergeVisibleBackIntoAll = React.useCallback(
    (allIds: number[], previousVisibleIds: number[], nextVisibleIds: number[]): number[] => {
      if (previousVisibleIds.length === 0) return allIds;
      const visibleSet = new Set(previousVisibleIds);
      const queue = nextVisibleIds.slice();
      return allIds.map((id) => (visibleSet.has(id) ? (queue.shift() ?? id) : id));
    },
    [],
  );

  const reorderNodesByIds = React.useCallback((prev: NodeRow[], orderedIds: number[]) => {
    const lookup = new Map(prev.map((node) => [node.id, node]));
    const next: NodeRow[] = [];
    for (const id of orderedIds) {
      const item = lookup.get(id);
      if (item) next.push(item);
    }
    if (next.length !== prev.length) {
      const seen = new Set(orderedIds);
      for (const item of prev) {
        if (!seen.has(item.id)) next.push(item);
      }
    }
    return next;
  }, []);

  const persistReorder = React.useCallback(
    async (orderedIds: number[]): Promise<boolean> => {
      if (!token) return false;
      try {
        await updateNodesDisplayOrder(orderedIds);
        return true;
      } catch (error) {
        apiError(error, t('admin_reorder_sync_failed'));
        return false;
      }
    },
    [apiError, t, token],
  );

  React.useEffect(() => {
    const clearGlobalDrag = () => {
      resetDragState();
    };
    document.addEventListener('dragend', clearGlobalDrag);
    document.addEventListener('drop', clearGlobalDrag);
    return () => {
      document.removeEventListener('dragend', clearGlobalDrag);
      document.removeEventListener('drop', clearGlobalDrag);
    };
  }, [resetDragState]);

  const dragStart = React.useCallback(
    (id: number) => (event: React.DragEvent) => {
      setDraggingId(id);
      setDragOverId(id);
      sessionRef.current = {
        sourceId: id,
        allIds: nodes.map((node) => node.id),
        visibleIds: filteredNodeIds.slice(),
        didChange: false,
      };
      event.dataTransfer.effectAllowed = 'move';
      event.dataTransfer.setData('text/plain', String(id));
    },
    [filteredNodeIds, nodes],
  );

  const dragOver = React.useCallback(
    (targetId: number) => (event: React.DragEvent) => {
      event.preventDefault();
      const session = sessionRef.current;
      if (!session) return;
      if (session.sourceId === targetId) return;

      const nextVisibleIds = reorderWithinVisible(session.visibleIds, session.sourceId, targetId);
      if (nextVisibleIds === session.visibleIds) return;

      const nextAllIds = mergeVisibleBackIntoAll(
        session.allIds,
        session.visibleIds,
        nextVisibleIds,
      );
      sessionRef.current = {
        ...session,
        allIds: nextAllIds,
        visibleIds: nextVisibleIds,
        didChange: true,
      };
      setNodes((prev) => reorderNodesByIds(prev, nextAllIds));
    },
    [mergeVisibleBackIntoAll, reorderNodesByIds, reorderWithinVisible, setNodes],
  );

  const drop = React.useCallback(
    (targetId: number) => async (event: React.DragEvent) => {
      event.preventDefault();
      const session = sessionRef.current;
      resetDragState();
      if (!session) return;

      const shouldPersist = session.didChange || session.sourceId !== targetId;
      if (!shouldPersist) {
        return;
      }
      const finalIds = session.allIds;

      setNodes((prev) => {
        const next = reorderNodesByIds(prev, finalIds);
        return next.map((node, index, arr) => ({
          ...node,
          displayOrder: arr.length - index,
        }));
      });

      const success = await persistReorder(finalIds);
      if (success) {
        pushBanner(t('admin_node_order_updated'), { tone: 'info' });
      } else {
        refreshNodes();
      }
    },
    [persistReorder, pushBanner, refreshNodes, reorderNodesByIds, resetDragState, setNodes, t],
  );

  const dragEnd = React.useCallback(() => {
    resetDragState();
  }, [resetDragState]);

  return { draggingId, dragOverId, dragStart, dragOver, drop, dragEnd };
};
