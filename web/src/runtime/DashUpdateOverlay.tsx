import React from 'react';
import CheckCircle2 from 'lucide-react/dist/esm/icons/check-circle-2';
import CircleAlert from 'lucide-react/dist/esm/icons/circle-alert';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import WifiOff from 'lucide-react/dist/esm/icons/wifi-off';
import Button from '@components/ui/Button';
import { Modal } from '@components/ui/Modal';
import { useI18n } from '@i18n';

export type DashUpdateOverlayPhase = 'updating' | 'reconnecting' | 'success' | 'failed';

type Props = {
  phase: DashUpdateOverlayPhase;
  targetVersion: string;
  observedVersion: string;
  onReload: () => void;
  onDismiss: () => void;
};

export const DashUpdateOverlay: React.FC<Props> = ({
  phase,
  targetVersion,
  observedVersion,
  onReload,
  onDismiss,
}) => {
  const { t } = useI18n();
  const titleId = React.useId();
  const actionRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    if (phase === 'success' || phase === 'failed') {
      actionRef.current?.querySelector('button')?.focus();
    }
  }, [phase]);

  const success = phase === 'success';
  const failed = phase === 'failed';
  const reconnecting = phase === 'reconnecting';
  const title = success
    ? t('admin_dash_update_overlay_success_title')
    : failed
      ? t('admin_dash_update_overlay_failed_title')
      : reconnecting
        ? t('admin_dash_update_overlay_reconnecting_title')
        : t('admin_dash_update_overlay_updating_title');
  const message = success
    ? t('admin_dash_update_overlay_success_message', {
        version: observedVersion || targetVersion,
      })
    : failed
      ? t('admin_dash_update_overlay_failed_message')
      : reconnecting
        ? t('admin_dash_update_overlay_reconnecting_message')
        : t('admin_dash_update_overlay_updating_message');

  return (
    <Modal
      isOpen
      onClose={() => undefined}
      maxWidth="max-w-md"
      className="focus:outline-none"
      zIndex={100}
      ariaLabelledby={titleId}
    >
      <div className="px-6 py-7 text-center sm:p-8">
        <div
          className={`mx-auto flex size-20 items-center justify-center rounded-full border ${
            success
              ? 'border-(--theme-border-success-muted) bg-(--theme-bg-success-muted) text-(--theme-fg-success)'
              : failed
                ? 'border-(--theme-border-danger-muted) bg-(--theme-bg-danger-subtle) text-(--theme-fg-danger)'
                : reconnecting
                  ? 'border-(--theme-border-warning-muted) bg-(--theme-bg-warning-muted) text-(--theme-fg-warning)'
                  : 'border-(--theme-border-interactive-muted) bg-(--theme-bg-accent-muted) text-(--theme-fg-interactive)'
          }`}
          aria-hidden="true"
        >
          {success ? (
            <CheckCircle2 className="size-9" />
          ) : failed ? (
            <CircleAlert className="size-9" />
          ) : reconnecting ? (
            <WifiOff className="size-8" />
          ) : (
            <RefreshCw className="size-8 animate-spin motion-reduce:animate-none" />
          )}
        </div>

        <div className="mt-5" aria-live={failed ? 'assertive' : 'polite'} aria-atomic="true">
          <h2 id={titleId} className="text-xl font-semibold text-(--theme-fg-strong)">
            {title}
          </h2>
          <p className="mt-2 text-sm/6 text-(--theme-fg-muted)">{message}</p>
        </div>

        <div className="mt-5 rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-muted) px-4 py-3 text-left dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle)">
          <span className="block text-xs font-medium text-(--theme-fg-muted)">
            {t('admin_dash_update_overlay_target')}
          </span>
          <span className="mt-1 block font-mono text-sm font-semibold tabular-nums text-(--theme-fg-default)">
            {targetVersion}
          </span>
        </div>

        {success ? (
          <div ref={actionRef} className="mt-6">
            <Button className="w-full" onClick={onReload}>
              {t('admin_dash_update_overlay_reload')}
            </Button>
          </div>
        ) : failed ? (
          <div ref={actionRef} className="mt-6">
            <Button variant="secondary" className="w-full" onClick={onDismiss}>
              {t('admin_dash_update_overlay_failed_dismiss')}
            </Button>
          </div>
        ) : (
          <div
            className="mt-6 h-1.5 overflow-hidden rounded-full bg-(--theme-bg-muted)"
            role="progressbar"
            aria-label={title}
          >
            <div className="h-full w-1/2 animate-pulse rounded-full bg-(--theme-fg-interactive) motion-reduce:animate-none" />
          </div>
        )}
      </div>
    </Modal>
  );
};
