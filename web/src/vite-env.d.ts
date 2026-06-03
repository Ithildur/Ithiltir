/// <reference types="vite/client" />

import type { ThemeMode } from '@app-types/theme';

type BrowserThemeRuntime = {
  get: () => ThemeMode;
  set: (theme: ThemeMode) => void;
  apply: (theme: ThemeMode) => void;
  onSystemChange: (handler: () => void) => () => void;
};

type BrowserThemePackageRuntime = {
  manifest: unknown | null;
  manifestPromise: Promise<unknown | null>;
  refresh: () => void;
};

declare global {
  interface Window {
    __theme?: BrowserThemeRuntime;
    __themePackage?: BrowserThemePackageRuntime;
  }
}

export {};
