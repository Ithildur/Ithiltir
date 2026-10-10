import { create } from 'zustand';
import type { Group } from '@app-types/api';
import { createGroup, deleteGroup, fetchGroupList, updateGroup } from '@lib/adminApi';
import { createSeqGate, reloadLatestLoad, runLatestLoad } from '@utils/seqGate';

type AdminGroupInput = { name: string; remark?: string };

interface AdminGroupsState {
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

export const resetAdminGroupsStore = (): void => {
  loadGate.invalidate();
  useAdminGroupsStore.setState(initialAdminGroupsState);
};

const loadGate = createSeqGate();

const replaceAdminGroups = (groups: Group[]): void => {
  useAdminGroupsStore.setState({ groups });
};

const setAdminGroupsLoading = (loading: boolean): void => {
  useAdminGroupsStore.setState({ loading });
};

const reloadAdminGroups = async (): Promise<void> => {
  await reloadLatestLoad(loadGate, fetchGroupList, replaceAdminGroups, setAdminGroupsLoading);
};

export const loadAdminGroups = async (params: { signal?: AbortSignal } = {}): Promise<Group[]> => {
  return runLatestLoad(
    loadGate,
    () => fetchGroupList(params),
    replaceAdminGroups,
    setAdminGroupsLoading,
  );
};

export const saveAdminGroup = async (
  id: number | null,
  input: AdminGroupInput,
): Promise<boolean> => {
  if (useAdminGroupsStore.getState().saving) return false;
  useAdminGroupsStore.setState({ saving: true });
  try {
    if (id === null) {
      await createGroup(input);
    } else {
      await updateGroup(id, input);
    }
    await reloadAdminGroups();
    return true;
  } finally {
    useAdminGroupsStore.setState({ saving: false });
  }
};

export const removeAdminGroup = async (id: number): Promise<boolean> => {
  if (useAdminGroupsStore.getState().deleting) return false;
  useAdminGroupsStore.setState({ deleting: true });
  try {
    await deleteGroup(id);
    await reloadAdminGroups();
    return true;
  } finally {
    useAdminGroupsStore.setState({ deleting: false });
  }
};
