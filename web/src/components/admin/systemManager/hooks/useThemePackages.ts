import React from 'react';
import type { ThemeManifest, ThemePackage } from '@app-types/admin';
import { useI18n } from '@i18n';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import type { ConfirmAction } from '@hooks/useConfirmDialog';
import * as adminApi from '@lib/adminApi';
import { defaultThemeManifest, refreshActiveThemeStyles } from '@lib/themePackageRuntime';
import { isCanceledRequestError } from '@utils/errors';
import { createSeqGate, runLatestLoad } from '@utils/seqGate';

type ThemeTarget = Pick<ThemeManifest, 'id' | 'name'> & { active: boolean };

type ThemePackageBusy =
  | { kind: 'idle' }
  | { kind: 'upload' }
  | { kind: 'apply'; id: string }
  | { kind: 'delete'; id: string };

const idleThemePackageBusy: ThemePackageBusy = { kind: 'idle' };

const sortPackages = (items: ThemePackage[]): ThemePackage[] =>
  items.slice().sort((a, b) => {
    const aUnavailable = Boolean(a.missing || a.broken);
    const bUnavailable = Boolean(b.missing || b.broken);

    if (a.active !== b.active) return a.active ? -1 : 1;
    if (aUnavailable !== bUnavailable) return aUnavailable ? 1 : -1;
    if (a.built_in !== b.built_in) return a.built_in ? -1 : 1;
    return a.name.localeCompare(b.name);
  });

const fetchThemePackages = async (params: { signal?: AbortSignal } = {}): Promise<ThemePackage[]> =>
  sortPackages(
    (await adminApi.fetchThemePackages(params)).filter(
      (item) => item.id !== defaultThemeManifest.id,
    ),
  );

export const useThemePackages = ({
  enabled,
  confirmAction,
}: {
  enabled: boolean;
  confirmAction: ConfirmAction;
}) => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const [packages, setPackages] = React.useState<ThemePackage[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [loaded, setLoaded] = React.useState(false);
  const [busy, setBusy] = React.useState<ThemePackageBusy>(idleThemePackageBusy);
  const busyRef = React.useRef<ThemePackageBusy>(idleThemePackageBusy);
  const loadGate = React.useMemo(createSeqGate, []);

  const beginBusy = React.useCallback((next: ThemePackageBusy): boolean => {
    if (busyRef.current.kind !== 'idle') return false;
    busyRef.current = next;
    setBusy(next);
    return true;
  }, []);

  const endBusy = React.useCallback(() => {
    busyRef.current = idleThemePackageBusy;
    setBusy(idleThemePackageBusy);
  }, []);

  const loadPackages = React.useCallback(
    async (params: { signal?: AbortSignal } = {}) => {
      return runLatestLoad(
        loadGate,
        () => fetchThemePackages(params),
        (nextPackages) => {
          setPackages(nextPackages);
          setLoaded(true);
        },
        setLoading,
      );
    },
    [loadGate],
  );

  const fetchPackages = React.useCallback(
    async (params: { signal?: AbortSignal } = {}) => {
      try {
        await loadPackages(params);
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_theme_fetch_failed' });
      }
    },
    [apiError, loadPackages],
  );

  const syncPackages = React.useCallback(async () => {
    try {
      await loadPackages();
    } catch (error) {
      if (isCanceledRequestError(error)) return;
      throw error;
    }
  }, [loadPackages]);

  React.useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    void fetchPackages({ signal: controller.signal });
    return () => {
      controller.abort();
    };
  }, [enabled, fetchPackages]);

  const uploadTheme = React.useCallback(
    async (file: File) => {
      if (!beginBusy({ kind: 'upload' })) return;
      try {
        const uploaded = await adminApi.uploadThemePackage(file);
        if (uploaded.active) {
          refreshActiveThemeStyles();
        }
        await syncPackages();
        pushTopBanner(t('admin_theme_upload_success', { name: uploaded.name }), {
          tone: 'info',
        });
      } catch (error) {
        apiError(error, t('admin_theme_upload_failed'));
      } finally {
        endBusy();
      }
    },
    [apiError, beginBusy, endBusy, syncPackages, t],
  );

  const applyTheme = React.useCallback(
    async (target: ThemeTarget) => {
      if (target.active || !beginBusy({ kind: 'apply', id: target.id })) return;
      try {
        await adminApi.applyThemePackage(target.id);
        refreshActiveThemeStyles();
        await syncPackages();
        pushTopBanner(t('admin_theme_apply_success', { name: target.name }), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_theme_apply_failed'));
      } finally {
        endBusy();
      }
    },
    [apiError, beginBusy, endBusy, syncPackages, t],
  );

  const deleteTheme = React.useCallback(
    async (pkg: ThemePackage) => {
      if (busyRef.current.kind !== 'idle') return;
      await confirmAction(
        {
          title: t('common_confirm'),
          message: t('admin_theme_delete_confirm', { name: pkg.name }),
          confirmLabel: t('common_delete'),
          cancelLabel: t('common_cancel'),
          tone: 'danger',
        },
        async () => {
          if (!beginBusy({ kind: 'delete', id: pkg.id })) return;
          try {
            await adminApi.deleteThemePackage(pkg.id);
            await syncPackages();
            pushTopBanner(t('admin_theme_delete_success', { name: pkg.name }), { tone: 'info' });
          } catch (error) {
            apiError(error, t('admin_theme_delete_failed'));
          } finally {
            endBusy();
          }
        },
      );
    },
    [apiError, beginBusy, confirmAction, endBusy, syncPackages, t],
  );

  const isBusy = busy.kind !== 'idle';
  const uploading = busy.kind === 'upload';
  const applyingId = busy.kind === 'apply' ? busy.id : null;
  const deletingId = busy.kind === 'delete' ? busy.id : null;
  const defaultActive = loaded && !packages.some((item) => item.active);

  return {
    packages,
    loading,
    busy,
    isBusy,
    uploading,
    applyingId,
    deletingId,
    defaultActive,
    uploadTheme,
    applyTheme,
    deleteTheme,
    refresh: fetchPackages,
  };
};
