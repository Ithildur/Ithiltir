import { create } from 'zustand';
import type { AlertChannel } from '@app-types/admin';
import * as adminApi from '@lib/adminApi';
import { pendingIds } from '@utils/pendingIds';
import { createSeqGate, reloadLatestLoad, runLatestLoad } from '@utils/seqGate';

interface AlertChannelsState {
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

const enabledDeliveryStatus = (channel: AlertChannel): AlertChannel['delivery_status'] => {
  if (channel.consecutive_failures > 0 || channel.blocked_count > 0) return 'degraded';
  return channel.last_success_at ? 'healthy' : 'unknown';
};

const patchAlertChannelEnabled = (id: number, enabled: boolean): void => {
  useAlertChannelsStore.setState((state) => ({
    channels: state.channels.map((channel) =>
      channel.id === id
        ? {
            ...channel,
            enabled,
            delivery_status: enabled ? enabledDeliveryStatus(channel) : 'disabled',
          }
        : channel,
    ),
  }));
};

const reloadAlertChannels = async (params: { signal?: AbortSignal } = {}): Promise<boolean> => {
  return reloadLatestLoad(
    loadGate,
    () => fetchAlertChannels(params),
    replaceAlertChannels,
    setAlertChannelsLoading,
  );
};

export const refreshAlertChannels = async (
  params: { signal?: AbortSignal } = {},
): Promise<boolean> => {
  try {
    return await reloadAlertChannels(params);
  } catch {
    return false;
  }
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
    patchAlertChannelEnabled(id, enabled);
    await refreshAlertChannels();
    return true;
  } finally {
    setAlertChannelToggling(id, false);
  }
};

export const testAlertChannel = async (id: number): Promise<boolean> => {
  if (getAlertChannelsState().testingIds.includes(id)) return false;
  setAlertChannelTesting(id, true);
  try {
    try {
      await adminApi.testAlertChannel(id);
    } catch (error) {
      void reloadAlertChannels().catch(() => undefined);
      throw error;
    }
    await refreshAlertChannels();
    return true;
  } finally {
    setAlertChannelTesting(id, false);
  }
};

export const saveAlertChannel = async (
  id: number | null,
  input: adminApi.AlertChannelInput,
): Promise<'busy' | 'synced' | 'stale'> => {
  if (getAlertChannelsState().saving) return 'busy';
  useAlertChannelsStore.setState({ saving: true });
  try {
    if (id === null) {
      await adminApi.createAlertChannel(input);
    } else {
      await adminApi.updateAlertChannel(id, input);
    }
    return (await refreshAlertChannels()) ? 'synced' : 'stale';
  } finally {
    useAlertChannelsStore.setState({ saving: false });
  }
};

export const deleteAlertChannel = async (id: number): Promise<boolean> => {
  await adminApi.deleteAlertChannel(id);
  useAlertChannelsStore.setState((state) => ({
    channels: state.channels.filter((channel) => channel.id !== id),
  }));
  await refreshAlertChannels();
  return true;
};
