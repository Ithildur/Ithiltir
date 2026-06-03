import { create } from 'zustand';
import type { AlertChannel } from '@app-types/admin';
import * as adminApi from '@lib/adminApi';
import { pendingIds } from '@utils/pendingIds';
import { actionBusy, actionOk, type ActionOutcome } from '@utils/actionOutcome';
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

const reloadAlertChannels = async (): Promise<ActionOutcome<AlertChannel[]>> => {
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

export const updateAlertChannelEnabled = async (
  id: number,
  enabled: boolean,
): Promise<ActionOutcome> => {
  if (getAlertChannelsState().togglingIds.includes(id)) return actionBusy;
  setAlertChannelToggling(id, true);
  try {
    await adminApi.updateAlertChannelEnabled(id, { enabled });
    loadGate.invalidate();
    patchAlertChannelEnabled(id, enabled);
    return actionOk();
  } finally {
    setAlertChannelToggling(id, false);
  }
};

export const testAlertChannel = async (id: number): Promise<ActionOutcome> => {
  if (getAlertChannelsState().testingIds.includes(id)) return actionBusy;
  setAlertChannelTesting(id, true);
  try {
    await adminApi.testAlertChannel(id);
    return actionOk();
  } finally {
    setAlertChannelTesting(id, false);
  }
};

export const saveAlertChannel = async (
  id: number | null,
  input: adminApi.AlertChannelInput,
): Promise<ActionOutcome<AlertChannel[]>> => {
  if (getAlertChannelsState().saving) return actionBusy;
  useAlertChannelsStore.setState({ saving: true });
  try {
    if (id === null) {
      await adminApi.createAlertChannel(input);
    } else {
      await adminApi.updateAlertChannel(id, input);
    }
    return await reloadAlertChannels();
  } finally {
    useAlertChannelsStore.setState({ saving: false });
  }
};

export const deleteAlertChannel = async (id: number): Promise<ActionOutcome<AlertChannel[]>> => {
  await adminApi.deleteAlertChannel(id);
  return await reloadAlertChannels();
};
