import React from 'react';
import { useI18n, type TranslationKey } from '@i18n';
import type { AlertChannel } from '@app-types/admin';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import type { ConfirmAction } from '@hooks/useConfirmDialog';
import {
  deleteAlertChannel,
  loadAlertChannels,
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
import { isActionOk } from '@utils/actionOutcome';
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

  React.useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    void fetchChannels({ signal: controller.signal });
    return () => {
      controller.abort();
    };
  }, [enabled, fetchChannels]);

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
        const updated = await updateAlertChannelEnabled(channel.id, nextEnabled);
        if (!isActionOk(updated)) return;
      } catch (error) {
        apiError(error, t('admin_alerts_channels_toggle_failed', { name: channel.name }));
      }
    },
    [apiError, t],
  );

  const testChannel = React.useCallback(
    async (channel: AlertChannel) => {
      try {
        const tested = await testAlertChannel(channel.id);
        if (!isActionOk(tested)) return;
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
        const saved = await saveAlertChannel(editingChannelId, result.input);
        if (!isActionOk(saved)) return;
        pushTopBanner(
          editingChannelId !== null
            ? t('admin_alerts_channels_update_success')
            : t('admin_alerts_channels_create_success'),
          { tone: 'info' },
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
            const deleted = await deleteAlertChannel(channel.id);
            if (!isActionOk(deleted)) return;
            pushTopBanner(t('admin_alerts_channels_delete_success'), { tone: 'info' });
          } catch (error) {
            apiError(error, t('admin_alerts_channels_delete_failed'));
          }
        },
      );
    },
    [confirmAction, apiError, t],
  );

  const modalForm = React.useMemo(
    () => (editingChannel ? formFromChannel(editingChannel) : undefined),
    [editingChannel],
  );

  return {
    channels,
    loading,
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
    refresh: fetchChannels,
  };
};
