import React from 'react';
import { useI18n } from '@i18n';
import type { AlertRule, AlertRuleInput } from '@app-types/admin';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import type { ConfirmAction, ConfirmRequest } from '@hooks/useConfirmDialog';
import {
  deleteAlertRule,
  loadAlertRules,
  renameAlertRule,
  saveAlertRule,
  toggleAlertRuleEnabled,
  useAlertRulesStore,
} from '@stores/alertRulesStore';
import { isActionOk } from '@utils/actionOutcome';
import { isCanceledRequestError } from '@utils/errors';

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
  const apiError = useApiErrorHandler();
  const rules = useAlertRulesStore((state) => state.rules);
  const loading = useAlertRulesStore((state) => state.loading);
  const saving = useAlertRulesStore((state) => state.saving);
  const togglingIds = useAlertRulesStore((state) => state.togglingIds);
  const renamingIds = useAlertRulesStore((state) => state.renamingIds);
  const [modal, setModal] = React.useState<{ editingRuleId: number | null } | null>(null);
  const isModalOpen = modal !== null;
  const editingRuleId = modal?.editingRuleId ?? null;
  const editingRule = React.useMemo(
    () => rules.find((rule) => rule.id === editingRuleId) ?? null,
    [editingRuleId, rules],
  );

  React.useEffect(() => {
    if (!enabled && isModalOpen && !saving) setModal(null);
  }, [enabled, isModalOpen, saving]);

  React.useEffect(() => {
    if (!isModalOpen || editingRuleId === null || editingRule || loading || saving) return;
    setModal(null);
  }, [editingRule, editingRuleId, isModalOpen, loading, saving]);

  const fetchRules = React.useCallback(
    async (params: { signal?: AbortSignal } = {}) => {
      try {
        await loadAlertRules(params);
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_alerts_rules_fetch_failed' });
      }
    },
    [apiError],
  );

  React.useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    void fetchRules({ signal: controller.signal });
    return () => {
      controller.abort();
    };
  }, [enabled, fetchRules]);

  const openAdd = React.useCallback(() => {
    setModal({ editingRuleId: null });
  }, []);

  const openEdit = React.useCallback((rule: AlertRule) => {
    setModal({ editingRuleId: rule.id });
  }, []);

  const closeModal = React.useCallback(() => {
    setModal(null);
  }, []);

  const afterSave = React.useCallback(() => {
    setModal(null);
  }, []);

  const saveRule = React.useCallback(
    async (id: number | null, input: AlertRuleInput): Promise<boolean> => {
      try {
        const saved = await saveAlertRule(id, input);
        if (!isActionOk(saved)) return false;
        pushTopBanner(t('admin_alerts_toast_saved'), { tone: 'info' });
        return true;
      } catch (error) {
        apiError(error, t('admin_alerts_toast_save_failed'));
        return false;
      }
    },
    [apiError, t],
  );

  const toggleEnabled = React.useCallback(
    async (rule: AlertRule) => {
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
        const updated = await toggleAlertRuleEnabled(rule.id, !rule.enabled);
        if (!isActionOk(updated)) return;
        pushTopBanner(
          !rule.enabled
            ? t('admin_alerts_toast_enabled', { name: rule.name })
            : t('admin_alerts_toast_disabled', { name: rule.name }),
          { tone: 'info' },
        );
      } catch (error) {
        apiError(error, t('admin_alerts_toast_toggle_failed', { name: rule.name }));
      }
    },
    [apiError, confirm, t],
  );

  const rename = React.useCallback(
    async (rule: AlertRule, nextName: string) => {
      const ok = await confirm({
        title: t('common_confirm'),
        message: t('admin_alerts_confirm_rename_rule', { name: rule.name, next: nextName }),
        confirmLabel: t('common_save_changes'),
        cancelLabel: t('common_cancel'),
        tone: 'default',
      });
      if (!ok) return;
      try {
        const updated = await renameAlertRule(rule.id, nextName);
        if (!isActionOk(updated)) return;
        pushTopBanner(t('admin_alerts_toast_renamed', { name: nextName }), { tone: 'info' });
      } catch (error) {
        apiError(error, t('admin_alerts_toast_rename_failed', { name: rule.name }));
      }
    },
    [apiError, confirm, t],
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
            const deleted = await deleteAlertRule(id);
            if (!isActionOk(deleted)) return;
            pushTopBanner(t('admin_alerts_toast_deleted', { name: deletedName }), { tone: 'info' });
          } catch (error) {
            apiError(error, t('admin_alerts_toast_delete_failed', { name: deletedName }));
          }
        },
      );
    },
    [apiError, confirmAction, rules, t],
  );

  return {
    rules,
    loading,
    saving,
    togglingIds,
    renamingIds,
    isModalOpen,
    editingRule,
    openAdd,
    openEdit,
    closeModal,
    afterSave,
    saveRule,
    toggleEnabled,
    rename,
    deleteRule,
    refresh: fetchRules,
  };
};
