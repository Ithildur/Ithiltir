import React from 'react';
import {
  clearDashUpdateTarget,
  dashUpdateTargetEvent,
  readDashUpdateTarget,
  rememberDashUpdateReload,
  type DashUpdateTarget,
} from '@lib/dashUpdateSession';
import { fetchAppVersion } from '@lib/versionApi';
import { isVersionAtLeast } from '@utils/version';
import { DashUpdateOverlay, type DashUpdateOverlayPhase } from './DashUpdateOverlay';

const pollIntervalMs = 60_000;
const updatePollIntervalMs = 10_000;
const updateRequestTimeoutMs = 5_000;
const buildVersion = __DASH_BUILD_VERSION__.trim();

export const DashVersionRuntime: React.FC = () => {
  const loadedVersionRef = React.useRef(buildVersion);
  const [target, setTarget] = React.useState<DashUpdateTarget | null>(() => readDashUpdateTarget());
  const [updatePhase, setUpdatePhase] = React.useState<DashUpdateOverlayPhase>('updating');
  const [observedVersion, setObservedVersion] = React.useState('');

  React.useEffect(() => {
    const syncTarget = () => {
      const next = readDashUpdateTarget();
      setTarget(next);
      if (next) {
        setObservedVersion('');
        setUpdatePhase('updating');
      }
    };
    window.addEventListener(dashUpdateTargetEvent, syncTarget);
    return () => window.removeEventListener(dashUpdateTargetEvent, syncTarget);
  }, []);

  const checkVersion = React.useCallback(async (signal: AbortSignal) => {
    try {
      const next = await fetchAppVersion({ signal });
      if (signal.aborted) return false;
      if (readDashUpdateTarget()) return false;

      const version = next.version.trim();
      if (!version) return false;
      if (!loadedVersionRef.current) {
        loadedVersionRef.current = version;
        return false;
      }
      if (version === loadedVersionRef.current) return false;

      rememberDashUpdateReload(version);
      window.location.reload();
      return true;
    } catch {
      // The local service may be unavailable during cutover. A later visible
      // poll retries without surfacing a transient update error to every page.
      return false;
    }
  }, []);

  React.useEffect(() => {
    if (target) return;

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
  }, [checkVersion, target]);

  React.useEffect(() => {
    if (!target || target.status === 'failed') return;

    let timer: number | undefined;
    let request: AbortController | null = null;

    const poll = async () => {
      timer = undefined;
      const startedAt = Date.now();
      const controller = new AbortController();
      request = controller;
      try {
        const next = await fetchAppVersion({
          signal: controller.signal,
          timeoutMs: updateRequestTimeoutMs,
        });
        if (request !== controller || controller.signal.aborted) return;

        const version = next.version.trim();
        if (version && isVersionAtLeast(version, target.version)) {
          loadedVersionRef.current = version;
          setObservedVersion(version);
          setUpdatePhase('success');
          request = null;
          return;
        }
        setUpdatePhase('updating');
      } catch {
        if (request !== controller || controller.signal.aborted) return;
        setUpdatePhase('reconnecting');
      }
      request = null;
      const elapsed = Date.now() - startedAt;
      timer = window.setTimeout(() => void poll(), Math.max(0, updatePollIntervalMs - elapsed));
    };

    void poll();
    return () => {
      if (timer !== undefined) window.clearTimeout(timer);
      request?.abort();
    };
  }, [target]);

  const reloadUpdatedPage = React.useCallback(() => {
    if (!target || !clearDashUpdateTarget(target.version)) return;
    window.location.reload();
  }, [target]);

  const dismissFailedUpdate = React.useCallback(() => {
    if (!target || target.status !== 'failed') return;
    clearDashUpdateTarget(target.version);
  }, [target]);

  if (!target) return null;
  return (
    <DashUpdateOverlay
      phase={target.status === 'failed' ? 'failed' : updatePhase}
      targetVersion={target.version}
      observedVersion={observedVersion}
      onReload={reloadUpdatedPage}
      onDismiss={dismissFailedUpdate}
    />
  );
};
