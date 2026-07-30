import React from 'react';
import { useI18n, type TranslationKey } from '@i18n';
import type { AlertChannel, AlertSettings } from '@app-types/admin';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import type { ConfirmAction } from '@hooks/useConfirmDialog';
import * as adminApi from '@lib/adminApi';
import {
  deleteAlertChannel,
  loadAlertChannels,
  refreshAlertChannels,
  saveAlertChannel,
  testAlertChannel,
  updateAlertChannelEnabled,
  useAlertChannelsStore,
} from '@stores/alertChannelsStore';
import {
  channelInputFromForm,
  formFromChannel,
  type AlertChannelForm,
  type AlertChannelFormIssue,
} from '@components/admin/alertManager/alertChannelForm';
import { isCanceledRequestError } from '@utils/errors';

const formIssueKeys = {
  name: 'admin_alerts_channels_name_required',
  telegram_api_id: 'admin_alerts_channels_api_id_invalid',
  email_port: 'admin_alerts_channels_smtp_port_invalid',
} satisfies Record<AlertChannelFormIssue, TranslationKey>;

export const useAlertChannels = ({
  enabled,
  confirmAction,
}: {
  enabled: boolean;
  confirmAction: ConfirmAction;
}) => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const channels = useAlertChannelsStore((state) => state.channels);
  const loading = useAlertChannelsStore((state) => state.loading);
  const testingIds = useAlertChannelsStore((state) => state.testingIds);
  const togglingIds = useAlertChannelsStore((state) => state.togglingIds);
  const saving = useAlertChannelsStore((state) => state.saving);
  const [settings, setSettings] = React.useState<AlertSettings | null>(null);
  const [loadingSettings, setLoadingSettings] = React.useState(false);
  const [savingSettings, setSavingSettings] = React.useState(false);
  const [modal, setModal] = React.useState<{ editingChannelId: number | null } | null>(null);
  const isModalOpen = modal !== null;
  const editingChannelId = modal?.editingChannelId ?? null;
  const editingChannel = React.useMemo(
    () => channels.find((channel) => channel.id === editingChannelId) ?? null,
    [channels, editingChannelId],
  );

  React.useEffect(() => {
    if (!enabled && isModalOpen && !saving) setModal(null);
  }, [enabled, isModalOpen, saving]);

  React.useEffect(() => {
    if (!isModalOpen || editingChannelId === null || editingChannel || loading || saving) return;
    setModal(null);
  }, [editingChannel, editingChannelId, isModalOpen, loading, saving]);

  const fetchChannels = React.useCallback(
    async (params: { signal?: AbortSignal } = {}) => {
      try {
        await loadAlertChannels(params);
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_alerts_channels_fetch_failed' });
      }
    },
    [apiError],
  );

  const fetchSettings = React.useCallback(
    async (params: { signal?: AbortSignal } = {}) => {
      setLoadingSettings(true);
      try {
        setSettings(await adminApi.fetchAlertSettings(params));
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_alerts_settings_fetch_failed' });
      } finally {
        if (!params.signal?.aborted) setLoadingSettings(false);
      }
    },
    [apiError],
  );

  React.useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    let refreshTimer: number | undefined;
    const scheduleRefresh = () => {
      if (controller.signal.aborted) return;
      refreshTimer = window.setTimeout(() => {
        void refreshAlertChannels({ signal: controller.signal }).finally(scheduleRefresh);
      }, 15_000);
    };
    void fetchChannels({ signal: controller.signal }).finally(scheduleRefresh);
    void fetchSettings({ signal: controller.signal });
    return () => {
      if (refreshTimer !== undefined) window.clearTimeout(refreshTimer);
      controller.abort();
    };
  }, [enabled, fetchChannels, fetchSettings]);

  const saveSettings = React.useCallback(
    async (next: { enabled: boolean; channelIds: number[] }) => {
      if (!settings || savingSettings) return;
      const channelIds = [...new Set(next.channelIds)].filter((id) => id > 0);
      setSavingSettings(true);
      try {
        await adminApi.updateAlertSettings({
          enabled: next.enabled,
          channel_ids: channelIds,
        });
        setSettings((current) =>
          current
            ? {
                ...current,
                enabled: next.enabled,
                channel_ids: channelIds,
              }
            : current,
        );
        pushTopBanner(t('admin_alerts_settings_saved'), { tone: 'info' });
      } catch (error) {
        apiError(error, { key: 'admin_alerts_settings_save_failed' });
      } finally {
        setSavingSettings(false);
      }
    },
    [apiError, savingSettings, settings, t],
  );

  const toggleSettingsEnabled = React.useCallback(() => {
    if (!settings) return;
    void saveSettings({
      enabled: !settings.enabled,
      channelIds: settings.channel_ids,
    });
  }, [saveSettings, settings]);

  const toggleSettingsChannel = React.useCallback(
    (id: number) => {
      if (!settings) return;
      const selected = new Set(settings.channel_ids);
      if (selected.has(id)) selected.delete(id);
      else selected.add(id);
      void saveSettings({
        enabled: settings.enabled,
        channelIds: [...selected],
      });
    },
    [saveSettings, settings],
  );

  const openAdd = React.useCallback(() => {
    setModal({ editingChannelId: null });
  }, []);

  const openEdit = React.useCallback((channel: AlertChannel) => {
    setModal({ editingChannelId: channel.id });
  }, []);

  const closeModal = React.useCallback(() => {
    if (saving) return;
    setModal(null);
  }, [saving]);

  const toggleEnabled = React.useCallback(
    async (channel: AlertChannel) => {
      const nextEnabled = !channel.enabled;
      try {
        const didUpdate = await updateAlertChannelEnabled(channel.id, nextEnabled);
        if (!didUpdate) return;
      } catch (error) {
        apiError(error, t('admin_alerts_channels_toggle_failed', { name: channel.name }));
      }
    },
    [apiError, t],
  );

  const testChannel = React.useCallback(
    async (channel: AlertChannel) => {
      try {
        const didTest = await testAlertChannel(channel.id);
        if (!didTest) return;
        pushTopBanner(t('admin_alerts_channels_test_success'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_alerts_channels_test_failed'));
      }
    },
    [apiError, t],
  );

  const saveChannel = React.useCallback(
    async (input: AlertChannelForm) => {
      if (saving) return;
      const enabled = editingChannel?.enabled ?? true;
      const result = channelInputFromForm(input, enabled);
      if (!result.ok) {
        pushTopBanner(t(formIssueKeys[result.issue]), { tone: 'warning' });
        return;
      }

      try {
        const saveStatus = await saveAlertChannel(editingChannelId, result.input);
        if (saveStatus === 'busy') return;
        pushTopBanner(
          saveStatus === 'stale'
            ? t('admin_alerts_channels_saved_refresh_failed')
            : editingChannelId !== null
              ? t('admin_alerts_channels_update_success')
              : t('admin_alerts_channels_create_success'),
          { tone: saveStatus === 'stale' ? 'warning' : 'info' },
        );
        setModal(null);
      } catch (error) {
        apiError(
          error,
          editingChannelId !== null
            ? t('admin_alerts_channels_update_failed')
            : t('admin_alerts_channels_create_failed'),
        );
      }
    },
    [apiError, editingChannel, editingChannelId, saving, t],
  );

  const deleteChannel = React.useCallback(
    async (channel: AlertChannel) => {
      await confirmAction(
        {
          title: t('common_confirm'),
          message: t('admin_alerts_channels_delete_confirm', { name: channel.name }),
          confirmLabel: t('common_delete'),
          cancelLabel: t('common_cancel'),
          tone: 'danger',
        },
        async () => {
          try {
            const didDelete = await deleteAlertChannel(channel.id);
            if (!didDelete) return;
            if (settings?.channel_ids.includes(channel.id)) {
              await fetchSettings();
            }
            pushTopBanner(t('admin_alerts_channels_delete_success'), { tone: 'info' });
          } catch (error) {
            apiError(error, t('admin_alerts_channels_delete_failed'));
          }
        },
      );
    },
    [confirmAction, apiError, fetchSettings, settings, t],
  );

  const modalForm = React.useMemo(
    () => (editingChannel ? formFromChannel(editingChannel) : undefined),
    [editingChannel],
  );

  return {
    channels,
    loading,
    settings,
    loadingSettings,
    savingSettings,
    testingIds,
    togglingIds,
    saving,
    isModalOpen,
    editingChannel,
    editingChannelId,
    modalForm,
    openAdd,
    openEdit,
    closeModal,
    toggleEnabled,
    testChannel,
    saveChannel,
    deleteChannel,
    toggleSettingsEnabled,
    toggleSettingsChannel,
    refresh: fetchChannels,
  };
};
