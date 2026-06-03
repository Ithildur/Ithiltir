import type {
  NodeTrafficDirectionMode,
  TrafficCycleMode,
  TrafficDirectionMode,
  TrafficSettings,
} from '@app-types/traffic';
import type { NodeTrafficPatch } from '@app-types/api';
import type { TranslationKey } from '@i18n';

export type { NodeTrafficPatch } from '@app-types/api';

export type NodeTrafficCycleMode = 'default' | TrafficCycleMode;

export interface NodeTrafficDraft {
  cycleMode: NodeTrafficCycleMode;
  billingStartDay: number;
  billingAnchorDate: string;
  billingTimezone: string;
  directionMode: NodeTrafficDirectionMode;
}

export interface NodeTrafficPolicy {
  trafficCycleMode: NodeTrafficCycleMode;
  trafficBillingStartDay: number;
  trafficBillingAnchorDate: string;
  trafficBillingTimezone: string;
  trafficDirectionMode: NodeTrafficDirectionMode;
}

export type TrafficCycleFields = Pick<
  TrafficSettings,
  'cycle_mode' | 'billing_start_day' | 'billing_anchor_date' | 'billing_timezone'
>;

export type TrafficCyclePatch = Partial<Pick<TrafficSettings, 'cycle_mode'>> &
  Partial<Pick<TrafficSettings, 'billing_start_day' | 'billing_anchor_date' | 'billing_timezone'>>;

export const trafficCycleModes = [
  'calendar_month',
  'whmcs_compatible',
  'clamp_to_month_end',
] as const;
export const nodeTrafficCycleModes = ['default', ...trafficCycleModes] as const;
export const trafficDirectionModes = ['out', 'both', 'max'] as const;
export const nodeTrafficDirectionModes = ['default', ...trafficDirectionModes] as const;

const trafficCycleModeSet = new Set<string>(trafficCycleModes);
const nodeTrafficCycleModeSet = new Set<string>(nodeTrafficCycleModes);
const trafficDirectionModeSet = new Set<string>(trafficDirectionModes);
const nodeTrafficDirectionModeSet = new Set<string>(nodeTrafficDirectionModes);

export const trafficCycleLabelKey: Record<TrafficCycleMode, TranslationKey> = {
  calendar_month: 'traffic_cycle_calendar_month',
  whmcs_compatible: 'traffic_cycle_whmcs_compatible',
  clamp_to_month_end: 'traffic_cycle_clamp_to_month_end',
};

export const trafficDirectionLabelKey: Record<TrafficDirectionMode, TranslationKey> = {
  out: 'traffic_direction_out',
  both: 'traffic_direction_both',
  max: 'traffic_direction_max',
};

export const nodeTrafficCycleLabelKey: Record<NodeTrafficCycleMode, TranslationKey> = {
  default: 'admin_node_cycle_mode_default',
  ...trafficCycleLabelKey,
};

export const nodeTrafficDirectionLabelKey: Record<NodeTrafficDirectionMode, TranslationKey> = {
  default: 'admin_node_direction_mode_default',
  out: trafficDirectionLabelKey.out,
  both: trafficDirectionLabelKey.both,
  max: trafficDirectionLabelKey.max,
};

export const parseTrafficCycleMode = (value: string): TrafficCycleMode | null =>
  trafficCycleModeSet.has(value) ? (value as TrafficCycleMode) : null;

export const parseNodeTrafficCycleMode = (value: string): NodeTrafficCycleMode | null =>
  nodeTrafficCycleModeSet.has(value) ? (value as NodeTrafficCycleMode) : null;

export const parseTrafficDirectionMode = (value: string): TrafficDirectionMode | null =>
  trafficDirectionModeSet.has(value) ? (value as TrafficDirectionMode) : null;

export const parseNodeTrafficDirectionMode = (value: string): NodeTrafficDirectionMode | null =>
  nodeTrafficDirectionModeSet.has(value) ? (value as NodeTrafficDirectionMode) : null;

export const defaultTrafficSettings: TrafficSettings = {
  guest_access_mode: 'disabled',
  usage_mode: 'lite',
  cycle_mode: 'calendar_month',
  billing_start_day: 1,
  billing_anchor_date: '',
  billing_timezone: '',
  direction_mode: 'out',
};

export const trafficCoverageWarningThreshold = 0.995;

export const clampBillingDay = (day: number) => Math.max(1, Math.min(31, Math.trunc(day)));

export const billingDayFromAnchor = (anchorDate: string, fallback: number): number => {
  const day = anchorDate ? Number(anchorDate.slice(-2)) : Number.NaN;
  return Number.isFinite(day) ? clampBillingDay(day) : fallback;
};

export const cycleNeedsBillingStartDay = (mode: NodeTrafficCycleMode): boolean =>
  mode === 'clamp_to_month_end';

export const cycleNeedsAnchorDate = (mode: NodeTrafficCycleMode): boolean =>
  mode === 'whmcs_compatible';

export const cycleNeedsTimezone = (mode: NodeTrafficCycleMode): boolean => mode !== 'default';

export const normalizeTrafficCycleFields = (draft: TrafficCycleFields): TrafficCycleFields => ({
  cycle_mode: draft.cycle_mode,
  billing_start_day: cycleNeedsBillingStartDay(draft.cycle_mode)
    ? clampBillingDay(draft.billing_start_day)
    : draft.cycle_mode === 'whmcs_compatible'
      ? billingDayFromAnchor(draft.billing_anchor_date.trim(), 1)
      : 1,
  billing_anchor_date:
    draft.cycle_mode === 'whmcs_compatible' ? draft.billing_anchor_date.trim() : '',
  billing_timezone: draft.billing_timezone.trim(),
});

export const trafficCyclePatchFromFields = (
  draft: TrafficCycleFields,
  saved: TrafficCycleFields,
): TrafficCyclePatch => {
  const fields = normalizeTrafficCycleFields(draft);
  const base = normalizeTrafficCycleFields(saved);
  const modeChanged = fields.cycle_mode !== base.cycle_mode;
  const patch: TrafficCyclePatch = {};

  if (modeChanged) patch.cycle_mode = fields.cycle_mode;
  if (
    cycleNeedsBillingStartDay(fields.cycle_mode) &&
    (modeChanged || fields.billing_start_day !== base.billing_start_day)
  ) {
    patch.billing_start_day = fields.billing_start_day;
  }
  if (
    cycleNeedsAnchorDate(fields.cycle_mode) &&
    (modeChanged || fields.billing_anchor_date !== base.billing_anchor_date)
  ) {
    patch.billing_anchor_date = fields.billing_anchor_date;
  }
  if (cycleNeedsTimezone(fields.cycle_mode) && fields.billing_timezone !== base.billing_timezone) {
    patch.billing_timezone = fields.billing_timezone;
  }
  return patch;
};

export const trafficCycleValid = (draft: TrafficCycleFields): boolean =>
  !cycleNeedsAnchorDate(draft.cycle_mode) || draft.billing_anchor_date.trim() !== '';

export const trafficCycleChanged = (
  draft: TrafficCycleFields,
  saved: TrafficCycleFields,
): boolean => {
  return Object.keys(trafficCyclePatchFromFields(draft, saved)).length > 0;
};

export const trafficSettingsWithCycleMode = (
  current: TrafficSettings,
  mode: TrafficCycleMode,
): TrafficSettings => ({
  ...current,
  cycle_mode: mode,
  billing_start_day:
    mode === 'clamp_to_month_end'
      ? current.billing_start_day
      : mode === 'whmcs_compatible'
        ? billingDayFromAnchor(current.billing_anchor_date, 1)
        : 1,
  billing_anchor_date: mode === 'whmcs_compatible' ? current.billing_anchor_date : '',
});

export const nodeTrafficDraftFromPolicy = (
  node: NodeTrafficPolicy,
  globalSettings: TrafficSettings,
): NodeTrafficDraft => {
  if (node.trafficCycleMode === 'default') {
    return {
      cycleMode: 'default',
      billingStartDay: globalSettings.billing_start_day,
      billingAnchorDate: globalSettings.billing_anchor_date,
      billingTimezone: globalSettings.billing_timezone,
      directionMode: node.trafficDirectionMode,
    };
  }
  return {
    cycleMode: node.trafficCycleMode,
    billingStartDay: node.trafficBillingStartDay,
    billingAnchorDate: node.trafficBillingAnchorDate,
    billingTimezone: node.trafficBillingTimezone,
    directionMode: node.trafficDirectionMode,
  };
};

export const nodeTrafficDraftWithCycleMode = (
  current: NodeTrafficDraft,
  mode: NodeTrafficCycleMode,
  globalSettings: TrafficSettings,
): NodeTrafficDraft => {
  if (mode === 'default') {
    return {
      ...current,
      cycleMode: mode,
      billingStartDay: globalSettings.billing_start_day,
      billingAnchorDate: globalSettings.billing_anchor_date,
      billingTimezone: globalSettings.billing_timezone,
    };
  }
  if (mode === 'calendar_month') {
    return { ...current, cycleMode: mode, billingStartDay: 1, billingAnchorDate: '' };
  }
  if (mode === 'clamp_to_month_end') {
    return { ...current, cycleMode: mode, billingAnchorDate: '' };
  }
  return {
    ...current,
    cycleMode: mode,
    billingStartDay: billingDayFromAnchor(current.billingAnchorDate, 1),
  };
};

export const nodeTrafficDraftValid = (draft: NodeTrafficDraft): boolean =>
  !cycleNeedsAnchorDate(draft.cycleMode) || draft.billingAnchorDate.trim() !== '';

export const nodeTrafficPatchFromDraft = (draft: NodeTrafficDraft): NodeTrafficPatch => {
  if (draft.cycleMode === 'default') {
    return {
      traffic_cycle_mode: 'default',
      traffic_direction_mode: draft.directionMode,
    };
  }

  const billingStartDay =
    draft.cycleMode === 'whmcs_compatible'
      ? billingDayFromAnchor(draft.billingAnchorDate.trim(), 1)
      : draft.cycleMode === 'calendar_month'
        ? 1
        : clampBillingDay(draft.billingStartDay);
  if (draft.cycleMode === 'calendar_month') {
    return {
      traffic_cycle_mode: draft.cycleMode,
      traffic_billing_timezone: draft.billingTimezone.trim(),
      traffic_direction_mode: draft.directionMode,
    };
  }
  if (draft.cycleMode === 'clamp_to_month_end') {
    return {
      traffic_cycle_mode: draft.cycleMode,
      traffic_billing_start_day: billingStartDay,
      traffic_billing_timezone: draft.billingTimezone.trim(),
      traffic_direction_mode: draft.directionMode,
    };
  }
  return {
    traffic_cycle_mode: draft.cycleMode,
    traffic_billing_anchor_date: draft.billingAnchorDate.trim(),
    traffic_billing_timezone: draft.billingTimezone.trim(),
    traffic_direction_mode: draft.directionMode,
  };
};

export const nodeTrafficDraftChanged = (
  draft: NodeTrafficDraft,
  saved: NodeTrafficDraft,
): boolean => {
  const left = nodeTrafficPatchFromDraft(draft);
  const right = nodeTrafficPatchFromDraft(saved);
  return (
    left.traffic_cycle_mode !== right.traffic_cycle_mode ||
    left.traffic_billing_start_day !== right.traffic_billing_start_day ||
    left.traffic_billing_anchor_date !== right.traffic_billing_anchor_date ||
    left.traffic_billing_timezone !== right.traffic_billing_timezone ||
    left.traffic_direction_mode !== right.traffic_direction_mode
  );
};
