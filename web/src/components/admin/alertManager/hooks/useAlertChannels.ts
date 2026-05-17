import React from 'react';
import { useI18n } from '@i18n';
import type { AlertChannel } from '@app-types/admin';
import * as adminApi from '@lib/adminApi';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import type { ConfirmAction } from '@hooks/useConfirmDialog';
import {
  channelInputFromForm,
  formFromChannel,
  type AlertChannelForm,
} from '@components/admin/alertManager/alertChannelForm';

const sortChannels = (items: AlertChannel[] | null | undefined): AlertChannel[] =>
  (items || []).slice().sort((a, b) => a.id - b.id);

export const useAlertChannels = ({
  enabled,
  confirmAction,
}: {
  enabled: boolean;
  confirmAction: ConfirmAction;
}) => {
  const { t } = useI18n();
  const pushBanner = useTopBanner();
  const apiError = useApiErrorHandler();
  const [channels, setChannels] = React.useState<AlertChannel[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [testingId, setTestingId] = React.useState<number | null>(null);
  const [togglingId, setTogglingId] = React.useState<number | null>(null);
  const [saving, setSaving] = React.useState(false);
  const [isModalOpen, setIsModalOpen] = React.useState(false);
  const [editingChannel, setEditingChannel] = React.useState<AlertChannel | null>(null);

  const fetchChannels = React.useCallback(async () => {
    setLoading(true);
    try {
      const items = await adminApi.fetchAlertChannels();
      setChannels(sortChannels(items));
    } catch (error) {
      apiError(error, t('admin_alerts_channels_fetch_failed'));
    } finally {
      setLoading(false);
    }
  }, [apiError, t]);

  React.useEffect(() => {
    if (!enabled) return;
    void fetchChannels();
  }, [enabled, fetchChannels]);

  const openAdd = React.useCallback(() => {
    setEditingChannel(null);
    setIsModalOpen(true);
  }, []);

  const openEdit = React.useCallback((channel: AlertChannel) => {
    setEditingChannel(channel);
    setIsModalOpen(true);
  }, []);

  const closeModal = React.useCallback(() => {
    if (saving) return;
    setIsModalOpen(false);
    setEditingChannel(null);
  }, [saving]);

  const toggleEnabled = React.useCallback(
    async (channel: AlertChannel) => {
      if (togglingId === channel.id) return;
      const nextEnabled = !channel.enabled;
      try {
        setTogglingId(channel.id);
        await adminApi.updateAlertChannelEnabled(channel.id, { enabled: nextEnabled });
        setChannels((prev) =>
          prev.map((item) => (item.id === channel.id ? { ...item, enabled: nextEnabled } : item)),
        );
      } catch (error) {
        apiError(error, t('admin_alerts_channels_toggle_failed', { name: channel.name }));
      } finally {
        setTogglingId(null);
      }
    },
    [apiError, t, togglingId],
  );

  const testChannel = React.useCallback(
    async (channel: AlertChannel) => {
      if (testingId === channel.id) return;
      try {
        setTestingId(channel.id);
        await adminApi.testAlertChannel(channel.id, {
          title: t('admin_alerts_channels_test_title'),
          message: t('admin_alerts_channels_test_message'),
        });
        pushBanner(t('admin_alerts_channels_test_success'), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_alerts_channels_test_failed'));
      } finally {
        setTestingId(null);
      }
    },
    [apiError, pushBanner, t, testingId],
  );

  const saveChannel = React.useCallback(
    async (input: AlertChannelForm) => {
      if (saving) return;
      const enabled = editingChannel?.enabled ?? true;
      const channelInput = channelInputFromForm(input, enabled);

      try {
        setSaving(true);
        if (editingChannel) {
          await adminApi.updateAlertChannel(editingChannel.id, channelInput);
          pushBanner(t('admin_alerts_channels_update_success'), { tone: 'info' });
        } else {
          await adminApi.createAlertChannel(channelInput);
          pushBanner(t('admin_alerts_channels_create_success'), { tone: 'info' });
        }
        setIsModalOpen(false);
        setEditingChannel(null);
        await fetchChannels();
      } catch (error) {
        apiError(
          error,
          editingChannel
            ? t('admin_alerts_channels_update_failed')
            : t('admin_alerts_channels_create_failed'),
        );
      } finally {
        setSaving(false);
      }
    },
    [editingChannel, fetchChannels, apiError, pushBanner, saving, t],
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
            await adminApi.deleteAlertChannel(channel.id);
            pushBanner(t('admin_alerts_channels_delete_success'), { tone: 'info' });
            await fetchChannels();
          } catch (error) {
            apiError(error, t('admin_alerts_channels_delete_failed'));
          }
        },
      );
    },
    [confirmAction, fetchChannels, apiError, pushBanner, t],
  );

  const modalForm = React.useMemo(
    () => (editingChannel ? formFromChannel(editingChannel) : undefined),
    [editingChannel],
  );

  return {
    channels,
    loading,
    testingId,
    togglingId,
    saving,
    isModalOpen,
    editingChannel,
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
