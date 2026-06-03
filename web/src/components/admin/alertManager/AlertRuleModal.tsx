import React from 'react';
import SlidersHorizontal from 'lucide-react/dist/esm/icons/sliders-horizontal';
import Button from '@components/ui/Button';
import Input from '@components/ui/Input';
import Select from '@components/ui/Select';
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@components/ui/Modal';
import type { AlertRule, AlertRuleThresholdMode, AlertRuleInput } from '@app-types/admin';
import { useI18n } from '@i18n';
import { alertMetricName, alertMetricValues } from './alertLabels';

interface Props {
  isOpen: boolean;
  initialRule: AlertRule | null;
  saving: boolean;
  onClose: () => void;
  onSave: (id: number | null, input: AlertRuleInput) => Promise<boolean>;
  onSuccess: () => void;
}

const defaultRule: AlertRuleInput = {
  name: '',
  enabled: true,
  metric: 'cpu.usage_ratio',
  operator: '>=',
  threshold: 0.9,
  duration_sec: 60,
  cooldown_min: 0,
  threshold_mode: 'static',
  threshold_offset: 0,
};

const pickRuleInput = (rule: AlertRuleInput): AlertRuleInput => ({
  name: rule.name,
  enabled: rule.enabled,
  metric: rule.metric,
  operator: rule.operator,
  threshold: rule.threshold,
  duration_sec: rule.duration_sec,
  cooldown_min: rule.cooldown_min,
  threshold_mode: rule.threshold_mode,
  threshold_offset: rule.threshold_offset,
});

type DraftState = {
  sourceId: number | null;
  sourceVersion: string;
  dirty: boolean;
  draft: Partial<AlertRuleInput>;
};

const sourceDraftFromRule = (rule: AlertRule | null): AlertRuleInput =>
  rule ? pickRuleInput(rule) : defaultRule;

const sourceVersionFromRule = (rule: AlertRule | null): string => rule?.updated_at ?? 'new';

const AlertRuleModal: React.FC<Props> = ({
  isOpen,
  initialRule,
  saving,
  onClose,
  onSave,
  onSuccess,
}) => {
  const { t } = useI18n();
  const titleId = React.useId();
  const nameId = React.useId();
  const thresholdModeId = React.useId();
  const durationId = React.useId();
  const cooldownId = React.useId();
  const metricId = React.useId();
  const operatorId = React.useId();
  const thresholdId = React.useId();
  const thresholdOffsetId = React.useId();
  const sourceId = initialRule?.id ?? null;
  const sourceVersion = sourceVersionFromRule(initialRule);
  const sourceDraft = React.useMemo(() => sourceDraftFromRule(initialRule), [initialRule]);
  const [draftState, setDraftState] = React.useState<DraftState>(() => ({
    sourceId,
    sourceVersion,
    dirty: false,
    draft: sourceDraft,
  }));
  const sourceChanged = draftState.sourceId !== sourceId;
  const sourceRefreshed = draftState.sourceVersion !== sourceVersion;
  const draft =
    sourceChanged || (sourceRefreshed && !draftState.dirty) ? sourceDraft : draftState.draft;
  const durationSec = draft.duration_sec ?? 60;
  const durationValue =
    durationSec === 0 || durationSec === 60 || durationSec === 300 ? String(durationSec) : 'custom';

  const setField = <K extends keyof AlertRuleInput>(field: K, value: AlertRuleInput[K]) => {
    setDraftState({
      sourceId,
      sourceVersion,
      dirty: true,
      draft: { ...draft, [field]: value },
    });
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (saving) return;

    const thresholdMode = (draft.threshold_mode ?? 'static') as AlertRuleThresholdMode;
    const input: AlertRuleInput = {
      name: draft.name ?? '',
      enabled: draft.enabled ?? true,
      metric: draft.metric ?? 'cpu.usage_ratio',
      operator: draft.operator ?? '>=',
      threshold: Number(draft.threshold ?? 0),
      duration_sec: Number(draft.duration_sec ?? 0),
      cooldown_min: Number(draft.cooldown_min ?? 0),
      threshold_mode: thresholdMode,
      threshold_offset: thresholdMode === 'static' ? 0 : Number(draft.threshold_offset ?? 0),
    };

    if (await onSave(initialRule?.id ?? null, input)) {
      onSuccess();
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} ariaLabelledby={titleId}>
      <ModalHeader
        title={initialRule ? t('edit_rule') : t('add_rule')}
        icon={
          <SlidersHorizontal className="text-(--theme-border-underline-nav-active)" size={20} />
        }
        onClose={onClose}
        id={titleId}
      />
      <form onSubmit={submit} className="flex min-h-0 flex-1 flex-col">
        <ModalBody className="space-y-8">
          <div className="space-y-4">
            <h3 className="text-sm font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong) uppercase tracking-wider mb-3">
              {t('admin_alerts_rule_settings')}
            </h3>

            <div className="space-y-1.5">
              <label
                htmlFor={nameId}
                className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1"
              >
                {t('name')}
              </label>
              <Input
                id={nameId}
                value={draft.name ?? ''}
                onChange={(e) => setField('name', e.target.value)}
                required
              />
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div className="space-y-1.5">
                <label
                  htmlFor={thresholdModeId}
                  className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1"
                >
                  {t('admin_alerts_field_threshold_mode')}
                </label>
                <Select
                  id={thresholdModeId}
                  value={draft.threshold_mode ?? 'static'}
                  onChange={(e) => {
                    const next = e.target.value as AlertRuleThresholdMode;
                    setDraftState({
                      sourceId,
                      sourceVersion,
                      dirty: true,
                      draft: {
                        ...draft,
                        threshold_mode: next,
                        threshold_offset: next === 'static' ? 0 : (draft.threshold_offset ?? 0),
                      },
                    });
                  }}
                >
                  <option value="static">static</option>
                  <option value="core_plus">core_plus</option>
                </Select>
              </div>

              <div className="space-y-1.5">
                <label
                  htmlFor={durationId}
                  className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1"
                >
                  {t('admin_alerts_field_duration')}
                </label>
                <Select
                  id={durationId}
                  value={durationValue}
                  onChange={(e) => setField('duration_sec', Number(e.target.value))}
                >
                  {durationValue === 'custom' && (
                    <option value="custom" disabled>
                      {`${durationSec}s`}
                    </option>
                  )}
                  <option value="0">{t('admin_alerts_duration_now')}</option>
                  <option value="60">{t('admin_alerts_duration_1m')}</option>
                  <option value="300">{t('admin_alerts_duration_5m')}</option>
                </Select>
              </div>

              <div className="space-y-1.5">
                <label
                  htmlFor={cooldownId}
                  className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1"
                >
                  {t('admin_alerts_field_cooldown')}
                </label>
                <Input
                  id={cooldownId}
                  type="number"
                  min="0"
                  step="1"
                  value={(draft.cooldown_min ?? 0).toString()}
                  onChange={(e) => setField('cooldown_min', Number(e.target.value))}
                />
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div className="space-y-1.5">
                <label
                  htmlFor={metricId}
                  className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1"
                >
                  {t('admin_alerts_field_metric')}
                </label>
                <Select
                  id={metricId}
                  value={draft.metric ?? 'cpu.usage_ratio'}
                  onChange={(e) => setField('metric', e.target.value)}
                >
                  {alertMetricValues.map((metric) => (
                    <option key={metric} value={metric}>
                      {alertMetricName(metric, t)}
                    </option>
                  ))}
                </Select>
              </div>

              <div className="space-y-1.5">
                <label
                  htmlFor={operatorId}
                  className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1"
                >
                  {t('admin_alerts_field_operator')}
                </label>
                <Select
                  id={operatorId}
                  className="font-mono"
                  value={draft.operator ?? '>='}
                  onChange={(e) =>
                    setField('operator', e.target.value as AlertRuleInput['operator'])
                  }
                >
                  <option value=">">&gt;</option>
                  <option value=">=">&ge;</option>
                  <option value="<">&lt;</option>
                  <option value="<=">&le;</option>
                  <option value="==">==</option>
                  <option value="!=">!=</option>
                </Select>
              </div>

              {(draft.threshold_mode ?? 'static') === 'core_plus' ? (
                <div className="space-y-1.5">
                  <label
                    htmlFor={thresholdOffsetId}
                    className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1"
                  >
                    {t('admin_alerts_field_threshold_offset')}
                  </label>
                  <Input
                    id={thresholdOffsetId}
                    type="number"
                    value={(draft.threshold_offset ?? 0).toString()}
                    onChange={(e) => setField('threshold_offset', Number(e.target.value))}
                    step="0.01"
                  />
                  <p className="text-[11px] text-(--theme-fg-subtle) ml-1">
                    {t('admin_alerts_core_plus_hint')}
                  </p>
                </div>
              ) : (
                <div className="space-y-1.5">
                  <label
                    htmlFor={thresholdId}
                    className="text-xs font-medium text-(--theme-fg-muted) dark:text-(--theme-fg-strong) ml-1"
                  >
                    {t('admin_alerts_field_threshold')}
                  </label>
                  <Input
                    id={thresholdId}
                    value={(draft.threshold ?? 0).toString()}
                    onChange={(e) => setField('threshold', Number(e.target.value))}
                    type="number"
                    step="0.01"
                  />
                </div>
              )}
            </div>
          </div>
        </ModalBody>

        <ModalFooter>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            {t('common_cancel')}
          </Button>
          <Button variant="primary" type="submit" disabled={saving}>
            {saving ? t('loading') : t('common_save')}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
};

export default AlertRuleModal;
