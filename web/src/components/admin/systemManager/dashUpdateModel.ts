import type { DashUpdateChannel, DashUpdateMode } from '@app-types/admin';
import type { BadgeColor } from '@components/ui/Badge';
import type { TranslationKey } from '@i18n';
import { ApiError } from '@lib/api';
import * as adminApi from '@lib/adminApi';
import { isCanceledRequestError } from '@utils/errors';
import { compareVersions, parseVersion } from '@utils/version';
import type { DashReleaseNote } from './dashReleaseNotesModel';

type VersionStatus = {
  labelKey:
    | 'admin_dash_update_status_available'
    | 'admin_dash_update_status_current'
    | 'admin_dash_update_status_ahead'
    | 'admin_dash_update_status_unknown';
  color: BadgeColor;
};

export type NotifyTargets = {
  settingsEnabled: boolean;
  selectedCount: number;
  activeCount: number;
};

export const updateModeOptions: Array<{
  mode: DashUpdateMode;
  labelKey: TranslationKey;
  hintKey: TranslationKey;
}> = [
  {
    mode: 'manual',
    labelKey: 'admin_dash_update_mode_manual',
    hintKey: 'admin_dash_update_mode_manual_hint',
  },
  {
    mode: 'notify',
    labelKey: 'admin_dash_update_mode_notify',
    hintKey: 'admin_dash_update_mode_notify_hint',
  },
  {
    mode: 'auto',
    labelKey: 'admin_dash_update_mode_auto',
    hintKey: 'admin_dash_update_mode_auto_hint',
  },
];

export const statusFromCheck = (
  status: adminApi.DashUpdateVersionStatus | undefined,
): VersionStatus => {
  switch (status) {
    case 'available':
      return { labelKey: 'admin_dash_update_status_available', color: 'amber' };
    case 'current':
      return { labelKey: 'admin_dash_update_status_current', color: 'emerald' };
    case 'ahead':
      return { labelKey: 'admin_dash_update_status_ahead', color: 'indigo' };
    default:
      return { labelKey: 'admin_dash_update_status_unknown', color: 'slate' };
  }
};

const channelFromVersion = (version: string): DashUpdateChannel | null => {
  const parsed = parseVersion(version);
  if (!parsed) return null;
  return parsed.pre ? 'prerelease' : 'release';
};

export const latestNoteForChannel = (
  notes: DashReleaseNote[],
  channel: DashUpdateChannel,
): DashReleaseNote | null => {
  let latest: DashReleaseNote | null = null;
  for (const note of notes) {
    if (channelFromVersion(note.version) !== channel) continue;
    if (!latest) {
      latest = note;
      continue;
    }
    const compared = compareVersions(note.version, latest.version);
    if (compared !== null && compared > 0) latest = note;
  }
  return latest;
};

export const updateJobBadge = (
  status: adminApi.DashUpdateStatus | null,
): { labelKey: TranslationKey; color: BadgeColor } => {
  switch (status?.status) {
    case 'running':
      return { labelKey: 'admin_dash_update_job_status_running', color: 'indigo' };
    case 'completed':
      return { labelKey: 'admin_dash_update_job_status_completed', color: 'emerald' };
    case 'failed':
      return { labelKey: 'admin_dash_update_job_status_failed', color: 'rose' };
    default:
      return { labelKey: 'admin_dash_update_job_status_idle', color: 'slate' };
  }
};

export const notificationTargetPath = '/admin?tab=alerts&alerts_tab=channels';

export const readableError = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) return error.message || fallback;
  if (error instanceof Error && error.message) return error.message;
  return fallback;
};

export const checkStep = async <T>(task: Promise<T>, fallback: string): Promise<T> => {
  try {
    return await task;
  } catch (error) {
    if (isCanceledRequestError(error)) throw error;
    throw new Error(readableError(error, fallback), { cause: error });
  }
};

export const conflictStatus = (error: unknown): adminApi.DashUpdateStatus | null => {
  if (!(error instanceof ApiError) || error.status !== 409) return null;
  const details = error.details;
  if (!details || typeof details !== 'object') return null;

  const status = (details as Partial<adminApi.DashUpdateStatus>).status;
  if (status !== 'idle' && status !== 'running' && status !== 'completed' && status !== 'failed') {
    return null;
  }
  return details as adminApi.DashUpdateStatus;
};

export const updateStatusKey = (status: adminApi.DashUpdateStatus): string =>
  status.id || `${status.status}:${status.started_at ?? ''}:${status.finished_at ?? ''}`;
