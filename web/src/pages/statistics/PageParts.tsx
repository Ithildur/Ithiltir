import React from 'react';
import ArrowLeft from 'lucide-react/dist/esm/icons/arrow-left';
import Maximize2 from 'lucide-react/dist/esm/icons/maximize-2';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import SlidersHorizontal from 'lucide-react/dist/esm/icons/sliders-horizontal';
import X from 'lucide-react/dist/esm/icons/x';
import { Link } from 'react-router-dom';
import type { MetricHistoryAggregation, MetricHistoryRange } from '@app-types/metricsHistory';
import MetricHistoryChart from '@components/dashboard/MetricHistoryChart';
import { useI18n } from '@i18n';
import type { TranslationKey } from '@i18n';
import Button from '@components/ui/Button';
import { Modal, ModalBody, ModalHeader } from '@components/ui/Modal';
import Select from '@components/ui/Select';
import ThemeToggle from '@components/ui/ThemeToggle';
import { formatScaledValue, resolveUnitScale } from '@utils/metricFormat';
import {
  AGGREGATION_OPTIONS,
  RANGE_OPTIONS,
  type MetricConfig,
  type MetricSection,
} from './config';
import type { useStatisticsDetail } from './hooks/useStatisticsDetail';
import type { useStatisticsOverview } from './hooks/useStatisticsOverview';
import { formatDateTime, metricSeriesKey } from './viewModel';

type OverviewState = ReturnType<typeof useStatisticsOverview>;
type DetailState = ReturnType<typeof useStatisticsDetail>;

const StatCell = ({
  label,
  value,
  align = 'left',
}: {
  label: string;
  value: string;
  align?: 'left' | 'center';
}) => {
  const alignClass = align === 'center' ? 'items-center text-center' : 'items-start text-left';

  return (
    <div className={`flex flex-col gap-1 text-xs ${alignClass}`}>
      <span className="text-[11px] font-semibold text-(--theme-fg-muted) dark:text-(--theme-fg-control-hover)">
        {label}
      </span>
      <span className="font-mono text-(--theme-fg-default) dark:text-(--theme-fg-default)">
        {value}
      </span>
    </div>
  );
};

const LanguageToggleButton = ({
  lang,
  onToggle,
  className,
}: {
  lang: string;
  onToggle: () => void;
  className: string;
}) => (
  <button type="button" onClick={onToggle} className={className}>
    {lang.toUpperCase()}
  </button>
);

const ExpandButton = ({
  label,
  onClick,
  className,
}: {
  label: string;
  onClick: () => void;
  className: string;
}) => (
  <button type="button" onClick={onClick} className={className} aria-label={label} title={label}>
    <Maximize2 size={16} />
  </button>
);

const MetricRow = ({
  seriesKey,
  onVisible,
  className = '',
  children,
}: {
  seriesKey: string;
  onVisible?: (seriesKey: string) => void;
  className?: string;
  children: React.ReactNode;
}) => {
  const rowRef = React.useRef<HTMLDivElement | null>(null);

  React.useEffect(() => {
    if (!onVisible) return;
    const node = rowRef.current;
    if (!node) return;
    if (typeof window === 'undefined' || !('IntersectionObserver' in window)) {
      onVisible(seriesKey);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting) {
          onVisible(seriesKey);
          observer.unobserve(node);
        }
      },
      { rootMargin: '120px 0px' },
    );

    observer.observe(node);
    return () => observer.disconnect();
  }, [seriesKey, onVisible]);

  return (
    <div ref={rowRef} className={className}>
      {children}
    </div>
  );
};

export const StatisticsHeader = ({ serverLabel }: { serverLabel: string }) => {
  const { lang, setLang, t } = useI18n();
  const [isPreferencesOpen, setIsPreferencesOpen] = React.useState(false);
  const preferencesRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    if (!isPreferencesOpen) return;
    const closeOnOutside = (event: MouseEvent) => {
      if (preferencesRef.current && !preferencesRef.current.contains(event.target as Node)) {
        setIsPreferencesOpen(false);
      }
    };
    document.addEventListener('mousedown', closeOnOutside);
    return () => document.removeEventListener('mousedown', closeOnOutside);
  }, [isPreferencesOpen]);

  return (
    <header className="sticky top-0 z-40 backdrop-blur-md bg-(--theme-surface-control) dark:bg-(--theme-bg-inset)/90 border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
      <div className="max-w-410 mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        <div className="flex items-center gap-3 min-w-0">
          <Link
            to="/"
            className="inline-flex items-center gap-2 text-xs font-semibold text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default) transition-colors"
          >
            <ArrowLeft size={14} />
            <span>{t('common_back_to_dashboard')}</span>
          </Link>
          <div className="w-px h-5 bg-(--theme-border-subtle) dark:bg-(--theme-border-default)" />
          <div className="min-w-0">
            <div className="text-sm font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong) truncate">
              {t('dashboard_statistics')}
            </div>
            <div className="text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) truncate">
              {serverLabel}
            </div>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <div className="relative group md:hidden" ref={preferencesRef}>
            <Button
              type="button"
              onClick={() => setIsPreferencesOpen((prev) => !prev)}
              variant="icon"
              aria-label={t('theme')}
              icon={SlidersHorizontal}
            />
            {isPreferencesOpen && (
              <div
                className="menu-pop menu-pop-top-right menu-pop-anim"
                role="dialog"
                aria-label={t('settings')}
              >
                <div className="menu-head">
                  <span className="text-sm font-bold text-(--theme-fg-default)">
                    {t('settings')}
                  </span>
                  <button
                    type="button"
                    onClick={() => setIsPreferencesOpen(false)}
                    className="icon-ghost"
                    aria-label={t('common_close')}
                    title={t('common_close')}
                  >
                    <X size={16} strokeWidth={2.5} />
                  </button>
                </div>
                <div className="p-2">
                  <button
                    type="button"
                    onClick={() => setLang((current) => (current === 'zh' ? 'en' : 'zh'))}
                    className="menu-item menu-item-hover"
                  >
                    {t('admin_change_lang')}: {lang === 'zh' ? t('language_zh') : t('language_en')}
                  </button>
                  <ThemeToggle
                    size="md"
                    variant="plain"
                    showLabel
                    labelMode="action"
                    actionLabel={t('admin_change_theme')}
                    className="menu-item menu-item-hover justify-start"
                  />
                </div>
              </div>
            )}
          </div>

          <div className="hidden md:flex items-center gap-3">
            <LanguageToggleButton
              lang={lang}
              onToggle={() => setLang((current) => (current === 'zh' ? 'en' : 'zh'))}
              className="p-2 rounded-full hover:bg-(--theme-bg-muted) dark:hover:bg-(--theme-bg-default)/30 text-(--theme-fg-default) dark:text-(--theme-fg-control-hover) transition-colors font-mono text-xs font-bold border border-transparent hover:border-(--theme-border-subtle) dark:hover:border-(--theme-border-default)/60"
            />
            <ThemeToggle size="sm" variant="soft" />
          </div>
        </div>
      </div>
    </header>
  );
};

export const StatisticsOverviewPanel = ({
  serverLabel,
  overview,
}: {
  serverLabel: string;
  overview: OverviewState;
}) => {
  const { t, lang } = useI18n();

  return (
    <section className="bg-(--theme-bg-default) dark:bg-(--theme-bg-muted) border border-(--theme-border-subtle) dark:border-(--theme-border-default) rounded-xl shadow-sm p-4">
      <div className="relative flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="space-y-1 pr-24 sm:pr-0">
          <div className="text-xs font-semibold uppercase tracking-wider text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
            {t('dashboard_statistics')}
          </div>
          <div className="text-sm text-(--theme-fg-default) dark:text-(--theme-fg-default)">
            {serverLabel}
            {overview.serverMissing && (
              <span className="ml-2 text-xs text-(--theme-fg-danger) dark:text-(--theme-fg-danger)">
                {t('stats_no_server')}
              </span>
            )}
          </div>
          <div className="text-xs text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted) flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-2">
            {overview.stepSec ? <span>{`${t('stats_step')} ${overview.stepSec}s`}</span> : null}
            {overview.stepSec && overview.lastUpdatedAt ? (
              <span className="hidden sm:inline">·</span>
            ) : null}
            {overview.lastUpdatedAt ? (
              <span>{`${t('stats_updated')} ${formatDateTime(overview.lastUpdatedAt, lang)}`}</span>
            ) : null}
          </div>
        </div>

        <div className="absolute right-0 top-0 w-24 sm:static sm:w-full lg:w-auto">
          <Select
            value={overview.range}
            onChange={(event) => overview.setRange(event.target.value as MetricHistoryRange)}
            className="h-auto w-full text-xs/4 py-0.5 bg-(--theme-bg-muted) dark:bg-(--theme-bg-default) sm:w-full lg:w-auto"
            aria-label={t('stats_range')}
          >
            {RANGE_OPTIONS.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </Select>
        </div>
      </div>
    </section>
  );
};

export const StatisticsMetricSection = ({
  section,
  sectionIndex,
  overview,
  onOpenDetail,
}: {
  section: MetricSection;
  sectionIndex: number;
  overview: OverviewState;
  onOpenDetail: (metric: MetricConfig) => void;
}) => {
  const { t } = useI18n();
  const canRenderMetrics =
    (section.key !== 'disk_io' || Boolean(overview.ioDevice)) &&
    (section.key !== 'disk_usage' || Boolean(overview.mountDevice));
  const showIoDeviceSelect = section.key === 'disk_io';
  const showMountDeviceSelect = section.key === 'disk_usage';

  return (
    <div
      className={`grid grid-cols-1 lg:grid-cols-[160px_minmax(0,1fr)] ${
        sectionIndex > 0
          ? 'border-t border-(--theme-border-subtle) dark:border-(--theme-border-default)'
          : ''
      }`}
    >
      <div className="p-4 bg-(--theme-bg-muted) dark:bg-(--theme-bg-default) border-b border-(--theme-border-subtle) dark:border-(--theme-border-default) lg:border-b-0 lg:border-r">
        <div className="text-xs font-semibold uppercase tracking-wider text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
          {t(section.titleKey)}
        </div>

        {showIoDeviceSelect &&
          (overview.ioDeviceOptions.length > 0 ? (
            <Select
              value={overview.ioDevice}
              onChange={(event) => overview.setIoDevice(event.target.value)}
              className="mt-2 w-full text-xs py-1 bg-(--theme-bg-muted) dark:bg-(--theme-bg-default)"
              aria-label={t('disk_io_device')}
            >
              {overview.ioDeviceOptions.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </Select>
          ) : (
            <div className="mt-2 text-xs text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted)">
              {t('stats_device_required')}
            </div>
          ))}

        {showMountDeviceSelect &&
          (overview.mountOptions.length > 0 ? (
            <Select
              value={overview.mountDevice}
              onChange={(event) => overview.setMountDevice(event.target.value)}
              className="mt-2 w-full text-xs py-1 bg-(--theme-bg-muted) dark:bg-(--theme-bg-default)"
              aria-label={t('stats_partition_device')}
            >
              {overview.mountOptions.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </Select>
          ) : (
            <div className="mt-2 text-xs text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted)">
              {t('stats_partition_required')}
            </div>
          ))}
      </div>

      <div className="divide-y divide-(--theme-border-subtle) dark:divide-(--theme-border-default)">
        {!canRenderMetrics
          ? null
          : section.metrics.map((metric) => {
              const key = metricSeriesKey(metric);
              const isMetricUnavailable = metric.historyAvailable === false;
              const isMetricLoading = !isMetricUnavailable && overview.loadingMetricsSet.has(key);
              const hasMetricError = overview.failedMetricsSet.has(key);
              const stats = isMetricUnavailable ? undefined : overview.statsLookup.get(key);
              const unitScale = stats?.unitScale ?? resolveUnitScale(metric.unit, 0);
              const unitLabel = unitScale.unitLabel || metric.unit || '';
              const precision = metric.precision ?? 2;
              const formatSummaryValue = (value: number | null | undefined) =>
                stats?.count ? formatScaledValue(value ?? null, 2, unitScale, true, '-') : '-';
              const maxText = formatSummaryValue(stats?.max);
              const minText = formatSummaryValue(stats?.min);
              const avgText = formatSummaryValue(stats?.avg);
              const metricTitle = metric.label || t(metric.titleKey);
              const canOpenDetail = metric.historyAvailable !== false;

              return (
                <MetricRow
                  key={key}
                  seriesKey={key}
                  onVisible={canOpenDetail ? overview.showMetric : undefined}
                  className="grid grid-cols-1 md:grid-cols-[120px_minmax(0,55%)_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_24px] gap-3 md:gap-2 items-center px-4 py-3"
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="truncate text-xs font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-default)">
                      {metricTitle}
                    </span>
                    {unitLabel && (
                      <span className="text-[11px] font-semibold text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
                        {unitLabel}
                      </span>
                    )}
                    {canOpenDetail && (
                      <ExpandButton
                        label={t('stats_expand')}
                        onClick={() => onOpenDetail(metric)}
                        className="ml-auto md:hidden p-1 rounded text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted) hover:bg-(--theme-bg-muted) dark:hover:bg-(--theme-bg-default) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default) transition-colors"
                      />
                    )}
                  </div>

                  <div className="min-h-21">
                    <MetricHistoryChart
                      title={metricTitle}
                      unit={metric.unit}
                      precision={precision}
                      data={isMetricUnavailable ? [] : (overview.seriesLookup[key] ?? [])}
                      height={84}
                      variant="inline"
                      className="w-full"
                      chartClassName="bg-transparent border-0"
                      loading={isMetricLoading}
                      emptyState={
                        isMetricUnavailable ? (
                          <span className="text-(--theme-fg-subtle) dark:text-(--theme-fg-neutral)">
                            {t('stats_metric_unavailable')}
                          </span>
                        ) : hasMetricError ? (
                          <span className="inline-flex items-center gap-1 text-(--theme-fg-danger) dark:text-(--theme-fg-danger)">
                            <span>{t('stats_metric_error')}</span>
                            <button
                              type="button"
                              onClick={() => overview.reloadMetric(key)}
                              className="inline-flex size-4 shrink-0 items-center justify-center rounded transition-colors hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default)"
                              aria-label={`${metricTitle} ${t('stats_refresh')}`}
                              title={`${metricTitle} ${t('stats_refresh')}`}
                            >
                              <RefreshCw size={12} />
                            </button>
                          </span>
                        ) : undefined
                      }
                    />
                  </div>

                  <div className="hidden md:flex">
                    <StatCell label={t('stats_max')} value={maxText} />
                  </div>
                  <div className="hidden md:flex">
                    <StatCell label={t('stats_min')} value={minText} />
                  </div>
                  <div className="hidden md:flex">
                    <StatCell label={t('stats_avg')} value={avgText} />
                  </div>
                  <div className="hidden md:flex items-center justify-end gap-2 text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted)">
                    {canOpenDetail && (
                      <ExpandButton
                        label={t('stats_expand')}
                        onClick={() => onOpenDetail(metric)}
                        className="p-1 rounded hover:bg-(--theme-bg-muted) dark:hover:bg-(--theme-bg-default) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-default) transition-colors"
                      />
                    )}
                  </div>

                  <div className="grid grid-cols-3 gap-2 text-xs md:hidden">
                    <StatCell label={t('stats_max')} value={maxText} align="center" />
                    <StatCell label={t('stats_min')} value={minText} align="center" />
                    <StatCell label={t('stats_avg')} value={avgText} align="center" />
                  </div>
                </MetricRow>
              );
            })}
      </div>
    </div>
  );
};

export const StatisticsDetailModal = ({ detail }: { detail: DetailState }) => {
  const { t, lang } = useI18n();
  const metricTitle = detail.metric ? detail.metric.label || t(detail.metric.titleKey) : '';

  return (
    <Modal
      isOpen={detail.isOpen}
      onClose={detail.close}
      maxWidth="max-w-6xl"
      ariaLabelledby="stats-detail-title"
    >
      <ModalHeader
        id="stats-detail-title"
        title={
          <div className="flex items-baseline gap-2">
            <span>{metricTitle || t('dashboard_statistics')}</span>
            {detail.unitLabel && (
              <span className="text-sm font-semibold text-(--theme-fg-subtle) dark:text-(--theme-fg-neutral)">
                {detail.unitLabel}
              </span>
            )}
          </div>
        }
        onClose={detail.close}
      />
      <ModalBody className="space-y-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap items-center gap-4">
            <div className="flex items-center gap-2">
              <span className="min-w-20 text-xs/5 font-medium text-(--theme-fg-default) dark:text-(--theme-fg-default) whitespace-nowrap text-right">
                {t('stats_range')}
              </span>
              <Select
                value={detail.range}
                onChange={(event) => detail.setRange(event.target.value as MetricHistoryRange)}
                className="text-xs py-1 bg-(--theme-bg-muted) dark:bg-(--theme-bg-default) min-w-32"
                aria-label={t('stats_range')}
              >
                {RANGE_OPTIONS.map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </Select>
            </div>

            <div className="flex items-center gap-2">
              <span className="min-w-20 text-xs/5 font-medium text-(--theme-fg-default) dark:text-(--theme-fg-default) whitespace-nowrap text-right">
                {t('stats_agg')}
              </span>
              <Select
                value={detail.aggregation}
                onChange={(event) =>
                  detail.setAggregation(event.target.value as MetricHistoryAggregation)
                }
                className="text-xs py-1 bg-(--theme-bg-muted) dark:bg-(--theme-bg-default) min-w-32"
                aria-label={t('stats_agg')}
              >
                {AGGREGATION_OPTIONS.map((option) => (
                  <option key={option} value={option}>
                    {t(`stats_agg_${option}` as TranslationKey)}
                  </option>
                ))}
              </Select>
            </div>

            {detail.deviceLabel && (
              <div className="text-[11px] text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted)">
                {detail.deviceLabel}
              </div>
            )}
          </div>

          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="secondary"
              icon={RefreshCw}
              onClick={() => void detail.reload()}
              disabled={detail.isLoading}
            >
              {t('stats_refresh')}
            </Button>
          </div>
        </div>

        <div className="rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-muted) dark:bg-(--theme-bg-default) p-3">
          <MetricHistoryChart
            title={metricTitle}
            unit={detail.metric?.unit}
            precision={detail.metric?.precision ?? 2}
            data={detail.series}
            height={340}
            variant="inline"
            showXAxis
            xAxisMaxLabels={detail.range === '15d' || detail.range === '30d' ? 10 : undefined}
            className="w-full"
            chartClassName="bg-transparent border-0"
            loading={detail.isLoading}
          />
        </div>

        {detail.stepSec || detail.updatedAt ? (
          <div className="text-xs text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted) flex flex-wrap gap-x-3 gap-y-1">
            {detail.stepSec ? <span>{`${t('stats_step')} ${detail.stepSec}s`}</span> : null}
            {detail.updatedAt ? (
              <span>{`${t('stats_updated')} ${formatDateTime(detail.updatedAt, lang)}`}</span>
            ) : null}
          </div>
        ) : null}

        {detail.errorKey ? (
          <div className="text-xs text-(--theme-fg-danger)">{t(detail.errorKey)}</div>
        ) : null}
      </ModalBody>
    </Modal>
  );
};
