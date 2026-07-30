import SlidersHorizontal from 'lucide-react/dist/esm/icons/sliders-horizontal';
import Button from '@components/ui/Button';
import Checkbox from '@components/ui/Checkbox';
import IOSSwitch from '@components/ui/IOSSwitch';
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@components/ui/Modal';
import type { AlertMountNode, AlertMountRule } from '@app-types/admin';
import { useI18n } from '@i18n';

type RuleLabel = (rule: AlertMountRule) => string;
type MountedLookup = (nodeId: number, ruleId: number) => boolean;

interface BatchModalProps {
  isOpen: boolean;
  titleId: string;
  title: string;
  rules: AlertMountRule[];
  selectedNodeCount: number;
  selectedRuleCount: number;
  selectedRules: ReadonlySet<number>;
  allSelected: boolean;
  someSelected: boolean;
  canSubmit: boolean;
  onClose: () => void;
  onSubmit: () => Promise<void>;
  onToggleAll: () => void;
  onToggleRule: (id: number) => void;
  ruleName: RuleLabel;
  metricName: RuleLabel;
}

export const AlertMountsBatchModal = ({
  isOpen,
  titleId,
  title,
  rules,
  selectedNodeCount,
  selectedRuleCount,
  selectedRules,
  allSelected,
  someSelected,
  canSubmit,
  onClose,
  onSubmit,
  onToggleAll,
  onToggleRule,
  ruleName,
  metricName,
}: BatchModalProps) => {
  const { t } = useI18n();

  return (
    <Modal isOpen={isOpen} onClose={onClose} maxWidth="max-w-3xl" ariaLabelledby={titleId}>
      <ModalHeader
        id={titleId}
        title={title}
        icon={
          <SlidersHorizontal className="text-(--theme-border-underline-nav-active)" size={20} />
        }
        onClose={onClose}
      />
      <ModalBody className="space-y-3">
        <div className="flex flex-wrap items-center gap-3 text-xs text-(--theme-fg-muted)">
          <span>
            {t('admin_alerts_mounts_selected_nodes', {
              count: String(selectedNodeCount),
            })}
          </span>
          <span>
            {t('admin_alerts_mounts_selected_rules', {
              count: String(selectedRuleCount),
            })}
          </span>
        </div>
        {rules.length === 0 ? (
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
                        checked={allSelected}
                        indeterminate={!allSelected && someSelected}
                        onChange={onToggleAll}
                        aria-label={t('admin_alerts_mounts_select_all_rules')}
                      />
                    </div>
                  </th>
                  <th className="px-3 py-2.5">{t('admin_alerts_col_name')}</th>
                  <th className="px-3 py-2.5">{t('admin_alerts_col_metric')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
                {rules.map((rule) => (
                  <tr
                    key={rule.id}
                    className="hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle) transition-colors"
                  >
                    <td className="px-3 py-2.5 align-middle">
                      <Checkbox
                        checked={selectedRules.has(rule.id)}
                        onChange={() => onToggleRule(rule.id)}
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
                    <td className="px-3 py-2.5 text-xs text-(--theme-fg-muted)" title={rule.metric}>
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
        <Button variant="secondary" onClick={onClose}>
          {t('common_cancel')}
        </Button>
        <Button disabled={!canSubmit} onClick={() => void onSubmit()}>
          {title}
        </Button>
      </ModalFooter>
    </Modal>
  );
};

interface CustomModalProps {
  titleId: string;
  node: AlertMountNode | null;
  rules: AlertMountRule[];
  saving: boolean;
  onClose: () => void;
  onSetMounts: (ruleIds: number[], serverIds: number[], mounted: boolean) => Promise<boolean>;
  ruleName: RuleLabel;
  metricName: RuleLabel;
  mounted: MountedLookup;
}

export const AlertMountsCustomModal = ({
  titleId,
  node,
  rules,
  saving,
  onClose,
  onSetMounts,
  ruleName,
  metricName,
  mounted,
}: CustomModalProps) => {
  const { t } = useI18n();

  return (
    <Modal isOpen={Boolean(node)} onClose={onClose} maxWidth="max-w-2xl" ariaLabelledby={titleId}>
      <ModalHeader
        id={titleId}
        title={t('admin_alerts_mounts_custom_modal_title', {
          name: node?.name ?? '',
        })}
        icon={
          <SlidersHorizontal className="text-(--theme-border-underline-nav-active)" size={20} />
        }
        onClose={onClose}
      />
      <ModalBody className="space-y-3">
        {rules.length === 0 ? (
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
                {rules.map((rule) => {
                  const isMounted = node ? mounted(node.id, rule.id) : false;
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
                          disabled={saving || !node}
                          ariaLabel={t('admin_alerts_mounts_toggle', {
                            node: node?.name ?? '',
                            rule: ruleName(rule),
                          })}
                          onChange={() => {
                            if (!node) return;
                            void onSetMounts([rule.id], [node.id], !isMounted);
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
        <Button variant="secondary" onClick={onClose}>
          {t('common_close')}
        </Button>
      </ModalFooter>
    </Modal>
  );
};
