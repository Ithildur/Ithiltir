import React from 'react';
import type { AlertMountNode, AlertMountRule } from '@app-types/admin';
import { useIdSelection } from '@hooks/useIdSelection';

export type AlertMountsBatchMode = 'mount' | 'unmount';

interface UseAlertMountsPanelParams {
  rules: AlertMountRule[];
  nodes: AlertMountNode[];
  saving: boolean;
  onSetMounts: (ruleIds: number[], serverIds: number[], mounted: boolean) => Promise<boolean>;
}

const useMountSession = (rules: AlertMountRule[], nodes: AlertMountNode[]) => {
  const [customNodeId, setCustomNodeId] = React.useState<number | null>(null);
  const [batchMode, setBatchMode] = React.useState<AlertMountsBatchMode | null>(null);
  const {
    selectedIds: batchRuleIds,
    selectedIdSet: batchRules,
    setSelectedIds: setBatchRuleIds,
    toggle: toggleBatchRule,
    prune: pruneBatchRules,
    clear: clearBatchRules,
  } = useIdSelection();

  const customNode = React.useMemo(
    () => nodes.find((node) => node.id === customNodeId) ?? null,
    [customNodeId, nodes],
  );

  React.useEffect(() => {
    if (customNodeId === null) return;
    if (!nodes.some((node) => node.id === customNodeId)) setCustomNodeId(null);
  }, [customNodeId, nodes]);

  React.useEffect(() => {
    if (batchRuleIds.length === 0) return;
    pruneBatchRules(rules.map((rule) => rule.id));
  }, [batchRuleIds.length, pruneBatchRules, rules]);

  const resetBatch = React.useCallback(() => {
    setBatchMode(null);
    clearBatchRules();
  }, [clearBatchRules]);

  const openBatch = React.useCallback(
    (mode: AlertMountsBatchMode) => {
      setBatchMode(mode);
      clearBatchRules();
    },
    [clearBatchRules],
  );

  const closeCustom = React.useCallback(() => {
    setCustomNodeId(null);
  }, []);

  return {
    customNode,
    setCustomNodeId,
    closeCustom,
    batchMode,
    batchRuleIds,
    batchRules,
    setBatchRuleIds,
    toggleBatchRule,
    resetBatch,
    openBatch,
  };
};

export const useAlertMountsPanel = ({
  rules,
  nodes,
  saving,
  onSetMounts,
}: UseAlertMountsPanelParams) => {
  const [search, setSearch] = React.useState('');
  const {
    selectedIds: selectedNodeIds,
    selectedIdSet: selectedNodes,
    toggle: toggleSelectedNode,
    toggleVisible: toggleVisibleMountNodes,
    prune: pruneMountSelectedNodes,
  } = useIdSelection();
  const {
    selectedIds: filteredRuleIds,
    selectedIdSet: filteredRules,
    setSelectedIds: setFilteredRuleIds,
    prune: pruneMountFilteredRules,
  } = useIdSelection();
  const {
    customNode,
    setCustomNodeId,
    closeCustom,
    batchMode,
    batchRuleIds,
    batchRules,
    setBatchRuleIds,
    toggleBatchRule,
    resetBatch,
    openBatch,
  } = useMountSession(rules, nodes);

  const mountMap = React.useMemo(() => {
    const byNode = new Map<number, Map<number, boolean>>();
    for (const node of nodes) {
      const row = new Map<number, boolean>();
      for (const mount of node.mounts) {
        row.set(mount.rule_id, mount.mounted);
      }
      byNode.set(node.id, row);
    }
    return byNode;
  }, [nodes]);

  const builtinRules = React.useMemo(() => rules.filter((rule) => rule.builtin), [rules]);
  const customRules = React.useMemo(() => rules.filter((rule) => !rule.builtin), [rules]);
  const allRuleIds = React.useMemo(() => rules.map((rule) => rule.id), [rules]);
  const batchList = React.useMemo(
    () => [...builtinRules, ...customRules],
    [builtinRules, customRules],
  );
  const batchListIds = React.useMemo(() => batchList.map((rule) => rule.id), [batchList]);
  const hasRuleFilter = filteredRules.size > 0;
  const visibleBuiltinRules = React.useMemo(
    () => builtinRules.filter((rule) => !hasRuleFilter || filteredRules.has(rule.id)),
    [builtinRules, filteredRules, hasRuleFilter],
  );
  const visibleCustomRules = React.useMemo(
    () => customRules.filter((rule) => !hasRuleFilter || filteredRules.has(rule.id)),
    [customRules, filteredRules, hasRuleFilter],
  );
  const showCustomRules = !hasRuleFilter || visibleCustomRules.length > 0;

  const visibleNodes = React.useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return nodes;
    return nodes.filter((node) =>
      [node.name, node.hostname, node.ip ?? '', String(node.id)]
        .join(' ')
        .toLowerCase()
        .includes(keyword),
    );
  }, [nodes, search]);

  const visibleNodeIds = React.useMemo(() => visibleNodes.map((node) => node.id), [visibleNodes]);
  const allVisibleSelected =
    visibleNodeIds.length > 0 && visibleNodeIds.every((id) => selectedNodes.has(id));
  const someVisibleSelected = visibleNodeIds.some((id) => selectedNodes.has(id));

  React.useEffect(() => {
    if (selectedNodeIds.length === 0) return;
    pruneMountSelectedNodes(nodes.map((node) => node.id));
  }, [nodes, pruneMountSelectedNodes, selectedNodeIds.length]);

  React.useEffect(() => {
    if (filteredRuleIds.length === 0) return;
    pruneMountFilteredRules(rules.map((rule) => rule.id));
  }, [filteredRuleIds.length, pruneMountFilteredRules, rules]);

  const toggleVisibleNodes = React.useCallback(() => {
    toggleVisibleMountNodes(visibleNodeIds);
  }, [toggleVisibleMountNodes, visibleNodeIds]);

  const mounted = React.useCallback(
    (nodeId: number, ruleId: number) => mountMap.get(nodeId)?.get(ruleId) ?? false,
    [mountMap],
  );

  const customMountedCount = React.useCallback(
    (nodeId: number) =>
      visibleCustomRules.filter((rule) => rule.enabled && mounted(nodeId, rule.id)).length,
    [mounted, visibleCustomRules],
  );

  const tableColumnCount = visibleBuiltinRules.length + (showCustomRules ? 1 : 0) + 3;
  const allBatchSelected =
    batchListIds.length > 0 && batchListIds.every((id) => batchRules.has(id));
  const someBatchSelected = batchListIds.some((id) => batchRules.has(id));
  const canOpenBatch = selectedNodeIds.length > 0 && batchListIds.length > 0 && !saving;
  const canSubmitBatch =
    batchMode !== null && selectedNodeIds.length > 0 && batchRuleIds.length > 0 && !saving;

  const toggleAllBatchRules = React.useCallback(() => {
    setBatchRuleIds(allBatchSelected ? [] : batchListIds);
  }, [allBatchSelected, batchListIds, setBatchRuleIds]);

  const startBatch = React.useCallback(
    (mode: AlertMountsBatchMode) => {
      if (!canOpenBatch) return;
      openBatch(mode);
    },
    [canOpenBatch, openBatch],
  );

  const submitBatch = React.useCallback(async () => {
    if (!batchMode || !canSubmitBatch) return;
    const saved = await onSetMounts(batchRuleIds, selectedNodeIds, batchMode === 'mount');
    if (saved) {
      resetBatch();
    }
  }, [batchMode, batchRuleIds, canSubmitBatch, onSetMounts, resetBatch, selectedNodeIds]);

  return {
    filter: {
      search,
      setSearch,
      selectedRuleIds: filteredRuleIds,
      setSelectedRuleIds: setFilteredRuleIds,
    },
    table: {
      nodes: visibleNodes,
      visibleBuiltinRules,
      showCustomRules,
      allRuleIds,
      selectedNodeIds,
      selectedNodes,
      allVisibleSelected,
      someVisibleSelected,
      tableColumnCount,
      canOpenBatch,
      toggleVisibleNodes,
      toggleSelectedNode,
      openCustomNode: setCustomNodeId,
      startBatch,
      mounted,
      customMountedCount,
    },
    batch: {
      isOpen: batchMode !== null,
      mode: batchMode,
      rules: batchList,
      selectedNodeCount: selectedNodeIds.length,
      selectedRuleCount: batchRuleIds.length,
      selectedRules: batchRules,
      allSelected: allBatchSelected,
      someSelected: someBatchSelected,
      canSubmit: canSubmitBatch,
      close: resetBatch,
      submit: submitBatch,
      toggleAllRules: toggleAllBatchRules,
      toggleRule: toggleBatchRule,
    },
    custom: {
      node: customNode,
      rules: visibleCustomRules,
      close: closeCustom,
    },
  };
};
