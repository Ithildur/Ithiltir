import React from 'react';
import Search from 'lucide-react/dist/esm/icons/search';
import SlidersHorizontal from 'lucide-react/dist/esm/icons/sliders-horizontal';
import Card from '@components/ui/Card';
import Button from '@components/ui/Button';
import Checkbox from '@components/ui/Checkbox';
import Input from '@components/ui/Input';
import IOSSwitch from '@components/ui/IOSSwitch';
import MultiSelectFilter from '@components/ui/MultiSelectFilter';
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@components/ui/Modal';
import type { AlertMountNode, AlertMountRule } from '@app-types/admin';
import { useI18n } from '@i18n';
import { alertMetricName, alertRuleName } from './alertLabels';

interface Props {
  rules: AlertMountRule[];
  nodes: AlertMountNode[];
  loading: boolean;
  saving: boolean;
  onSetMounts: (ruleIds: number[], serverIds: number[], mounted: boolean) => Promise<boolean>;
}

type BatchMode = 'mount' | 'unmount';

const AlertMountsPanel: React.FC<Props> = ({ rules, nodes, loading, saving, onSetMounts }) => {
  const { t } = useI18n();
  const [search, setSearch] = React.useState('');
  const [selectedNodes, setSelectedNodes] = React.useState<Set<number>>(new Set());
  const [filteredRules, setFilteredRules] = React.useState<Set<number>>(new Set());
  const [customNodeId, setCustomNodeId] = React.useState<number | null>(null);
  const [batchMode, setBatchMode] = React.useState<BatchMode | null>(null);
  const [batchRules, setBatchRules] = React.useState<Set<number>>(new Set());
  const customTitleId = React.useId();
  const batchTitleId = React.useId();

  const ruleName = React.useCallback((rule: AlertMountRule) => alertRuleName(rule, t), [t]);
  const metricName = React.useCallback(
    (rule: AlertMountRule) => alertMetricName(rule.metric, t),
    [t],
  );

  const mountMap = React.useMemo(() => {
    const byNode = new Map<number, Map<number, boolean>>();
    for (const node of nodes) {
      const row = new Map<number, boolean>();
      for (const mount of node.mounts) {
        row.set(mount.rule_id, mount.mounted);
      }
      byNode.set(node.id, row);
    }
    return byNode;
  }, [nodes]);

  const builtinRules = React.useMemo(() => rules.filter((rule) => rule.builtin), [rules]);
  const customRules = React.useMemo(() => rules.filter((rule) => !rule.builtin), [rules]);
  const allRuleIds = React.useMemo(() => rules.map((rule) => rule.id), [rules]);
  const batchList = React.useMemo(
    () => [...builtinRules, ...customRules],
    [builtinRules, customRules],
  );
  const batchListIds = React.useMemo(() => batchList.map((rule) => rule.id), [batchList]);
  const filteredRuleIds = React.useMemo(() => Array.from(filteredRules), [filteredRules]);
  const hasRuleFilter = filteredRules.size > 0;
  const visibleBuiltinRules = React.useMemo(
    () => builtinRules.filter((rule) => !hasRuleFilter || filteredRules.has(rule.id)),
    [builtinRules, filteredRules, hasRuleFilter],
  );
  const visibleCustomRules = React.useMemo(
    () => customRules.filter((rule) => !hasRuleFilter || filteredRules.has(rule.id)),
    [customRules, filteredRules, hasRuleFilter],
  );
  const showCustomRules = !hasRuleFilter || visibleCustomRules.length > 0;
  const customNode = React.useMemo(
    () => nodes.find((node) => node.id === customNodeId) ?? null,
    [customNodeId, nodes],
  );

  const visibleNodes = React.useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return nodes;
    return nodes.filter((node) =>
      [node.name, node.hostname, node.ip ?? '', String(node.id)]
        .join(' ')
        .toLowerCase()
        .includes(keyword),
    );
  }, [nodes, search]);

  const visibleNodeIds = React.useMemo(() => visibleNodes.map((node) => node.id), [visibleNodes]);
  const allVisibleSelected =
    visibleNodeIds.length > 0 && visibleNodeIds.every((id) => selectedNodes.has(id));
  const someVisibleSelected = visibleNodeIds.some((id) => selectedNodes.has(id));

  const toggleNode = (id: number) => {
    setSelectedNodes((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const toggleVisibleNodes = () => {
    setSelectedNodes((prev) => {
      const next = new Set(prev);
      if (allVisibleSelected) {
        for (const id of visibleNodeIds) next.delete(id);
      } else {
        for (const id of visibleNodeIds) next.add(id);
      }
      return next;
    });
  };

  const selectedNodeIds = React.useMemo(() => Array.from(selectedNodes), [selectedNodes]);
  const selectedBatchRuleIds = React.useMemo(() => Array.from(batchRules), [batchRules]);
  const allBatchSelected =
    batchListIds.length > 0 && batchListIds.every((id) => batchRules.has(id));
  const someBatchSelected = batchListIds.some((id) => batchRules.has(id));
  const canOpenBatch = selectedNodeIds.length > 0 && batchListIds.length > 0 && !saving;
  const canSubmitBatch =
    batchMode !== null && selectedNodeIds.length > 0 && selectedBatchRuleIds.length > 0 && !saving;
  const batchTitle =
    batchMode === 'mount'
      ? t('admin_alerts_mounts_apply')
      : batchMode === 'unmount'
        ? t('admin_alerts_mounts_cancel')
        : '';
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

  const mounted = (nodeId: number, ruleId: number) => mountMap.get(nodeId)?.get(ruleId) ?? false;
  const customMountedCount = (nodeId: number) =>
    visibleCustomRules.filter((rule) => rule.enabled && mounted(nodeId, rule.id)).length;
  const tableColumnCount = visibleBuiltinRules.length + (showCustomRules ? 1 : 0) + 3;

  const toggleBatchRule = (id: number) => {
    setBatchRules((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const toggleAllBatchRules = () => {
    setBatchRules(() => (allBatchSelected ? new Set() : new Set(batchListIds)));
  };

  const closeBatch = () => {
    setBatchMode(null);
    setBatchRules(new Set());
  };

  const openBatch = (mode: BatchMode) => {
    if (!canOpenBatch) return;
    setBatchMode(mode);
    setBatchRules(new Set());
  };

  const submitBatch = async () => {
    if (!batchMode || !canSubmitBatch) return;
    const saved = await onSetMounts(selectedBatchRuleIds, selectedNodeIds, batchMode === 'mount');
    if (saved) {
      closeBatch();
    }
  };

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
          <Input
            icon={Search}
            placeholder={t('admin_alerts_mounts_search_placeholder')}
            aria-label={t('admin_alerts_mounts_search_placeholder')}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            data-search-input="true"
            wrapperClassName="flex-1 max-w-md"
          />
          <MultiSelectFilter
            items={ruleFilterItems}
            selectedIds={filteredRuleIds}
            onChange={(ids) => setFilteredRules(new Set(ids))}
            label={t('admin_alerts_mounts_rules')}
            title={t('admin_alerts_mounts_rules')}
            emptyLabel={t('no_data')}
            clearLabel={t('common_clear')}
            closeLabel={t('common_close')}
            align="left"
          />
        </div>
      </div>

      <Card className="overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
            <thead className="bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-default) text-xs font-semibold border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
              <tr>
                <th className="px-3 py-2.5 w-10 align-middle">
                  <div className="flex h-5 items-center">
                    <Checkbox
                      checked={allVisibleSelected}
                      indeterminate={!allVisibleSelected && someVisibleSelected}
                      onChange={toggleVisibleNodes}
                      aria-label={t('admin_alerts_mounts_select_visible')}
                    />
                  </div>
                </th>
                <th className="px-3 py-2.5 min-w-48 align-middle">
                  <div className="flex h-5 items-center">{t('admin_nodes_column_node')}</div>
                </th>
                {visibleBuiltinRules.map((rule) => (
                  <th key={rule.id} className="px-3 py-2.5 min-w-36 align-middle">
                    <div className="flex h-5 items-center">{ruleName(rule)}</div>
                  </th>
                ))}
                {showCustomRules && (
                  <th className="px-3 py-2.5 min-w-40 align-middle">
                    <div className="flex h-5 items-center">
                      {t('admin_alerts_mounts_custom_rules')}
                    </div>
                  </th>
                )}
                <th className="px-3 py-2.5 text-right w-36" aria-label={t('common_actions')} />
              </tr>
            </thead>
            <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
              {visibleNodes.length === 0 ? (
                <tr>
                  <td
                    colSpan={tableColumnCount}
                    className="px-4 py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)"
                  >
                    {t('no_data')}
                  </td>
                </tr>
              ) : (
                visibleNodes.map((node) => (
                  <tr
                    key={node.id}
                    className="hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle) transition-colors"
                  >
                    <td className="px-3 py-2.5 align-middle">
                      <Checkbox
                        checked={selectedNodes.has(node.id)}
                        onChange={() => toggleNode(node.id)}
                        aria-label={t('admin_alerts_mounts_select_node', { name: node.name })}
                      />
                    </td>
                    <td className="px-3 py-2.5">
                      <div className="flex flex-col">
                        <span className="font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-default)">
                          {node.name}
                        </span>
                        <span className="text-[11px] text-(--theme-fg-muted-alt) font-mono">
                          {node.ip || node.hostname || `ID: ${node.id}`}
                        </span>
                      </div>
                    </td>
                    {visibleBuiltinRules.map((rule) => {
                      const isMounted = mounted(node.id, rule.id);
                      return (
                        <td key={rule.id} className="px-3 py-2.5">
                          <IOSSwitch
                            size="sm"
                            checked={isMounted}
                            disabled={saving}
                            onChange={() => {
                              void onSetMounts([rule.id], [node.id], !isMounted);
                            }}
                          />
                        </td>
                      );
                    })}
                    {showCustomRules && (
                      <td className="px-3 py-2.5">
                        <button
                          type="button"
                          onClick={() => setCustomNodeId(node.id)}
                          className="inline-flex items-center rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-muted) px-2 py-1 text-xs font-semibold text-(--theme-fg-muted) hover:text-(--theme-fg-default) dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle)"
                        >
                          {t('admin_alerts_mounts_custom_enabled_count', {
                            count: String(customMountedCount(node.id)),
                          })}
                        </button>
                      </td>
                    )}
                    <td className="px-3 py-2.5 text-right">
                      <button
                        type="button"
                        disabled={saving || allRuleIds.length === 0}
                        onClick={() => {
                          void onSetMounts(allRuleIds, [node.id], false);
                        }}
                        className="rounded-md px-2 py-1 text-xs font-semibold text-(--theme-bg-danger-emphasis) hover:bg-[#ffebe9] disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-(--theme-canvas-muted)"
                      >
                        {t('admin_alerts_mounts_disable_all')}
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        {selectedNodeIds.length > 0 && (
          <div className="flex flex-col gap-2 border-t border-(--theme-border-subtle) bg-(--theme-bg-muted) px-4 py-3 text-xs text-(--theme-fg-muted) sm:flex-row sm:items-center sm:justify-between dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle) dark:text-(--theme-fg-muted)">
            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="secondary"
                disabled={!canOpenBatch}
                onClick={() => openBatch('mount')}
              >
                {t('admin_alerts_mounts_apply')}
              </Button>
              <Button
                variant="secondary"
                disabled={!canOpenBatch}
                onClick={() => openBatch('unmount')}
              >
                {t('admin_alerts_mounts_cancel')}
              </Button>
            </div>
            <div className="flex flex-wrap items-center gap-3 sm:justify-end">
              <span>
                {t('admin_alerts_mounts_selected_nodes', {
                  count: String(selectedNodeIds.length),
                })}
              </span>
            </div>
          </div>
        )}
      </Card>

      <Modal
        isOpen={batchMode !== null}
        onClose={closeBatch}
        maxWidth="max-w-3xl"
        ariaLabelledby={batchTitleId}
      >
        <ModalHeader
          id={batchTitleId}
          title={batchTitle}
          icon={
            <SlidersHorizontal className="text-(--theme-border-underline-nav-active)" size={20} />
          }
          onClose={closeBatch}
        />
        <ModalBody className="space-y-3">
          <div className="flex flex-wrap items-center gap-3 text-xs text-(--theme-fg-muted)">
            <span>
              {t('admin_alerts_mounts_selected_nodes', {
                count: String(selectedNodeIds.length),
              })}
            </span>
            <span>
              {t('admin_alerts_mounts_selected_rules', {
                count: String(selectedBatchRuleIds.length),
              })}
            </span>
          </div>
          {batchList.length === 0 ? (
            <div className="rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) px-3 py-6 text-center text-sm text-(--theme-fg-muted)">
              {t('no_data')}
            </div>
          ) : (
            <div className="overflow-hidden rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default)">
              <table className="w-full text-sm text-left bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
                <thead className="bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-default) text-xs font-semibold border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
                  <tr>
                    <th className="px-3 py-2.5 w-10 align-middle">
                      <div className="flex h-5 items-center">
                        <Checkbox
                          checked={allBatchSelected}
                          indeterminate={!allBatchSelected && someBatchSelected}
                          onChange={toggleAllBatchRules}
                          aria-label={t('admin_alerts_mounts_select_all_rules')}
                        />
                      </div>
                    </th>
                    <th className="px-3 py-2.5">{t('admin_alerts_col_name')}</th>
                    <th className="px-3 py-2.5">{t('admin_alerts_col_metric')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
                  {batchList.map((rule) => (
                    <tr
                      key={rule.id}
                      className="hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle) transition-colors"
                    >
                      <td className="px-3 py-2.5 align-middle">
                        <Checkbox
                          checked={batchRules.has(rule.id)}
                          onChange={() => toggleBatchRule(rule.id)}
                          aria-label={t('admin_alerts_mounts_select_rule', {
                            name: ruleName(rule),
                          })}
                        />
                      </td>
                      <td className="px-3 py-2.5">
                        <div className="flex flex-col gap-1">
                          <span className="font-semibold text-(--theme-fg-default)">
                            {ruleName(rule)}
                          </span>
                          {!rule.enabled && (
                            <span className="text-[11px] text-(--theme-bg-danger-emphasis)">
                              {t('admin_alerts_mounts_rule_disabled')}
                            </span>
                          )}
                        </div>
                      </td>
                      <td
                        className="px-3 py-2.5 text-xs text-(--theme-fg-muted)"
                        title={rule.metric}
                      >
                        {metricName(rule)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </ModalBody>
        <ModalFooter>
          <Button variant="secondary" onClick={closeBatch}>
            {t('common_cancel')}
          </Button>
          <Button disabled={!canSubmitBatch} onClick={() => void submitBatch()}>
            {batchTitle}
          </Button>
        </ModalFooter>
      </Modal>

      <Modal
        isOpen={Boolean(customNode)}
        onClose={() => setCustomNodeId(null)}
        maxWidth="max-w-2xl"
        ariaLabelledby={customTitleId}
      >
        <ModalHeader
          id={customTitleId}
          title={t('admin_alerts_mounts_custom_modal_title', {
            name: customNode?.name ?? '',
          })}
          icon={
            <SlidersHorizontal className="text-(--theme-border-underline-nav-active)" size={20} />
          }
          onClose={() => setCustomNodeId(null)}
        />
        <ModalBody className="space-y-3">
          {visibleCustomRules.length === 0 ? (
            <div className="rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) px-3 py-6 text-center text-sm text-(--theme-fg-muted)">
              {t('admin_alerts_mounts_custom_empty')}
            </div>
          ) : (
            <div className="overflow-hidden rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default)">
              <table className="w-full text-sm text-left bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
                <thead className="bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-default) text-xs font-semibold border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
                  <tr>
                    <th className="px-3 py-2.5">{t('admin_alerts_col_name')}</th>
                    <th className="px-3 py-2.5">{t('admin_alerts_col_metric')}</th>
                    <th className="px-3 py-2.5 w-24">{t('admin_alerts_col_enabled')}</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
                  {visibleCustomRules.map((rule) => {
                    const isMounted = customNode ? mounted(customNode.id, rule.id) : false;
                    return (
                      <tr
                        key={rule.id}
                        className="hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle) transition-colors"
                      >
                        <td className="px-3 py-2.5">
                          <div className="flex flex-col">
                            <span className="font-semibold text-(--theme-fg-default)">
                              {ruleName(rule)}
                            </span>
                            {!rule.enabled && (
                              <span className="text-[11px] text-(--theme-bg-danger-emphasis)">
                                {t('admin_alerts_mounts_rule_disabled')}
                              </span>
                            )}
                          </div>
                        </td>
                        <td
                          className="px-3 py-2.5 text-xs text-(--theme-fg-muted)"
                          title={rule.metric}
                        >
                          {metricName(rule)}
                        </td>
                        <td className="px-3 py-2.5">
                          <IOSSwitch
                            size="sm"
                            checked={isMounted}
                            disabled={saving || !customNode}
                            onChange={() => {
                              if (!customNode) return;
                              void onSetMounts([rule.id], [customNode.id], !isMounted);
                            }}
                          />
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </ModalBody>
        <ModalFooter>
          <Button variant="secondary" onClick={() => setCustomNodeId(null)}>
            {t('common_close')}
          </Button>
        </ModalFooter>
      </Modal>
    </div>
  );
};

export default AlertMountsPanel;
