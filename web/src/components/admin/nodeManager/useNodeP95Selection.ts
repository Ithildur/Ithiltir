import React from 'react';
import type { NodeRow } from '@app-types/admin';
import { useIdSelection } from '@hooks/useIdSelection';

export const useNodeP95Selection = ({
  nodes,
  filteredNodeIds,
  savingP95NodeIds,
  isLoading,
}: {
  nodes: NodeRow[];
  filteredNodeIds: number[];
  savingP95NodeIds: number[];
  isLoading: boolean;
}) => {
  const {
    selectedIds: selectedP95NodeIds,
    selectedIdSet: selectedP95NodeIdSet,
    toggle: toggleP95NodeSelection,
    toggleVisible: toggleVisibleP95Nodes,
    prune: pruneSelectedP95Nodes,
    clear: clearP95Selection,
  } = useIdSelection();

  React.useEffect(() => {
    if (selectedP95NodeIds.length === 0) return;
    pruneSelectedP95Nodes(nodes.map((node) => node.id));
  }, [nodes, pruneSelectedP95Nodes, selectedP95NodeIds.length]);

  const savingP95NodeIdSet = React.useMemo(() => new Set(savingP95NodeIds), [savingP95NodeIds]);
  const allVisibleP95Selected =
    filteredNodeIds.length > 0 && filteredNodeIds.every((id) => selectedP95NodeIdSet.has(id));
  const someVisibleP95Selected = filteredNodeIds.some((id) => selectedP95NodeIdSet.has(id));
  const canBatchP95 =
    selectedP95NodeIds.length > 0 &&
    !isLoading &&
    !selectedP95NodeIds.some((id) => savingP95NodeIdSet.has(id));

  return {
    selectedP95Ids: selectedP95NodeIds,
    selectedP95NodeIdSet,
    savingP95NodeIdSet,
    allVisibleP95Selected,
    someVisibleP95Selected,
    canBatchP95,
    toggleP95NodeSelection,
    toggleVisibleP95Nodes: () => toggleVisibleP95Nodes(filteredNodeIds),
    clearP95Selection,
  };
};
