import React from 'react';
import Palette from 'lucide-react/dist/esm/icons/palette';
import RotateCcw from 'lucide-react/dist/esm/icons/rotate-ccw';
import Save from 'lucide-react/dist/esm/icons/save';
import Settings2 from 'lucide-react/dist/esm/icons/settings-2';
import Upload from 'lucide-react/dist/esm/icons/upload';
import Button from '@components/ui/Button';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import Input from '@components/ui/Input';
import IOSSwitch from '@components/ui/IOSSwitch';
import SettingRow from '@components/admin/systemManager/SettingRow';
import ThemeManager from '@components/admin/systemManager/ThemeManager';
import TrafficSettings from '@components/admin/systemManager/TrafficSettings';
import type {
  HistoryGuestAccessMode,
  SystemSettings as SystemSettingsView,
} from '@app-types/admin';
import type { SiteBrand } from '@app-types/site';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { useSiteBrand } from '@context/SiteBrandContext';
import { useI18n } from '@i18n';
import * as adminApi from '@lib/adminApi';
import { defaultSiteBrand, normalizeSiteBrand } from '@lib/siteBrandApi';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useConfirmDialog } from '@hooks/useConfirmDialog';

type SubTab = 'settings' | 'themes';

const tabClass = (active: boolean) =>
  `px-1 py-2 text-sm font-semibold border-b-2 -mb-px transition-colors ${
    active
      ? 'border-(--theme-border-underline-nav-active) text-(--theme-fg-default)'
      : 'border-transparent text-(--theme-fg-muted) dark:text-(--theme-fg-muted) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default)'
  }`;

const logoMaxBytes = 512 * 1024;

const readFileAsDataURL = (file: File): Promise<string> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === 'string') {
        resolve(reader.result);
        return;
      }
      reject(new Error('invalid file result'));
    };
    reader.onerror = () => reject(reader.error ?? new Error('failed to read file'));
    reader.readAsDataURL(file);
  });

const isSupportedLogoFile = (file: File): boolean => {
  if (file.type.startsWith('image/')) return true;
  return /\.(svg|png|jpe?g|webp|ico)$/i.test(file.name);
};

const SystemSettings: React.FC = () => {
  const { t } = useI18n();
  const confirmDialog = useConfirmDialog();
  const apiError = useApiErrorHandler();
  const pushBanner = useTopBanner();
  const { setBrand: setSiteBrand } = useSiteBrand();
  const [activeTab, setActiveTab] = React.useState<SubTab>('settings');
  const [settings, setSettings] = React.useState<SystemSettingsView | null>(null);
  const [brandDraft, setBrandDraft] = React.useState<SiteBrand | null>(null);
  const [loadingSettings, setLoadingSettings] = React.useState(false);
  const [savingSettings, setSavingSettings] = React.useState(false);
  const logoInputRef = React.useRef<HTMLInputElement | null>(null);

  const loadSettings = React.useCallback(async () => {
    setLoadingSettings(true);
    try {
      const next = await adminApi.fetchSystemSettings();
      setSettings(next);
      setBrandDraft(normalizeSiteBrand(next));
    } catch (error) {
      apiError(error, t('admin_system_settings_fetch_failed'));
    } finally {
      setLoadingSettings(false);
    }
  }, [apiError, t]);

  React.useEffect(() => {
    if (activeTab !== 'settings' || settings) return;
    void loadSettings();
  }, [activeTab, loadSettings, settings]);

  const updateBrandDraft = React.useCallback((field: keyof SiteBrand, value: string) => {
    setBrandDraft((current) => ({
      ...(current ?? defaultSiteBrand),
      [field]: value,
    }));
  }, []);

  const updateHistoryMode = React.useCallback(
    async (mode: HistoryGuestAccessMode) => {
      if (!settings || savingSettings) return;
      const previous = settings;
      const next = { ...settings, history_guest_access_mode: mode };
      setSettings(next);
      setSavingSettings(true);
      try {
        await adminApi.updateSystemSettings({ history_guest_access_mode: mode });
        pushBanner(t('admin_system_settings_saved'), { tone: 'info' });
      } catch (error) {
        setSettings(previous);
        apiError(error, t('admin_system_settings_save_failed'));
      } finally {
        setSavingSettings(false);
      }
    },
    [apiError, pushBanner, savingSettings, settings, t],
  );

  const saveBrandSettings = React.useCallback(async () => {
    if (!settings || !brandDraft || savingSettings) return;
    const previous = settings;
    const next = { ...settings, ...brandDraft };
    setSettings(next);
    setSavingSettings(true);
    try {
      await adminApi.updateSystemSettings(brandDraft);
      const savedBrand = setSiteBrand(brandDraft);
      setSettings({ ...next, ...savedBrand });
      setBrandDraft(savedBrand);
      pushBanner(t('admin_system_settings_saved'), { tone: 'info' });
    } catch (error) {
      setSettings(previous);
      setBrandDraft(normalizeSiteBrand(previous));
      apiError(error, t('admin_system_settings_save_failed'));
    } finally {
      setSavingSettings(false);
    }
  }, [apiError, brandDraft, pushBanner, savingSettings, setSiteBrand, settings, t]);

  const selectLogoFile = React.useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.currentTarget.files?.[0];
      event.currentTarget.value = '';
      if (!file) return;
      if (file.size > logoMaxBytes) {
        pushBanner(t('admin_system_brand_logo_too_large'), { tone: 'error' });
        return;
      }
      if (!isSupportedLogoFile(file)) {
        pushBanner(t('admin_system_brand_logo_type_invalid'), { tone: 'error' });
        return;
      }
      try {
        updateBrandDraft('logo_url', await readFileAsDataURL(file));
      } catch {
        pushBanner(t('admin_system_brand_logo_read_failed'), { tone: 'error' });
      }
    },
    [pushBanner, t, updateBrandDraft],
  );

  const historyByNode = settings?.history_guest_access_mode === 'by_node';
  const savedBrand = settings ? normalizeSiteBrand(settings) : null;
  const draftLogoURL = brandDraft?.logo_url.trim() || defaultSiteBrand.logo_url;
  const brandChanged =
    Boolean(savedBrand && brandDraft) &&
    (brandDraft?.logo_url !== savedBrand?.logo_url ||
      brandDraft?.page_title !== savedBrand?.page_title ||
      brandDraft?.topbar_text !== savedBrand?.topbar_text);

  return (
    <div className="space-y-4 md:space-y-6">
      <ConfirmDialog {...confirmDialog.dialogProps} />

      <div className="flex flex-col justify-between gap-3 md:flex-row md:gap-4">
        <div className="flex w-full md:w-auto md:flex-1">
          <div className="flex gap-4 border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
            <button
              type="button"
              onClick={() => setActiveTab('settings')}
              aria-current={activeTab === 'settings' ? 'page' : undefined}
              className={tabClass(activeTab === 'settings')}
            >
              <span className="inline-flex items-center gap-2">
                <Settings2 className="size-4" aria-hidden="true" />
                {t('admin_tab_system')}
              </span>
            </button>

            <button
              type="button"
              onClick={() => setActiveTab('themes')}
              aria-current={activeTab === 'themes' ? 'page' : undefined}
              className={tabClass(activeTab === 'themes')}
            >
              <span className="inline-flex items-center gap-2">
                <Palette className="size-4" aria-hidden="true" />
                {t('admin_system_tab_themes')}
              </span>
            </button>
          </div>
        </div>
      </div>

      {activeTab === 'settings' && (
        <div className="space-y-4">
          <div className="rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-default) p-5 shadow-sm transition-[border-color,background-color] hover:border-(--theme-border-hover) hover:bg-(--theme-surface-row-hover) dark:border-(--theme-border-default) dark:bg-(--theme-bg-default) dark:hover:bg-(--theme-canvas-subtle)">
            <div className="flex flex-wrap items-center justify-between gap-4">
              <div className="min-w-0">
                <div className="text-sm font-semibold text-(--theme-fg-default)">
                  {t('admin_system_brand_title')}
                </div>
                <div className="mt-1 max-w-160 text-xs/5 text-(--theme-fg-muted)">
                  {t('admin_system_brand_desc')}
                </div>
              </div>
              <Button
                type="button"
                icon={Save}
                disabled={!settings || !brandChanged || loadingSettings || savingSettings}
                onClick={() => void saveBrandSettings()}
              >
                {savingSettings ? t('admin_system_settings_saving') : t('common_save_changes')}
              </Button>
            </div>

            <div className="mt-5 grid gap-5 lg:grid-cols-[minmax(340px,520px)_minmax(260px,360px)] lg:justify-start">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
                <div className="flex size-24 shrink-0 items-center justify-center rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-muted) p-4 dark:border-(--theme-border-default)">
                  <img
                    src={draftLogoURL}
                    alt={t('admin_system_brand_logo_preview')}
                    className="size-full object-contain"
                  />
                </div>
                <input
                  ref={logoInputRef}
                  type="file"
                  accept="image/svg+xml,image/png,image/jpeg,image/webp,image/x-icon"
                  className="hidden"
                  onChange={(event) => void selectLogoFile(event)}
                />
                <div className="min-w-0 space-y-2">
                  <div className="flex flex-wrap gap-2">
                    <Button
                      type="button"
                      variant="secondary"
                      icon={Upload}
                      disabled={!settings || loadingSettings || savingSettings}
                      onClick={() => logoInputRef.current?.click()}
                    >
                      {t('admin_system_brand_logo_upload')}
                    </Button>
                    <Button
                      type="button"
                      variant="secondary"
                      icon={RotateCcw}
                      disabled={!brandDraft || loadingSettings || savingSettings}
                      onClick={() => updateBrandDraft('logo_url', defaultSiteBrand.logo_url)}
                    >
                      {t('admin_system_brand_logo_reset')}
                    </Button>
                  </div>
                  <div className="text-xs/5 text-(--theme-fg-subtle)">
                    {t('admin_system_brand_logo_hint')}
                  </div>
                </div>
              </div>

              <div className="grid content-start gap-4">
                <label className="grid gap-1.5">
                  <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                    {t('admin_system_brand_page_title')}
                  </span>
                  <Input
                    value={brandDraft?.page_title ?? ''}
                    disabled={!settings || loadingSettings || savingSettings}
                    maxLength={120}
                    onChange={(event) => updateBrandDraft('page_title', event.target.value)}
                  />
                </label>
                <label className="grid gap-1.5">
                  <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                    {t('admin_system_brand_topbar_text')}
                  </span>
                  <Input
                    value={brandDraft?.topbar_text ?? ''}
                    disabled={!settings || loadingSettings || savingSettings}
                    maxLength={64}
                    onChange={(event) => updateBrandDraft('topbar_text', event.target.value)}
                  />
                </label>
              </div>
            </div>
          </div>

          <SettingRow
            title={t('admin_system_history_guest_access')}
            description={t('admin_system_history_guest_access_desc')}
          >
            <IOSSwitch
              checked={historyByNode}
              disabled={!settings || loadingSettings || savingSettings}
              ariaLabel={t('admin_system_history_guest_access')}
              onChange={() => void updateHistoryMode(historyByNode ? 'disabled' : 'by_node')}
            />
          </SettingRow>

          <TrafficSettings />
        </div>
      )}

      {activeTab === 'themes' && <ThemeManager enabled confirmAction={confirmDialog.run} />}
    </div>
  );
};

export default SystemSettings;
