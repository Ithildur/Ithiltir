import React from 'react';
import ExternalLink from 'lucide-react/dist/esm/icons/external-link';
import LoaderCircle from 'lucide-react/dist/esm/icons/loader-circle';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import RotateCcw from 'lucide-react/dist/esm/icons/rotate-ccw';
import { Link } from 'react-router-dom';
import type { AppVersion } from '@app-types/api';
import type { DashUpdateChannel, DashUpdateMode, SystemSettings } from '@app-types/admin';
import Badge, { type BadgeColor } from '@components/ui/Badge';
import Button from '@components/ui/Button';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import IOSSwitch from '@components/ui/IOSSwitch';
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@components/ui/Modal';
import { Tooltip } from '@components/ui/Tooltip';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { setBundledNodeVersion } from '@stores/adminNodesStore';
import { useI18n, type TranslationKey } from '@i18n';
import { ApiError } from '@lib/api';
import * as adminApi from '@lib/adminApi';
import { fetchAppVersion } from '@lib/versionApi';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useConfirmDialog } from '@hooks/useConfirmDialog';
import { isCanceledRequestError } from '@utils/errors';
import { compareVersions, parseVersion } from '@utils/version';
import { parseDashReleaseNotes, type DashReleaseNote } from './dashReleaseNotesModel';

interface Props {
  enabled: boolean;
  settings: SystemSettings | null;
  loadingSettings: boolean;
  savingChannel: boolean;
  savingMode: boolean;
  onChannelChange: (channel: DashUpdateChannel) => void;
  onModeChange: (mode: DashUpdateMode) => void;
}

type VersionStatus = {
  labelKey:
    | 'admin_dash_update_status_available'
    | 'admin_dash_update_status_current'
    | 'admin_dash_update_status_ahead'
    | 'admin_dash_update_status_unknown';
  color: BadgeColor;
};

type NotifyTargets = {
  settingsEnabled: boolean;
  selectedCount: number;
  activeCount: number;
};

const unknownVersionStatus: VersionStatus = {
  labelKey: 'admin_dash_update_status_unknown',
  color: 'slate',
};

const updateModeOptions: Array<{
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

const statusFromCheck = (status: adminApi.DashUpdateVersionStatus | undefined): VersionStatus => {
  switch (status) {
    case 'available':
      return { labelKey: 'admin_dash_update_status_available', color: 'amber' };
    case 'current':
      return { labelKey: 'admin_dash_update_status_current', color: 'emerald' };
    case 'ahead':
      return { labelKey: 'admin_dash_update_status_ahead', color: 'indigo' };
    default:
      return unknownVersionStatus;
  }
};

const channelFromVersion = (version: string): DashUpdateChannel | null => {
  const parsed = parseVersion(version);
  if (!parsed) return null;
  return parsed.pre ? 'prerelease' : 'release';
};

const latestNoteForChannel = (
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

const updateJobBadge = (
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

const notificationTargetPath = '/admin?tab=alerts&alerts_tab=channels';

const UpdateRow: React.FC<{
  label: string;
  children: React.ReactNode;
}> = ({ label, children }) => (
  <div className="grid border-b border-(--theme-border-subtle) last:border-b-0 dark:border-(--theme-border-default) md:grid-cols-[160px_minmax(0,1fr)]">
    <div className="bg-(--theme-bg-muted)/70 p-4 text-sm font-semibold text-(--theme-fg-default) dark:bg-(--theme-canvas-subtle)">
      {label}
    </div>
    <div className="min-w-0 p-4 text-sm text-(--theme-fg-default)">{children}</div>
  </div>
);

const VersionText: React.FC<{ children: React.ReactNode; muted?: boolean }> = ({
  children,
  muted = false,
}) => (
  <span
    className={`font-mono text-sm font-semibold tabular-nums ${muted ? 'text-(--theme-fg-muted)' : 'text-(--theme-fg-default)'}`}
  >
    {children}
  </span>
);

const LoadingState: React.FC<{ label: string }> = ({ label }) => (
  <span className="inline-flex items-center gap-2 text-sm text-(--theme-fg-muted)">
    <LoaderCircle className="size-4 animate-spin" aria-hidden="true" />
    <span>{label}</span>
  </span>
);

const modeButtonClass = (active: boolean) =>
  `min-h-[5.25rem] rounded-lg border px-3 py-2 text-left transition-[background-color,border-color,box-shadow,color] ${
    active
      ? 'border-(--theme-fg-interactive) bg-(--theme-bg-accent-muted) text-(--theme-fg-default) shadow-[inset_0_0_0_1px_var(--theme-fg-interactive)]'
      : 'border-(--theme-border-subtle) bg-(--theme-bg-default) text-(--theme-fg-default) hover:bg-(--theme-surface-row-hover) dark:border-(--theme-border-default) dark:hover:bg-(--theme-canvas-subtle)'
  } disabled:cursor-not-allowed disabled:opacity-50`;

const readableError = (error: unknown, fallback: string): string => {
  if (error instanceof ApiError) return error.message || fallback;
  if (error instanceof Error && error.message) return error.message;
  return fallback;
};

const checkStep = async <T,>(task: Promise<T>, fallback: string): Promise<T> => {
  try {
    return await task;
  } catch (error) {
    if (isCanceledRequestError(error)) throw error;
    throw new Error(readableError(error, fallback));
  }
};

const conflictStatus = (error: unknown): adminApi.DashUpdateStatus | null => {
  if (!(error instanceof ApiError) || error.status !== 409) return null;
  const details = error.details;
  if (!details || typeof details !== 'object') return null;

  const status = (details as Partial<adminApi.DashUpdateStatus>).status;
  if (status !== 'idle' && status !== 'running' && status !== 'completed' && status !== 'failed') {
    return null;
  }
  return details as adminApi.DashUpdateStatus;
};

const updateStatusKey = (
  status: adminApi.DashUpdateStatusValue,
  id?: string,
  startedAt?: string,
  finishedAt?: string,
): string => id || `${status}:${startedAt ?? ''}:${finishedAt ?? ''}`;

const CheckFailedDialog: React.FC<{
  message: string;
  title: string;
  closeLabel: string;
  onClose: () => void;
}> = ({ message, title, closeLabel, onClose }) => {
  const titleId = React.useId();

  return (
    <Modal
      isOpen={Boolean(message)}
      onClose={onClose}
      maxWidth="max-w-md"
      zIndex={60}
      ariaLabelledby={titleId}
    >
      <ModalHeader title={title} onClose={onClose} id={titleId} className="py-3" />
      <ModalBody className="text-sm/relaxed text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) dark:text-(--theme-fg-control-hover)">
        {message}
      </ModalBody>
      <ModalFooter>
        <Button type="button" onClick={onClose}>
          {closeLabel}
        </Button>
      </ModalFooter>
    </Modal>
  );
};

const NoteLinks: React.FC<{
  note: DashReleaseNote | null;
  releaseNotesURL: string;
  sourceLabel: string;
}> = ({ note, releaseNotesURL, sourceLabel }) => {
  if (!note?.releaseUrl && !releaseNotesURL) return null;

  return (
    <div className="flex flex-wrap gap-3 text-xs">
      {note?.releaseUrl ? (
        <a
          href={note.releaseUrl}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1 font-semibold text-(--theme-fg-interactive) hover:text-(--theme-fg-interactive-hover)"
        >
          GitHub <ExternalLink size={12} />
        </a>
      ) : null}
      {releaseNotesURL ? (
        <a
          href={releaseNotesURL}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1 text-(--theme-fg-muted) hover:text-(--theme-fg-default)"
        >
          {sourceLabel} <ExternalLink size={12} />
        </a>
      ) : null}
    </div>
  );
};

const DashUpdateSettings: React.FC<Props> = ({
  enabled,
  settings,
  loadingSettings,
  savingChannel,
  savingMode,
  onChannelChange,
  onModeChange,
}) => {
  const { lang, t } = useI18n();
  const apiError = useApiErrorHandler();
  const {
    dialogProps: confirmDialogProps,
    request: requestConfirm,
    run: confirmAction,
  } = useConfirmDialog();
  const [version, setVersion] = React.useState<AppVersion | null>(null);
  const [versionCheck, setVersionCheck] = React.useState<adminApi.DashUpdateCheck | null>(null);
  const [loadingVersion, setLoadingVersion] = React.useState(false);
  const [releaseNotes, setReleaseNotes] = React.useState<DashReleaseNote[]>([]);
  const [releaseNotesURL, setReleaseNotesURL] = React.useState('');
  const [loadingNotes, setLoadingNotes] = React.useState(false);
  const [checkError, setCheckError] = React.useState('');
  const [checkRequested, setCheckRequested] = React.useState(false);
  const [lastCheckedAt, setLastCheckedAt] = React.useState<number | null>(null);
  const [updateStatus, setUpdateStatus] = React.useState<adminApi.DashUpdateStatus | null>(null);
  const [loadingUpdateStatus, setLoadingUpdateStatus] = React.useState(false);
  const [notifyTargets, setNotifyTargets] = React.useState<NotifyTargets | null>(null);
  const [loadingNotifyTargets, setLoadingNotifyTargets] = React.useState(false);
  const [notifyTargetsFailed, setNotifyTargetsFailed] = React.useState(false);
  const [startingUpdate, setStartingUpdate] = React.useState<adminApi.DashUpdateAction | null>(
    null,
  );
  const checkSeqRef = React.useRef(0);
  const checkRequestRef = React.useRef<{ seq: number; controller: AbortController } | null>(null);
  const lastUpdateStatusRef = React.useRef<adminApi.DashUpdateStatusValue | null>(null);
  const announcedUpdateRef = React.useRef('');
  const syncedCompletedUpdateRef = React.useRef('');

  const channel = settings?.dash_update_channel ?? 'release';
  const updateMode = settings?.dash_update_mode ?? 'manual';
  const updateStatusValue = updateStatus?.status ?? null;
  const updateStatusID = updateStatus?.id;
  const updateStartedAt = updateStatus?.started_at;
  const updateFinishedAt = updateStatus?.finished_at;
  const updateChannel = updateStatus?.channel;
  const isPrerelease = channel === 'prerelease';
  const needsNotifyTarget = updateMode === 'notify' || updateMode === 'auto';
  const latestNote = React.useMemo(
    () => latestNoteForChannel(releaseNotes, channel),
    [channel, releaseNotes],
  );
  const hasChecked = lastCheckedAt !== null;
  const checkingStatus = loadingVersion || loadingUpdateStatus;
  const checking = checkingStatus || loadingNotes;
  const checkingUpdate = checkRequested && checking;
  const isUpdateRunning = updateStatusValue === 'running' || startingUpdate !== null;
  const hasUpdateJob = Boolean(updateStatus && updateStatusValue !== 'idle');
  const showJob = hasChecked || hasUpdateJob;
  const versionStatus = statusFromCheck(versionCheck?.version_status);
  const jobBadge = updateJobBadge(updateStatus);
  const updateUnavailable = updateStatus?.available === false;
  const actionDisabled =
    !settings ||
    loadingSettings ||
    savingChannel ||
    savingMode ||
    checkingStatus ||
    isUpdateRunning ||
    updateUnavailable;
  const actionHint = updateUnavailable
    ? updateStatus?.unavailable_reason || t('admin_dash_update_job_unavailable')
    : '';
  const checkDisabled =
    !settings || loadingSettings || savingChannel || checking || isUpdateRunning || updateUnavailable;
  const lastCheckedLabel = React.useMemo(() => {
    if (!lastCheckedAt) return '';
    return new Intl.DateTimeFormat(lang === 'en' ? 'en-US' : 'zh-CN', {
      dateStyle: 'short',
      timeStyle: 'medium',
    }).format(new Date(lastCheckedAt));
  }, [lang, lastCheckedAt]);
  const currentVersionText =
    version?.version ??
    versionCheck?.current_version ??
    (loadingVersion ? t('loading') : t('common_unknown'));
  const nodeVersionText =
    version?.node_version ??
    versionCheck?.bundled_node_version ??
    (loadingVersion ? t('loading') : t('common_unknown'));
  const latestVersionText = versionCheck?.latest_version ?? '-';
  const updaterBadge = React.useMemo((): { label: string; color: BadgeColor } => {
    if (loadingUpdateStatus && !updateStatus) return { label: t('loading'), color: 'slate' };
    if (updateStatus?.available === false) {
      return { label: t('admin_dash_update_runner_unavailable'), color: 'rose' };
    }
    if (updateStatus?.available === true) {
      return { label: t('admin_dash_update_runner_available'), color: 'emerald' };
    }
    return { label: t('admin_dash_update_status_unknown'), color: 'slate' };
  }, [loadingUpdateStatus, t, updateStatus]);
  const updaterStatusHint =
    updateStatus?.available === false
      ? updateStatus.unavailable_reason || t('admin_dash_update_job_unavailable')
      : t('admin_dash_update_runner_available_hint');
  const notifyTargetBadge = React.useMemo((): { label: string; color: BadgeColor } => {
    if (loadingNotifyTargets) return { label: t('loading'), color: 'slate' };
    if (notifyTargetsFailed) {
      return { label: t('admin_dash_update_notify_target_failed'), color: 'rose' };
    }
    if (!notifyTargets?.settingsEnabled) {
      return { label: t('admin_dash_update_notify_target_disabled'), color: 'amber' };
    }
    if (notifyTargets.activeCount > 0) {
      return {
        label: t('admin_dash_update_notify_target_ready', {
          count: String(notifyTargets.activeCount),
        }),
        color: 'emerald',
      };
    }
    return { label: t('admin_dash_update_notify_target_missing'), color: 'rose' };
  }, [loadingNotifyTargets, notifyTargets, notifyTargetsFailed, t]);
  const notifyTargetHint = React.useMemo(() => {
    if (!notifyTargets?.settingsEnabled) return t('admin_dash_update_notify_target_disabled_hint');
    if ((notifyTargets?.selectedCount ?? 0) > 0 && notifyTargets?.activeCount === 0) {
      return t('admin_dash_update_notify_target_paused_hint');
    }
    return t('admin_dash_update_notify_target_hint');
  }, [notifyTargets, t]);

  const loadVersion = React.useCallback(
    async (signal: AbortSignal) => {
      setLoadingVersion(true);
      try {
        const next = await fetchAppVersion({ signal });
        setVersion(next);
        setBundledNodeVersion(next.node_version ?? '');
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_dash_update_version_fetch_failed' });
      } finally {
        if (!signal.aborted) setLoadingVersion(false);
      }
    },
    [apiError],
  );

  const loadUpdateStatus = React.useCallback(
    async (signal: AbortSignal, silent = false) => {
      setLoadingUpdateStatus(true);
      try {
        setUpdateStatus(await adminApi.fetchDashUpdateStatus({ signal }));
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        if (!silent) apiError(error, { key: 'admin_dash_update_status_fetch_failed' });
      } finally {
        if (!signal.aborted) setLoadingUpdateStatus(false);
      }
    },
    [apiError],
  );

  const loadNotifyTargets = React.useCallback(
    async (signal: AbortSignal) => {
      setLoadingNotifyTargets(true);
      setNotifyTargetsFailed(false);
      try {
        const [settingsDoc, channels] = await Promise.all([
          adminApi.fetchAlertSettings({ signal }),
          adminApi.fetchAlertChannels({ signal }),
        ]);
        if (signal.aborted) return;
        const selectedIDs = new Set(settingsDoc.channel_ids);
        let activeCount = 0;
        for (const channel of channels) {
          if (channel.enabled && selectedIDs.has(channel.id)) activeCount += 1;
        }
        setNotifyTargets({
          settingsEnabled: settingsDoc.enabled,
          selectedCount: settingsDoc.channel_ids.length,
          activeCount,
        });
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        setNotifyTargets(null);
        setNotifyTargetsFailed(true);
      } finally {
        if (!signal.aborted) setLoadingNotifyTargets(false);
      }
    },
    [],
  );

  const syncCompletedUpdate = React.useCallback(
    async (targetChannel: DashUpdateChannel, signal: AbortSignal) => {
      setCheckRequested(true);
      setLoadingVersion(true);
      setVersionCheck(null);

      const versionTask = fetchAppVersion({ signal }).then(
        (next) => ({ ok: true as const, next }),
        (error: unknown) => ({ ok: false as const, error }),
      );
      const checkTask = adminApi.fetchDashUpdateCheck({ channel: targetChannel, signal }).then(
        (next) => ({ ok: true as const, next }),
        (error: unknown) => ({ ok: false as const, error }),
      );

      try {
        const [versionResult, checkResult] = await Promise.all([versionTask, checkTask]);
        if (signal.aborted) return;

        if (versionResult.ok) {
          setVersion(versionResult.next);
          setBundledNodeVersion(versionResult.next.node_version ?? '');
        }
        if (checkResult.ok) {
          setVersionCheck(checkResult.next);
          if (!versionResult.ok) {
            setVersion({
              version: checkResult.next.current_version,
              node_version: checkResult.next.bundled_node_version,
            });
            setBundledNodeVersion(checkResult.next.bundled_node_version);
          }
          setLastCheckedAt(Date.now());
        }

        const error = !checkResult.ok
          ? checkResult.error
          : !versionResult.ok
            ? versionResult.error
            : null;
        if (error && !isCanceledRequestError(error)) {
          apiError(error, { key: 'admin_dash_update_version_fetch_failed' });
        }
      } finally {
        if (!signal.aborted) setLoadingVersion(false);
      }
    },
    [apiError],
  );

  React.useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    void loadVersion(controller.signal);
    void loadUpdateStatus(controller.signal, true);
    void loadNotifyTargets(controller.signal);
    return () => {
      controller.abort();
    };
  }, [enabled, loadNotifyTargets, loadUpdateStatus, loadVersion]);

  const cancelCheck = React.useCallback((resetLoading = true) => {
    const current = checkRequestRef.current;
    if (!current) return;

    checkSeqRef.current += 1;
    checkRequestRef.current = null;
    current.controller.abort();
    if (!resetLoading) return;
    setLoadingVersion(false);
    setLoadingNotes(false);
    setLoadingUpdateStatus(false);
  }, []);

  React.useEffect(() => () => cancelCheck(false), [cancelCheck]);

  React.useEffect(() => {
    cancelCheck();
    setVersionCheck(null);
    setReleaseNotes([]);
    setReleaseNotesURL('');
    setCheckRequested(false);
    setLastCheckedAt(null);
  }, [cancelCheck, channel, lang]);

  React.useEffect(() => {
    if (!enabled || updateStatusValue !== 'running') return;
    const controller = new AbortController();
    const timer = window.setInterval(() => {
      void loadUpdateStatus(controller.signal, true);
    }, 2500);
    return () => {
      window.clearInterval(timer);
      controller.abort();
    };
  }, [enabled, loadUpdateStatus, updateStatusValue]);

  React.useEffect(() => {
    const previous = lastUpdateStatusRef.current;
    lastUpdateStatusRef.current = updateStatusValue;

    if (
      !enabled ||
      (updateStatusValue !== 'completed' && updateStatusValue !== 'failed')
    ) {
      return;
    }

    const statusKey = updateStatusKey(
      updateStatusValue,
      updateStatusID,
      updateStartedAt,
      updateFinishedAt,
    );
    if (previous === 'running' && announcedUpdateRef.current !== statusKey) {
      announcedUpdateRef.current = statusKey;
      pushTopBanner(
        updateStatusValue === 'completed'
          ? t('admin_dash_update_job_completed')
          : t('admin_dash_update_job_failed'),
        { tone: updateStatusValue === 'completed' ? 'info' : 'error', durationMs: 6000 },
      );
    }

    if (updateStatusValue !== 'completed' || syncedCompletedUpdateRef.current === statusKey) {
      return;
    }
    syncedCompletedUpdateRef.current = statusKey;

    const controller = new AbortController();
    void syncCompletedUpdate(updateChannel ?? channel, controller.signal);
    return () => {
      controller.abort();
    };
  }, [
    channel,
    enabled,
    syncCompletedUpdate,
    t,
    updateChannel,
    updateFinishedAt,
    updateStartedAt,
    updateStatusID,
    updateStatusValue,
  ]);

  const checkUpdate = React.useCallback(async () => {
    cancelCheck(false);
    const seq = checkSeqRef.current + 1;
    const controller = new AbortController();
    checkSeqRef.current = seq;
    checkRequestRef.current = { seq, controller };
    const hadChecked = hasChecked;
    const langCode = lang === 'en' ? 'en' : 'zh';
    const isCurrent = () => checkRequestRef.current?.seq === seq;

    setCheckRequested(true);
    setCheckError('');
    setLoadingVersion(true);
    setLoadingNotes(true);
    setLoadingUpdateStatus(true);

    const notesTask = adminApi
      .fetchDashReleaseNotes({ lang: langCode, signal: controller.signal })
      .then(
        (doc) => ({ ok: true as const, doc }),
        (error: unknown) => ({ ok: false as const, error }),
      );

    try {
      const [nextCheck, nextStatus] = await Promise.all([
        checkStep(
          adminApi.fetchDashUpdateCheck({ channel, signal: controller.signal }),
          t('admin_dash_update_version_fetch_failed'),
        ),
        checkStep(
          adminApi.fetchDashUpdateStatus({ signal: controller.signal }),
          t('admin_dash_update_status_fetch_failed'),
        ),
      ]);
      if (!isCurrent() || controller.signal.aborted) return;
      setVersionCheck(nextCheck);
      setVersion({
        version: nextCheck.current_version,
        node_version: nextCheck.bundled_node_version,
      });
      setBundledNodeVersion(nextCheck.bundled_node_version);
      setUpdateStatus(nextStatus);
      setLastCheckedAt(Date.now());
      setLoadingVersion(false);
      setLoadingUpdateStatus(false);

      const notes = await notesTask;
      if (!isCurrent() || controller.signal.aborted) return;
      if (notes.ok) {
        setReleaseNotes(parseDashReleaseNotes(notes.doc.html));
        setReleaseNotesURL(notes.doc.source_url);
      } else if (!isCanceledRequestError(notes.error)) {
        setReleaseNotes([]);
        setReleaseNotesURL('');
        apiError(notes.error, { key: 'admin_dash_update_notes_fetch_failed' });
      }
    } catch (error) {
      if (!isCurrent()) return;
      controller.abort();
      if (isCanceledRequestError(error)) return;
      if (!hadChecked) {
        setVersionCheck(null);
        setReleaseNotes([]);
        setReleaseNotesURL('');
        setCheckRequested(false);
        setLastCheckedAt(null);
      }
      setCheckError(readableError(error, t('admin_dash_update_check_failed_message')));
    } finally {
      if (isCurrent()) {
        checkRequestRef.current = null;
        setLoadingVersion(false);
        setLoadingNotes(false);
        setLoadingUpdateStatus(false);
      }
    }
  }, [apiError, cancelCheck, channel, hasChecked, lang, t]);

  const togglePrerelease = React.useCallback(async () => {
    if (isPrerelease) {
      onChannelChange('release');
      return;
    }

    const confirmed = await requestConfirm({
      title: t('admin_dash_update_channel_prerelease_confirm_title'),
      message: t('admin_dash_update_channel_desc'),
      confirmLabel: t('admin_dash_update_channel_enable'),
      cancelLabel: t('common_cancel'),
    });
    if (!confirmed) return;
    onChannelChange('prerelease');
  }, [isPrerelease, onChannelChange, requestConfirm, t]);

  const runUpdate = React.useCallback(
    (action: adminApi.DashUpdateAction) => {
      const titleKey: TranslationKey =
        action === 'reinstall'
          ? 'admin_dash_update_reinstall_confirm_title'
          : 'admin_dash_update_run_confirm_title';
      const confirmKey: TranslationKey =
        action === 'reinstall' ? 'admin_dash_update_run_reinstall' : 'admin_dash_update_run_update';

      void confirmAction(
        {
          title: t(titleKey),
          message: t('admin_dash_update_run_confirm_message', {
            channel: t(
              isPrerelease
                ? 'admin_dash_update_channel_prerelease'
                : 'admin_dash_update_channel_release',
            ),
          }),
          confirmLabel: t(confirmKey),
          cancelLabel: t('common_cancel'),
        },
        async () => {
          setStartingUpdate(action);
          try {
            const next = await adminApi.runDashUpdate({
              action,
              channel,
              lang: lang === 'en' ? 'en' : 'zh',
            });
            setUpdateStatus(next);
            setLastCheckedAt(Date.now());
            pushTopBanner(t('admin_dash_update_started'), { tone: 'info', durationMs: 5000 });
          } catch (error) {
            const current = conflictStatus(error);
            if (current) {
              setUpdateStatus(current);
              setLastCheckedAt(Date.now());
              return;
            }
            apiError(error, { key: 'admin_dash_update_run_failed' });
          } finally {
            setStartingUpdate(null);
          }
        },
      );
    },
    [apiError, channel, confirmAction, isPrerelease, lang, t],
  );

  return (
    <div className="space-y-4">
      <ConfirmDialog {...confirmDialogProps} />
      <CheckFailedDialog
        message={checkError}
        title={t('admin_dash_update_check_failed_title')}
        closeLabel={t('common_close')}
        onClose={() => setCheckError('')}
      />

      <section className="overflow-hidden rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-default) dark:border-(--theme-border-default)">
        <UpdateRow label={t('admin_dash_update_runner_title')}>
          <div className="flex min-w-0 flex-col gap-2">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <Badge color={updaterBadge.color}>{updaterBadge.label}</Badge>
            </div>
            <p className="wrap-break-word text-xs/5 text-(--theme-fg-muted)">{updaterStatusHint}</p>
          </div>
        </UpdateRow>

        <UpdateRow label={t('admin_dash_update_current_version')}>
          <VersionText>{currentVersionText}</VersionText>
        </UpdateRow>

        <UpdateRow label={t('admin_dash_update_bundled_node')}>
          <VersionText>{nodeVersionText}</VersionText>
        </UpdateRow>

        {checkRequested ? (
          <UpdateRow label={t('admin_dash_update_latest_version')}>
            {checkingUpdate ? (
              <LoadingState label={t('admin_dash_update_checking')} />
            ) : (
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <VersionText muted={!versionCheck?.latest_version}>{latestVersionText}</VersionText>
                {hasChecked ? (
                  <Badge color={versionStatus.color}>{t(versionStatus.labelKey)}</Badge>
                ) : null}
                {latestNote?.publishedAt ? (
                  <span className="text-xs text-(--theme-fg-muted)">{latestNote.publishedAt}</span>
                ) : null}
                {lastCheckedLabel ? (
                  <span className="text-xs text-(--theme-fg-muted)">
                    {t('admin_dash_update_last_checked', { time: lastCheckedLabel })}
                  </span>
                ) : null}
              </div>
            )}
          </UpdateRow>
        ) : null}

        <UpdateRow label={t('admin_dash_update_channel_prerelease')}>
          <IOSSwitch
            checked={isPrerelease}
            disabled={
              !settings || loadingSettings || savingChannel || savingMode || isUpdateRunning
            }
            ariaLabel={t('admin_dash_update_channel_prerelease')}
            onChange={() => void togglePrerelease()}
          />
        </UpdateRow>

        <UpdateRow label={t('admin_dash_update_mode_title')}>
          <div className="grid w-full gap-2 sm:grid-cols-3">
            {updateModeOptions.map((option) => {
              const active = updateMode === option.mode;
              return (
                <button
                  key={option.mode}
                  type="button"
                  className={modeButtonClass(active)}
                  aria-pressed={active}
                  disabled={!settings || loadingSettings || savingMode || isUpdateRunning}
                  onClick={() => onModeChange(option.mode)}
                >
                  <span className="block text-sm font-semibold">{t(option.labelKey)}</span>
                  <span className="mt-1 block text-xs/5 text-(--theme-fg-muted)">
                    {t(option.hintKey)}
                  </span>
                </button>
              );
            })}
          </div>
        </UpdateRow>

        {needsNotifyTarget ? (
          <UpdateRow label={t('admin_dash_update_notify_target')}>
            <div className="flex min-w-0 flex-col gap-2">
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <Badge color={notifyTargetBadge.color}>{notifyTargetBadge.label}</Badge>
                <Link
                  to={notificationTargetPath}
                  className="text-xs font-semibold text-(--theme-fg-interactive) underline-offset-2 hover:underline"
                >
                  {t('admin_dash_update_notify_target_configure')}
                </Link>
              </div>
              <p className="text-xs/5 text-(--theme-fg-muted)">{notifyTargetHint}</p>
            </div>
          </UpdateRow>
        ) : null}

        {showJob ? (
          <UpdateRow label={t('admin_dash_update_job_title')}>
            <Badge color={jobBadge.color}>{t(jobBadge.labelKey)}</Badge>
          </UpdateRow>
        ) : null}

        {hasChecked ? (
          <UpdateRow label={t('admin_dash_update_action_title')}>
            <div className="flex flex-wrap gap-2">
              <Tooltip content={actionHint} className="inline-flex">
                <span className="inline-flex">
                  <Button
                    type="button"
                    icon={RefreshCw}
                    disabled={actionDisabled}
                    onClick={() => runUpdate('update')}
                  >
                    {startingUpdate === 'update'
                      ? t('admin_dash_update_starting')
                      : t('admin_dash_update_run_update')}
                  </Button>
                </span>
              </Tooltip>
              <Tooltip content={actionHint} className="inline-flex">
                <span className="inline-flex">
                  <Button
                    type="button"
                    variant="secondary"
                    icon={RotateCcw}
                    disabled={actionDisabled}
                    onClick={() => runUpdate('reinstall')}
                  >
                    {startingUpdate === 'reinstall'
                      ? t('admin_dash_update_starting')
                      : t('admin_dash_update_run_reinstall')}
                  </Button>
                </span>
              </Tooltip>
            </div>
          </UpdateRow>
        ) : null}

        {updateStatus?.status === 'failed' && updateStatus.log_tail ? (
          <UpdateRow label={t('admin_dash_update_log_tail')}>
            <pre className="max-h-72 overflow-auto rounded-lg bg-(--theme-bg-muted) p-3 text-xs/5 text-(--theme-fg-default) dark:bg-(--theme-canvas-subtle)">
              {updateStatus.log_tail}
            </pre>
          </UpdateRow>
        ) : null}

        {hasChecked ? (
          <UpdateRow label={t('admin_dash_update_release_notes_title')}>
            <div className="space-y-4">
              <NoteLinks
                note={latestNote}
                releaseNotesURL={releaseNotesURL}
                sourceLabel={t('admin_dash_update_notes_source')}
              />

              {latestNote ? (
                <div className="grid gap-x-8 gap-y-5 lg:grid-cols-2">
                  {latestNote.sections.map((section) => (
                    <section key={section.title} className="min-w-0">
                      <h4 className="text-sm font-semibold text-(--theme-fg-default)">
                        {section.title}
                      </h4>
                      <ul className="mt-2 space-y-1.5 text-sm/6 text-(--theme-fg-default)">
                        {section.items.map((item) => (
                          <li key={item} className="flex gap-2">
                            <span className="mt-2 size-1.5 shrink-0 rounded-full bg-(--theme-fg-muted)" />
                            <span>{item}</span>
                          </li>
                        ))}
                      </ul>
                    </section>
                  ))}
                </div>
              ) : (
                <div className="py-2 text-sm text-(--theme-fg-muted)">
                  {loadingNotes ? t('loading') : t('admin_dash_update_notes_empty')}
                </div>
              )}
            </div>
          </UpdateRow>
        ) : null}
      </section>

      <div className="flex justify-start">
        <Button type="button" disabled={checkDisabled} onClick={() => void checkUpdate()}>
          {checkingUpdate
            ? t('admin_dash_update_checking')
            : hasChecked
              ? t('admin_dash_update_recheck')
              : t('admin_dash_update_start_check')}
        </Button>
      </div>
    </div>
  );
};

export default DashUpdateSettings;
