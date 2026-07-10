import React from 'react';
import Search from 'lucide-react/dist/esm/icons/search';
import Card from '@components/ui/Card';
import MultiSelectFilter from '@components/ui/MultiSelectFilter';
import SearchInput from '@components/ui/SearchInput';
import type { AlertMountNode, AlertMountRule } from '@app-types/admin';
import { useI18n } from '@i18n';
import { alertRuleName, alertSummaryMetricName } from './alertLabels';
import { AlertMountsBatchModal, AlertMountsCustomModal } from './AlertMountsModals';
import { AlertMountsTable } from './AlertMountsTable';
import { useAlertMountsPanel } from './useAlertMountsPanel';

interface Props {
  rules: AlertMountRule[];
  nodes: AlertMountNode[];
  loading: boolean;
  saving: boolean;
  onSetMounts: (ruleIds: number[], serverIds: number[], mounted: boolean) => Promise<boolean>;
}

const AlertMountsPanel: React.FC<Props> = ({ rules, nodes, loading, saving, onSetMounts }) => {
  const { t } = useI18n();
  const customTitleId = React.useId();
  const batchTitleId = React.useId();

  const ruleName = React.useCallback((rule: AlertMountRule) => alertRuleName(rule, t), [t]);
  const summaryMetricName = React.useCallback(
    (rule: AlertMountRule) => alertSummaryMetricName(rule.metric, t),
    [t],
  );

  const { filter, table, batch, custom } = useAlertMountsPanel({
    rules,
    nodes,
    saving,
    onSetMounts,
  });

  const ruleFilterItems = React.useMemo(
    () =>
      rules.map((rule) => ({
        id: rule.id,
        label: ruleName(rule),
        trailing: !rule.enabled ? (
          <span className="shrink-0 text-[11px] text-(--theme-bg-danger-emphasis)">
            {t('admin_alerts_mounts_rule_disabled')}
          </span>
        ) : undefined,
      })),
    [ruleName, rules, t],
  );

  const batchTitle =
    batch.mode === 'mount'
      ? t('admin_alerts_mounts_apply')
      : batch.mode === 'unmount'
        ? t('admin_alerts_mounts_cancel')
        : '';

  if (loading && nodes.length === 0) {
    return (
      <Card className="bg-(--theme-bg-default) dark:bg-(--theme-bg-default) p-8 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
        {t('loading')}
      </Card>
    );
  }

  return (
    <div className="space-y-4 md:space-y-6">
      <div className="flex flex-col md:flex-row justify-between gap-3 md:gap-4">
        <div className="flex w-full md:w-auto md:flex-1 gap-2">
          <SearchInput
            icon={Search}
            placeholder={t('admin_alerts_mounts_search_placeholder')}
            aria-label={t('admin_alerts_mounts_search_placeholder')}
            value={filter.search}
            onChange={(event) => filter.setSearch(event.target.value)}
            wrapperClassName="flex-1 max-w-md"
          />
          <MultiSelectFilter
            items={ruleFilterItems}
            selectedIds={filter.selectedRuleIds}
            onChange={filter.setSelectedRuleIds}
            label={t('admin_alerts_mounts_rules')}
            title={t('admin_alerts_mounts_rules')}
            emptyLabel={t('no_data')}
            clearLabel={t('common_clear')}
            closeLabel={t('common_close')}
            align="left"
          />
        </div>
      </div>

      <AlertMountsTable
        nodes={table.nodes}
        visibleBuiltinRules={table.visibleBuiltinRules}
        showCustomRules={table.showCustomRules}
        allRuleIds={table.allRuleIds}
        selectedNodeIds={table.selectedNodeIds}
        selectedNodes={table.selectedNodes}
        allVisibleSelected={table.allVisibleSelected}
        someVisibleSelected={table.someVisibleSelected}
        tableColumnCount={table.tableColumnCount}
        saving={saving}
        canOpenBatch={table.canOpenBatch}
        onToggleVisibleNodes={table.toggleVisibleNodes}
        onToggleSelectedNode={table.toggleSelectedNode}
        onOpenCustomNode={table.openCustomNode}
        onStartBatch={table.startBatch}
        onSetMounts={onSetMounts}
        ruleName={ruleName}
        mounted={table.mounted}
        customMountedCount={table.customMountedCount}
      />

      <AlertMountsBatchModal
        isOpen={batch.isOpen}
        titleId={batchTitleId}
        title={batchTitle}
        rules={batch.rules}
        selectedNodeCount={batch.selectedNodeCount}
        selectedRuleCount={batch.selectedRuleCount}
        selectedRules={batch.selectedRules}
        allSelected={batch.allSelected}
        someSelected={batch.someSelected}
        canSubmit={batch.canSubmit}
        onClose={batch.close}
        onSubmit={batch.submit}
        onToggleAll={batch.toggleAllRules}
        onToggleRule={batch.toggleRule}
        ruleName={ruleName}
        metricName={summaryMetricName}
      />

      <AlertMountsCustomModal
        titleId={customTitleId}
        node={custom.node}
        rules={custom.rules}
        saving={saving}
        onClose={custom.close}
        onSetMounts={onSetMounts}
        ruleName={ruleName}
        metricName={summaryMetricName}
        mounted={table.mounted}
      />
    </div>
  );
};

export default AlertMountsPanel;
