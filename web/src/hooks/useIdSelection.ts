import React from 'react';
import { pruneIds, toggleId, toggleVisibleIds } from '@utils/idSelection';

export const useIdSelection = (initialIds: number[] = []) => {
  const [selectedIds, setSelectedIds] = React.useState<number[]>(initialIds);

  const selectedIdSet = React.useMemo(() => new Set(selectedIds), [selectedIds]);

  const toggle = React.useCallback((id: number) => {
    setSelectedIds((ids) => toggleId(ids, id));
  }, []);

  const toggleVisible = React.useCallback((visibleIds: number[]) => {
    setSelectedIds((ids) => toggleVisibleIds(ids, visibleIds));
  }, []);

  const prune = React.useCallback((validIds: Iterable<number>) => {
    setSelectedIds((ids) => pruneIds(ids, validIds));
  }, []);

  const clear = React.useCallback(() => {
    setSelectedIds([]);
  }, []);

  return {
    selectedIds,
    selectedIdSet,
    setSelectedIds,
    toggle,
    toggleVisible,
    prune,
    clear,
  };
};
