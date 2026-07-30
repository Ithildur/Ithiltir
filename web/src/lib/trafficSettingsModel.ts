import type {
  NodeTrafficDirectionMode,
  TrafficCycleMode,
  TrafficDirectionMode,
  TrafficSettings,
} from '@app-types/traffic';
import type { NodeTrafficPatch } from '@app-types/api';
import type { TranslationKey } from '@i18n';

export type { NodeTrafficPatch } from '@app-types/api';

export type NodeTrafficCycleMode = TrafficCycleMode;

export interface NodeTrafficDraft {
  cycleMode: NodeTrafficCycleMode;
  billingStartDay: number;
  billingAnchorDate: string;
  billingTimezone: string;
  directionMode: NodeTrafficDirectionMode;
}

interface NodeTrafficPolicy {
  trafficCycleMode: NodeTrafficCycleMode;
  trafficBillingStartDay: number;
  trafficBillingAnchorDate: string;
  trafficBillingTimezone: string;
  trafficDirectionMode: NodeTrafficDirectionMode;
}

export const nodeTrafficCycleModes = [
  'calendar_month',
  'whmcs_compatible',
  'clamp_to_month_end',
] as const;
export const trafficDirectionModes = ['out', 'both', 'max'] as const;
export const nodeTrafficDirectionModes = ['default', ...trafficDirectionModes] as const;

const nodeTrafficCycleModeSet = new Set<string>(nodeTrafficCycleModes);
const trafficDirectionModeSet = new Set<string>(trafficDirectionModes);
const nodeTrafficDirectionModeSet = new Set<string>(nodeTrafficDirectionModes);

export const nodeTrafficCycleLabelKey: Record<NodeTrafficCycleMode, TranslationKey> = {
  calendar_month: 'traffic_cycle_calendar_month',
  whmcs_compatible: 'traffic_cycle_whmcs_compatible',
  clamp_to_month_end: 'traffic_cycle_clamp_to_month_end',
};

export const trafficDirectionLabelKey: Record<TrafficDirectionMode, TranslationKey> = {
  out: 'traffic_direction_out',
  both: 'traffic_direction_both',
  max: 'traffic_direction_max',
};

export const nodeTrafficDirectionLabelKey: Record<NodeTrafficDirectionMode, TranslationKey> = {
  default: 'admin_node_direction_mode_default',
  out: trafficDirectionLabelKey.out,
  both: trafficDirectionLabelKey.both,
  max: trafficDirectionLabelKey.max,
};

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

export const cycleNeedsBillingStartDay = (mode: TrafficCycleMode): boolean =>
  mode === 'clamp_to_month_end';

export const cycleNeedsAnchorDate = (mode: TrafficCycleMode): boolean =>
  mode === 'whmcs_compatible';

export const nodeTrafficDraftFromPolicy = (node: NodeTrafficPolicy): NodeTrafficDraft => ({
  cycleMode: node.trafficCycleMode,
  billingStartDay: node.trafficBillingStartDay,
  billingAnchorDate: node.trafficBillingAnchorDate,
  billingTimezone: node.trafficBillingTimezone,
  directionMode: node.trafficDirectionMode,
});

export const nodeTrafficDraftWithCycleMode = (
  current: NodeTrafficDraft,
  mode: NodeTrafficCycleMode,
): NodeTrafficDraft => {
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

const nodeTrafficCyclePatch = (draft: NodeTrafficDraft): NodeTrafficPatch => {
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
    };
  }
  if (draft.cycleMode === 'clamp_to_month_end') {
    return {
      traffic_cycle_mode: draft.cycleMode,
      traffic_billing_start_day: billingStartDay,
      traffic_billing_timezone: draft.billingTimezone.trim(),
    };
  }
  return {
    traffic_cycle_mode: draft.cycleMode,
    traffic_billing_anchor_date: draft.billingAnchorDate.trim(),
    traffic_billing_timezone: draft.billingTimezone.trim(),
  };
};

export const nodeTrafficPatch = (
  draft: NodeTrafficDraft,
  saved: NodeTrafficDraft,
): NodeTrafficPatch => {
  const left = nodeTrafficCyclePatch(draft);
  const right = nodeTrafficCyclePatch(saved);
  const cycleChanged =
    left.traffic_cycle_mode !== right.traffic_cycle_mode ||
    left.traffic_billing_start_day !== right.traffic_billing_start_day ||
    left.traffic_billing_anchor_date !== right.traffic_billing_anchor_date ||
    left.traffic_billing_timezone !== right.traffic_billing_timezone;

  return {
    ...(cycleChanged ? left : {}),
    ...(draft.directionMode !== saved.directionMode
      ? { traffic_direction_mode: draft.directionMode }
      : {}),
  };
};

export const nodeTrafficDraftChanged = (
  draft: NodeTrafficDraft,
  saved: NodeTrafficDraft,
): boolean => Object.keys(nodeTrafficPatch(draft, saved)).length > 0;
