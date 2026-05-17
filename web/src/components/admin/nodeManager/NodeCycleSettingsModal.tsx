import React from 'react';
import ReceiptText from 'lucide-react/dist/esm/icons/receipt-text';
import Button from '@components/ui/Button';
import Input from '@components/ui/Input';
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@components/ui/Modal';
import Select from '@components/ui/Select';
import type { NodeRow } from '@app-types/admin';
import type { TrafficCycleMode, TrafficSettings } from '@app-types/traffic';
import { useI18n, type TranslationKey } from '@i18n';

type NodeCycleMode = NodeRow['trafficCycleMode'];

export interface NodeCycleSettingsInput {
  traffic_cycle_mode: NodeCycleMode;
  traffic_billing_start_day: number;
  traffic_billing_anchor_date: string;
  traffic_billing_timezone: string;
}

interface Props {
  isOpen: boolean;
  node: NodeRow;
  globalSettings: TrafficSettings;
  saving: boolean;
  onClose: () => void;
  onSave: (input: NodeCycleSettingsInput) => Promise<boolean>;
}

interface CycleDraft {
  mode: NodeCycleMode;
  billingStartDay: number;
  billingAnchorDate: string;
  billingTimezone: string;
}

const cycleModes: TrafficCycleMode[] = ['calendar_month', 'whmcs_compatible', 'clamp_to_month_end'];
const nodeCycleModes: NodeCycleMode[] = ['default', ...cycleModes];

const clampDay = (day: number) => Math.max(1, Math.min(31, Math.trunc(day)));

const draftFromNode = (node: NodeRow, globalSettings: TrafficSettings): CycleDraft => {
  if (node.trafficCycleMode === 'default') {
    return {
      mode: 'default',
      billingStartDay: globalSettings.billing_start_day,
      billingAnchorDate: globalSettings.billing_anchor_date,
      billingTimezone: globalSettings.billing_timezone,
    };
  }
  return {
    mode: node.trafficCycleMode,
    billingStartDay: node.trafficBillingStartDay,
    billingAnchorDate: node.trafficBillingAnchorDate,
    billingTimezone: node.trafficBillingTimezone,
  };
};

const inputFromDraft = (draft: CycleDraft): NodeCycleSettingsInput => {
  const billingStartDay = draft.mode === 'calendar_month' ? 1 : clampDay(draft.billingStartDay);
  return {
    traffic_cycle_mode: draft.mode,
    traffic_billing_start_day: billingStartDay,
    traffic_billing_anchor_date:
      draft.mode === 'whmcs_compatible' ? draft.billingAnchorDate.trim() : '',
    traffic_billing_timezone: draft.billingTimezone.trim(),
  };
};

const sameDraft = (left: CycleDraft, right: CycleDraft) => {
  const a = inputFromDraft(left);
  const b = inputFromDraft(right);
  return (
    a.traffic_cycle_mode === b.traffic_cycle_mode &&
    a.traffic_billing_start_day === b.traffic_billing_start_day &&
    a.traffic_billing_anchor_date === b.traffic_billing_anchor_date &&
    a.traffic_billing_timezone === b.traffic_billing_timezone
  );
};

const NodeCycleSettingsModal: React.FC<Props> = ({
  isOpen,
  node,
  globalSettings,
  saving,
  onClose,
  onSave,
}) => {
  const { t } = useI18n();
  const titleId = React.useId();
  const savedDraft = React.useMemo(
    () => draftFromNode(node, globalSettings),
    [globalSettings, node],
  );
  const [draft, setDraft] = React.useState<CycleDraft>(savedDraft);

  React.useEffect(() => {
    setDraft(savedDraft);
  }, [savedDraft]);

  const inherited = draft.mode === 'default';
  const changed = !sameDraft(draft, savedDraft);

  const setMode = (mode: NodeCycleMode) => {
    setDraft((current) => {
      if (mode === 'default') return { ...draftFromNode(node, globalSettings), mode };
      if (mode === 'calendar_month') {
        return { ...current, mode, billingStartDay: 1, billingAnchorDate: '' };
      }
      if (mode === 'clamp_to_month_end') {
        return { ...current, mode, billingAnchorDate: '' };
      }
      return { ...current, mode };
    });
  };

  const save = async () => {
    if (saving || !changed) return;
    const ok = await onSave(inputFromDraft(draft));
    if (ok) onClose();
  };

  if (!isOpen) return null;

  return (
    <Modal isOpen={isOpen} onClose={onClose} maxWidth="max-w-xl" ariaLabelledby={titleId}>
      <ModalHeader
        id={titleId}
        title={t('admin_node_cycle_settings_title', { name: node.name })}
        icon={<ReceiptText className="size-5" aria-hidden="true" />}
        onClose={onClose}
      />
      <ModalBody className="space-y-5">
        <div className="grid gap-1.5">
          <label
            htmlFor={`${titleId}-mode`}
            className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)"
          >
            {t('admin_nodes_column_cycle_mode')}
          </label>
          <Select
            id={`${titleId}-mode`}
            value={draft.mode}
            disabled={saving}
            onChange={(event) => setMode(event.target.value as NodeCycleMode)}
          >
            {nodeCycleModes.map((mode) => (
              <option key={mode} value={mode}>
                {mode === 'default'
                  ? t('admin_node_cycle_mode_default')
                  : t(`traffic_cycle_${mode}` as TranslationKey)}
              </option>
            ))}
          </Select>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <label className="grid gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
              {t('traffic_billing_start_day')}
            </span>
            <Input
              type="number"
              min={1}
              max={31}
              disabled={saving || inherited || draft.mode === 'calendar_month'}
              aria-label={t('traffic_billing_start_day')}
              value={draft.billingStartDay}
              onChange={(event) => {
                const next = Number(event.target.value);
                setDraft((current) => ({
                  ...current,
                  billingStartDay: Number.isFinite(next) ? clampDay(next) : current.billingStartDay,
                }));
              }}
            />
          </label>

          <label className="grid gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
              {t('traffic_anchor_date')}
            </span>
            <Input
              type="date"
              disabled={saving || inherited || draft.mode !== 'whmcs_compatible'}
              aria-label={t('traffic_anchor_date')}
              value={draft.billingAnchorDate}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  billingAnchorDate: event.target.value,
                  billingStartDay: event.target.value
                    ? Number(event.target.value.slice(-2))
                    : current.billingStartDay,
                }))
              }
            />
          </label>
        </div>

        <label className="grid gap-1.5">
          <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
            {t('traffic_billing_timezone')}
          </span>
          <Input
            value={draft.billingTimezone}
            disabled={saving || inherited}
            aria-label={t('traffic_billing_timezone')}
            placeholder={t('traffic_billing_timezone_placeholder')}
            onChange={(event) =>
              setDraft((current) => ({ ...current, billingTimezone: event.target.value }))
            }
          />
        </label>

        <div className="rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-muted) px-3 py-2 text-xs/5 text-(--theme-fg-muted) dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)">
          {inherited
            ? t('admin_node_cycle_settings_inherited_hint')
            : t('admin_node_cycle_settings_override_hint')}
        </div>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose} disabled={saving}>
          {t('common_cancel')}
        </Button>
        <Button variant="primary" onClick={() => void save()} disabled={saving || !changed}>
          {saving ? t('admin_system_settings_saving') : t('common_save_changes')}
        </Button>
      </ModalFooter>
    </Modal>
  );
};

export default NodeCycleSettingsModal;
