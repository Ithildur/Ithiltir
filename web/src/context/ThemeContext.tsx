import React from 'react';
import type { ThemeManifest } from '@app-types/admin';
import {
  getThemeManifest,
  resolveThemeManifest,
  refreshThemeManifest,
  themeRefreshEvent,
} from '@lib/themePackageRuntime';

type ThemeContextValue = {
  manifest: ThemeManifest;
  refresh: () => Promise<void>;
};

const ThemeContext = React.createContext<ThemeContextValue>({
  manifest: getThemeManifest(),
  refresh: async () => {},
});

export const ThemeProvider: React.FC<React.PropsWithChildren> = ({ children }) => {
  const [manifest, setManifest] = React.useState<ThemeManifest>(() => getThemeManifest());

  React.useEffect(() => {
    let done = false;
    void resolveThemeManifest()
      .then((nextManifest) => {
        if (!done) {
          setManifest(nextManifest);
        }
      })
      .catch((error) => {
        console.warn('Failed to resolve theme manifest', error);
      });
    return () => {
      done = true;
    };
  }, []);

  const refresh = React.useCallback(async () => {
    setManifest(await refreshThemeManifest());
  }, []);

  React.useEffect(() => {
    const onRefresh = () => {
      void refresh().catch((error) => {
        console.warn('Failed to refresh theme manifest', error);
      });
    };

    window.addEventListener(themeRefreshEvent, onRefresh);
    return () => {
      window.removeEventListener(themeRefreshEvent, onRefresh);
    };
  }, [refresh]);

  const value = React.useMemo(
    () => ({
      manifest,
      refresh,
    }),
    [manifest, refresh],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
};

export const useTheme = (): ThemeContextValue => React.useContext(ThemeContext);
