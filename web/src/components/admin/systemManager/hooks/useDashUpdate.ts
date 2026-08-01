import React from 'react';
import type { AppVersion } from '@app-types/api';
import type { DashUpdateChannel } from '@app-types/admin';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useConfirmDialog } from '@hooks/useConfirmDialog';
import { useI18n } from '@i18n';
import * as adminApi from '@lib/adminApi';
import { clearDashUpdateReload, readDashUpdateReload } from '@lib/dashUpdateSession';
import { fetchAppVersion } from '@lib/versionApi';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { isCanceledRequestError } from '@utils/errors';
import { createSeqGate } from '@utils/seqGate';
import {
  checkStep,
  conflictStatus,
  readableError,
  updateStatusKey,
  type NotifyTargets,
} from '../dashUpdateModel';
import { parseDashReleaseNotes, type DashReleaseNote } from '../dashReleaseNotesModel';

type Options = {
  enabled: boolean;
  channel: DashUpdateChannel;
  onChannelChange: (channel: DashUpdateChannel) => void;
};

type Resource<T> = {
  value: T | null;
  loading: boolean;
};

type CheckState = {
  version: adminApi.DashUpdateCheck | null;
  notes: DashReleaseNote[];
  notesURL: string;
  checkedAt: number | null;
  error: string;
  checking: boolean;
  loadingNotes: boolean;
};

type NotifyState = {
  value: NotifyTargets | null;
  phase: 'loading' | 'ready' | 'failed';
};

const initialCheck: CheckState = {
  version: null,
  notes: [],
  notesURL: '',
  checkedAt: null,
  error: '',
  checking: false,
  loadingNotes: false,
};

const updateStatusPollInterval = 30_000;
const runningUpdateStatusPollInterval = 2_500;

export const useDashUpdate = ({ enabled, channel, onChannelChange }: Options) => {
  const { lang, t } = useI18n();
  const apiError = useApiErrorHandler();
  const {
    dialogProps: confirmDialogProps,
    request: requestConfirm,
    run: confirmAction,
  } = useConfirmDialog();
  const [version, setVersion] = React.useState<Resource<AppVersion>>({
    value: null,
    loading: false,
  });
  const [status, setStatus] = React.useState<Resource<adminApi.DashUpdateStatus>>({
    value: null,
    loading: false,
  });
  const [check, setCheck] = React.useState<CheckState>(initialCheck);
  const [notify, setNotify] = React.useState<NotifyState>({ value: null, phase: 'loading' });
  const [startingUpdate, setStartingUpdate] = React.useState<adminApi.DashUpdateAction | null>(
    null,
  );
  const [updatedVersion, setUpdatedVersion] = React.useState('');
  const versionGate = React.useMemo(() => createSeqGate(), []);
  const statusGate = React.useMemo(() => createSeqGate(), []);
  const checkRequestRef = React.useRef<AbortController | null>(null);
  const startRequestRef = React.useRef<AbortController | null>(null);
  const previousStatusRef = React.useRef<adminApi.DashUpdateStatusValue | null>(null);
  const announcedStatusRef = React.useRef('');

  const acceptVersion = React.useCallback(
    (value: AppVersion) => {
      versionGate.invalidate();
      setVersion({ value, loading: false });
    },
    [versionGate],
  );

  const loadStatus = React.useCallback(
    async (signal: AbortSignal, background = false): Promise<adminApi.DashUpdateStatus> => {
      const seq = statusGate.next();
      if (!background) setStatus((current) => ({ ...current, loading: true }));
      try {
        const value = await adminApi.fetchDashUpdateStatus({ signal });
        if (!signal.aborted && statusGate.isCurrent(seq)) {
          setStatus({ value, loading: false });
        }
        return value;
      } finally {
        if (statusGate.isCurrent(seq)) {
          setStatus((current) => (current.loading ? { ...current, loading: false } : current));
        }
      }
    },
    [statusGate],
  );

  const acceptStatus = React.useCallback(
    (value: adminApi.DashUpdateStatus) => {
      statusGate.invalidate();
      setStatus({ value, loading: false });
    },
    [statusGate],
  );

  const cancelCheck = React.useCallback(() => {
    checkRequestRef.current?.abort();
    checkRequestRef.current = null;
  }, []);

  React.useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    const versionSeq = versionGate.next();
    setVersion((current) => ({ ...current, loading: true }));

    void (async () => {
      try {
        const value = await fetchAppVersion({ signal: controller.signal });
        if (!controller.signal.aborted && versionGate.isCurrent(versionSeq)) {
          setVersion({ value, loading: false });
        }
      } catch (error) {
        if (isCanceledRequestError(error) || !versionGate.isCurrent(versionSeq)) return;
        setVersion((current) => ({ ...current, loading: false }));
        apiError(error, { key: 'admin_dash_update_version_fetch_failed' });
      }
    })();

    void (async () => {
      try {
        const [settings, channels] = await Promise.all([
          adminApi.fetchAlertSettings({ signal: controller.signal }),
          adminApi.fetchAlertChannels({ signal: controller.signal }),
        ]);
        if (controller.signal.aborted) return;
        const selectedIDs = new Set(settings.channel_ids);
        const activeCount = channels.filter(
          (item) => item.config !== null && item.enabled && selectedIDs.has(item.id),
        ).length;
        setNotify({
          value: {
            settingsEnabled: settings.enabled,
            selectedCount: settings.channel_ids.length,
            activeCount,
          },
          phase: 'ready',
        });
      } catch (error) {
        if (!isCanceledRequestError(error)) setNotify({ value: null, phase: 'failed' });
      }
    })();

    return () => controller.abort();
  }, [apiError, enabled, versionGate]);

  const statusValue = status.value?.status ?? null;

  React.useEffect(() => {
    if (!enabled || startingUpdate !== null) return;
    const controller = new AbortController();
    let timer = 0;
    const interval =
      statusValue === 'running' ? runningUpdateStatusPollInterval : updateStatusPollInterval;
    const poll = async (background = true) => {
      try {
        await loadStatus(controller.signal, background);
      } catch (error) {
        if (isCanceledRequestError(error)) return;
        // Polling retries on the next interval.
      }
      if (!controller.signal.aborted) timer = window.setTimeout(poll, interval);
    };
    if (statusValue === null) {
      void poll(false);
    } else {
      timer = window.setTimeout(poll, interval);
    }
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [enabled, loadStatus, startingUpdate, statusValue]);

  React.useEffect(() => {
    if (!enabled) {
      startRequestRef.current?.abort();
      startRequestRef.current = null;
      setStartingUpdate(null);
      return;
    }
    return () => {
      startRequestRef.current?.abort();
      startRequestRef.current = null;
    };
  }, [enabled]);

  React.useEffect(() => {
    const previous = previousStatusRef.current;
    previousStatusRef.current = statusValue;
    if (!enabled || (statusValue !== 'completed' && statusValue !== 'failed')) return;

    const current = status.value;
    if (!current) return;
    const reload = readDashUpdateReload();
    if (reload) {
      if (statusValue === 'failed') {
        clearDashUpdateReload();
      } else if (statusValue === 'completed') {
        clearDashUpdateReload();
        if (!current.target_version || current.target_version === reload.version) {
          setUpdatedVersion(reload.version);
        }
      }
    }
    const key = updateStatusKey(current);
    if (previous !== 'running' || announcedStatusRef.current === key) return;

    if (statusValue === 'completed') {
      cancelCheck();
      setCheck(initialCheck);
    }
    announcedStatusRef.current = key;
    pushTopBanner(
      statusValue === 'completed'
        ? t('admin_dash_update_job_completed')
        : t('admin_dash_update_job_failed'),
      { tone: statusValue === 'completed' ? 'info' : 'error', durationMs: 6000 },
    );
  }, [cancelCheck, enabled, status.value, statusValue, t]);

  React.useEffect(() => {
    cancelCheck();
    setCheck(initialCheck);
    return cancelCheck;
  }, [cancelCheck, channel, lang]);

  const checkUpdate = React.useCallback(async () => {
    cancelCheck();
    const controller = new AbortController();
    checkRequestRef.current = controller;
    const hadChecked = check.checkedAt !== null;
    const isCurrent = () => checkRequestRef.current === controller && !controller.signal.aborted;

    setCheck((current) => ({
      ...current,
      error: '',
      checking: true,
      loadingNotes: true,
    }));
    const notesTask = adminApi
      .fetchDashReleaseNotes({
        lang: lang === 'en' ? 'en' : 'zh',
        signal: controller.signal,
      })
      .then(
        (doc) => ({ ok: true as const, doc }),
        (error: unknown) => ({ ok: false as const, error }),
      );

    try {
      const [nextCheck] = await Promise.all([
        checkStep(
          adminApi.fetchDashUpdateCheck({ channel, signal: controller.signal }),
          t('admin_dash_update_version_fetch_failed'),
        ),
        checkStep(loadStatus(controller.signal), t('admin_dash_update_status_fetch_failed')),
      ]);
      if (!isCurrent()) return;

      acceptVersion({
        version: nextCheck.current_version,
        node_version: nextCheck.bundled_node_version,
      });
      setCheck((current) => ({
        ...current,
        version: nextCheck,
        checkedAt: Date.now(),
        checking: false,
      }));

      const notes = await notesTask;
      if (!isCurrent()) return;
      if (notes.ok) {
        setCheck((current) => ({
          ...current,
          notes: parseDashReleaseNotes(notes.doc.html),
          notesURL: notes.doc.source_url,
          loadingNotes: false,
        }));
      } else if (!isCanceledRequestError(notes.error)) {
        setCheck((current) => ({
          ...current,
          notes: [],
          notesURL: '',
          loadingNotes: false,
        }));
        apiError(notes.error, { key: 'admin_dash_update_notes_fetch_failed' });
      }
    } catch (error) {
      if (!isCurrent()) return;
      controller.abort();
      if (isCanceledRequestError(error)) return;

      const message = readableError(error, t('admin_dash_update_check_failed_message'));
      setCheck((current) =>
        hadChecked
          ? { ...current, error: message, checking: false, loadingNotes: false }
          : { ...initialCheck, error: message },
      );
    } finally {
      if (checkRequestRef.current === controller) {
        checkRequestRef.current = null;
        setCheck((current) =>
          current.checking || current.loadingNotes
            ? { ...current, checking: false, loadingNotes: false }
            : current,
        );
      }
    }
  }, [acceptVersion, apiError, cancelCheck, channel, check.checkedAt, lang, loadStatus, t]);

  const togglePrerelease = React.useCallback(async () => {
    if (channel === 'prerelease') {
      onChannelChange('release');
      return;
    }

    const confirmed = await requestConfirm({
      title: t('admin_dash_update_channel_prerelease_confirm_title'),
      message: t('admin_dash_update_channel_desc'),
      confirmLabel: t('admin_dash_update_channel_enable'),
      cancelLabel: t('common_cancel'),
    });
    if (confirmed) onChannelChange('prerelease');
  }, [channel, onChannelChange, requestConfirm, t]);

  const runUpdate = React.useCallback(
    (action: adminApi.DashUpdateAction) => {
      const titleKey =
        action === 'reinstall'
          ? 'admin_dash_update_reinstall_confirm_title'
          : 'admin_dash_update_run_confirm_title';
      const confirmKey =
        action === 'reinstall' ? 'admin_dash_update_run_reinstall' : 'admin_dash_update_run_update';

      void confirmAction(
        {
          title: t(titleKey),
          message: t('admin_dash_update_run_confirm_message', {
            channel: t(
              channel === 'prerelease'
                ? 'admin_dash_update_channel_prerelease'
                : 'admin_dash_update_channel_release',
            ),
          }),
          confirmLabel: t(confirmKey),
          cancelLabel: t('common_cancel'),
        },
        async () => {
          const checked = check.version;
          if (!checked || checked.target_channel !== channel) {
            pushTopBanner(t('admin_dash_update_check_failed_message'), {
              tone: 'error',
              durationMs: 5000,
            });
            return;
          }

          startRequestRef.current?.abort();
          const controller = new AbortController();
          startRequestRef.current = controller;
          const previousStatus = status.value;
          const previousStatusKey = previousStatus ? updateStatusKey(previousStatus) : '';
          statusGate.invalidate();
          setStartingUpdate(action);
          try {
            const next = await adminApi.runDashUpdate(
              {
                action,
                channel,
                lang: lang === 'en' ? 'en' : 'zh',
                target_version: checked.latest_version,
                expected_current_version: checked.current_version,
                expected_install_revision: checked.install_revision,
              },
              { signal: controller.signal },
            );
            acceptStatus(next);
            pushTopBanner(t('admin_dash_update_started'), { tone: 'info', durationMs: 5000 });
          } catch (error) {
            if (controller.signal.aborted || isCanceledRequestError(error)) return;
            const current = conflictStatus(error);
            if (current) {
              acceptStatus(current);
              return;
            }

            try {
              const confirmed = await adminApi.fetchDashUpdateStatus({
                signal: controller.signal,
              });
              const confirmedKey = updateStatusKey(confirmed);
              if (confirmed.status !== 'idle' && confirmedKey !== previousStatusKey) {
                acceptStatus(confirmed);
                if (confirmed.status === 'completed') {
                  cancelCheck();
                  setCheck(initialCheck);
                }
                if (confirmed.status === 'running') {
                  pushTopBanner(t('admin_dash_update_started'), {
                    tone: 'info',
                    durationMs: 5000,
                  });
                }
                return;
              }
            } catch (confirmError) {
              if (controller.signal.aborted || isCanceledRequestError(confirmError)) return;
            }
            apiError(error, { key: 'admin_dash_update_run_failed' });
          } finally {
            if (startRequestRef.current === controller) {
              startRequestRef.current = null;
              setStartingUpdate(null);
            }
          }
        },
      );
    },
    [
      acceptStatus,
      apiError,
      cancelCheck,
      channel,
      check.version,
      confirmAction,
      lang,
      status.value,
      statusGate,
      t,
    ],
  );

  return {
    version: version.value,
    versionCheck: check.version,
    loadingVersion: version.loading || check.checking,
    releaseNotes: check.notes,
    releaseNotesURL: check.notesURL,
    loadingNotes: check.loadingNotes,
    checkError: check.error,
    checkRequested: check.checking || check.checkedAt !== null,
    lastCheckedAt: check.checkedAt,
    updateStatus: status.value,
    loadingUpdateStatus: status.loading,
    notifyTargets: notify.value,
    loadingNotifyTargets: notify.phase === 'loading',
    notifyTargetsFailed: notify.phase === 'failed',
    startingUpdate,
    updatedVersion,
    confirmDialogProps,
    checkUpdate,
    togglePrerelease,
    runUpdate,
    dismissUpdateSuccess: () => setUpdatedVersion(''),
    dismissCheckError: () => setCheck((current) => ({ ...current, error: '' })),
  };
};

export type DashUpdateController = ReturnType<typeof useDashUpdate>;
