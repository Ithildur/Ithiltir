import React from 'react';
import Palette from 'lucide-react/dist/esm/icons/palette';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import RotateCcw from 'lucide-react/dist/esm/icons/rotate-ccw';
import Save from 'lucide-react/dist/esm/icons/save';
import Settings2 from 'lucide-react/dist/esm/icons/settings-2';
import Upload from 'lucide-react/dist/esm/icons/upload';
import { AdminSectionTabs } from '@components/admin/AdminSectionTabs';
import { BrandImage } from '@components/BrandLogo';
import Button from '@components/ui/Button';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import Input from '@components/ui/Input';
import IOSSwitch from '@components/ui/IOSSwitch';
import SettingRow, {
  SettingPanel,
  SettingPanelFooter,
} from '@components/admin/systemManager/SettingRow';
import DashUpdateSettings from '@components/admin/systemManager/DashUpdateSettings';
import ThemeManager from '@components/admin/systemManager/ThemeManager';
import TrafficSettings from '@components/admin/systemManager/TrafficSettings';
import type {
  DashUpdateChannel,
  DashUpdateMode,
  HistoryGuestAccessMode,
  SystemSettings as SystemSettingsData,
} from '@app-types/admin';
import type { SiteBrand } from '@app-types/site';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { patchBrand, refreshBrand } from '@stores/siteBrandStore';
import { cacheHistoryGuestAccess } from '@stores/statisticsAccessStore';
import { useI18n } from '@i18n';
import * as adminApi from '@lib/adminApi';
import { defaultSiteBrand, displayLogoURL, normalizeSiteBrand } from '@lib/siteBrandModel';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useConfirmDialog } from '@hooks/useConfirmDialog';
import { isCanceledRequestError } from '@utils/errors';
import { createSeqGate, runLatestLoad } from '@utils/seqGate';

const logoMaxBytes = 512 * 1024;
const logoMediaTypes = new Map([
  ['image/svg+xml', 'image/svg+xml'],
  ['image/png', 'image/png'],
  ['image/jpeg', 'image/jpeg'],
  ['image/gif', 'image/gif'],
  ['image/webp', 'image/webp'],
  ['image/ico', 'image/x-icon'],
  ['image/x-icon', 'image/x-icon'],
  ['image/vnd.microsoft.icon', 'image/x-icon'],
]);
const logoExtensionTypes = new Map([
  ['svg', 'image/svg+xml'],
  ['png', 'image/png'],
  ['jpg', 'image/jpeg'],
  ['jpeg', 'image/jpeg'],
  ['gif', 'image/gif'],
  ['webp', 'image/webp'],
  ['ico', 'image/x-icon'],
]);

const tabs = [
  { key: 'settings', labelKey: 'admin_tab_system', icon: Settings2 },
  { key: 'themes', labelKey: 'admin_system_tab_themes', icon: Palette },
  { key: 'dashUpdate', labelKey: 'admin_system_tab_dash_update', icon: RefreshCw },
] as const;

type SystemManagerTab = (typeof tabs)[number]['key'];

const readFileAsDataURL = (file: File, mediaType: string): Promise<string> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === 'string') {
        const comma = reader.result.indexOf(',');
        if (comma >= 0) {
          resolve(`data:${mediaType};base64,${reader.result.slice(comma + 1)}`);
          return;
        }
      }
      reject(new Error('invalid file result'));
    };
    reader.onerror = () => reject(reader.error ?? new Error('failed to read file'));
    reader.readAsDataURL(file);
  });

const logoMediaType = (file: File): string | null => {
  const byType = logoMediaTypes.get(file.type.toLowerCase());
  if (byType) return byType;
  const extension = file.name.toLowerCase().match(/\.([^.]+)$/)?.[1];
  return extension ? (logoExtensionTypes.get(extension) ?? null) : null;
};

const SystemSettings: React.FC = () => {
  const { t } = useI18n();
  const { dialogProps: confirmDialogProps, run: confirmAction } = useConfirmDialog();
  const apiError = useApiErrorHandler();
  const [settings, setSettings] = React.useState<SystemSettingsData | null>(null);
  const [loadingSettings, setLoadingSettings] = React.useState(false);
  const [savingBrand, setSavingBrand] = React.useState(false);
  const [savingHistoryMode, setSavingHistoryMode] = React.useState(false);
  const [savingDashUpdateChannel, setSavingDashUpdateChannel] = React.useState(false);
  const [savingDashUpdateMode, setSavingDashUpdateMode] = React.useState(false);
  const [activeTab, setActiveTab] = React.useState<SystemManagerTab>('settings');
  const [brandDraft, setBrandDraft] = React.useState<SiteBrand | null>(null);
  const brandDirty = React.useRef(false);
  const loadGate = React.useMemo(() => createSeqGate(), []);
  const logoInputRef = React.useRef<HTMLInputElement | null>(null);

  const loadSettings = React.useCallback(
    async (signal: AbortSignal) => {
      try {
        await runLatestLoad(
          loadGate,
          () => adminApi.fetchSystemSettings({ signal }),
          setSettings,
          setLoadingSettings,
        );
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_system_settings_fetch_failed' });
      }
    },
    [apiError, loadGate],
  );

  React.useEffect(() => {
    if ((activeTab !== 'settings' && activeTab !== 'dashUpdate') || settings) return;
    const controller = new AbortController();

    void loadSettings(controller.signal);

    return () => {
      controller.abort();
    };
  }, [activeTab, loadSettings, settings]);

  React.useEffect(() => {
    if (!settings) {
      brandDirty.current = false;
      setBrandDraft(null);
      return;
    }
    if (brandDirty.current) return;
    setBrandDraft(normalizeSiteBrand(settings));
  }, [settings]);

  const updateBrandDraft = React.useCallback((field: keyof SiteBrand, value: string) => {
    brandDirty.current = true;
    setBrandDraft((current) => ({
      ...(current ?? defaultSiteBrand),
      [field]: value,
    }));
  }, []);

  const updateHistoryMode = React.useCallback(
    async (mode: HistoryGuestAccessMode) => {
      if (!settings || savingHistoryMode) return;
      setSavingHistoryMode(true);
      try {
        await adminApi.updateSystemSettings({ history_guest_access_mode: mode });
        setSettings((current) =>
          current ? { ...current, history_guest_access_mode: mode } : current,
        );
        cacheHistoryGuestAccess(mode);
        pushTopBanner(t('admin_system_settings_saved'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_system_settings_save_failed'));
      } finally {
        setSavingHistoryMode(false);
      }
    },
    [apiError, savingHistoryMode, settings, t],
  );

  const saveBrandSettings = React.useCallback(async () => {
    if (!settings || !brandDraft || savingBrand) return;
    const nextBrand = normalizeSiteBrand(brandDraft);
    const previousBrand = normalizeSiteBrand(settings);
    const updates: Partial<SiteBrand> = {};
    if (nextBrand.logo_url !== previousBrand.logo_url) updates.logo_url = nextBrand.logo_url;
    if (nextBrand.page_title !== previousBrand.page_title) {
      updates.page_title = nextBrand.page_title;
    }
    if (nextBrand.topbar_text !== previousBrand.topbar_text) {
      updates.topbar_text = nextBrand.topbar_text;
    }
    if (Object.keys(updates).length === 0) {
      brandDirty.current = false;
      setBrandDraft(previousBrand);
      return;
    }
    setSavingBrand(true);
    try {
      await adminApi.updateSystemSettings(updates);
      let savedBrand: SiteBrand;
      try {
        savedBrand = await refreshBrand();
      } catch (error) {
        savedBrand = patchBrand(updates);
        apiError(error, { key: 'brand_runtime_load_failed' });
      }
      setSettings((current) => (current ? { ...current, ...savedBrand } : current));
      brandDirty.current = false;
      setBrandDraft(savedBrand);
      pushTopBanner(t('admin_system_settings_saved'), { tone: 'info' });
    } catch (error) {
      apiError(error, t('admin_system_settings_save_failed'));
    } finally {
      setSavingBrand(false);
    }
  }, [apiError, brandDraft, savingBrand, settings, t]);

  const updateDashUpdateChannel = React.useCallback(
    async (channel: DashUpdateChannel) => {
      if (!settings || savingDashUpdateChannel || channel === settings.dash_update_channel) return;
      setSavingDashUpdateChannel(true);
      try {
        await adminApi.updateSystemSettings({ dash_update_channel: channel });
        setSettings((current) =>
          current ? { ...current, dash_update_channel: channel } : current,
        );
        pushTopBanner(t('admin_system_settings_saved'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_system_settings_save_failed'));
      } finally {
        setSavingDashUpdateChannel(false);
      }
    },
    [apiError, savingDashUpdateChannel, settings, t],
  );

  const updateDashUpdateMode = React.useCallback(
    async (mode: DashUpdateMode) => {
      if (!settings || savingDashUpdateMode || mode === settings.dash_update_mode) return;
      setSavingDashUpdateMode(true);
      try {
        await adminApi.updateSystemSettings({ dash_update_mode: mode });
        setSettings((current) => (current ? { ...current, dash_update_mode: mode } : current));
        pushTopBanner(t('admin_system_settings_saved'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_system_settings_save_failed'));
      } finally {
        setSavingDashUpdateMode(false);
      }
    },
    [apiError, savingDashUpdateMode, settings, t],
  );

  const selectLogoFile = React.useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      const file = event.currentTarget.files?.[0];
      event.currentTarget.value = '';
      if (!file) return;
      if (file.size > logoMaxBytes) {
        pushTopBanner(t('admin_system_brand_logo_too_large'), { tone: 'error' });
        return;
      }
      const mediaType = logoMediaType(file);
      if (!mediaType) {
        pushTopBanner(t('admin_system_brand_logo_type_invalid'), { tone: 'error' });
        return;
      }
      try {
        updateBrandDraft('logo_url', await readFileAsDataURL(file, mediaType));
      } catch {
        pushTopBanner(t('admin_system_brand_logo_read_failed'), { tone: 'error' });
      }
    },
    [t, updateBrandDraft],
  );

  const historyByNode = settings?.history_guest_access_mode === 'by_node';
  const savedBrand = settings ? normalizeSiteBrand(settings) : null;
  const draftLogoURL = displayLogoURL(brandDraft?.logo_url.trim() || defaultSiteBrand.logo_url);
  const brandChanged =
    Boolean(savedBrand && brandDraft) &&
    (brandDraft?.logo_url !== savedBrand?.logo_url ||
      brandDraft?.page_title !== savedBrand?.page_title ||
      brandDraft?.topbar_text !== savedBrand?.topbar_text);

  return (
    <div className="space-y-4 md:space-y-6">
      <ConfirmDialog {...confirmDialogProps} />

      <div className="flex flex-col justify-between gap-3 md:flex-row md:gap-4">
        <div className="flex w-full md:w-auto md:flex-1">
          <AdminSectionTabs tabs={tabs} activeKey={activeTab} onChange={setActiveTab} />
        </div>
      </div>

      {activeTab === 'settings' && (
        <div className="space-y-4">
          <SettingPanel
            title={t('admin_system_brand_title')}
            description={t('admin_system_brand_desc')}
          >
            <SettingRow
              title={t('admin_system_brand_logo_preview')}
              description={t('admin_system_brand_logo_hint')}
              controlClassName="md:justify-start"
            >
              <div className="flex w-full flex-col gap-3 sm:flex-row sm:items-center">
                <div className="flex size-24 shrink-0 items-center justify-center rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-muted) p-4 dark:border-(--theme-border-default)">
                  <BrandImage
                    src={draftLogoURL}
                    alt={t('admin_system_brand_logo_preview')}
                    className="size-full object-contain"
                  />
                </div>
                <input
                  ref={logoInputRef}
                  type="file"
                  accept=".svg,.png,.jpg,.jpeg,.gif,.webp,.ico,image/svg+xml,image/png,image/jpeg,image/gif,image/webp,image/x-icon,image/vnd.microsoft.icon"
                  className="hidden"
                  onChange={(event) => void selectLogoFile(event)}
                />
                <div className="flex min-w-0 flex-wrap gap-2">
                  <Button
                    type="button"
                    variant="secondary"
                    icon={Upload}
                    disabled={!settings || loadingSettings || savingBrand}
                    onClick={() => logoInputRef.current?.click()}
                  >
                    {t('admin_system_brand_logo_upload')}
                  </Button>
                  <Button
                    type="button"
                    variant="secondary"
                    icon={RotateCcw}
                    disabled={!brandDraft || loadingSettings || savingBrand}
                    onClick={() => updateBrandDraft('logo_url', defaultSiteBrand.logo_url)}
                  >
                    {t('admin_system_brand_logo_reset')}
                  </Button>
                </div>
              </div>
            </SettingRow>

            <SettingRow
              title={t('admin_system_brand_page_title')}
              controlClassName="md:justify-start"
            >
              <Input
                wrapperClassName="w-full max-w-md"
                aria-label={t('admin_system_brand_page_title')}
                value={brandDraft?.page_title ?? ''}
                disabled={!settings || loadingSettings || savingBrand}
                maxLength={120}
                onChange={(event) => updateBrandDraft('page_title', event.target.value)}
              />
            </SettingRow>

            <SettingRow
              title={t('admin_system_brand_topbar_text')}
              controlClassName="md:justify-start"
            >
              <Input
                wrapperClassName="w-full max-w-md"
                aria-label={t('admin_system_brand_topbar_text')}
                value={brandDraft?.topbar_text ?? ''}
                disabled={!settings || loadingSettings || savingBrand}
                maxLength={64}
                onChange={(event) => updateBrandDraft('topbar_text', event.target.value)}
              />
            </SettingRow>

            <SettingPanelFooter>
              <Button
                type="button"
                icon={Save}
                disabled={!settings || !brandChanged || loadingSettings || savingBrand}
                onClick={() => void saveBrandSettings()}
              >
                {savingBrand ? t('admin_system_settings_saving') : t('common_save_changes')}
              </Button>
            </SettingPanelFooter>
          </SettingPanel>

          <SettingPanel>
            <SettingRow
              title={t('admin_system_history_guest_access')}
              description={t('admin_system_history_guest_access_desc')}
            >
              <IOSSwitch
                checked={historyByNode}
                disabled={!settings || loadingSettings || savingHistoryMode}
                ariaLabel={t('admin_system_history_guest_access')}
                onChange={() => void updateHistoryMode(historyByNode ? 'disabled' : 'by_node')}
              />
            </SettingRow>
          </SettingPanel>

          <TrafficSettings />
        </div>
      )}

      {activeTab === 'dashUpdate' && (
        <DashUpdateSettings
          enabled
          settings={settings}
          loadingSettings={loadingSettings}
          savingChannel={savingDashUpdateChannel}
          savingMode={savingDashUpdateMode}
          onChannelChange={(channel) => void updateDashUpdateChannel(channel)}
          onModeChange={(mode) => void updateDashUpdateMode(mode)}
        />
      )}

      {activeTab === 'themes' && <ThemeManager enabled confirmAction={confirmAction} />}
    </div>
  );
};

export default SystemSettings;
