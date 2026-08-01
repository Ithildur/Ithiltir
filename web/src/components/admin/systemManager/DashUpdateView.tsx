import React from 'react';
import CheckCircle2 from 'lucide-react/dist/esm/icons/check-circle-2';
import ExternalLink from 'lucide-react/dist/esm/icons/external-link';
import LoaderCircle from 'lucide-react/dist/esm/icons/loader-circle';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import RotateCcw from 'lucide-react/dist/esm/icons/rotate-ccw';
import { Link } from 'react-router';
import type { DashUpdateMode, SystemSettings } from '@app-types/admin';
import Badge, { type BadgeColor } from '@components/ui/Badge';
import Button from '@components/ui/Button';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import IOSSwitch from '@components/ui/IOSSwitch';
import { Modal, ModalBody, ModalFooter, ModalHeader } from '@components/ui/Modal';
import { Tooltip } from '@components/ui/Tooltip';
import { useI18n } from '@i18n';
import {
  latestNoteForChannel,
  notificationTargetPath,
  statusFromCheck,
  updateJobBadge,
  updateModeOptions,
} from './dashUpdateModel';
import type { DashReleaseNote } from './dashReleaseNotesModel';
import type { DashUpdateController } from './hooks/useDashUpdate';

type Props = {
  settings: SystemSettings | null;
  loadingSettings: boolean;
  savingChannel: boolean;
  savingMode: boolean;
  onModeChange: (mode: DashUpdateMode) => void;
  controller: DashUpdateController;
};

const UpdateRow: React.FC<{ label: string; children: React.ReactNode }> = ({ label, children }) => (
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

const NoticeDialog: React.FC<{
  isOpen: boolean;
  message: string;
  title: string;
  closeLabel: string;
  onClose: () => void;
  icon?: React.ReactNode;
}> = ({ isOpen, message, title, closeLabel, onClose, icon }) => {
  const titleId = React.useId();

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      maxWidth="max-w-md"
      zIndex={60}
      ariaLabelledby={titleId}
    >
      <ModalHeader title={title} onClose={onClose} icon={icon} id={titleId} className="py-3" />
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

export const DashUpdateView: React.FC<Props> = ({
  settings,
  loadingSettings,
  savingChannel,
  savingMode,
  onModeChange,
  controller,
}) => {
  const { lang, t } = useI18n();
  const {
    version,
    versionCheck,
    loadingVersion,
    releaseNotes,
    releaseNotesURL,
    loadingNotes,
    checkError,
    checkRequested,
    lastCheckedAt,
    updateStatus,
    loadingUpdateStatus,
    notifyTargets,
    loadingNotifyTargets,
    notifyTargetsFailed,
    startingUpdate,
    updatedVersion,
    confirmDialogProps,
    checkUpdate,
    togglePrerelease,
    runUpdate,
    dismissUpdateSuccess,
    dismissCheckError,
  } = controller;
  const channel = settings?.dash_update_channel ?? 'release';
  const updateMode = settings?.dash_update_mode ?? 'manual';
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
  const isUpdateRunning = updateStatus?.status === 'running' || startingUpdate !== null;
  const showJob = hasChecked || Boolean(updateStatus && updateStatus.status !== 'idle');
  const versionStatus = statusFromCheck(versionCheck?.version_status);
  const jobBadge = updateJobBadge(updateStatus);
  const updateUnavailable = updateStatus?.available === false;
  const targetIsOlder = versionCheck?.version_status === 'ahead';
  const targetIsCurrent = versionCheck?.version_status === 'current';
  const actionDisabled =
    !settings ||
    !versionCheck ||
    loadingSettings ||
    savingChannel ||
    savingMode ||
    checkingStatus ||
    isUpdateRunning ||
    updateUnavailable ||
    targetIsOlder;
  const actionHint = updateUnavailable
    ? updateStatus?.unavailable_reason || t('admin_dash_update_job_unavailable')
    : targetIsOlder
      ? t('admin_dash_update_action_ahead')
      : '';
  const updateDisabled = actionDisabled || targetIsCurrent;
  const updateHint = targetIsCurrent ? t('admin_dash_update_action_current') : actionHint;
  const checkDisabled =
    !settings ||
    loadingSettings ||
    savingChannel ||
    checking ||
    isUpdateRunning ||
    updateUnavailable;
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

  return (
    <div className="space-y-4">
      <ConfirmDialog {...confirmDialogProps} />
      <NoticeDialog
        isOpen={Boolean(checkError)}
        message={checkError}
        title={t('admin_dash_update_check_failed_title')}
        closeLabel={t('common_close')}
        onClose={dismissCheckError}
      />
      <NoticeDialog
        isOpen={Boolean(updatedVersion)}
        message={t('admin_dash_update_success_message', { version: updatedVersion })}
        title={t('admin_dash_update_success_title')}
        closeLabel={t('common_close')}
        onClose={dismissUpdateSuccess}
        icon={<CheckCircle2 className="size-5 text-(--theme-fg-success)" aria-hidden="true" />}
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
              <Tooltip content={updateHint} className="inline-flex">
                <span className="inline-flex">
                  <Button
                    type="button"
                    icon={RefreshCw}
                    disabled={updateDisabled}
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
