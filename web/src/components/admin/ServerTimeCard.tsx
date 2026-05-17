import React from 'react';
import { useI18n } from '@i18n';
import { formatTimeInTimeZone, formatUTCOffsetInTimeZone } from '@utils/time';

interface Props {
  className?: string;
  label?: string;
  timeZone?: string;
}

const ServerTimeCard: React.FC<Props> = ({ className = '', label, timeZone = 'UTC' }) => {
  const { t, lang } = useI18n();
  const [now, setNow] = React.useState<Date>(() => new Date());
  const [useLocal, setUseLocal] = React.useState(false);
  const browserTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const effectiveTimeZone = useLocal ? browserTimeZone : timeZone;

  React.useEffect(() => {
    const id = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(id);
  }, []);

  const badgeLabel = useLocal ? t('admin_server_time_local') : t('admin_server_time_gmt');

  const formatted = React.useMemo(
    () =>
      `${formatTimeInTimeZone(now, lang, effectiveTimeZone, true)} ${formatUTCOffsetInTimeZone(now, effectiveTimeZone)}`,
    [effectiveTimeZone, lang, now],
  );

  const toggleTz = React.useCallback(() => {
    setUseLocal((prev) => !prev);
  }, []);

  return (
    <button
      type="button"
      onClick={toggleTz}
      className={`bg-(--theme-surface-control-strong) dark:bg-(--theme-bg-muted) border border-(--theme-border-subtle) dark:border-(--theme-border-default) rounded-lg px-4 py-2 text-xs font-mono text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) flex flex-col items-center shadow-sm transition-colors hover:ring-(--theme-bg-accent-emphasis) hover:border-(--theme-bg-accent-emphasis) text-center ${className}`}
      aria-live="polite"
      title={
        useLocal
          ? t('admin_server_time_switch_to_tz', { tz: timeZone })
          : t('admin_server_time_switch_to_local')
      }
    >
      <span className="flex items-center gap-2">
        {label ?? t('admin_server_time')}
        <span className="text-[10px] px-2 py-0.5 rounded-full bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-strong) border border-(--theme-border-subtle) dark:border-(--theme-border-default) inline-flex min-w-12 justify-center">
          {badgeLabel}
        </span>
      </span>
      <span className="text-(--theme-fg-default) dark:text-(--theme-fg-strong) text-sm">
        {formatted}
      </span>
    </button>
  );
};

export default ServerTimeCard;
