import React from 'react';
import { useI18n } from '@i18n';
import { fetchUptimeDay } from '@lib/uptimeApi';
import { isCanceledRequestError } from '@utils/errors';

export type UptimeHoursCache = Map<string, { expires: number; hours: Array<number | null> }>;

interface Props {
  serverID: string;
  date: string;
  percent: number | null;
  warningSLA: number;
  errorSLA: number;
  asOf: string;
  cache: UptimeHoursCache;
}

export const UptimeDayTooltip: React.FC<Props> = ({
  serverID,
  date,
  percent,
  warningSLA,
  errorSLA,
  asOf,
  cache,
}) => {
  const { t } = useI18n();
  const [hours, setHours] = React.useState<Array<number | null> | null>(() => {
    const cached = cache.get(date);
    return cached && cached.expires > Date.now() ? cached.hours : null;
  });
  const [failed, setFailed] = React.useState(false);

  React.useEffect(() => {
    const cached = cache.get(date);
    if (cached && cached.expires > Date.now()) {
      setHours(cached.hours);
      return;
    }
    const controller = new AbortController();
    // Crossing several bars should only request the day the pointer rests on.
    const timer = window.setTimeout(() => {
      void fetchUptimeDay(serverID, date, controller.signal)
        .then((day) => {
          if (controller.signal.aborted) return;
          cache.set(date, { expires: Date.now() + 60_000, hours: day.hours });
          if (cache.size > 45) cache.delete(cache.keys().next().value!);
          setHours(day.hours);
          setFailed(false);
        })
        .catch((error) => {
          if (!controller.signal.aborted && !isCanceledRequestError(error)) {
            setHours(null);
            setFailed(true);
          }
        });
    }, 150);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [asOf, cache, date, serverID]);

  const color = (rate: number | null | undefined) => {
    if (rate == null) return 'bg-(--theme-border-default)';
    if (rate < errorSLA) return 'bg-(--theme-fg-danger-muted)';
    if (rate < warningSLA) return 'bg-(--theme-fg-warning-muted)';
    return 'bg-(--theme-fg-success-muted)';
  };

  return (
    <div className="w-52 max-w-[calc(100vw-3rem)] whitespace-normal">
      <div className="flex items-center justify-between gap-3 font-mono text-[11px]/4 tabular-nums">
        <span className="text-(--theme-fg-muted)">{date}</span>
        <span>{percent == null ? t('no_data') : `${percent.toFixed(2)}%`}</span>
      </div>
      <div
        className="mt-1.5 flex gap-0.5"
        role="group"
        aria-label={t('dashboard_uptime_hours')}
        aria-busy={hours === null && !failed}
      >
        {Array.from({ length: 24 }, (_, hour) => {
          const rate = hours?.[hour];
          const label = `${String(hour).padStart(2, '0')}:00 · ${rate == null ? t('no_data') : `${rate.toFixed(2)}%`}`;
          return (
            <span
              key={hour}
              role="img"
              aria-label={label}
              className={`h-4 min-w-0 flex-1 rounded-xs ${color(rate)}`}
            />
          );
        })}
      </div>
      <div
        className="mt-1 flex justify-between font-mono text-[9px]/3 tabular-nums text-(--theme-fg-muted)"
        aria-hidden="true"
      >
        <span>00</span>
        <span>06</span>
        <span>12</span>
        <span>18</span>
        <span>23</span>
      </div>
      {failed && (
        <p className="mt-1 text-[11px] text-(--theme-fg-muted)" role="status">
          {t('dashboard_uptime_fetch_failed')}
        </p>
      )}
    </div>
  );
};
