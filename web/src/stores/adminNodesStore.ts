import { create } from 'zustand';
import type { NodeDeploy, NodeTrafficPatch } from '@app-types/api';
import type { NodeRow } from '@app-types/admin';
import {
  createNode,
  deleteNode,
  fetchNodeDeploy,
  fetchNodes,
  requestNodeUpgrade,
  updateNode,
  updateNodesDisplayOrder,
  updateNodesTrafficP95,
} from '@lib/adminApi';
import { buildGroupLookup, nodeRowsFromManaged } from '@lib/adminNodeModel';
import { fetchAppVersion } from '@lib/versionApi';
import { pendingIds } from '@utils/pendingIds';
import { createSeqGate, runLatestLoad } from '@utils/seqGate';
import { getAdminGroups, loadAdminGroups } from './adminGroupsStore';

type NodeRowsUpdate = NodeRow[] | ((current: NodeRow[]) => NodeRow[]);

interface AdminNodesState {
  nodes: NodeRow[];
  deploy: NodeDeploy | null;
  bundledNodeVersion: string;
  loading: boolean;
  creating: boolean;
  savingGuestVisibleNodeIds: number[];
  savingP95NodeIds: number[];
  savingTrafficSettingsNodeIds: number[];
  savingSettingsNodeIds: number[];
  upgradingNodeIds: number[];
}

const initialAdminNodesState = {
  nodes: [],
  deploy: null,
  bundledNodeVersion: '',
  loading: false,
  creating: false,
  savingGuestVisibleNodeIds: [],
  savingP95NodeIds: [],
  savingTrafficSettingsNodeIds: [],
  savingSettingsNodeIds: [],
  upgradingNodeIds: [],
};

export const useAdminNodesStore = create<AdminNodesState>()(() => initialAdminNodesState);

const getAdminNodesState = (): AdminNodesState => useAdminNodesStore.getState();

export const resetAdminNodesStore = (): void => {
  nodesGate.invalidate();
  overviewGate.invalidate();
  deployGate.invalidate();
  versionGate.invalidate();
  useAdminNodesStore.setState(initialAdminNodesState);
};

const nodesGate = createSeqGate();
const overviewGate = createSeqGate();
const deployGate = createSeqGate();
const versionGate = createSeqGate();

const fetchAdminNodeRows = async (
  groupLookup: Record<number, string> = buildGroupLookup(getAdminGroups()),
  params: { signal?: AbortSignal } = {},
): Promise<NodeRow[]> => nodeRowsFromManaged(await fetchNodes(params), groupLookup);

const replaceAdminNodes = (nodes: NodeRow[]): void => {
  useAdminNodesStore.setState({ nodes });
};

const setAdminNodes = (next: NodeRowsUpdate): void => {
  useAdminNodesStore.setState((state) => ({
    nodes: typeof next === 'function' ? next(state.nodes) : next,
  }));
};

const setAdminNodesLoading = (loading: boolean): void => {
  useAdminNodesStore.setState({ loading });
};

const invalidateAdminNodeRowsLoad = (): void => {
  nodesGate.invalidate();
};

const nodesInOrder = (nodes: NodeRow[], orderedIds: readonly number[]): NodeRow[] => {
  const lookup = new Map(nodes.map((node) => [node.id, node]));
  const next: NodeRow[] = [];

  for (const id of orderedIds) {
    const node = lookup.get(id);
    if (node) next.push(node);
  }

  if (next.length === nodes.length) return next;

  const seen = new Set(orderedIds);
  for (const node of nodes) {
    if (!seen.has(node.id)) next.push(node);
  }
  return next;
};

const syncAdminNodes = async (): Promise<void> => {
  const seq = nodesGate.next();
  const nodes = await fetchAdminNodeRows();
  if (!nodesGate.isCurrent(seq)) return;
  replaceAdminNodes(nodes);
};

export const loadAdminNodeOverview = async (
  params: { signal?: AbortSignal } = {},
): Promise<NodeRow[]> => {
  const overviewLoad = overviewGate.next();
  const nodesLoad = nodesGate.next();
  setAdminNodesLoading(true);
  try {
    const [groups, nodeRes] = await Promise.all([loadAdminGroups(params), fetchNodes(params)]);
    const groupLookup = buildGroupLookup(groups);
    const nodes = nodeRowsFromManaged(nodeRes, groupLookup);
    if (overviewGate.isCurrent(overviewLoad) && nodesGate.isCurrent(nodesLoad)) {
      replaceAdminNodes(nodes);
    }
    return nodes;
  } finally {
    if (overviewGate.isCurrent(overviewLoad)) {
      setAdminNodesLoading(false);
    }
  }
};

export const refreshAdminNodes = async (
  groupLookup: Record<number, string> = buildGroupLookup(getAdminGroups()),
  params: { signal?: AbortSignal } = {},
): Promise<NodeRow[]> => {
  return runLatestLoad(nodesGate, () => fetchAdminNodeRows(groupLookup, params), replaceAdminNodes);
};

export const loadAdminNodeDeploy = async (
  params: { signal?: AbortSignal } = {},
): Promise<NodeDeploy> => {
  return runLatestLoad(
    deployGate,
    () => fetchNodeDeploy(params),
    (deploy) => {
      useAdminNodesStore.setState({ deploy });
    },
  );
};

export const clearAdminNodeDeploy = (): void => {
  deployGate.invalidate();
  useAdminNodesStore.setState({ deploy: null });
};

export const loadBundledNodeVersion = async (
  params: { signal?: AbortSignal } = {},
): Promise<string> => {
  return runLatestLoad(
    versionGate,
    async () => (await fetchAppVersion(params)).node_version?.trim() ?? '',
    (bundledNodeVersion) => {
      useAdminNodesStore.setState({ bundledNodeVersion });
    },
  );
};

export const clearBundledNodeVersion = (): void => {
  versionGate.invalidate();
  useAdminNodesStore.setState({ bundledNodeVersion: '' });
};

const getNode = (id: number): NodeRow | null =>
  getAdminNodesState().nodes.find((node) => node.id === id) ?? null;

const patchNode = (id: number, patch: Partial<NodeRow>): void => {
  setAdminNodes((nodes) => nodes.map((node) => (node.id === id ? { ...node, ...patch } : node)));
};

const setNodeSavingP95 = (id: number, saving: boolean): void => {
  useAdminNodesStore.setState((state) => ({
    savingP95NodeIds: pendingIds(state.savingP95NodeIds, id, saving),
  }));
};

const setNodeSavingGuestVisible = (id: number, saving: boolean): void => {
  useAdminNodesStore.setState((state) => ({
    savingGuestVisibleNodeIds: pendingIds(state.savingGuestVisibleNodeIds, id, saving),
  }));
};

const setNodeSavingTrafficSettings = (id: number, saving: boolean): void => {
  useAdminNodesStore.setState((state) => ({
    savingTrafficSettingsNodeIds: pendingIds(state.savingTrafficSettingsNodeIds, id, saving),
  }));
};

const setNodeSavingSettings = (id: number, saving: boolean): void => {
  useAdminNodesStore.setState((state) => ({
    savingSettingsNodeIds: pendingIds(state.savingSettingsNodeIds, id, saving),
  }));
};

const setNodeUpgrading = (id: number, upgrading: boolean): void => {
  useAdminNodesStore.setState((state) => ({
    upgradingNodeIds: pendingIds(state.upgradingNodeIds, id, upgrading),
  }));
};

const isSavingGuestVisible = (id: number): boolean =>
  getAdminNodesState().savingGuestVisibleNodeIds.includes(id);
const isSavingP95 = (id: number): boolean => getAdminNodesState().savingP95NodeIds.includes(id);
const isSavingTrafficSettings = (id: number): boolean =>
  getAdminNodesState().savingTrafficSettingsNodeIds.includes(id);
const isSavingSettings = (id: number): boolean =>
  getAdminNodesState().savingSettingsNodeIds.includes(id);
const isUpgradingNode = (id: number): boolean => getAdminNodesState().upgradingNodeIds.includes(id);

export type AdminNodeSettingsInput = {
  name: string;
  secret: string;
  guestVisible: boolean;
  groupIds: number[];
  tags?: string[];
};

export const addAdminNode = async (): Promise<boolean> => {
  if (getAdminNodesState().creating) return false;
  useAdminNodesStore.setState({ creating: true });
  try {
    await createNode();
    await syncAdminNodes();
    return true;
  } finally {
    useAdminNodesStore.setState({ creating: false });
  }
};

export const renameAdminNode = async (id: number, name: string): Promise<boolean> => {
  await updateNode(id, { name });
  invalidateAdminNodeRowsLoad();
  patchNode(id, { name });
  return true;
};

export const setAdminNodeGuestVisible = async (
  id: number,
  guestVisible: boolean,
): Promise<boolean> => {
  if (isSavingGuestVisible(id)) return false;

  setNodeSavingGuestVisible(id, true);
  try {
    await updateNode(id, { is_guest_visible: guestVisible });
    invalidateAdminNodeRowsLoad();
    patchNode(id, { guestVisible });
    return true;
  } finally {
    setNodeSavingGuestVisible(id, false);
  }
};

export const setAdminNodeTrafficP95 = async (id: number, enabled: boolean): Promise<boolean> => {
  const node = getNode(id);
  if (!node || isSavingP95(id)) return false;

  setNodeSavingP95(id, true);
  invalidateAdminNodeRowsLoad();
  patchNode(id, { trafficP95Enabled: enabled });
  try {
    await updateNode(id, { traffic_p95_enabled: enabled });
    invalidateAdminNodeRowsLoad();
    patchNode(id, { trafficP95Enabled: enabled });
    return true;
  } catch (error) {
    invalidateAdminNodeRowsLoad();
    patchNode(id, { trafficP95Enabled: node.trafficP95Enabled });
    throw error;
  } finally {
    setNodeSavingP95(id, false);
  }
};

export const setAdminNodesTrafficP95 = async (
  ids: number[],
  enabled: boolean,
): Promise<boolean> => {
  const targetIds = [...new Set(ids)];
  if (targetIds.length === 0 || targetIds.some(isSavingP95)) return false;

  const idSet = new Set(targetIds);
  const previous = new Map(
    getAdminNodesState()
      .nodes.filter((node) => idSet.has(node.id))
      .map((node) => [node.id, node.trafficP95Enabled]),
  );
  const appliedIds = [...previous.keys()];
  if (appliedIds.length === 0) return false;
  for (const id of appliedIds) setNodeSavingP95(id, true);
  invalidateAdminNodeRowsLoad();
  setAdminNodes((nodes) =>
    nodes.map((node) => (previous.has(node.id) ? { ...node, trafficP95Enabled: enabled } : node)),
  );
  try {
    await updateNodesTrafficP95(appliedIds, enabled);
    invalidateAdminNodeRowsLoad();
    setAdminNodes((nodes) =>
      nodes.map((node) => (previous.has(node.id) ? { ...node, trafficP95Enabled: enabled } : node)),
    );
    return true;
  } catch (error) {
    invalidateAdminNodeRowsLoad();
    setAdminNodes((nodes) =>
      nodes.map((node) =>
        previous.has(node.id)
          ? { ...node, trafficP95Enabled: previous.get(node.id) ?? node.trafficP95Enabled }
          : node,
      ),
    );
    throw error;
  } finally {
    for (const id of appliedIds) setNodeSavingP95(id, false);
  }
};

export const saveAdminNodeTrafficSettings = async (
  id: number,
  patch: NodeTrafficPatch,
): Promise<boolean> => {
  if (isSavingTrafficSettings(id)) return false;

  setNodeSavingTrafficSettings(id, true);
  try {
    await updateNode(id, patch);
    await syncAdminNodes();
    return true;
  } finally {
    setNodeSavingTrafficSettings(id, false);
  }
};

export const removeAdminNode = async (id: number): Promise<boolean> => {
  await deleteNode(id);
  invalidateAdminNodeRowsLoad();
  setAdminNodes((nodes) => nodes.filter((node) => node.id !== id));
  return true;
};

export const requestAdminNodeUpgrade = async (id: number): Promise<boolean> => {
  if (isUpgradingNode(id)) return false;

  setNodeUpgrading(id, true);
  try {
    await requestNodeUpgrade(id);
    await syncAdminNodes();
    return true;
  } finally {
    setNodeUpgrading(id, false);
  }
};

export const saveAdminNodeSettings = async (
  id: number,
  input: AdminNodeSettingsInput,
): Promise<boolean> => {
  if (isSavingSettings(id)) return false;

  setNodeSavingSettings(id, true);
  try {
    await updateNode(id, {
      name: input.name,
      secret: input.secret,
      is_guest_visible: input.guestVisible,
      group_ids: input.groupIds,
      ...(input.tags !== undefined ? { tags: input.tags } : {}),
    });
    await syncAdminNodes();
    return true;
  } finally {
    setNodeSavingSettings(id, false);
  }
};

export const saveAdminNodeOrder = async (ids: number[]): Promise<void> => {
  await updateNodesDisplayOrder(ids);
};

export const previewAdminNodeOrder = (ids: readonly number[]): void => {
  invalidateAdminNodeRowsLoad();
  setAdminNodes((nodes) => nodesInOrder(nodes, ids));
};

export const commitAdminNodeOrder = (ids: readonly number[]): void => {
  invalidateAdminNodeRowsLoad();
  setAdminNodes((nodes) =>
    nodesInOrder(nodes, ids).map((node, index, ordered) => ({
      ...node,
      displayOrder: ordered.length - index,
    })),
  );
};

export const rollbackAdminNodeOrder = (
  ids: readonly number[],
  displayOrders: ReadonlyMap<number, number>,
): void => {
  invalidateAdminNodeRowsLoad();
  setAdminNodes((nodes) =>
    nodesInOrder(nodes, ids).map((node) => ({
      ...node,
      displayOrder: displayOrders.get(node.id) ?? node.displayOrder,
    })),
  );
};
