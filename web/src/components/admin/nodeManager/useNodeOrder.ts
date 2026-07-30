import React from 'react';
import { useI18n } from '@i18n';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import type { NodeRow } from '@app-types/admin';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import {
  commitAdminNodeOrder,
  previewAdminNodeOrder,
  rollbackAdminNodeOrder,
  saveAdminNodeOrder,
} from '@stores/adminNodesStore';

export const useNodeOrder = ({
  token,
  nodes,
  filteredNodeIds,
  refreshNodes,
}: {
  token: string | null;
  nodes: NodeRow[];
  filteredNodeIds: number[];
  refreshNodes: () => Promise<void>;
}): {
  draggingId: number | null;
  dragOverId: number | null;
  dragStart: (id: number) => (event: React.DragEvent) => void;
  dragOver: (targetId: number) => (event: React.DragEvent) => void;
  drop: (targetId: number) => (event: React.DragEvent) => Promise<void>;
  dragEnd: () => void;
  move: (id: number, offset: -1 | 1) => Promise<void>;
} => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();

  const [draggingId, setDraggingId] = React.useState<number | null>(null);
  const [dragOverId, setDragOverId] = React.useState<number | null>(null);

  type DragSession = {
    sourceId: number;
    originalIds: number[];
    originalVisibleIds: number[];
    originalDisplayOrders: Map<number, number>;
    allIds: number[];
    visibleIds: number[];
    didChange: boolean;
  };

  const sessionRef = React.useRef<DragSession | null>(null);
  const keyboardBusyRef = React.useRef(false);

  const clearDragState = React.useCallback(() => {
    sessionRef.current = null;
    setDraggingId(null);
    setDragOverId(null);
  }, []);

  const cancelDrag = React.useCallback(() => {
    const session = sessionRef.current;
    if (session?.didChange) {
      rollbackAdminNodeOrder(session.originalIds, session.originalDisplayOrders);
    }
    clearDragState();
  }, [clearDragState]);

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

  const idsEqual = React.useCallback((left: number[], right: number[]): boolean => {
    if (left.length !== right.length) return false;
    return left.every((id, index) => id === right[index]);
  }, []);

  const idsFromDropTarget = React.useCallback(
    (session: DragSession, targetId: number): number[] => {
      const nextVisibleIds = reorderWithinVisible(
        session.originalVisibleIds,
        session.sourceId,
        targetId,
      );
      if (nextVisibleIds === session.originalVisibleIds) return session.originalIds;
      return mergeVisibleBackIntoAll(
        session.originalIds,
        session.originalVisibleIds,
        nextVisibleIds,
      );
    },
    [mergeVisibleBackIntoAll, reorderWithinVisible],
  );

  const persistReorder = React.useCallback(
    async (orderedIds: number[]): Promise<boolean> => {
      if (!token) return false;
      try {
        await saveAdminNodeOrder(orderedIds);
        return true;
      } catch (error) {
        apiError(error, t('admin_reorder_sync_failed'));
        return false;
      }
    },
    [apiError, t, token],
  );

  const commitOrder = React.useCallback(
    async (
      orderedIds: number[],
      originalIds: number[],
      originalDisplayOrders: Map<number, number>,
    ) => {
      commitAdminNodeOrder(orderedIds);

      if (await persistReorder(orderedIds)) {
        pushTopBanner(t('admin_node_order_updated'), { tone: 'info' });
        return;
      }
      try {
        await refreshNodes();
      } catch (error) {
        apiError(error, t('admin_fetch_nodes_failed'));
        rollbackAdminNodeOrder(originalIds, originalDisplayOrders);
      }
    },
    [apiError, persistReorder, refreshNodes, t],
  );

  React.useEffect(() => {
    const clearGlobalDrag = () => {
      cancelDrag();
    };
    document.addEventListener('dragend', clearGlobalDrag);
    return () => {
      document.removeEventListener('dragend', clearGlobalDrag);
    };
  }, [cancelDrag]);

  const dragStart = React.useCallback(
    (id: number) => (event: React.DragEvent) => {
      setDraggingId(id);
      setDragOverId(null);
      sessionRef.current = {
        sourceId: id,
        originalIds: nodes.map((node) => node.id),
        originalVisibleIds: filteredNodeIds.slice(),
        originalDisplayOrders: new Map(nodes.map((node) => [node.id, node.displayOrder])),
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
      if (session.sourceId === targetId) {
        setDragOverId(null);
        return;
      }
      setDragOverId(targetId);

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
      previewAdminNodeOrder(nextAllIds);
    },
    [mergeVisibleBackIntoAll, reorderWithinVisible],
  );

  const drop = React.useCallback(
    (targetId: number) => async (event: React.DragEvent) => {
      event.preventDefault();
      const session = sessionRef.current;
      clearDragState();
      if (!session) return;

      const shouldPersist = session.didChange || session.sourceId !== targetId;
      if (!shouldPersist) {
        return;
      }
      const finalIds = session.didChange ? session.allIds : idsFromDropTarget(session, targetId);
      if (idsEqual(finalIds, session.originalIds)) {
        if (session.didChange) {
          rollbackAdminNodeOrder(session.originalIds, session.originalDisplayOrders);
        }
        return;
      }

      await commitOrder(finalIds, session.originalIds, session.originalDisplayOrders);
    },
    [clearDragState, commitOrder, idsEqual, idsFromDropTarget],
  );

  const dragEnd = React.useCallback(() => {
    cancelDrag();
  }, [cancelDrag]);

  const move = React.useCallback(
    async (id: number, offset: -1 | 1) => {
      if (keyboardBusyRef.current) return;
      const sourceIndex = filteredNodeIds.indexOf(id);
      const targetId = filteredNodeIds[sourceIndex + offset];
      if (sourceIndex < 0 || targetId === undefined) return;

      const originalIds = nodes.map((node) => node.id);
      const originalDisplayOrders = new Map(nodes.map((node) => [node.id, node.displayOrder]));
      const visibleIds = reorderWithinVisible(filteredNodeIds, id, targetId);
      const orderedIds = mergeVisibleBackIntoAll(originalIds, filteredNodeIds, visibleIds);
      if (idsEqual(orderedIds, originalIds)) return;

      keyboardBusyRef.current = true;
      try {
        await commitOrder(orderedIds, originalIds, originalDisplayOrders);
      } finally {
        keyboardBusyRef.current = false;
      }
    },
    [commitOrder, filteredNodeIds, idsEqual, mergeVisibleBackIntoAll, nodes, reorderWithinVisible],
  );

  return { draggingId, dragOverId, dragStart, dragOver, drop, dragEnd, move };
};
