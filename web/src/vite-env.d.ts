/// <reference types="vite/client" />

import type { ThemeManifest } from '@app-types/admin';

type ThemeMode = 'light' | 'dark' | 'system';

declare global {
  interface Window {
    __themeInitialized?: boolean;
    __theme?: {
      get: () => ThemeMode;
      set: (theme: ThemeMode) => void;
      apply: (theme: ThemeMode) => void;
      onSystemChange?: (handler: (e: MediaQueryListEvent) => void) => () => void;
    };
    __themePackage?: {
      manifest?: Partial<ThemeManifest> | null;
      manifestPromise?: Promise<Partial<ThemeManifest> | null>;
      refresh: () => void;
    };
  }
}

export {};
