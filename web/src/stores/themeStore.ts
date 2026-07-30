import type { ThemeManifest } from '@app-types/admin';
import type { ThemeMode } from '@app-types/theme';
import {
  defaultThemeManifest,
  getThemeManifest,
  refreshThemeManifest,
  resolveThemeManifest,
} from '@lib/themePackageRuntime';
import { create } from 'zustand';
import { readThemeMode } from './themeModeBridge';
import { createSeqGate } from '@utils/seqGate';

interface ThemeState {
  themeMode: ThemeMode;
  themeManifest: ThemeManifest;
  setThemeMode: (mode: ThemeMode) => void;
}

export const useThemeStore = create<ThemeState>()((set) => ({
  themeMode: 'system',
  themeManifest: defaultThemeManifest,
  setThemeMode: (themeMode) => set({ themeMode }),
}));

export const syncThemeRuntimeState = (): void => {
  useThemeStore.setState({
    themeMode: readThemeMode(),
    themeManifest: getThemeManifest(),
  });
};

const getThemeState = (): ThemeState => useThemeStore.getState();

const themeGate = createSeqGate();

const applyThemeManifest = (manifest: ThemeManifest): void => {
  useThemeStore.setState({ themeManifest: manifest });
};

const runThemeRequest = async (
  load: (signal?: AbortSignal) => Promise<ThemeManifest>,
  signal?: AbortSignal,
): Promise<ThemeManifest> => {
  const seq = themeGate.next();
  const manifest = await load(signal);
  if (!themeGate.isCurrent(seq)) return getThemeState().themeManifest;
  applyThemeManifest(manifest);
  return manifest;
};

export const resolveTheme = async (signal?: AbortSignal): Promise<ThemeManifest> => {
  return runThemeRequest(resolveThemeManifest, signal);
};

export const refreshTheme = async (signal?: AbortSignal): Promise<ThemeManifest> => {
  return runThemeRequest(refreshThemeManifest, signal);
};
