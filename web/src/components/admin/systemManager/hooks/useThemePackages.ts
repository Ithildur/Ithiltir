import React from 'react';
import type { ThemeManifest, ThemePackage } from '@app-types/admin';
import { useI18n } from '@i18n';
import * as adminApi from '@lib/adminApi';
import { defaultThemeManifest, refreshActiveThemeStyles } from '@lib/themePackageRuntime';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import type { ConfirmAction } from '@hooks/useConfirmDialog';

const sortPackages = (items: ThemePackage[]): ThemePackage[] =>
  items.slice().sort((a, b) => {
    const aUnavailable = Boolean(a.missing || a.broken);
    const bUnavailable = Boolean(b.missing || b.broken);

    if (a.active !== b.active) return a.active ? -1 : 1;
    if (aUnavailable !== bUnavailable) return aUnavailable ? 1 : -1;
    if (a.built_in !== b.built_in) return a.built_in ? -1 : 1;
    return a.name.localeCompare(b.name);
  });

type BusyState =
  | { kind: 'idle' }
  | { kind: 'upload' }
  | { kind: 'apply'; id: string }
  | { kind: 'delete'; id: string };

type ThemeTarget = Pick<ThemeManifest, 'id' | 'name'> & { active: boolean };

const idleBusy: BusyState = { kind: 'idle' };

export const useThemePackages = ({
  enabled,
  confirmAction,
}: {
  enabled: boolean;
  confirmAction: ConfirmAction;
}) => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const pushBanner = useTopBanner();
  const [packages, setPackages] = React.useState<ThemePackage[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [loaded, setLoaded] = React.useState(false);
  const [busy, setBusy] = React.useState<BusyState>(idleBusy);
  const fetchSeqRef = React.useRef(0);
  const busyKindRef = React.useRef<BusyState['kind']>('idle');

  React.useEffect(() => {
    busyKindRef.current = busy.kind;
  }, [busy.kind]);

  const beginBusy = React.useCallback((next: BusyState): boolean => {
    if (busyKindRef.current !== 'idle') return false;
    busyKindRef.current = next.kind;
    setBusy(next);
    return true;
  }, []);

  const endBusy = React.useCallback(() => {
    busyKindRef.current = 'idle';
    setBusy(idleBusy);
  }, []);

  const fetchPackages = React.useCallback(async () => {
    const seq = fetchSeqRef.current + 1;
    fetchSeqRef.current = seq;
    setLoading(true);
    try {
      const items = sortPackages(
        (await adminApi.fetchThemePackages()).filter((item) => item.id !== defaultThemeManifest.id),
      );
      if (seq !== fetchSeqRef.current) return;
      setPackages(items);
    } catch (error) {
      if (seq !== fetchSeqRef.current) return;
      apiError(error, t('admin_theme_fetch_failed'));
    } finally {
      if (seq === fetchSeqRef.current) {
        setLoading(false);
        setLoaded(true);
      }
    }
  }, [apiError, t]);

  React.useEffect(() => {
    if (!enabled) return;
    void fetchPackages();
  }, [enabled, fetchPackages]);

  const uploadTheme = React.useCallback(
    async (file: File) => {
      if (!beginBusy({ kind: 'upload' })) return;
      try {
        const pkg = await adminApi.uploadThemePackage(file);
        pushBanner(t('admin_theme_upload_success', { name: pkg.name }), { tone: 'info' });
        if (pkg.active) {
          refreshActiveThemeStyles();
        }
        await fetchPackages();
      } catch (error) {
        apiError(error, t('admin_theme_upload_failed'));
      } finally {
        endBusy();
      }
    },
    [apiError, beginBusy, endBusy, fetchPackages, pushBanner, t],
  );

  const applyTheme = React.useCallback(
    async (target: ThemeTarget) => {
      if (target.active || !beginBusy({ kind: 'apply', id: target.id })) return;
      try {
        await adminApi.applyThemePackage(target.id);
        refreshActiveThemeStyles();
        pushBanner(t('admin_theme_apply_success', { name: target.name }), { tone: 'info' });
        await fetchPackages();
      } catch (error) {
        apiError(error, t('admin_theme_apply_failed'));
      } finally {
        endBusy();
      }
    },
    [apiError, beginBusy, endBusy, fetchPackages, pushBanner, t],
  );

  const deleteTheme = React.useCallback(
    async (pkg: ThemePackage) => {
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
            pushBanner(t('admin_theme_delete_success', { name: pkg.name }), { tone: 'info' });
            await fetchPackages();
          } catch (error) {
            apiError(error, t('admin_theme_delete_failed'));
          } finally {
            endBusy();
          }
        },
      );
    },
    [apiError, beginBusy, confirmAction, endBusy, fetchPackages, pushBanner, t],
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
