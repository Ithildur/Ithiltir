import React from 'react';
import { useI18n } from '@i18n';

interface Props {
  date: string;
  percent: number | null;
  warningSLA: number;
  errorSLA: number;
  hours: Array<number | null> | null;
}

export const UptimeDayTooltip: React.FC<Props> = ({
  date,
  percent,
  warningSLA,
  errorSLA,
  hours,
}) => {
  const { t } = useI18n();

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
      {hours === null ? (
        <p className="mt-1 text-[11px] text-(--theme-fg-muted)" role="status">
          {t('dashboard_uptime_fetch_failed')}
        </p>
      ) : (
        <>
          <div
            className="mt-1.5 flex gap-0.5"
            role="group"
            aria-label={t('dashboard_uptime_hours')}
          >
            {Array.from({ length: 24 }, (_, hour) => {
              const rate = hours[hour];
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
        </>
      )}
    </div>
  );
};
