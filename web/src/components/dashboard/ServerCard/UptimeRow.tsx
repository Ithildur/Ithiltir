import React from 'react';
import Activity from 'lucide-react/dist/esm/icons/activity';
import { Tooltip } from '@components/ui/Tooltip';
import type { TooltipHandle } from '@components/ui/Tooltip';
import { useI18n } from '@i18n';
import type { UptimeHistory } from '@pages/dashboard/viewModel';
import { fetchUptimeDay } from '@lib/uptimeApi';
import { isCanceledRequestError } from '@utils/errors';
import { observeUptimeCanvas, uptimeDays as dayCount } from './uptimeCanvas';
import { UptimeDayTooltip } from './UptimeDayTooltip';

interface Hours {
  date: string;
  hours: Array<number | null> | null;
}

const useUptimeHours = (serverID: string, date: string | undefined, asOf: string) => {
  const [open, setOpen] = React.useState(false);
  const [detail, setDetail] = React.useState<Hours | null>(null);
  const hoursCache = React.useRef(
    new Map<string, { expires: number; hours: Array<number | null> }>(),
  );

  React.useEffect(() => {
    if (!open || !date) return;
    const now = Date.now();
    const cache = hoursCache.current;
    const cached = cache.get(date);
    if (cached && cached.expires > now) {
      setDetail({ date, hours: cached.hours });
      return;
    }

    setDetail(null);
    const controller = new AbortController();
    // Wait for the pointer to rest on a day before loading its tooltip.
    const timer = window.setTimeout(() => {
      void fetchUptimeDay(serverID, date, controller.signal)
        .then((day) => {
          if (controller.signal.aborted) return;
          cache.set(date, { expires: Date.now() + 60_000, hours: day.hours });
          if (cache.size > dayCount) cache.delete(cache.keys().next().value!);
          setDetail({ date, hours: day.hours });
        })
        .catch((error) => {
          if (!controller.signal.aborted && !isCanceledRequestError(error)) {
            setDetail({ date, hours: null });
          }
        });
    }, 200);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [date, asOf, open, serverID]);

  const changeOpen = React.useCallback((next: boolean) => {
    setOpen(next);
    if (!next) setDetail(null);
  }, []);

  return [detail, changeOpen] as const;
};

interface Props {
  serverID: string;
  history: UptimeHistory;
}

const UptimeRow: React.FC<Props> = ({ serverID, history }) => {
  const { t } = useI18n();
  const [activeIndex, setActiveIndex] = React.useState(dayCount - 1);
  const canvasRef = React.useRef<HTMLCanvasElement>(null);
  const tooltipRef = React.useRef<TooltipHandle>(null);
  const recent = history?.days.slice(-dayCount) ?? [];
  const days = Array.from(
    { length: dayCount },
    (_, index) => recent[index - (dayCount - recent.length)],
  );
  const date = days[activeIndex]?.date;
  const [detail, changeOpen] = useUptimeHours(serverID, date, history.asOf);
  const description = (day: (typeof days)[number]) =>
    day
      ? `${day.date} · ${day.percent === null ? t('no_data') : `${day.percent.toFixed(2)}%`}`
      : t('no_data');

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
      onOpenChange={changeOpen}
      content={() => {
        const day = days[activeIndex];
        if (!day || detail?.date !== day.date) return null;
        return (
          <UptimeDayTooltip
            date={day.date}
            percent={day.percent}
            warningSLA={history.warningSLA}
            errorSLA={history.errorSLA}
            hours={detail.hours}
          />
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
