import React from 'react';
import Edit2 from 'lucide-react/dist/esm/icons/edit-2';
import Trash2 from 'lucide-react/dist/esm/icons/trash-2';
import type { AlertRule } from '@app-types/admin';
import { useI18n } from '@i18n';
import IOSSwitch from '@components/ui/IOSSwitch';
import Input from '@components/ui/Input';
import { alertMetricName } from './alertLabels';

interface Props {
  rules: AlertRule[];
  loading: boolean;
  togglingId: number | null;
  onToggleEnabled: (rule: AlertRule) => void;
  renamingId: number | null;
  onRename: (rule: AlertRule, nextName: string) => void;
  onEdit: (rule: AlertRule) => void;
  onDelete: (id: number) => void;
}

const AlertRuleTable: React.FC<Props> = ({
  rules,
  loading,
  togglingId,
  onToggleEnabled,
  renamingId,
  onRename,
  onEdit,
  onDelete,
}) => {
  const { t } = useI18n();
  const [editingId, setEditingId] = React.useState<number | null>(null);
  const [draftName, setDraftName] = React.useState('');
  const [originalName, setOriginalName] = React.useState('');

  if (loading && rules.length === 0) {
    return (
      <div className="p-8 text-center text-sm text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
        {t('loading')}
      </div>
    );
  }

  const durationLabel = (seconds: number) => {
    if (seconds === 0) return t('admin_alerts_duration_now');
    if (seconds === 60) return t('admin_alerts_duration_1m');
    if (seconds === 300) return t('admin_alerts_duration_5m');
    return `${seconds}s`;
  };
  const cooldownLabel = (minutes: number) => (minutes <= 0 ? '-' : `${minutes}m`);

  const corePlusLabel = (offset: number) => {
    if (offset === 0) return t('admin_alerts_cpu_cores');
    const sign = offset > 0 ? '+' : '';
    return `${t('admin_alerts_cpu_cores')}${sign}${offset}`;
  };

  const startEditName = (rule: AlertRule) => {
    setEditingId(rule.id);
    setDraftName(rule.name);
    setOriginalName(rule.name);
  };

  const commitName = (rule: AlertRule) => {
    if (editingId !== rule.id) return;
    const trimmed = draftName.trim();
    if (!trimmed || trimmed === originalName.trim()) {
      setDraftName(originalName);
      setEditingId(null);
      return;
    }
    onRename(rule, trimmed);
    setEditingId(null);
  };

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm text-left bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
        <thead className="bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-default) text-xs font-semibold border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
          <tr>
            <th className="px-3 py-2.5 w-16">ID</th>
            <th className="px-3 py-2.5 min-w-37.5">{t('admin_alerts_col_name')}</th>
            <th className="px-3 py-2.5 w-20">{t('admin_alerts_col_enabled')}</th>
            <th className="px-3 py-2.5 min-w-25">{t('admin_alerts_col_metric')}</th>
            <th className="px-3 py-2.5 min-w-35">{t('admin_alerts_col_condition')}</th>
            <th className="px-3 py-2.5 w-28">{t('admin_alerts_col_mode')}</th>
            <th className="px-3 py-2.5 w-28">{t('admin_alerts_col_duration')}</th>
            <th className="px-3 py-2.5 w-24">{t('admin_alerts_col_cooldown')}</th>
            <th className="px-3 py-2.5 text-right w-24"></th>
          </tr>
        </thead>
        <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
          {rules.length === 0 ? (
            <tr>
              <td
                colSpan={9}
                className="px-4 py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-action-muted)"
              >
                {t('no_data')}
              </td>
            </tr>
          ) : (
            rules.map((rule) => (
              <tr
                key={rule.id}
                className="hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle) transition-colors"
              >
                <td className="px-3 py-2.5 font-mono text-xs text-(--theme-fg-muted)">{rule.id}</td>
                <td className="px-3 py-2.5 font-medium text-(--theme-fg-strong) dark:text-(--theme-fg-control-hover)">
                  {editingId === rule.id ? (
                    <Input
                      autoFocus
                      enterKeyHint="done"
                      value={draftName}
                      disabled={renamingId === rule.id}
                      onChange={(event) => setDraftName(event.target.value)}
                      onBlur={() => commitName(rule)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' && !event.nativeEvent.isComposing) {
                          event.preventDefault();
                          event.currentTarget.blur();
                        }
                        if (event.key === 'Escape') {
                          setDraftName(originalName);
                          setEditingId(null);
                        }
                      }}
                    />
                  ) : (
                    <button
                      type="button"
                      className="ui-focus-ring rounded-sm font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-default) hover:text-(--theme-fg-interactive-strong) dark:hover:text-(--theme-fg-interactive-hover) hover:underline transition-colors"
                      onClick={() => startEditName(rule)}
                    >
                      {rule.name}
                    </button>
                  )}
                </td>
                <td className="px-3 py-2.5">
                  <IOSSwitch
                    size="sm"
                    checked={rule.enabled}
                    disabled={togglingId === rule.id}
                    onChange={() => onToggleEnabled(rule)}
                  />
                </td>
                <td className="px-3 py-2.5 text-xs" title={rule.metric}>
                  {alertMetricName(rule.metric, t)}
                </td>
                <td className="px-3 py-2.5 font-mono text-xs">
                  <span className="text-(--theme-fg-interactive-strong) dark:text-(--theme-fg-interactive-hover) font-bold mr-1">
                    {rule.operator}
                  </span>
                  {rule.threshold_mode === 'core_plus'
                    ? corePlusLabel(rule.threshold_offset)
                    : rule.threshold}
                </td>
                <td className="px-3 py-2.5 text-(--theme-fg-muted) dark:text-(--theme-fg-muted)">
                  <span className="inline-flex px-1.5 py-0.5 text-xs font-medium rounded-md border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-muted) dark:bg-(--theme-canvas-muted) text-(--theme-fg-muted) dark:text-(--theme-fg-muted)">
                    {rule.threshold_mode}
                  </span>
                </td>
                <td className="px-3 py-2.5 text-(--theme-fg-muted) dark:text-(--theme-fg-muted)">
                  {durationLabel(rule.duration_sec)}
                </td>
                <td className="px-3 py-2.5 text-(--theme-fg-muted) dark:text-(--theme-fg-muted)">
                  {cooldownLabel(rule.cooldown_min)}
                </td>
                <td className="px-3 py-2.5 text-right">
                  <div className="flex items-center justify-end gap-1">
                    <button
                      type="button"
                      onClick={() => onEdit(rule)}
                      className="ui-focus-ring p-1.5 text-(--theme-fg-action-muted) hover:text-(--theme-fg-interactive) dark:hover:text-(--theme-fg-interactive-hover) hover:bg-(--theme-bg-interactive-hover) dark:hover:bg-(--theme-bg-interactive-hover) rounded transition-colors"
                      aria-label={t('common_edit')}
                    >
                      <Edit2 className="size-4" />
                    </button>
                    <button
                      type="button"
                      onClick={() => onDelete(rule.id)}
                      className="ui-focus-ring p-1.5 text-(--theme-fg-danger-muted) hover:text-(--theme-fg-danger) dark:text-(--theme-fg-danger) dark:hover:text-(--theme-fg-danger-soft) hover:bg-(--theme-bg-danger-muted) dark:hover:bg-(--theme-bg-danger-subtle) rounded transition-colors"
                      aria-label={t('common_delete')}
                    >
                      <Trash2 className="size-4" />
                    </button>
                  </div>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
};

export default AlertRuleTable;
