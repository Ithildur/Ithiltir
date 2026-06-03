import React from 'react';
import CircleHelp from 'lucide-react/dist/esm/icons/circle-help';
import { Tooltip } from '@components/ui/Tooltip';
import { useI18n, type TranslationKey } from '@i18n';
import type { ChartPoint, TrafficChartMode } from './pageModel';
import { formatTrafficBytes } from './viewModel';

const chartWidth = 760;
const chartHeight = 280;
const chartBasePad = { left: 48, right: 28, top: 26, bottom: 44 };
const chartAxisLabelGap = 10;
const chartAxisLabelCharWidth = 7;
const placeholderBars = [0.42, 0.58, 0.36, 0.68, 0.5, 0.74, 0.46, 0.62];

const chartModeOptions: { mode: TrafficChartMode; label: TranslationKey; daily: boolean }[] = [
  { mode: 'previous_daily', label: 'traffic_chart_previous_daily', daily: true },
  { mode: 'current_daily', label: 'traffic_chart_current_daily', daily: true },
  { mode: 'monthly', label: 'traffic_chart_monthly', daily: false },
];

const isDailyChartMode = (mode: TrafficChartMode) => mode !== 'monthly';

export const TrafficTrendChart = ({
  points,
  mode,
  dailyAvailable,
  onModeChange,
}: {
  points: ChartPoint[];
  mode: TrafficChartMode;
  dailyAvailable: boolean;
  onModeChange: (mode: TrafficChartMode) => void;
}) => {
  const { t } = useI18n();
  const dailyUnavailable = isDailyChartMode(mode) && !dailyAvailable;
  const maxValue = Math.max(
    1,
    ...points.flatMap((item) => [item.inBytes, item.outBytes, item.inBytes + item.outBytes]),
  );
  const axisValues = [maxValue, maxValue / 2, 0];
  const axisLabels = axisValues.map((value) => (dailyUnavailable ? '' : formatTrafficBytes(value)));
  const axisLabelWidth =
    Math.max(0, ...axisLabels.map((label) => label.length)) * chartAxisLabelCharWidth;
  const chartPad = {
    ...chartBasePad,
    left: Math.max(chartBasePad.left, Math.ceil(axisLabelWidth + chartAxisLabelGap + 2)),
  };
  const innerWidth = chartWidth - chartPad.left - chartPad.right;
  const innerHeight = chartHeight - chartPad.top - chartPad.bottom;
  const barStep = points.length > 0 ? innerWidth / points.length : innerWidth;
  const barWidth = Math.min(18, Math.max(3, barStep / 4));
  const pointInset = Math.max(6, barWidth + 4);
  const usableWidth = Math.max(1, innerWidth - pointInset * 2);
  const step = points.length > 1 ? usableWidth / (points.length - 1) : usableWidth;
  const yFor = (value: number) => chartPad.top + innerHeight - (value / maxValue) * innerHeight;
  const xFor = (index: number) =>
    points.length > 1 ? chartPad.left + pointInset + index * step : chartPad.left + innerWidth / 2;
  const linePoints = points.map((item, index) => ({
    x: xFor(index),
    y: yFor(item.inBytes + item.outBytes),
  }));
  const linePath = linePoints
    .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x.toFixed(1)} ${point.y.toFixed(1)}`)
    .join(' ');
  const areaPath =
    linePoints.length > 0
      ? `${linePath} L ${linePoints[linePoints.length - 1].x.toFixed(1)} ${chartPad.top + innerHeight} L ${linePoints[0].x.toFixed(1)} ${chartPad.top + innerHeight} Z`
      : '';
  const gradientId = React.useId().replace(/:/g, '');
  const labelEvery = Math.max(1, Math.ceil(points.length / 8));
  const dailyUnavailableText = t('traffic_daily_billing_only');

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-3">
          <div className="text-sm font-semibold text-(--theme-fg-default)">
            {t('traffic_trend_chart')}
          </div>
          <div className="inline-flex rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-default) p-0.5 dark:border-(--theme-border-default)">
            {chartModeOptions.map((option) => {
              const selected = mode === option.mode;
              const unavailable = option.daily && !dailyAvailable;
              const button = (
                <button
                  type="button"
                  className={`rounded px-2.5 py-1 text-xs font-semibold transition-colors disabled:cursor-not-allowed ${
                    selected
                      ? unavailable
                        ? 'bg-(--theme-bg-muted) text-(--theme-fg-muted)'
                        : 'bg-(--theme-bg-accent-emphasis) text-(--theme-fg-on-emphasis)'
                      : unavailable
                        ? 'text-(--theme-fg-muted) opacity-60'
                        : 'text-(--theme-fg-muted) hover:text-(--theme-fg-default)'
                  }`}
                  disabled={unavailable}
                  onClick={() => onModeChange(option.mode)}
                >
                  {t(option.label)}
                </button>
              );
              return option.daily ? (
                <Tooltip
                  key={option.mode}
                  content={dailyAvailable ? null : dailyUnavailableText}
                  className="inline-flex"
                >
                  {button}
                </Tooltip>
              ) : (
                <React.Fragment key={option.mode}>{button}</React.Fragment>
              );
            })}
          </div>
          {dailyUnavailable && (
            <Tooltip content={dailyUnavailableText} className="inline-flex">
              <span className="inline-flex size-5 cursor-help items-center justify-center text-(--theme-fg-muted)">
                <CircleHelp size={14} />
              </span>
            </Tooltip>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-3 text-xs text-(--theme-fg-muted)">
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-(--theme-fg-accent)" />
            {t('traffic_in_total')}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="size-2 rounded-full bg-(--theme-fg-warning-strong)" />
            {t('traffic_out_total')}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-0.5 w-4 rounded-full bg-(--theme-fg-default)" />
            {t('traffic_total_trend')}
          </span>
        </div>
      </div>

      <svg
        className={`block h-auto w-full ${dailyUnavailable ? 'opacity-60 saturate-0' : ''}`}
        viewBox={`0 0 ${chartWidth} ${chartHeight}`}
        role="img"
        aria-label={t('traffic_trend_chart')}
      >
        <defs>
          <linearGradient id={`${gradientId}-area`} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="var(--theme-fg-accent)" stopOpacity="0.2" />
            <stop offset="100%" stopColor="var(--theme-fg-accent)" stopOpacity="0" />
          </linearGradient>
        </defs>

        {axisValues.map((value, index) => {
          const y = yFor(value);
          return (
            <g key={value}>
              <line
                x1={chartPad.left}
                x2={chartWidth - chartPad.right}
                y1={y}
                y2={y}
                stroke="var(--theme-border-subtle)"
                strokeDasharray="4 6"
              />
              <text
                x={chartPad.left - chartAxisLabelGap}
                y={y + 4}
                textAnchor="end"
                className="fill-(--theme-fg-muted) text-[11px] font-medium"
              >
                {axisLabels[index]}
              </text>
            </g>
          );
        })}

        {areaPath && <path d={areaPath} fill={`url(#${gradientId}-area)`} />}

        {dailyUnavailable && (
          <g opacity="0.55">
            {placeholderBars.map((height, index) => {
              const x = chartPad.left + (innerWidth / placeholderBars.length) * index + 10;
              const width = Math.max(8, innerWidth / placeholderBars.length - 22);
              const y = chartPad.top + innerHeight - innerHeight * height;
              return (
                <rect
                  key={index}
                  x={x}
                  y={y}
                  width={width}
                  height={innerHeight * height}
                  rx="3"
                  fill="var(--theme-fg-muted)"
                  opacity="0.25"
                />
              );
            })}
            <path
              d={`M ${chartPad.left + 12} ${chartPad.top + innerHeight * 0.64} C ${chartPad.left + innerWidth * 0.24} ${chartPad.top + innerHeight * 0.5}, ${chartPad.left + innerWidth * 0.38} ${chartPad.top + innerHeight * 0.72}, ${chartPad.left + innerWidth * 0.56} ${chartPad.top + innerHeight * 0.42} S ${chartPad.left + innerWidth * 0.86} ${chartPad.top + innerHeight * 0.34}, ${chartPad.left + innerWidth - 12} ${chartPad.top + innerHeight * 0.48}`}
              fill="none"
              stroke="var(--theme-fg-muted)"
              strokeLinecap="round"
              strokeWidth="2.25"
            />
          </g>
        )}

        {points.length === 0 && (
          <text
            x={chartPad.left + innerWidth / 2}
            y={chartPad.top + innerHeight / 2}
            textAnchor="middle"
            className="fill-(--theme-fg-muted) text-sm font-semibold"
          >
            {dailyUnavailable ? dailyUnavailableText : t('traffic_no_data')}
          </text>
        )}

        {points.map((item, index) => {
          const x = xFor(index);
          const totalBytes = item.inBytes + item.outBytes;
          const inHeight = chartPad.top + innerHeight - yFor(item.inBytes);
          const outHeight = chartPad.top + innerHeight - yFor(item.outBytes);
          const baseY = chartPad.top + innerHeight;
          const showLabel = index === 0 || index === points.length - 1 || index % labelEvery === 0;
          return (
            <g key={item.key}>
              <title>{`${item.title}
${t('total_trans')}: ${formatTrafficBytes(totalBytes)}
${t('traffic_tooltip_tx')}: ${formatTrafficBytes(item.outBytes)}
${t('traffic_tooltip_rx')}: ${formatTrafficBytes(item.inBytes)}`}</title>
              <rect
                x={x - barWidth - 2}
                y={baseY - inHeight}
                width={barWidth}
                height={Math.max(1, inHeight)}
                rx="3"
                fill="var(--theme-fg-accent)"
                opacity="0.7"
              />
              <rect
                x={x + 2}
                y={baseY - outHeight}
                width={barWidth}
                height={Math.max(1, outHeight)}
                rx="3"
                fill="var(--theme-fg-warning-strong)"
                opacity="0.72"
              />
              {showLabel && (
                <text
                  x={x}
                  y={chartHeight - 16}
                  textAnchor="middle"
                  className="fill-(--theme-fg-muted) text-[11px] font-semibold"
                >
                  {item.label}
                </text>
              )}
            </g>
          );
        })}

        {linePath && (
          <path
            d={linePath}
            fill="none"
            stroke="var(--theme-fg-default)"
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="2.25"
          />
        )}
        {linePoints.map((point) => (
          <circle
            key={`${point.x}-${point.y}`}
            cx={point.x}
            cy={point.y}
            r="3"
            fill="var(--theme-bg-default)"
            stroke="var(--theme-fg-default)"
            strokeWidth="1.75"
          />
        ))}
      </svg>
    </div>
  );
};
