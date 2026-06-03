import React from 'react';
import ReceiptText from 'lucide-react/dist/esm/icons/receipt-text';
import Button from '@components/ui/Button';
import Input from '@components/ui/Input';
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@components/ui/Modal';
import Select from '@components/ui/Select';
import TimezoneSelect from '@components/ui/TimezoneSelect';
import {
  billingDayFromAnchor,
  clampBillingDay,
  cycleNeedsAnchorDate,
  cycleNeedsBillingStartDay,
  cycleNeedsTimezone,
  nodeTrafficDraftValid,
  nodeTrafficCycleModes,
  nodeTrafficDirectionModes,
  nodeTrafficCycleLabelKey,
  nodeTrafficDraftChanged,
  nodeTrafficDraftFromPolicy,
  nodeTrafficDraftWithCycleMode,
  nodeTrafficDirectionLabelKey,
  nodeTrafficPatchFromDraft,
  parseNodeTrafficCycleMode,
  parseNodeTrafficDirectionMode,
  type NodeTrafficDraft,
  type NodeTrafficPatch,
} from '@lib/trafficSettingsModel';
import type { NodeRow } from '@app-types/admin';
import type { TrafficSettings } from '@app-types/traffic';
import { useI18n } from '@i18n';

interface Props {
  isOpen: boolean;
  node: NodeRow;
  globalSettings: TrafficSettings;
  saving: boolean;
  onClose: () => void;
  onSave: (patch: NodeTrafficPatch) => Promise<boolean>;
}

const NodeTrafficSettingsModal: React.FC<Props> = ({
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
    () => nodeTrafficDraftFromPolicy(node, globalSettings),
    [globalSettings, node],
  );
  const [draft, setDraft] = React.useState<NodeTrafficDraft>(savedDraft);

  React.useEffect(() => {
    setDraft(savedDraft);
  }, [savedDraft]);

  const cycleInherited = draft.cycleMode === 'default';
  const inherited = cycleInherited && draft.directionMode === 'default';
  const changed = nodeTrafficDraftChanged(draft, savedDraft);
  const valid = nodeTrafficDraftValid(draft);
  const showBillingStartDay = cycleNeedsBillingStartDay(draft.cycleMode);
  const showAnchorDate = cycleNeedsAnchorDate(draft.cycleMode);
  const showTimezone = cycleNeedsTimezone(draft.cycleMode);

  const setCycleMode = (mode: NodeTrafficDraft['cycleMode']) => {
    setDraft((current) => nodeTrafficDraftWithCycleMode(current, mode, globalSettings));
  };

  const save = async () => {
    if (saving || !changed || !valid) return;
    const ok = await onSave(nodeTrafficPatchFromDraft(draft));
    if (ok) onClose();
  };

  if (!isOpen) return null;

  return (
    <Modal isOpen={isOpen} onClose={onClose} maxWidth="max-w-xl" ariaLabelledby={titleId}>
      <ModalHeader
        id={titleId}
        title={t('admin_node_traffic_settings_title', { name: node.name })}
        icon={<ReceiptText className="size-5" aria-hidden="true" />}
        onClose={onClose}
      />
      <ModalBody className="space-y-5">
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="grid gap-1.5">
            <label
              htmlFor={`${titleId}-cycle-mode`}
              className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)"
            >
              {t('admin_nodes_column_cycle_mode')}
            </label>
            <Select
              id={`${titleId}-cycle-mode`}
              value={draft.cycleMode}
              disabled={saving}
              onChange={(event) => {
                const mode = parseNodeTrafficCycleMode(event.target.value);
                if (mode) setCycleMode(mode);
              }}
            >
              {nodeTrafficCycleModes.map((mode) => (
                <option key={mode} value={mode}>
                  {t(nodeTrafficCycleLabelKey[mode])}
                </option>
              ))}
            </Select>
          </div>

          <div className="grid gap-1.5">
            <label
              htmlFor={`${titleId}-direction-mode`}
              className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)"
            >
              {t('traffic_direction_mode')}
            </label>
            <Select
              id={`${titleId}-direction-mode`}
              value={draft.directionMode}
              disabled={saving}
              onChange={(event) => {
                const mode = parseNodeTrafficDirectionMode(event.target.value);
                if (!mode) return;
                setDraft((current) => ({
                  ...current,
                  directionMode: mode,
                }));
              }}
            >
              {nodeTrafficDirectionModes.map((mode) => (
                <option key={mode} value={mode}>
                  {t(nodeTrafficDirectionLabelKey[mode])}
                </option>
              ))}
            </Select>
          </div>
        </div>

        {(showBillingStartDay || showAnchorDate) && (
          <div className="grid gap-4 sm:grid-cols-2">
            {showBillingStartDay && (
              <label className="grid gap-1.5">
                <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                  {t('traffic_billing_start_day')}
                </span>
                <Input
                  type="number"
                  min={1}
                  max={31}
                  disabled={saving}
                  aria-label={t('traffic_billing_start_day')}
                  value={draft.billingStartDay}
                  onChange={(event) => {
                    const next = Number(event.target.value);
                    setDraft((current) => ({
                      ...current,
                      billingStartDay: Number.isFinite(next)
                        ? clampBillingDay(next)
                        : current.billingStartDay,
                    }));
                  }}
                />
              </label>
            )}

            {showAnchorDate && (
              <label className="grid gap-1.5">
                <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                  {t('traffic_anchor_date')}
                </span>
                <Input
                  type="date"
                  required
                  disabled={saving}
                  aria-label={t('traffic_anchor_date')}
                  value={draft.billingAnchorDate}
                  onChange={(event) =>
                    setDraft((current) => ({
                      ...current,
                      billingAnchorDate: event.target.value,
                      billingStartDay: billingDayFromAnchor(
                        event.target.value,
                        current.billingStartDay,
                      ),
                    }))
                  }
                />
              </label>
            )}
          </div>
        )}

        {showTimezone && (
          <label className="grid gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
              {t('traffic_billing_timezone')}
            </span>
            <TimezoneSelect
              value={draft.billingTimezone}
              disabled={saving}
              ariaLabel={t('traffic_billing_timezone')}
              placeholder={t('traffic_billing_timezone_placeholder')}
              systemLabel={t('traffic_billing_timezone_system')}
              emptyLabel={t('traffic_billing_timezone_empty')}
              onChange={(value) => setDraft((current) => ({ ...current, billingTimezone: value }))}
            />
          </label>
        )}

        <div className="rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-muted) px-3 py-2 text-xs/5 text-(--theme-fg-muted) dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)">
          {inherited
            ? t('admin_node_traffic_settings_inherited_hint')
            : t('admin_node_traffic_settings_override_hint')}
        </div>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose} disabled={saving}>
          {t('common_cancel')}
        </Button>
        <Button
          variant="primary"
          onClick={() => void save()}
          disabled={saving || !changed || !valid}
        >
          {saving ? t('admin_system_settings_saving') : t('common_save_changes')}
        </Button>
      </ModalFooter>
    </Modal>
  );
};

export default NodeTrafficSettingsModal;
