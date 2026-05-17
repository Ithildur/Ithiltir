import React from 'react';
import uPlot from 'uplot';
import { useI18n } from '@i18n';
import { formatScaledValue, resolveUnitScale } from '@utils/metricFormat';
import { formatLocalDateTime } from '@utils/time';
import {
  alignMetricPoints,
  axisLabel,
  xLabel,
  isDarkTheme,
  latestMetricValue,
  niceMax,
  themeColor,
  chartValueStats,
  chartSpanDays,
  sortMetricPoints,
  type MetricPoint,
} from './metricHistoryChartModel';

import 'uplot/dist/uPlot.min.css';
import './MetricHistoryChart.css';

export type { MetricPoint } from './metricHistoryChartModel';

interface Props {
  title: string;
  data: MetricPoint[];
  unit?: string;
  height?: number;
  precision?: number;
  className?: string;
  chartClassName?: string;
  variant?: 'card' | 'inline';
  loading?: boolean;
  showXAxis?: boolean;
  xAxisMaxLabels?: number;
  emptyState?: React.ReactNode;
}

const MetricHistoryChart: React.FC<Props> = ({
  title,
  data,
  unit,
  height = 180,
  precision = 2,
  className = '',
  chartClassName = '',
  variant = 'card',
  loading = false,
  showXAxis = false,
  xAxisMaxLabels,
  emptyState,
}) => {
  const { t, lang } = useI18n();
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const plotRef = React.useRef<uPlot | null>(null);
  const resizeObserverRef = React.useRef<ResizeObserver | null>(null);
  const alignedRef = React.useRef<uPlot.AlignedData | null>(null);
  const loadingRef = React.useRef(loading);
  const chartBodyRef = React.useRef<HTMLDivElement | null>(null);
  const tooltipRef = React.useRef<HTMLDivElement | null>(null);
  const tooltipDateRef = React.useRef<HTMLDivElement | null>(null);
  const tooltipTimeRef = React.useRef<HTMLDivElement | null>(null);
  const tooltipValueRef = React.useRef<HTMLDivElement | null>(null);
  const baseXRangeRef = React.useRef<{ min: number; max: number } | null>(null);
  const [dark, setDark] = React.useState(isDarkTheme);
  const [isZoomed, setIsZoomed] = React.useState(false);
  const isInline = variant === 'inline';

  const sorted = React.useMemo(() => sortMetricPoints(data), [data]);
  const aligned = React.useMemo(() => alignMetricPoints(sorted), [sorted]);
  const xSpanDays = React.useMemo(() => chartSpanDays(aligned), [aligned]);

  const normalizedMaxLabels =
    typeof xAxisMaxLabels === 'number' && Number.isFinite(xAxisMaxLabels)
      ? Math.max(1, Math.floor(xAxisMaxLabels))
      : null;
  const shouldCoarsenXAxis = showXAxis && (normalizedMaxLabels != null || xSpanDays >= 14);
  const xAxisValues = React.useCallback<uPlot.Axis.DynamicValues>(
    (_u, splits) => {
      if (!shouldCoarsenXAxis) {
        return splits.map((value) => xLabel(value, true));
      }
      const maxLabels = normalizedMaxLabels ?? 10;
      if (splits.length <= maxLabels) {
        return splits.map((value) => xLabel(value, false));
      }
      const stride = Math.ceil(splits.length / maxLabels);
      return splits.map((value, idx) => (idx % stride === 0 ? xLabel(value, false) : ''));
    },
    [normalizedMaxLabels, shouldCoarsenXAxis],
  );

  const stats = React.useMemo(() => chartValueStats(aligned), [aligned]);

  const unitScale = React.useMemo(() => resolveUnitScale(unit, stats.maxAbs), [unit, stats.maxAbs]);

  const yAxisMax = React.useMemo(() => {
    const scaledMax = Math.max(0, stats.max) / unitScale.divisor;
    const niceScaledMax = niceMax(scaledMax);
    return niceScaledMax * unitScale.divisor;
  }, [unitScale.divisor, stats.max]);

  const latestValue = React.useMemo(() => latestMetricValue(sorted), [sorted]);

  const formatValue = React.useCallback(
    (value: number | null, includeUnit: boolean) =>
      formatScaledValue(value, precision, unitScale, includeUnit),
    [precision, unitScale],
  );

  const formatTooltipValue = React.useCallback(
    (value: number | null) => formatScaledValue(value, 2, unitScale, true, '-'),
    [unitScale],
  );

  const formatTooltipDate = React.useCallback(
    (value: number) =>
      formatLocalDateTime(value, lang, {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
      }),
    [lang],
  );

  const formatTooltipTime = React.useCallback(
    (value: number) =>
      formatLocalDateTime(value, lang, {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hourCycle: 'h23',
      }),
    [lang],
  );

  const hideTooltip = React.useCallback(() => {
    const tooltip = tooltipRef.current;
    if (!tooltip) return;
    tooltip.style.opacity = '0';
    tooltip.style.visibility = 'hidden';
    tooltip.style.left = '-9999px';
    tooltip.style.top = '-9999px';
    tooltip.setAttribute('aria-hidden', 'true');
  }, []);

  const hasPoints = Boolean(aligned && aligned[0].length > 0);

  const updateSelect = React.useCallback((plot: uPlot) => {
    const selectDiv = plot.root.querySelector('.u-select') as HTMLDivElement | null;
    if (!selectDiv) return;
    const visible = plot.select.width > 0 && plot.select.height > 0;
    selectDiv.style.opacity = visible ? '1' : '0';
    selectDiv.style.visibility = visible ? 'visible' : 'hidden';
  }, []);

  const syncZoom = React.useCallback((plot: uPlot, scaleKey: string) => {
    if (scaleKey !== 'x') return;
    const base = baseXRangeRef.current;
    const min = plot.scales.x.min;
    const max = plot.scales.x.max;

    if (!base || min == null || max == null) {
      setIsZoomed(false);
      return;
    }

    const zoomed = Math.abs(min - base.min) > 0.5 || Math.abs(max - base.max) > 0.5;
    setIsZoomed(zoomed);
  }, []);

  const moveTooltip = React.useCallback(
    (plot: uPlot) => {
      const tooltip = tooltipRef.current;
      const dateNode = tooltipDateRef.current;
      const timeNode = tooltipTimeRef.current;
      const valueNode = tooltipValueRef.current;
      const current = alignedRef.current;

      if (!tooltip || !dateNode || !timeNode || !valueNode || !current || loadingRef.current) {
        hideTooltip();
        return;
      }

      const idx = plot.cursor.idx;
      if (idx == null || idx < 0) {
        hideTooltip();
        return;
      }

      const xs = current[0] as number[];
      const ys = current[1] as Array<number | null>;
      const xPoint = xs[idx];
      const yPoint = ys[idx];

      if (!Number.isFinite(xPoint) || yPoint == null || !Number.isFinite(yPoint)) {
        hideTooltip();
        return;
      }

      const timestamp = xPoint * 1000;
      dateNode.textContent = formatTooltipDate(timestamp);
      timeNode.textContent = formatTooltipTime(timestamp);
      valueNode.textContent = formatTooltipValue(yPoint);

      const xPos = plot.cursor.left;
      const yPos = plot.cursor.top;

      if (xPos == null || yPos == null || !Number.isFinite(xPos) || !Number.isFinite(yPos)) {
        hideTooltip();
        return;
      }

      const host = chartBodyRef.current || plot.root;
      if (!host) {
        hideTooltip();
        return;
      }

      plot.syncRect();
      const hostRect = host.getBoundingClientRect();
      const plotRect = plot.rect;
      const plotLeft = plotRect.left - hostRect.left;
      const plotTop = plotRect.top - hostRect.top;
      const plotWidth = plotRect.width;
      const plotHeight = plotRect.height;

      if (
        !Number.isFinite(plotLeft) ||
        !Number.isFinite(plotTop) ||
        plotWidth <= 0 ||
        plotHeight <= 0
      ) {
        hideTooltip();
        return;
      }

      const tooltipWidth = tooltip.offsetWidth;
      const tooltipHeight = tooltip.offsetHeight;
      const padding = 6;
      const sideOffset = 12;
      const minLeft = plotLeft + padding;
      const maxLeft = plotLeft + plotWidth - tooltipWidth - padding;
      const minTop = plotTop + padding;
      const maxTop = plotTop + plotHeight - tooltipHeight - padding;
      const rightCandidate = plotLeft + xPos + sideOffset;
      const leftCandidate = plotLeft + xPos - sideOffset - tooltipWidth;
      const canFitRight = rightCandidate <= maxLeft;
      const canFitLeft = leftCandidate >= minLeft;

      let nextLeft = rightCandidate;
      if (!canFitRight && canFitLeft) nextLeft = leftCandidate;
      if (!canFitRight && !canFitLeft) {
        nextLeft = plotLeft + xPos - tooltipWidth / 2;
      }
      if (nextLeft < minLeft) nextLeft = minLeft;
      if (nextLeft > maxLeft) nextLeft = maxLeft;

      let nextTop = plotTop + yPos - tooltipHeight / 2;
      if (nextTop < minTop) nextTop = minTop;
      if (nextTop > maxTop) nextTop = maxTop;

      tooltip.style.left = `${Math.round(nextLeft)}px`;
      tooltip.style.top = `${Math.round(nextTop)}px`;
      tooltip.style.opacity = '1';
      tooltip.style.visibility = 'visible';
      tooltip.setAttribute('aria-hidden', 'false');
    },
    [formatTooltipDate, formatTooltipTime, formatTooltipValue, hideTooltip],
  );

  const resetZoom = React.useCallback(() => {
    const plot = plotRef.current;
    const base = baseXRangeRef.current;
    if (!plot || !base) return;

    plot.setScale('x', { min: base.min, max: base.max });
    plot.setScale('y', { min: 0, max: yAxisMax });
    plot.setSelect({ left: 0, top: 0, width: 0, height: 0 }, false);
  }, [yAxisMax]);

  const options = React.useMemo<uPlot.Options>(() => {
    const axisColor = themeColor(dark ? '--theme-fg-muted-alt' : '--theme-fg-muted');
    const gridColor = themeColor('--theme-chart-grid');
    const tickColor = themeColor('--theme-border-default');
    const lineColor = themeColor('--theme-fg-accent');
    const fillColor = themeColor('--theme-chart-fill');
    const yAxisSize = isInline ? 44 : 56;
    const padding: uPlot.Padding = isInline ? [8, 10, 8, 10] : [10, 12, 20, 12];
    const showXAxisAxis = !isInline || showXAxis;

    return {
      width: 0,
      height,
      padding,
      scales: {
        x: { time: true },
        y: { auto: false, range: [0, yAxisMax] },
      },
      legend: { show: false },
      cursor: {
        show: true,
        x: true,
        y: false,
        drag: { x: true, y: false },
        points: { show: false },
        dataIdx: (_u, _seriesIdx, closestIdx) => closestIdx,
        hover: { prox: 24 },
      },
      hooks: {
        setCursor: [moveTooltip],
        setScale: [syncZoom],
        setSelect: [updateSelect],
      },
      axes: [
        {
          show: showXAxisAxis,
          stroke: axisColor,
          grid: { stroke: gridColor, width: 1, show: showXAxisAxis },
          ticks: { stroke: tickColor, width: 1, show: showXAxisAxis },
          values: xAxisValues,
          size: showXAxisAxis ? 28 : 0,
        },
        {
          stroke: axisColor,
          grid: { stroke: gridColor, width: 1 },
          ticks: { stroke: tickColor, width: 1 },
          splits: () => [0, yAxisMax / 2, yAxisMax],
          values: (_u, splits) => splits.map((value) => axisLabel(value, unitScale.divisor)),
          size: yAxisSize,
        },
      ],
      series: [
        {},
        {
          label: title,
          stroke: lineColor,
          width: isInline ? 1.5 : 2,
          fill: isInline ? 'transparent' : fillColor,
          points: { show: false },
        },
      ],
    };
  }, [
    height,
    dark,
    isInline,
    showXAxis,
    title,
    unitScale.divisor,
    moveTooltip,
    syncZoom,
    updateSelect,
    xAxisValues,
    yAxisMax,
  ]);

  React.useEffect(() => {
    if (typeof document === 'undefined') return undefined;
    const root = document.documentElement;
    const observer = new MutationObserver(() => setDark(root.classList.contains('dark')));
    observer.observe(root, { attributes: true, attributeFilter: ['class'] });
    return () => observer.disconnect();
  }, []);

  React.useEffect(() => {
    alignedRef.current = aligned;
    if (!aligned || aligned[0].length === 0) {
      baseXRangeRef.current = null;
      setIsZoomed(false);
      return;
    }
    const xs = aligned[0] as number[];
    baseXRangeRef.current = { min: xs[0], max: xs[xs.length - 1] };
    setIsZoomed(false);
  }, [aligned]);

  React.useEffect(() => {
    loadingRef.current = loading;
  }, [loading]);

  React.useEffect(() => {
    if (loading || !hasPoints) {
      hideTooltip();
    }
  }, [hasPoints, hideTooltip, loading]);

  React.useEffect(() => {
    const root = containerRef.current;
    if (!root) return undefined;

    if (!hasPoints || !alignedRef.current) {
      plotRef.current?.destroy();
      plotRef.current = null;
      resizeObserverRef.current?.disconnect();
      resizeObserverRef.current = null;
      return undefined;
    }

    plotRef.current?.destroy();
    resizeObserverRef.current?.disconnect();

    const width = Math.max(280, root.clientWidth);
    const plot = new uPlot({ ...options, width }, alignedRef.current, root);
    plotRef.current = plot;
    updateSelect(plot);
    hideTooltip();

    const hideOnLeave = () => hideTooltip();
    plot.over.addEventListener('mouseleave', hideOnLeave);

    const resizeObserver = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (!entry || !plotRef.current) return;
      const nextWidth = Math.floor(entry.contentRect.width);
      if (nextWidth > 0 && nextWidth !== plotRef.current.width) {
        plotRef.current.setSize({ width: nextWidth, height });
      }
    });

    resizeObserver.observe(root);
    resizeObserverRef.current = resizeObserver;

    return () => {
      plot.over.removeEventListener('mouseleave', hideOnLeave);
      resizeObserver.disconnect();
      plot.destroy();
      plotRef.current = null;
      resizeObserverRef.current = null;
    };
  }, [hasPoints, height, hideTooltip, options, updateSelect]);

  React.useEffect(() => {
    if (!hasPoints || !aligned) return;
    plotRef.current?.setData(aligned);
  }, [aligned, hasPoints]);

  const chartContainerClassName =
    variant === 'card'
      ? `relative rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-muted) dark:bg-(--theme-bg-default) ${chartClassName}`.trim()
      : `relative ${chartClassName}`.trim();

  const chartBody = (
    <div ref={chartBodyRef} className={chartContainerClassName}>
      <div ref={containerRef} style={{ height }} />
      {isZoomed && (
        <button
          type="button"
          className="metric-history-reset"
          onClick={resetZoom}
          aria-label={t('stats_reset_zoom')}
          title={t('stats_reset_zoom')}
        >
          {t('stats_reset_zoom')}
        </button>
      )}
      {loading && (
        <div className="metric-history-loading" role="status" aria-live="polite">
          <span className="metric-history-loading-spinner" aria-hidden="true" />
          <span>{t('loading')}</span>
        </div>
      )}
      {!loading && !hasPoints && (
        <div className="absolute inset-0 flex items-center justify-center text-xs text-(--theme-fg-subtle) dark:text-(--theme-fg-neutral)">
          {emptyState ?? t('no_data')}
        </div>
      )}
      <div ref={tooltipRef} className="metric-history-tooltip" aria-hidden="true">
        <div ref={tooltipDateRef} className="metric-history-tooltip-date" />
        <div ref={tooltipTimeRef} className="metric-history-tooltip-time" />
        <div ref={tooltipValueRef} className="metric-history-tooltip-value" />
      </div>
    </div>
  );

  if (variant === 'inline') {
    return <div className={`metric-history-chart ${className}`}>{chartBody}</div>;
  }

  return (
    <section
      className={`metric-history-chart bg-(--theme-bg-default) dark:bg-(--theme-bg-muted) border border-(--theme-border-subtle) dark:border-(--theme-border-default) rounded-xl shadow-sm p-4 ${className}`}
    >
      <div className="flex items-center justify-between mb-3">
        <div className="text-xs uppercase tracking-wider font-semibold text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
          {title}
        </div>
        <div className="flex items-baseline gap-1 font-mono text-sm text-(--theme-fg-default) dark:text-(--theme-fg-default)">
          <span>{formatValue(latestValue, false)}</span>
          {unitScale.unitLabel && (
            <span className="text-[10px] text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted)">
              {unitScale.unitLabel}
            </span>
          )}
        </div>
      </div>
      {chartBody}
    </section>
  );
};

export default MetricHistoryChart;
