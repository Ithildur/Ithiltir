import React from 'react';
import { useI18n } from '@i18n';
import type { AlertRule } from '@app-types/admin';
import * as adminApi from '@lib/adminApi';
import { useTopBanner } from '@components/ui/TopBannerStack';
import type { ConfirmAction, ConfirmRequest } from '@hooks/useConfirmDialog';

const sortRules = (items: AlertRule[] | null | undefined): AlertRule[] =>
  (items || []).slice().sort((a, b) => a.id - b.id);

export const useAlertRules = ({
  enabled,
  confirm,
  confirmAction,
}: {
  enabled: boolean;
  confirm: ConfirmRequest;
  confirmAction: ConfirmAction;
}) => {
  const { t } = useI18n();
  const pushBanner = useTopBanner();
  const [rules, setRules] = React.useState<AlertRule[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [isModalOpen, setIsModalOpen] = React.useState(false);
  const [editingRule, setEditingRule] = React.useState<AlertRule | null>(null);
  const [togglingId, setTogglingId] = React.useState<number | null>(null);
  const [renamingId, setRenamingId] = React.useState<number | null>(null);

  const fetchRules = React.useCallback(async () => {
    setLoading(true);
    try {
      const data = await adminApi.fetchAlertRules();
      setRules(sortRules(data));
    } catch (error) {
      console.error('Failed to fetch alert rules', error);
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    if (!enabled) return;
    void fetchRules();
  }, [enabled, fetchRules]);

  const openAdd = React.useCallback(() => {
    setEditingRule(null);
    setIsModalOpen(true);
  }, []);

  const openEdit = React.useCallback((rule: AlertRule) => {
    setEditingRule(rule);
    setIsModalOpen(true);
  }, []);

  const closeModal = React.useCallback(() => {
    setIsModalOpen(false);
  }, []);

  const afterSave = React.useCallback(() => {
    setIsModalOpen(false);
    void fetchRules();
  }, [fetchRules]);

  const toggleEnabled = React.useCallback(
    async (rule: AlertRule) => {
      if (togglingId === rule.id) return;
      const ok = await confirm({
        title: t('common_confirm'),
        message: rule.enabled
          ? t('admin_alerts_confirm_disable_rule', { name: rule.name })
          : t('admin_alerts_confirm_enable_rule', { name: rule.name }),
        confirmLabel: t('common_confirm'),
        cancelLabel: t('common_cancel'),
        tone: rule.enabled ? 'danger' : 'default',
      });
      if (!ok) return;
      try {
        setTogglingId(rule.id);
        await adminApi.updateAlertRule(rule.id, { enabled: !rule.enabled });
        await fetchRules();
        pushBanner(
          !rule.enabled
            ? t('admin_alerts_toast_enabled', { name: rule.name })
            : t('admin_alerts_toast_disabled', { name: rule.name }),
          { tone: 'info' },
        );
      } catch (error) {
        console.error('Failed to toggle rule enabled', error);
        pushBanner(t('admin_alerts_toast_toggle_failed', { name: rule.name }), { tone: 'error' });
      } finally {
        setTogglingId(null);
      }
    },
    [confirm, fetchRules, pushBanner, t, togglingId],
  );

  const rename = React.useCallback(
    async (rule: AlertRule, nextName: string) => {
      if (renamingId === rule.id) return;
      const ok = await confirm({
        title: t('common_confirm'),
        message: t('admin_alerts_confirm_rename_rule', { name: rule.name, next: nextName }),
        confirmLabel: t('common_save_changes'),
        cancelLabel: t('common_cancel'),
        tone: 'default',
      });
      if (!ok) return;
      try {
        setRenamingId(rule.id);
        await adminApi.updateAlertRule(rule.id, { name: nextName });
        pushBanner(t('admin_alerts_toast_renamed', { name: nextName }), { tone: 'info' });
      } catch (error) {
        console.error('Failed to rename rule', error);
        pushBanner(t('admin_alerts_toast_rename_failed', { name: rule.name }), { tone: 'error' });
      } finally {
        try {
          await fetchRules();
        } catch (error) {
          console.error('Failed to refresh rules after rename', error);
        }
        setRenamingId(null);
      }
    },
    [confirm, fetchRules, pushBanner, renamingId, t],
  );

  const deleteRule = React.useCallback(
    async (id: number) => {
      const deletedName = rules.find((item) => item.id === id)?.name ?? String(id);
      await confirmAction(
        {
          title: t('common_confirm'),
          message: t('admin_alerts_delete_confirm'),
          confirmLabel: t('common_delete'),
          cancelLabel: t('common_cancel'),
          tone: 'danger',
        },
        async () => {
          try {
            await adminApi.deleteAlertRule(id);
            await fetchRules();
            pushBanner(t('admin_alerts_toast_deleted', { name: deletedName }), { tone: 'info' });
          } catch (error) {
            console.error('Failed to delete rule', error);
            pushBanner(t('admin_alerts_toast_delete_failed', { name: deletedName }), {
              tone: 'error',
            });
          }
        },
      );
    },
    [confirmAction, fetchRules, pushBanner, rules, t],
  );

  return {
    rules,
    loading,
    togglingId,
    renamingId,
    isModalOpen,
    editingRule,
    openAdd,
    openEdit,
    closeModal,
    afterSave,
    toggleEnabled,
    rename,
    deleteRule,
    refresh: fetchRules,
  };
};
