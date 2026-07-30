import React from 'react';
import { fetchAppVersion } from '@lib/versionApi';

const pollIntervalMs = 60_000;
const buildVersion = __DASH_BUILD_VERSION__.trim();

export const DashVersionRuntime: React.FC = () => {
  const loadedVersionRef = React.useRef(buildVersion);

  const checkVersion = React.useCallback(async (signal: AbortSignal) => {
    try {
      const next = await fetchAppVersion({ signal });
      if (signal.aborted) return false;

      const version = next.version.trim();
      if (!version) return false;
      if (!loadedVersionRef.current) {
        loadedVersionRef.current = version;
        return false;
      }
      if (version === loadedVersionRef.current) return false;

      window.location.reload();
      return true;
    } catch {
      // The local service may be unavailable during cutover. A later visible
      // poll retries without surfacing a transient update error to every page.
      return false;
    }
  }, []);

  React.useEffect(() => {
    let timer: number | undefined;
    let request: AbortController | null = null;

    const stop = () => {
      if (timer !== undefined) {
        window.clearTimeout(timer);
        timer = undefined;
      }
      request?.abort();
      request = null;
    };

    const poll = async () => {
      timer = undefined;
      const controller = new AbortController();
      request = controller;
      const reloading = await checkVersion(controller.signal);
      if (request !== controller) return;
      request = null;
      if (!reloading && document.visibilityState === 'visible') {
        timer = window.setTimeout(() => void poll(), pollIntervalMs);
      }
    };

    const visibilityChanged = () => {
      stop();
      if (document.visibilityState === 'visible') void poll();
    };

    document.addEventListener('visibilitychange', visibilityChanged);
    if (document.visibilityState === 'visible') void poll();

    return () => {
      document.removeEventListener('visibilitychange', visibilityChanged);
      stop();
    };
  }, [checkVersion]);

  return null;
};
