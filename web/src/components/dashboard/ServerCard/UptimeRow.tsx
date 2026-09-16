import React from 'react';
import Activity from 'lucide-react/dist/esm/icons/activity';
import { Tooltip } from '@components/ui/Tooltip';
import type { TooltipHandle } from '@components/ui/Tooltip';
import { useI18n } from '@i18n';
import type { UptimeHistory } from '@pages/dashboard/viewModel';
import { observeUptimeCanvas, uptimeDays as dayCount } from './uptimeCanvas';

interface Props {
  history?: UptimeHistory;
}

const UptimeRow: React.FC<Props> = ({ history }) => {
  const { t } = useI18n();
  const [activeIndex, setActiveIndex] = React.useState(dayCount - 1);
  const canvasRef = React.useRef<HTMLCanvasElement>(null);
  const tooltipRef = React.useRef<TooltipHandle>(null);
  const recent = history?.days.slice(-dayCount) ?? [];
  const days = Array.from(
    { length: dayCount },
    (_, index) => recent[index - (dayCount - recent.length)],
  );
  const description = (day: (typeof days)[number]) =>
    day
      ? `${day.date} · ${day.percent === null ? t('no_data') : `${day.percent.toFixed(2)}%`}`
      : t('no_data');
  const color = (percent: number | null | undefined) => {
    if (percent == null || !history) return 'bg-(--theme-border-default)';
    if (percent < history.errorSLA) return 'bg-(--theme-fg-danger-muted)';
    if (percent < history.warningSLA) return 'bg-(--theme-fg-warning-muted)';
    return 'bg-(--theme-fg-success-muted)';
  };

  React.useLayoutEffect(() => {
    if (canvasRef.current) return observeUptimeCanvas(canvasRef.current, history);
  }, [history]);

  const selectDay = (event: React.PointerEvent<HTMLCanvasElement>) => {
    const bounds = event.currentTarget.getBoundingClientRect();
    if (bounds.width <= 0) return;
    setActiveIndex(
      Math.max(
        0,
        Math.min(
          dayCount - 1,
          Math.floor(((event.clientX - bounds.left) / (bounds.width + 1)) * dayCount),
        ),
      ),
    );
  };

  const moveFocus = (event: React.KeyboardEvent<HTMLCanvasElement>) => {
    if (recent.length === 0) return;
    const next =
      event.key === 'ArrowLeft'
        ? Math.max(dayCount - recent.length, activeIndex - 1)
        : event.key === 'ArrowRight'
          ? Math.min(dayCount - 1, activeIndex + 1)
          : event.key === 'Home'
            ? dayCount - recent.length
            : event.key === 'End'
              ? dayCount - 1
              : null;
    if (next === null) return;
    event.preventDefault();
    event.stopPropagation();
    setActiveIndex(next);
    tooltipRef.current?.show();
  };

  return (
    <Tooltip
      ref={tooltipRef}
      variant="surface"
      content={() => {
        const day = days[activeIndex];
        return (
          <div className="w-52 max-w-[calc(100vw-3rem)] whitespace-normal">
            <div className="flex items-center justify-between gap-3 font-mono text-[11px]/4 tabular-nums">
              <span className="text-(--theme-fg-muted)">
                {day?.date ?? t('dashboard_uptime_range', { days: dayCount })}
              </span>
              <span>{day?.percent == null ? t('no_data') : `${day.percent.toFixed(2)}%`}</span>
            </div>
            <div
              className="mt-1.5 flex gap-0.5"
              role="group"
              aria-label={t('dashboard_uptime_hours')}
            >
              {Array.from({ length: 24 }, (_, hour) => {
                const percent = day?.hours?.[hour];
                const label = `${String(hour).padStart(2, '0')}:00 · ${percent == null ? t('no_data') : `${percent.toFixed(2)}%`}`;
                return (
                  <span
                    key={hour}
                    role="img"
                    aria-label={label}
                    className={`h-4 min-w-0 flex-1 rounded-xs ${color(percent)}`}
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
          </div>
        );
      }}
    >
      <div className="flex items-center gap-2 rounded-lg border border-(--theme-border-muted) bg-(--theme-bg-default) px-3 py-1.5 dark:border-(--theme-border-default)">
        <span
          className="shrink-0 rounded bg-(--theme-bg-interactive-muted) p-1 text-(--theme-fg-interactive) dark:bg-(--theme-canvas-muted)/70"
          aria-hidden="true"
        >
          <Activity size={10} />
        </span>
        <canvas
          ref={canvasRef}
          className="h-4.5 min-w-0 flex-1 cursor-help outline-offset-2 focus-visible:outline-2 focus-visible:outline-(--theme-focus-ring)"
          role="img"
          tabIndex={recent.length > 0 ? 0 : -1}
          aria-label={`${t('dashboard_uptime_range', { days: dayCount })} · ${description(days[activeIndex])}`}
          onPointerMove={selectDay}
          onPointerDown={selectDay}
          onKeyDown={moveFocus}
          onClick={(event) => event.stopPropagation()}
        />
      </div>
    </Tooltip>
  );
};

export default React.memo(UptimeRow);
