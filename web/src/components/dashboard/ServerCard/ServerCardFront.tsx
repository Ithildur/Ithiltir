import React from 'react';
import AlertTriangle from 'lucide-react/dist/esm/icons/alert-triangle';
import BarChart3 from 'lucide-react/dist/esm/icons/bar-chart-3';
import ChevronRight from 'lucide-react/dist/esm/icons/chevron-right';
import Cpu from 'lucide-react/dist/esm/icons/cpu';
import MemoryStick from 'lucide-react/dist/esm/icons/memory-stick';
import Network from 'lucide-react/dist/esm/icons/network';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import TrendingUp from 'lucide-react/dist/esm/icons/trending-up';
import { useNavigate } from 'react-router-dom';
import { useI18n } from '@i18n';
import Button from '@components/ui/Button';
import { Tooltip } from '@components/ui/Tooltip';
import { formatBytes } from '@pages/dashboard/viewModel';
import type { ServerViewModel } from '@pages/dashboard/viewModel';
import MiniGauge from '@components/dashboard/MiniGauge';
import { NetworkRowFull } from '@components/dashboard/NetworkRow';
import { SystemLogo } from '@components/system/SystemLogo';

interface Props {
  view: ServerViewModel;
  canOpenHistory: boolean;
  canOpenTraffic: boolean;
}

const ServerCardFront: React.FC<Props> = ({ view, canOpenHistory, canOpenTraffic }) => {
  const { t } = useI18n();
  const isAlive = view.isAlive;
  const navigate = useNavigate();
  const tagRowRef = React.useRef<HTMLDivElement | null>(null);
  const tagMeasureRefs = React.useRef<Array<HTMLSpanElement | null>>([]);
  const moreMeasureRefs = React.useRef<Array<HTMLSpanElement | null>>([]);
  const [visibleTagCount, setVisibleTagCount] = React.useState(view.tags.length);

  const cardStyle = isAlive
    ? 'bg-(--theme-bg-default) dark:bg-(--theme-bg-default) border-(--theme-border-subtle) dark:border-(--theme-border-default) hover:border-(--theme-border-interactive-hover) dark:hover:border-(--theme-fg-accent)/40'
    : 'bg-(--theme-bg-default) dark:bg-(--theme-bg-default) border-(--theme-border-subtle) dark:border-(--theme-border-default) opacity-60 cursor-not-allowed';

  const cpuTooltip = view.cpu.hasDetails
    ? view.cpu.modelName || t('dashboard_unknown_cpu_model')
    : t('dashboard_cpu_info_unavailable');

  const toMB = (b: number) => (b / 1024 / 1024).toFixed(0);
  const memTooltip = `${t('mem_used')}: ${toMB(view.memory.used)} MB
${t('mem_buff')}: ${toMB(view.memory.buffers + view.memory.cached)} MB
${t('mem_avail')}: ${toMB(view.memory.available)} MB
${t('mem_total')}: ${toMB(view.memory.total)} MB`;
  const tagTooltip = view.tags.join('\n');
  const tagCount = view.tags.length;
  const visibleTags = view.tags.slice(0, visibleTagCount);
  const hiddenTagCount = view.tags.length - visibleTags.length;
  const tagSignature = view.tags.join('\u0000');
  const stopPropagation = React.useCallback((event: React.SyntheticEvent) => {
    event.stopPropagation();
  }, []);

  React.useLayoutEffect(() => {
    const row = tagRowRef.current;
    if (!row || tagCount === 0) {
      setVisibleTagCount(tagCount);
      return;
    }

    const calculate = () => {
      const width = row.clientWidth;
      if (width <= 0) return;

      const gap = 4;
      const tagWidths = view.tags.map(
        (_, index) => tagMeasureRefs.current[index]?.offsetWidth ?? 0,
      );
      const moreWidth = (hidden: number) => moreMeasureRefs.current[hidden]?.offsetWidth ?? 0;
      let used = 0;
      let nextVisible = 0;

      for (let index = 0; index < tagWidths.length; index += 1) {
        const nextUsed = used + (index > 0 ? gap : 0) + tagWidths[index];
        const hidden = tagWidths.length - index - 1;
        const total = hidden > 0 ? nextUsed + gap + moreWidth(hidden) : nextUsed;

        if (total > width) break;
        used = nextUsed;
        nextVisible = index + 1;
      }

      setVisibleTagCount(Math.max(1, nextVisible));
    };

    calculate();

    const resizeObserver = new ResizeObserver(calculate);
    resizeObserver.observe(row);

    return () => {
      resizeObserver.disconnect();
    };
  }, [tagCount, tagSignature, view.tags]);

  const openHistory = React.useCallback(
    (event: React.SyntheticEvent) => {
      event.stopPropagation();
      if (!canOpenHistory) return;
      navigate(`/statistics/${view.id}`);
    },
    [canOpenHistory, navigate, view.id],
  );

  const openTraffic = React.useCallback(
    (event: React.SyntheticEvent) => {
      event.stopPropagation();
      if (!canOpenTraffic) return;
      navigate(`/traffic/${view.id}`);
    },
    [canOpenTraffic, navigate, view.id],
  );

  const actionClass = (enabled: boolean) =>
    enabled
      ? 'group/stats bg-(--theme-bg-default) text-(--theme-fg-default) dark:text-(--theme-fg-default) hover:border-(--theme-fg-subtle) dark:hover:border-(--theme-fg-muted) hover:ring-1 hover:ring-(--theme-ring-muted) dark:hover:ring-(--theme-ring-muted)'
      : 'bg-(--theme-bg-muted) text-(--theme-fg-muted) dark:bg-(--theme-canvas-muted) dark:text-(--theme-fg-control-muted)';
  const labelClass = (enabled: boolean) =>
    enabled
      ? 'min-w-0 truncate text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) font-medium group-hover/stats:text-(--theme-fg-default) dark:group-hover/stats:text-(--theme-fg-default)'
      : 'min-w-0 truncate text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-control-muted) font-medium';
  const iconClass = (enabled: boolean) =>
    enabled
      ? 'p-1 rounded bg-(--theme-bg-interactive-muted) dark:bg-(--theme-canvas-muted)/70 text-(--theme-fg-interactive)'
      : 'p-1 rounded bg-(--theme-border-subtle) dark:bg-(--theme-canvas-muted)/70 text-(--theme-fg-muted) dark:text-(--theme-fg-control-muted)';
  const historyTitle = canOpenHistory ? t('dashboard_statistics') : t('stats_login_required');
  const trafficTitle = canOpenTraffic ? t('traffic_title') : t('stats_login_required');

  return (
    <div
      className={`absolute inset-0 backface-hidden rounded-xl border transition-all cursor-pointer overflow-hidden flex flex-col ${cardStyle}`}
    >
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-4 p-4 pb-1">
        <div className="flex min-w-0 items-center gap-3">
          <div
            className="p-2 flex shrink-0 items-center justify-center"
            title={`${view.system.platform || view.system.os || ''}`.trim()}
          >
            <SystemLogo system={view.system} size={24} />
          </div>
          <div className="min-w-0 flex-1 overflow-hidden">
            <h3
              className="truncate font-bold leading-tight text-(--theme-fg-default) dark:text-(--theme-fg-strong)"
              title={view.hostname}
            >
              {view.hostname}
            </h3>
            <div className="mt-1 flex min-w-0 items-center gap-2 text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
              <span
                className={`size-1.5 shrink-0 rounded-full ${isAlive ? 'bg-(--theme-fg-success-muted)' : 'bg-(--theme-fg-danger-muted)'}`}
              />
              <span className="shrink-0">{isAlive ? t('online') : t('offline')}</span>
              {isAlive && (
                <>
                  <span className="h-3 w-px shrink-0 bg-(--theme-border-default) dark:bg-(--theme-canvas-muted)/70" />
                  <span className="truncate font-mono text-[11px] text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
                    {view.uptime}
                  </span>
                </>
              )}
            </div>
          </div>
        </div>

        {isAlive && (
          <div className="flex shrink-0 justify-self-end gap-2">
            {view.raid.showAlert && (
              <div className="flex items-center gap-1 bg-(--theme-bg-danger-subtle) dark:bg-(--theme-bg-danger-muted) text-(--theme-fg-danger) dark:text-(--theme-fg-danger) px-2 py-1 rounded text-[10px] font-bold animate-pulse">
                <AlertTriangle size={12} />
                <span>{t('raid_short')}</span>
              </div>
            )}
            <span className="text-(--theme-border-default) hover:text-(--theme-fg-interactive) transition-colors">
              <RefreshCw size={18} />
            </span>
          </div>
        )}
      </div>

      <div className="px-4 pt-1 pb-2 flex-1 flex flex-col transition-opacity duration-300">
        <div className="flex flex-col flex-1 space-y-2.5">
          <div className="h-5 min-w-0 overflow-hidden" aria-hidden={tagCount === 0}>
            {tagCount > 0 ? (
              <Tooltip content={tagTooltip} className="min-w-0">
                <div
                  ref={tagRowRef}
                  className="relative flex h-5 w-full flex-nowrap items-center justify-start gap-1 overflow-hidden whitespace-nowrap"
                >
                  {visibleTags.map((tag, index) => (
                    <span
                      key={`${tag}-${index}`}
                      className={`max-w-64 truncate rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-muted) px-1.5 py-0.5 text-[10px]/3 font-medium text-(--theme-fg-muted) dark:border-(--theme-border-default) dark:bg-(--theme-canvas-muted) dark:text-(--theme-fg-neutral) ${
                        index === visibleTags.length - 1 ? 'min-w-0 shrink' : 'shrink-0'
                      }`}
                    >
                      {tag}
                    </span>
                  ))}
                  {hiddenTagCount > 0 && (
                    <span className="shrink-0 rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-muted) px-1.5 py-0.5 text-[10px]/3 font-medium text-(--theme-fg-subtle) dark:border-(--theme-border-default) dark:bg-(--theme-canvas-muted) dark:text-(--theme-fg-control-muted)">
                      +{hiddenTagCount}
                    </span>
                  )}
                  <span className="pointer-events-none invisible absolute left-0 top-0 flex gap-1 whitespace-nowrap">
                    {view.tags.map((tag, index) => (
                      <span
                        key={`measure-${tag}-${index}`}
                        ref={(node) => {
                          tagMeasureRefs.current[index] = node;
                        }}
                        className="max-w-64 shrink-0 truncate rounded-md border border-(--theme-border-subtle) px-1.5 py-0.5 text-[10px]/3 font-medium"
                      >
                        {tag}
                      </span>
                    ))}
                    {view.tags.map((_, index) => {
                      const hidden = index + 1;

                      return (
                        <span
                          key={`more-measure-${hidden}`}
                          ref={(node) => {
                            moreMeasureRefs.current[hidden] = node;
                          }}
                          className="shrink-0 rounded-md border border-(--theme-border-subtle) px-1.5 py-0.5 text-[10px]/3 font-medium"
                        >
                          +{hidden}
                        </span>
                      );
                    })}
                  </span>
                </div>
              </Tooltip>
            ) : null}
          </div>
          <div className="flex justify-around items-center py-0 gap-6">
            <Tooltip content={cpuTooltip} className="cursor-help">
              <MiniGauge
                value={view.cpu.usagePercent}
                label={t('cpu')}
                icon={Cpu}
                detail={
                  view.cpu.coresLogical ? (
                    <span className="text-xs text-(--theme-fg-default) dark:text-(--theme-fg-default) font-mono">
                      {view.cpu.coresLogical} Threads
                    </span>
                  ) : undefined
                }
              />
            </Tooltip>
            <div className="h-10 w-px bg-(--theme-border-default) dark:bg-(--theme-canvas-muted)/70" />
            <Tooltip content={memTooltip} className="cursor-help">
              <MiniGauge
                value={view.memory.usedPercent}
                label={t('memory')}
                icon={MemoryStick}
                detail={
                  <span className="text-xs text-(--theme-fg-default) dark:text-(--theme-fg-default) font-mono">
                    {(view.memory.used / 1024 / 1024 / 1024).toFixed(1)} G /{' '}
                    {(view.memory.total / 1024 / 1024 / 1024).toFixed(1)} G
                  </span>
                }
              />
            </Tooltip>
          </div>

          <div className="flex flex-col space-y-2">
            <NetworkRowFull
              label={t('curr_speed')}
              up={`${formatBytes(view.network.rateOut)}/s`}
              down={`${formatBytes(view.network.rateIn)}/s`}
            />
            <NetworkRowFull
              label={t('total_trans')}
              up={formatBytes(view.network.totalOut)}
              down={formatBytes(view.network.totalIn)}
              isTotal
            />
            <div className="flex justify-between items-center bg-(--theme-bg-default) dark:bg-(--theme-bg-default) px-3 py-1.5 rounded-lg border border-(--theme-border-muted) dark:border-(--theme-border-default)">
              <div className="flex items-center gap-2">
                <span className="p-1 rounded bg-(--theme-bg-interactive-muted) dark:bg-(--theme-canvas-muted)/70 text-(--theme-fg-interactive)">
                  <TrendingUp size={10} />
                </span>
                <span className="text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) font-medium">
                  {t('load')}
                </span>
              </div>
              <span className="text-xs font-mono text-(--theme-fg-default) dark:text-(--theme-fg-default)">
                {view.cpu.load1.toFixed(2)} / {view.cpu.load5.toFixed(2)} /{' '}
                {view.cpu.load15.toFixed(2)}
              </span>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <Button
                type="button"
                variant="plain"
                size="none"
                disabled={!canOpenHistory}
                onClick={openHistory}
                onMouseDown={stopPropagation}
                onKeyDown={stopPropagation}
                className={`min-w-0 justify-between gap-1 rounded-lg border border-(--theme-border-muted) dark:border-(--theme-border-default) px-2.5 py-1.5 text-xs font-medium tracking-normal ring-inset active:scale-100 ${actionClass(canOpenHistory)}`}
                aria-label={historyTitle}
                title={historyTitle}
              >
                <span className="inline-flex min-w-0 items-center gap-1.5">
                  <span className={iconClass(canOpenHistory)}>
                    <BarChart3 size={10} />
                  </span>
                  <span className={labelClass(canOpenHistory)}>{t('dashboard_statistics')}</span>
                </span>
                <ChevronRight
                  size={12}
                  className={
                    canOpenHistory
                      ? 'shrink-0 text-(--theme-fg-default) dark:text-(--theme-fg-default)'
                      : 'shrink-0 text-(--theme-fg-muted) dark:text-(--theme-fg-control-muted)'
                  }
                />
              </Button>
              <Button
                type="button"
                variant="plain"
                size="none"
                disabled={!canOpenTraffic}
                onClick={openTraffic}
                onMouseDown={stopPropagation}
                onKeyDown={stopPropagation}
                className={`min-w-0 justify-between gap-1 rounded-lg border border-(--theme-border-muted) dark:border-(--theme-border-default) px-2.5 py-1.5 text-xs font-medium tracking-normal ring-inset active:scale-100 ${actionClass(canOpenTraffic)}`}
                aria-label={trafficTitle}
                title={trafficTitle}
              >
                <span className="inline-flex min-w-0 items-center gap-1.5">
                  <span className={iconClass(canOpenTraffic)}>
                    <Network size={10} />
                  </span>
                  <span className={labelClass(canOpenTraffic)}>{t('traffic_short')}</span>
                </span>
                <ChevronRight
                  size={12}
                  className={
                    canOpenTraffic
                      ? 'shrink-0 text-(--theme-fg-default) dark:text-(--theme-fg-default)'
                      : 'shrink-0 text-(--theme-fg-muted) dark:text-(--theme-fg-control-muted)'
                  }
                />
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ServerCardFront;
