import { create } from 'zustand';
import type { AlertRule } from '@app-types/admin';
import * as adminApi from '@lib/adminApi';
import { pendingIds } from '@utils/pendingIds';
import { actionBusy, type ActionOutcome } from '@utils/actionOutcome';
import { createSeqGate, reloadLatestLoad, runLatestLoad } from '@utils/seqGate';

export interface AlertRulesState {
  rules: AlertRule[];
  loading: boolean;
  saving: boolean;
  togglingIds: number[];
  renamingIds: number[];
}

const initialAlertRulesState = {
  rules: [],
  loading: false,
  saving: false,
  togglingIds: [],
  renamingIds: [],
};

const sortRules = (items: AlertRule[]): AlertRule[] => items.slice().sort((a, b) => a.id - b.id);

export const useAlertRulesStore = create<AlertRulesState>()(() => initialAlertRulesState);

const getAlertRulesState = (): AlertRulesState => useAlertRulesStore.getState();

export const resetAlertRulesStore = (): void => {
  loadGate.invalidate();
  useAlertRulesStore.setState(initialAlertRulesState);
};

const loadGate = createSeqGate();

const fetchAlertRules = async (params: { signal?: AbortSignal } = {}): Promise<AlertRule[]> =>
  sortRules(await adminApi.fetchAlertRules(params));

const replaceAlertRules = (rules: AlertRule[]): void => {
  useAlertRulesStore.setState({ rules });
};

const setAlertRulesLoading = (loading: boolean): void => {
  useAlertRulesStore.setState({ loading });
};

const reloadAlertRules = async (): Promise<ActionOutcome<AlertRule[]>> => {
  return reloadLatestLoad(loadGate, fetchAlertRules, replaceAlertRules, setAlertRulesLoading);
};

export const loadAlertRules = async (
  params: { signal?: AbortSignal } = {},
): Promise<AlertRule[]> => {
  return runLatestLoad(
    loadGate,
    () => fetchAlertRules(params),
    replaceAlertRules,
    setAlertRulesLoading,
  );
};

const setAlertRuleToggling = (id: number, toggling: boolean): void => {
  useAlertRulesStore.setState((state) => ({
    togglingIds: pendingIds(state.togglingIds, id, toggling),
  }));
};

const setAlertRuleRenaming = (id: number, renaming: boolean): void => {
  useAlertRulesStore.setState((state) => ({
    renamingIds: pendingIds(state.renamingIds, id, renaming),
  }));
};

export const toggleAlertRuleEnabled = async (
  id: number,
  enabled: boolean,
): Promise<ActionOutcome<AlertRule[]>> => {
  if (getAlertRulesState().togglingIds.includes(id)) return actionBusy;
  setAlertRuleToggling(id, true);
  try {
    await adminApi.updateAlertRule(id, { enabled });
    return await reloadAlertRules();
  } finally {
    setAlertRuleToggling(id, false);
  }
};

export const renameAlertRule = async (
  id: number,
  name: string,
): Promise<ActionOutcome<AlertRule[]>> => {
  if (getAlertRulesState().renamingIds.includes(id)) return actionBusy;
  setAlertRuleRenaming(id, true);
  try {
    await adminApi.updateAlertRule(id, { name });
    return await reloadAlertRules();
  } finally {
    setAlertRuleRenaming(id, false);
  }
};

export const saveAlertRule = async (
  id: number | null,
  input: adminApi.CreateAlertRuleInput,
): Promise<ActionOutcome<AlertRule[]>> => {
  if (getAlertRulesState().saving) return actionBusy;
  useAlertRulesStore.setState({ saving: true });
  try {
    if (id === null) {
      await adminApi.createAlertRule(input);
    } else {
      await adminApi.updateAlertRule(id, input);
    }
    return await reloadAlertRules();
  } finally {
    useAlertRulesStore.setState({ saving: false });
  }
};

export const deleteAlertRule = async (id: number): Promise<ActionOutcome<AlertRule[]>> => {
  await adminApi.deleteAlertRule(id);
  return await reloadAlertRules();
};
