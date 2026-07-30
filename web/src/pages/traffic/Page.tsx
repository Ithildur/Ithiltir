import AlertTriangle from 'lucide-react/dist/esm/icons/alert-triangle';
import ArrowLeft from 'lucide-react/dist/esm/icons/arrow-left';
import Gauge from 'lucide-react/dist/esm/icons/gauge';
import LoaderCircle from 'lucide-react/dist/esm/icons/loader-circle';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import Wrench from 'lucide-react/dist/esm/icons/wrench';
import type { LucideIcon } from 'lucide-react';
import { Link } from 'react-router';
import Button from '@components/ui/Button';
import ConfirmDialog from '@components/ui/ConfirmDialog';
import Select from '@components/ui/Select';
import ThemeToggle from '@components/ui/ThemeToggle';
import {
  formatBandwidth,
  formatCoverage,
  formatCycleRange,
  formatOptionalBandwidth,
  formatTrafficBytes,
} from './viewModel';
import { TrafficTrendChart } from './TrafficTrendChart';
import { TrafficRebuildOverlay } from './TrafficRebuildOverlay';
import { type CurrentStatTone, type TrafficHeroStatTone } from './pageModel';
import { useTrafficPage } from './useTrafficPage';

const statTone = {
  accent: 'text-(--theme-fg-accent)',
  warning: 'text-(--theme-fg-warning-strong)',
  slate: 'text-(--theme-fg-muted)',
  red: 'text-(--theme-bg-danger-emphasis)',
} as const;

type StatTone = TrafficHeroStatTone;

const HeroStat = ({
  label,
  value,
  detail,
  icon: Icon,
  tone = 'slate',
  labelEmphasis = false,
}: {
  label: string;
  value: string;
  detail?: string;
  icon: LucideIcon;
  tone?: StatTone;
  labelEmphasis?: boolean;
}) => (
  <div className="min-w-0 py-3 md:border-l md:border-(--theme-border-subtle) md:pl-8 first:md:border-l-0 first:md:pl-0 dark:md:border-(--theme-border-default)">
    <div className="flex min-w-0 items-center gap-2">
      <Icon size={15} className={statTone[tone]} />
      <div
        className={`truncate font-semibold uppercase tracking-[0.14em] text-(--theme-fg-muted) ${
          labelEmphasis ? 'text-[13px]' : 'text-[11px]'
        }`}
      >
        {label}
      </div>
    </div>
    <div className="mt-2 truncate font-mono text-2xl font-semibold tracking-tight text-(--theme-fg-default)">
      {value}
    </div>
    {detail && <div className="mt-1 truncate text-xs text-(--theme-fg-muted)">{detail}</div>}
  </div>
);

const DirectionModeChip = ({ label }: { label: string }) => (
  <span className="inline-flex items-center gap-1.5 rounded-md border border-(--theme-border-warning-muted) bg-(--theme-bg-warning-muted) px-2 py-1 text-xs font-semibold text-(--theme-fg-warning-strong) dark:border-(--theme-border-warning-soft) dark:bg-(--theme-bg-warning-soft) dark:text-(--theme-fg-warning-strong)">
    <Gauge size={13} aria-hidden="true" />
    <span>{label}</span>
  </span>
);

const currentStatValueClass = (tone: CurrentStatTone = 'default') => {
  switch (tone) {
    case 'accent':
      return 'font-mono font-semibold text-(--theme-fg-accent)';
    case 'warning':
      return 'font-mono font-semibold text-(--theme-fg-warning-strong)';
    case 'muted':
      return 'font-mono font-semibold text-(--theme-fg-muted)';
    default:
      return 'font-mono font-semibold text-(--theme-fg-default)';
  }
};

const currentStatDotClass = (tone: CurrentStatTone = 'default') => {
  switch (tone) {
    case 'accent':
      return 'bg-(--theme-fg-accent)';
    case 'warning':
      return 'bg-(--theme-fg-warning-strong)';
    case 'muted':
      return 'bg-(--theme-fg-subtle)';
    default:
      return 'bg-(--theme-fg-muted)';
  }
};

const HealthChip = ({ label, value }: { label: string; value: string }) => (
  <span className="inline-flex items-center gap-1.5 text-xs font-medium text-(--theme-fg-muted)">
    <span>{label}</span>
    <span className="font-mono text-(--theme-fg-default)">{value}</span>
  </span>
);

const Page = () => {
  const {
    t,
    token,
    confirmDialogProps,
    isValidServerId,
    locale,
    ifaces,
    iface,
    summary,
    monthly,
    chartMode,
    setChartMode,
    setIface,
    loading,
    errorKey,
    trafficRebuildBusy,
    trafficRebuildStartBusy,
    nodeTrafficRebuildActive,
    serverLabel,
    directionLabel,
    showP95,
    showCoverage,
    statItems,
    currentStatGroups,
    chartPoints,
    chartCoverageStats,
    chartCoverageWarning,
    dailyAvailable,
    loadTraffic,
    rebuildNodeTraffic,
  } = useTrafficPage();

  if (!isValidServerId) {
    return (
      <div className="min-h-screen bg-(--theme-page-bg) text-(--theme-fg-default) dark:bg-(--theme-bg-default)">
        <main
          id="main-content"
          tabIndex={-1}
          className="mx-auto max-w-410 px-4 py-12 sm:px-6 lg:px-8"
        >
          <div className="rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-default) p-6 text-sm text-(--theme-fg-muted) dark:border-(--theme-border-default)">
            {t('stats_no_server')}
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-(--theme-page-bg) text-(--theme-fg-default) dark:bg-(--theme-bg-default)">
      <ConfirmDialog {...confirmDialogProps} />

      <header className="sticky top-0 z-40 border-b border-(--theme-border-subtle) bg-(--theme-surface-control) backdrop-blur-md dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)/90">
        <div className="mx-auto flex h-16 max-w-410 items-center justify-between px-4 sm:px-6 lg:px-8">
          <div className="flex min-w-0 items-center gap-3">
            <Link
              to="/"
              className="inline-flex items-center gap-2 text-xs font-semibold text-(--theme-fg-muted) transition-colors hover:text-(--theme-fg-default)"
            >
              <ArrowLeft size={14} />
              <span>{t('common_back_to_dashboard')}</span>
            </Link>
          </div>
          <div className="hidden items-center gap-3 md:flex">
            <ThemeToggle size="sm" variant="soft" />
          </div>
        </div>
      </header>

      <main
        id="main-content"
        tabIndex={-1}
        className="mx-auto max-w-410 space-y-5 px-4 py-8 sm:px-6 lg:px-8"
      >
        <section className="border-b border-(--theme-border-subtle) pb-6 dark:border-(--theme-border-default)">
          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                <h1 className="truncate text-3xl font-semibold tracking-tight text-(--theme-fg-default)">
                  {t('traffic_title')}
                </h1>
                <span className="h-6 w-px bg-(--theme-border-subtle) dark:bg-(--theme-border-default)" />
                <span className="truncate text-xl font-medium text-(--theme-fg-muted)">
                  {serverLabel}
                </span>
              </div>
              <div className="mt-4 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-(--theme-fg-muted)">
                <span className="font-semibold text-(--theme-fg-default)">
                  {t('traffic_current_cycle')}
                </span>
                {summary && (
                  <span>
                    {formatCycleRange(
                      summary.cycle.start,
                      summary.cycle.end,
                      locale,
                      summary.cycle.timezone,
                    )}
                  </span>
                )}
                {summary && <DirectionModeChip label={directionLabel} />}
              </div>
            </div>

            <div className="flex flex-wrap items-end gap-2 lg:justify-end">
              <label className="grid min-w-48 gap-1">
                <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-(--theme-fg-muted)">
                  {t('traffic_iface')}
                </span>
                <Select
                  value={iface}
                  disabled={ifaces.length === 0}
                  onChange={(event) => setIface(event.target.value)}
                >
                  {ifaces.map((name) => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </Select>
              </label>
              <Button
                type="button"
                variant="secondary"
                icon={RefreshCw}
                onClick={() => void loadTraffic()}
                disabled={loading || nodeTrafficRebuildActive}
              >
                {t('stats_refresh')}
              </Button>
            </div>
          </div>
        </section>

        {errorKey && <div className="text-sm text-(--theme-fg-danger)">{t(errorKey)}</div>}
        {loading && !summary && (
          <div className="text-sm text-(--theme-fg-muted)">{t('stats_loading')}</div>
        )}
        {!loading && !errorKey && !summary && !nodeTrafficRebuildActive && (
          <div className="text-sm text-(--theme-fg-muted)">{t('traffic_no_data')}</div>
        )}
        {!summary && nodeTrafficRebuildActive && (
          <div className="relative min-h-112 overflow-hidden border-y border-(--theme-border-subtle) dark:border-(--theme-border-default)">
            <TrafficRebuildOverlay />
          </div>
        )}

        {summary && (
          <div className="relative" aria-busy={nodeTrafficRebuildActive}>
            {nodeTrafficRebuildActive && <TrafficRebuildOverlay />}
            <section className="divide-y divide-(--theme-border-subtle) border-b border-(--theme-border-subtle) dark:divide-(--theme-border-default) dark:border-(--theme-border-default)">
              <div className="grid gap-x-8 gap-y-3 py-4 sm:grid-cols-2 lg:grid-cols-4">
                {statItems.map((item, index) => (
                  <div
                    key={item.key}
                    className={
                      index === 0
                        ? ''
                        : 'lg:border-l lg:border-(--theme-border-subtle) lg:pl-8 lg:dark:border-(--theme-border-default)'
                    }
                  >
                    <HeroStat
                      label={item.label}
                      value={item.value}
                      icon={item.icon}
                      tone={item.tone}
                      labelEmphasis={item.labelEmphasis}
                    />
                  </div>
                ))}
              </div>

              <div className="grid gap-8 py-7 xl:grid-cols-[minmax(0,1fr)_360px]">
                <div className="min-w-0">
                  <TrafficTrendChart
                    points={chartPoints}
                    mode={chartMode}
                    dailyAvailable={dailyAvailable}
                    onModeChange={setChartMode}
                  />

                  {chartCoverageStats && (
                    <div className="mt-4 flex flex-wrap gap-x-6 gap-y-2">
                      <HealthChip
                        label={t('traffic_samples')}
                        value={`${chartCoverageStats.sampleCount}/${chartCoverageStats.expectedSampleCount}`}
                      />
                      <HealthChip
                        label={t('traffic_coverage')}
                        value={formatCoverage(chartCoverageStats.coverageRatio)}
                      />
                      {chartCoverageWarning && (
                        <span className="inline-flex flex-wrap items-center gap-2">
                          <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-(--theme-fg-warning-strong)">
                            <AlertTriangle className="size-3.5" aria-hidden="true" />
                            {t('traffic_coverage_low')}
                          </span>
                          {token && summary?.usage_mode === 'billing' && (
                            <Button
                              type="button"
                              variant="plain"
                              size="none"
                              className="rounded-md border border-(--theme-border-subtle) bg-transparent p-1.5 text-xs text-(--theme-fg-muted) transition-colors hover:border-(--theme-border-hover) hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-default) dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle) dark:text-(--theme-fg-muted) dark:hover:border-(--theme-fg-muted) dark:hover:bg-(--theme-canvas-muted) dark:hover:text-(--theme-fg-control-hover)"
                              disabled={trafficRebuildBusy}
                              onClick={() => void rebuildNodeTraffic()}
                              aria-label={t('admin_node_traffic_rebuild_button', {
                                name: serverLabel,
                              })}
                              title={t('admin_node_traffic_rebuild')}
                            >
                              {trafficRebuildStartBusy ? (
                                <LoaderCircle className="size-4 animate-spin" aria-hidden="true" />
                              ) : (
                                <Wrench className="size-4" strokeWidth={2.25} aria-hidden="true" />
                              )}
                            </Button>
                          )}
                        </span>
                      )}
                    </div>
                  )}
                </div>

                <aside className="border-t border-(--theme-border-subtle) pt-6 xl:border-l xl:border-t-0 xl:pl-8 xl:pt-0 dark:border-(--theme-border-default)">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="text-sm font-semibold text-(--theme-fg-default)">
                        {t('traffic_current_cycle_stats')}
                      </div>
                      <div className="mt-1 text-xs text-(--theme-fg-muted)">{directionLabel}</div>
                    </div>
                  </div>
                  <div className="mt-5 grid gap-5 text-sm">
                    {currentStatGroups.map((group) => (
                      <section key={group.key} className="min-w-0">
                        <div
                          className={`mb-3 flex items-center gap-2 text-xs font-semibold ${currentStatValueClass(group.tone)}`}
                        >
                          <span
                            className={`size-2 rounded-full ${currentStatDotClass(group.tone)}`}
                          />
                          <span>{group.label}</span>
                        </div>
                        <dl className="grid gap-2.5">
                          {group.items.map((item) => (
                            <div
                              key={item.key}
                              className="flex min-w-0 items-center justify-between gap-4"
                            >
                              <dt className="min-w-0 text-(--theme-fg-muted)">{item.label}</dt>
                              <dd
                                className={`min-w-0 text-right ${currentStatValueClass(group.tone)}`}
                              >
                                {item.value}
                              </dd>
                            </div>
                          ))}
                        </dl>
                      </section>
                    ))}
                  </div>
                </aside>
              </div>
            </section>

            <section>
              <div className="py-4">
                <div className="text-sm font-semibold text-(--theme-fg-default)">
                  {t('traffic_monthly_history')}
                </div>
              </div>
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-(--theme-border-subtle) text-sm dark:divide-(--theme-border-default)">
                  <thead className="bg-(--theme-bg-muted)">
                    <tr className="text-left text-xs font-semibold uppercase tracking-wide text-(--theme-fg-muted)">
                      <th className="px-4 py-3">{t('traffic_cycle')}</th>
                      <th className="px-4 py-3">{t('traffic_in_total')}</th>
                      <th className="px-4 py-3">{t('traffic_out_total')}</th>
                      <th className="px-4 py-3">{t('traffic_in_peak')}</th>
                      <th className="px-4 py-3">{t('traffic_out_peak')}</th>
                      {showP95 && <th className="px-4 py-3">{t('traffic_in_p95')}</th>}
                      {showP95 && <th className="px-4 py-3">{t('traffic_out_p95')}</th>}
                      {showCoverage && <th className="px-4 py-3">{t('traffic_coverage')}</th>}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-(--theme-border-subtle) dark:divide-(--theme-border-default)">
                    {monthly.items.map((item) => (
                      <tr
                        key={`${item.cycle.start}-${item.iface}`}
                        className="bg-(--theme-bg-default) transition-colors hover:bg-(--theme-surface-row-hover)"
                      >
                        <td className="whitespace-nowrap px-4 py-3 font-mono text-xs">
                          {formatCycleRange(
                            item.cycle.start,
                            item.cycle.end,
                            locale,
                            item.cycle.timezone,
                          )}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 font-mono">
                          {formatTrafficBytes(item.stats.in_bytes)}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 font-mono">
                          {formatTrafficBytes(item.stats.out_bytes)}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 font-mono">
                          {formatBandwidth(item.stats.in_peak_bytes_per_sec)}
                        </td>
                        <td className="whitespace-nowrap px-4 py-3 font-mono">
                          {formatBandwidth(item.stats.out_peak_bytes_per_sec)}
                        </td>
                        {showP95 && (
                          <td className="whitespace-nowrap px-4 py-3 font-mono">
                            {formatOptionalBandwidth(item.stats.in_p95_bytes_per_sec)}
                          </td>
                        )}
                        {showP95 && (
                          <td className="whitespace-nowrap px-4 py-3 font-mono">
                            {formatOptionalBandwidth(item.stats.out_p95_bytes_per_sec)}
                          </td>
                        )}
                        {showCoverage && (
                          <td className="whitespace-nowrap px-4 py-3 font-mono">
                            {formatCoverage(item.stats.coverage_ratio)}
                          </td>
                        )}
                      </tr>
                    ))}
                    {monthly.items.length === 0 && (
                      <tr>
                        <td
                          colSpan={(showP95 ? 7 : 5) + (showCoverage ? 1 : 0)}
                          className="px-4 py-6 text-sm text-(--theme-fg-muted)"
                        >
                          {t('traffic_no_data')}
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </section>
          </div>
        )}
      </main>
    </div>
  );
};

export default Page;
