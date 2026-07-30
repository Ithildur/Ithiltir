import type { ThemeMode } from '@app-types/theme';

const themeRuntime = (): NonNullable<Window['__theme']> => {
  if (!window.__theme) {
    throw new Error('Theme runtime is not installed');
  }
  return window.__theme;
};

export const ensureThemeRuntime = (): void => {
  themeRuntime();
};

export const readThemeMode = (): ThemeMode => {
  return themeRuntime().get();
};

export const writeThemeMode = (mode: ThemeMode): void => {
  themeRuntime().set(mode);
};

export const applyThemeMode = (mode: ThemeMode): void => {
  themeRuntime().apply(mode);
};

export const subscribeSystemTheme = (handler: () => void): (() => void) => {
  return themeRuntime().onSystemChange(handler);
};
