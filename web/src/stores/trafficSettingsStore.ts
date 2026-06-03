import { create } from 'zustand';
import type { TrafficSettings } from '@app-types/traffic';
import { fetchTrafficSettings, updateTrafficSettings } from '@lib/statisticsApi';
import { defaultTrafficSettings } from '@lib/trafficSettingsModel';
import { cacheTrafficGuestAccess } from './statisticsAccessStore';
import { actionBusy, actionOk, type ActionOutcome } from '@utils/actionOutcome';
import { createSeqGate, runLatestLoad } from '@utils/seqGate';

export type TrafficSettingsSaveKind = 'cycle' | 'usageMode' | 'guestAccess' | 'direction';

export interface TrafficSettingsState {
  settings: TrafficSettings;
  loading: boolean;
  loaded: boolean;
  savingCycle: boolean;
  savingUsageMode: boolean;
  savingGuestAccess: boolean;
  savingDirection: boolean;
}

const initialTrafficSettingsState = {
  settings: defaultTrafficSettings,
  loading: false,
  loaded: false,
  savingCycle: false,
  savingUsageMode: false,
  savingGuestAccess: false,
  savingDirection: false,
};

export const useTrafficSettingsStore = create<TrafficSettingsState>()(
  () => initialTrafficSettingsState,
);

const getTrafficSettingsState = (): TrafficSettingsState => useTrafficSettingsStore.getState();

export const resetTrafficSettingsStore = (): void => {
  loadGate.invalidate();
  useTrafficSettingsStore.setState(initialTrafficSettingsState);
};

const loadGate = createSeqGate();

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
    case 'cycle':
      return state.savingCycle;
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
    case 'cycle':
      useTrafficSettingsStore.setState({ savingCycle: saving });
      return;
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

const beginSave = (kind: TrafficSettingsSaveKind): ActionOutcome => {
  if (isSaving(kind)) return actionBusy;
  loadGate.invalidate();
  setTrafficSettingsLoading(false);
  setSaving(kind, true);
  return actionOk();
};

export const loadTrafficSettings = async (
  params: { signal?: AbortSignal } = {},
): Promise<TrafficSettings> => {
  return runLatestLoad(
    loadGate,
    () => fetchTrafficSettings(params),
    replaceTrafficSettings,
    setTrafficSettingsLoading,
  );
};

export const patchTrafficSettings = async (
  patch: Partial<TrafficSettings>,
  params: {
    kind: TrafficSettingsSaveKind;
    commit?: Partial<TrafficSettings>;
  },
): Promise<ActionOutcome<TrafficSettings>> => {
  const begin = beginSave(params.kind);
  if (begin.status !== 'ok') return begin;
  try {
    await updateTrafficSettings(patch);
    const commit = params.commit ?? patch;
    patchTrafficSettingsState(commit);
    if (commit.guest_access_mode !== undefined) {
      cacheTrafficGuestAccess(commit.guest_access_mode);
    }
    return actionOk(getTrafficSettingsState().settings);
  } finally {
    setSaving(params.kind, false);
  }
};
