import { create } from 'zustand';
import type { TrafficSettings } from '@app-types/traffic';
import { fetchTrafficSettings, updateTrafficSettings } from '@lib/statisticsApi';
import { defaultTrafficSettings } from '@lib/trafficSettingsModel';
import { cacheTrafficGuestAccess } from './statisticsAccessStore';
import { createSeqGate, reloadLatestLoad, runLatestLoad } from '@utils/seqGate';

type TrafficSettingsSaveKind = 'usageMode' | 'guestAccess' | 'direction';

interface TrafficSettingsState {
  settings: TrafficSettings;
  loading: boolean;
  loaded: boolean;
  savingUsageMode: boolean;
  savingGuestAccess: boolean;
  savingDirection: boolean;
}

const initialTrafficSettingsState = {
  settings: defaultTrafficSettings,
  loading: false,
  loaded: false,
  savingUsageMode: false,
  savingGuestAccess: false,
  savingDirection: false,
};

export const useTrafficSettingsStore = create<TrafficSettingsState>()(
  () => initialTrafficSettingsState,
);

const getTrafficSettingsState = (): TrafficSettingsState => useTrafficSettingsStore.getState();

export const resetTrafficSettingsStore = (): void => {
  settingsGate.invalidate();
  useTrafficSettingsStore.setState(initialTrafficSettingsState);
};

const settingsGate = createSeqGate();

const replaceTrafficSettings = (settings: TrafficSettings): void => {
  useTrafficSettingsStore.setState({ settings, loaded: true });
};

const patchTrafficSettingsState = (patch: Partial<TrafficSettings>): void => {
  useTrafficSettingsStore.setState((state) => ({
    settings: { ...state.settings, ...patch },
    loaded: true,
  }));
};

const setTrafficSettingsLoading = (loading: boolean): void => {
  useTrafficSettingsStore.setState({ loading });
};

const isSaving = (kind: TrafficSettingsSaveKind): boolean => {
  const state = getTrafficSettingsState();
  switch (kind) {
    case 'usageMode':
      return state.savingUsageMode;
    case 'guestAccess':
      return state.savingGuestAccess;
    case 'direction':
      return state.savingDirection;
  }
};

const setSaving = (kind: TrafficSettingsSaveKind, saving: boolean): void => {
  switch (kind) {
    case 'usageMode':
      useTrafficSettingsStore.setState({ savingUsageMode: saving });
      return;
    case 'guestAccess':
      useTrafficSettingsStore.setState({ savingGuestAccess: saving });
      return;
    case 'direction':
      useTrafficSettingsStore.setState({ savingDirection: saving });
  }
};

const beginSave = (kind: TrafficSettingsSaveKind): boolean => {
  if (isSaving(kind)) return false;
  settingsGate.invalidate();
  setTrafficSettingsLoading(false);
  setSaving(kind, true);
  return true;
};

export const loadTrafficSettings = async (
  params: { signal?: AbortSignal } = {},
): Promise<TrafficSettings> => {
  return runLatestLoad(
    settingsGate,
    () => fetchTrafficSettings(params),
    replaceTrafficSettings,
    setTrafficSettingsLoading,
  );
};

export const patchTrafficSettings = async (
  patch: Partial<TrafficSettings>,
  params: {
    kind: TrafficSettingsSaveKind;
  },
): Promise<boolean> => {
  if (!beginSave(params.kind)) return false;
  try {
    await updateTrafficSettings(patch);
    patchTrafficSettingsState(patch);
    if (patch.guest_access_mode !== undefined) {
      cacheTrafficGuestAccess(patch.guest_access_mode);
    }
    try {
      await reloadLatestLoad(settingsGate, () => fetchTrafficSettings(), replaceTrafficSettings);
    } catch {
      // The PATCH is already committed. Optimistic fields remain valid, and a
      // later normal refresh will recover the complete authoritative snapshot.
    }
    return true;
  } finally {
    setSaving(params.kind, false);
  }
};
