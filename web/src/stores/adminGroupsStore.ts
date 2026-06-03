import { create } from 'zustand';
import type { Group } from '@app-types/api';
import { createGroup, deleteGroup, fetchGroupList, updateGroup } from '@lib/adminApi';
import { actionBusy, type ActionOutcome } from '@utils/actionOutcome';
import { createSeqGate, reloadLatestLoad, runLatestLoad } from '@utils/seqGate';

export type AdminGroupInput = { name: string; remark?: string };

export interface AdminGroupsState {
  groups: Group[];
  loading: boolean;
  saving: boolean;
  deleting: boolean;
}

const initialAdminGroupsState = {
  groups: [],
  loading: false,
  saving: false,
  deleting: false,
};

export const useAdminGroupsStore = create<AdminGroupsState>()(() => initialAdminGroupsState);

const getAdminGroupsState = (): AdminGroupsState => useAdminGroupsStore.getState();

export const getAdminGroups = (): Group[] => getAdminGroupsState().groups;

export const resetAdminGroupsStore = (): void => {
  loadGate.invalidate();
  useAdminGroupsStore.setState(initialAdminGroupsState);
};

const loadGate = createSeqGate();

const fetchAdminGroups = (params: { signal?: AbortSignal } = {}): Promise<Group[]> =>
  fetchGroupList(params);

const replaceAdminGroups = (groups: Group[]): void => {
  useAdminGroupsStore.setState({ groups });
};

const setAdminGroupsLoading = (loading: boolean): void => {
  useAdminGroupsStore.setState({ loading });
};

const reloadAdminGroups = async (): Promise<ActionOutcome<Group[]>> => {
  return reloadLatestLoad(loadGate, fetchAdminGroups, replaceAdminGroups, setAdminGroupsLoading);
};

export const loadAdminGroups = async (params: { signal?: AbortSignal } = {}): Promise<Group[]> => {
  return runLatestLoad(
    loadGate,
    () => fetchAdminGroups(params),
    replaceAdminGroups,
    setAdminGroupsLoading,
  );
};

export const saveAdminGroup = async (
  id: number | null,
  input: AdminGroupInput,
): Promise<ActionOutcome<Group[]>> => {
  if (getAdminGroupsState().saving) return actionBusy;
  useAdminGroupsStore.setState({ saving: true });
  try {
    if (id === null) {
      await createGroup(input);
    } else {
      await updateGroup(id, input);
    }
    return await reloadAdminGroups();
  } finally {
    useAdminGroupsStore.setState({ saving: false });
  }
};

export const removeAdminGroup = async (id: number): Promise<ActionOutcome<Group[]>> => {
  if (getAdminGroupsState().deleting) return actionBusy;
  useAdminGroupsStore.setState({ deleting: true });
  try {
    await deleteGroup(id);
    return await reloadAdminGroups();
  } finally {
    useAdminGroupsStore.setState({ deleting: false });
  }
};
