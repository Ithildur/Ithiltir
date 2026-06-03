import { create } from 'zustand';
import type { AlertChannel } from '@app-types/admin';
import * as adminApi from '@lib/adminApi';
import { pendingIds } from '@utils/pendingIds';
import { createSeqGate, reloadLatestLoad, runLatestLoad } from '@utils/seqGate';

export interface AlertChannelsState {
  channels: AlertChannel[];
  loading: boolean;
  testingIds: number[];
  togglingIds: number[];
  saving: boolean;
}

const initialAlertChannelsState = {
  channels: [],
  loading: false,
  testingIds: [],
  togglingIds: [],
  saving: false,
};

const sortChannels = (items: AlertChannel[]): AlertChannel[] =>
  items.slice().sort((a, b) => a.id - b.id);

export const useAlertChannelsStore = create<AlertChannelsState>()(() => initialAlertChannelsState);

const getAlertChannelsState = (): AlertChannelsState => useAlertChannelsStore.getState();

export const resetAlertChannelsStore = (): void => {
  loadGate.invalidate();
  useAlertChannelsStore.setState(initialAlertChannelsState);
};

const loadGate = createSeqGate();

const fetchAlertChannels = async (params: { signal?: AbortSignal } = {}): Promise<AlertChannel[]> =>
  sortChannels(await adminApi.fetchAlertChannels(params));

const replaceAlertChannels = (channels: AlertChannel[]): void => {
  useAlertChannelsStore.setState({ channels });
};

const setAlertChannelsLoading = (loading: boolean): void => {
  useAlertChannelsStore.setState({ loading });
};

const reloadAlertChannels = async (): Promise<void> => {
  return reloadLatestLoad(
    loadGate,
    fetchAlertChannels,
    replaceAlertChannels,
    setAlertChannelsLoading,
  );
};

export const loadAlertChannels = async (
  params: { signal?: AbortSignal } = {},
): Promise<AlertChannel[]> => {
  return runLatestLoad(
    loadGate,
    () => fetchAlertChannels(params),
    replaceAlertChannels,
    setAlertChannelsLoading,
  );
};

const patchAlertChannelEnabled = (id: number, enabled: boolean): void => {
  useAlertChannelsStore.setState((state) => ({
    channels: state.channels.map((channel) =>
      channel.id === id ? { ...channel, enabled } : channel,
    ),
  }));
};

const setAlertChannelTesting = (id: number, testing: boolean): void => {
  useAlertChannelsStore.setState((state) => ({
    testingIds: pendingIds(state.testingIds, id, testing),
  }));
};

const setAlertChannelToggling = (id: number, toggling: boolean): void => {
  useAlertChannelsStore.setState((state) => ({
    togglingIds: pendingIds(state.togglingIds, id, toggling),
  }));
};

export const updateAlertChannelEnabled = async (id: number, enabled: boolean): Promise<boolean> => {
  if (getAlertChannelsState().togglingIds.includes(id)) return false;
  setAlertChannelToggling(id, true);
  try {
    await adminApi.updateAlertChannelEnabled(id, { enabled });
    loadGate.invalidate();
    patchAlertChannelEnabled(id, enabled);
    return true;
  } finally {
    setAlertChannelToggling(id, false);
  }
};

export const testAlertChannel = async (id: number): Promise<boolean> => {
  if (getAlertChannelsState().testingIds.includes(id)) return false;
  setAlertChannelTesting(id, true);
  try {
    await adminApi.testAlertChannel(id);
    return true;
  } finally {
    setAlertChannelTesting(id, false);
  }
};

export const saveAlertChannel = async (
  id: number | null,
  input: adminApi.AlertChannelInput,
): Promise<boolean> => {
  if (getAlertChannelsState().saving) return false;
  useAlertChannelsStore.setState({ saving: true });
  try {
    if (id === null) {
      await adminApi.createAlertChannel(input);
    } else {
      await adminApi.updateAlertChannel(id, input);
    }
    await reloadAlertChannels();
    return true;
  } finally {
    useAlertChannelsStore.setState({ saving: false });
  }
};

export const deleteAlertChannel = async (id: number): Promise<boolean> => {
  await adminApi.deleteAlertChannel(id);
  await reloadAlertChannels();
  return true;
};
