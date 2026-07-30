import { create } from 'zustand';
import type { AlertMounts } from '@app-types/admin';
import * as adminApi from '@lib/adminApi';
import { createSeqGate, reloadLatestLoad, runLatestLoad } from '@utils/seqGate';

const emptyMounts: AlertMounts = { rules: [], nodes: [] };

interface AlertMountsState {
  data: AlertMounts;
  loading: boolean;
  saving: boolean;
}

const initialAlertMountsState = {
  data: emptyMounts,
  loading: false,
  saving: false,
};

export const useAlertMountsStore = create<AlertMountsState>()(() => initialAlertMountsState);

const getAlertMountsState = (): AlertMountsState => useAlertMountsStore.getState();

export const resetAlertMountsStore = (): void => {
  loadGate.invalidate();
  useAlertMountsStore.setState(initialAlertMountsState);
};

const loadGate = createSeqGate();

const fetchAlertMounts = (params: { signal?: AbortSignal } = {}): Promise<AlertMounts> =>
  adminApi.fetchAlertMounts(params);

const replaceAlertMounts = (data: AlertMounts): void => {
  useAlertMountsStore.setState({ data });
};

const setAlertMountsLoading = (loading: boolean): void => {
  useAlertMountsStore.setState({ loading });
};

const reloadAlertMounts = async (): Promise<void> => {
  await reloadLatestLoad(loadGate, fetchAlertMounts, replaceAlertMounts, setAlertMountsLoading);
};

export const loadAlertMounts = async (
  params: { signal?: AbortSignal } = {},
): Promise<AlertMounts> => {
  return runLatestLoad(
    loadGate,
    () => fetchAlertMounts(params),
    replaceAlertMounts,
    setAlertMountsLoading,
  );
};

export const setAlertMounts = async (
  ruleIds: number[],
  serverIds: number[],
  mounted: boolean,
): Promise<boolean> => {
  const state = getAlertMountsState();
  if (state.saving || ruleIds.length === 0 || serverIds.length === 0) return false;

  useAlertMountsStore.setState({ saving: true });
  try {
    await adminApi.updateAlertMounts({
      rule_ids: ruleIds,
      server_ids: serverIds,
      mounted,
    });
    await reloadAlertMounts();
    return true;
  } finally {
    useAlertMountsStore.setState({ saving: false });
  }
};
