import React from 'react';
import Badge from '@components/ui/Badge';
import Card from '@components/ui/Card';
import type { AlertEvent } from '@app-types/admin';
import { useI18n } from '@i18n';
import { formatLocalDateTime } from '@utils/time';
import { alertMetricName } from './alertLabels';

interface Props {
  items: AlertEvent[];
  loading: boolean;
  customRangeLabel: string;
}

const numberText = (value?: number | null): string => {
  if (value === null || value === undefined || !Number.isFinite(value)) return '-';
  return Math.abs(value) >= 100 ? value.toFixed(0) : value.toFixed(2).replace(/\.?0+$/, '');
};

const eventTitle = (item: AlertEvent, metricName: string): string =>
  item.title?.trim() || item.rule_name?.trim() || metricName;

const formatDate = (value: string | null | undefined, lang: 'zh' | 'en'): string => {
  if (!value) return '-';
  return formatLocalDateTime(value, lang, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  });
};

const formatDuration = (
  start: string,
  end: string | null | undefined,
  status: AlertEvent['status'],
  lang: 'zh' | 'en',
): string => {
  const startAt = new Date(start).getTime();
  const endAt = end ? new Date(end).getTime() : status === 'open' ? Date.now() : Number.NaN;
  if (!Number.isFinite(startAt) || !Number.isFinite(endAt) || endAt < startAt) return '-';
  const minutes = Math.max(1, Math.floor((endAt - startAt) / 60_000));
  const days = Math.floor(minutes / 1440);
  const hours = Math.floor((minutes % 1440) / 60);
  const mins = minutes % 60;
  if (lang === 'zh') {
    if (days > 0) return hours > 0 ? `${days}天 ${hours}小时` : `${days}天`;
    if (hours > 0) return mins > 0 ? `${hours}小时 ${mins}分` : `${hours}小时`;
    return `${mins}分`;
  }
  if (days > 0) return hours > 0 ? `${days}d ${hours}h` : `${days}d`;
  if (hours > 0) return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
  return `${mins}m`;
};

const StatusBadge: React.FC<{ status: AlertEvent['status'] }> = ({ status }) => {
  const { t } = useI18n();
  return status === 'open' ? (
    <Badge color="rose">{t('admin_alerts_events_status_open')}</Badge>
  ) : (
    <Badge color="emerald">{t('admin_alerts_events_status_closed')}</Badge>
  );
};

export const AlertEventsTable: React.FC<Props> = ({ items, loading, customRangeLabel }) => {
  const { lang, t } = useI18n();

  if (loading && items.length === 0) {
    return (
      <Card className="p-8 text-center text-sm text-(--theme-fg-muted)">
        {t('admin_alerts_events_loading')}
      </Card>
    );
  }

  return (
    <>
      {customRangeLabel && (
        <div className="text-xs text-(--theme-fg-muted)">
          {t('admin_alerts_events_custom_range_label', { range: customRangeLabel })}
        </div>
      )}

      <Card className="hidden overflow-hidden md:block">
        <div className="overflow-x-auto">
          <table className="w-full bg-(--theme-bg-default) text-left text-sm dark:bg-(--theme-bg-default)">
            <thead className="whitespace-nowrap border-b border-(--theme-border-subtle) bg-(--theme-bg-muted) text-xs font-semibold text-(--theme-fg-default) dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle)">
              <tr>
                <th className="min-w-56 px-3 py-2.5">
                  {t('admin_alerts_events_col_server')}
                </th>
                <th className="w-32 px-3 py-2.5">
                  {t('admin_alerts_events_col_metric')}
                </th>
                <th className="w-24 px-3 py-2.5">
                  {t('admin_alerts_events_col_status')}
                </th>
                <th className="w-32 px-3 py-2.5">
                  {t('admin_alerts_events_col_first')}
                </th>
                <th className="w-32 px-3 py-2.5">
                  {t('admin_alerts_events_col_last')}
                </th>
                <th className="w-28 px-3 py-2.5">
                  {t('admin_alerts_events_col_duration')}
                </th>
                <th className="w-32 px-3 py-2.5">
                  {t('admin_alerts_events_col_value')}
                </th>
                <th className="min-w-72 px-3 py-2.5">
                  {t('admin_alerts_events_col_summary')}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
              {items.length === 0 ? (
                <tr>
                  <td colSpan={8} className="px-4 py-12 text-center text-(--theme-fg-muted)">
                    {t('admin_alerts_events_empty')}
                  </td>
                </tr>
              ) : (
                items.map((item) => {
                  const metric = alertMetricName(item.metric, t);
                  const title = eventTitle(item, metric);
                  return (
                    <tr
                      key={item.id}
                      className="transition-colors hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)"
                    >
                      <td className="px-3 py-2.5">
                        <div className="min-w-0">
                          <div className="truncate font-semibold text-(--theme-fg-default)">
                            {item.server_name}
                          </div>
                          <div className="mt-1 flex min-w-0 gap-2 text-xs font-mono text-(--theme-fg-muted)">
                            <span className="shrink-0">{item.server_ip || '-'}</span>
                            <span className="min-w-0 truncate">{item.server_hostname || '-'}</span>
                          </div>
                        </div>
                      </td>
                      <td className="px-3 py-2.5 text-xs font-medium text-(--theme-fg-default)">
                        {metric}
                      </td>
                      <td className="px-3 py-2.5">
                        <StatusBadge status={item.status} />
                      </td>
                      <td className="px-3 py-2.5 text-xs text-(--theme-fg-muted)">
                        {formatDate(item.first_trigger_at, lang)}
                      </td>
                      <td className="px-3 py-2.5 text-xs text-(--theme-fg-muted)">
                        {formatDate(item.closed_at || item.last_trigger_at, lang)}
                      </td>
                      <td className="px-3 py-2.5 text-xs text-(--theme-fg-muted)">
                        {formatDuration(item.first_trigger_at, item.closed_at, item.status, lang)}
                      </td>
                      <td className="px-3 py-2.5 text-xs font-mono text-(--theme-fg-muted)">
                        {numberText(item.current_value)} / {numberText(item.effective_threshold)}
                      </td>
                      <td className="px-3 py-2.5">
                        <div className="min-w-0">
                          <div className="truncate text-xs font-semibold text-(--theme-fg-default)">
                            {title}
                          </div>
                          {item.message && (
                            <div className="mt-1 line-clamp-2 text-xs text-(--theme-fg-muted)">
                              {item.message}
                            </div>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </Card>

      <div className="grid gap-3 md:hidden">
        {items.length === 0 ? (
          <Card className="p-8 text-center text-sm text-(--theme-fg-muted)">
            {t('admin_alerts_events_empty')}
          </Card>
        ) : (
          items.map((item) => {
            const metric = alertMetricName(item.metric, t);
            const title = eventTitle(item, metric);
            return (
              <Card key={item.id} className="p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="truncate font-semibold text-(--theme-fg-default)">
                      {item.server_name}
                    </div>
                    <div className="mt-1 truncate text-xs font-mono text-(--theme-fg-muted)">
                      {item.server_ip || item.server_hostname || `ID: ${item.server_id}`}
                    </div>
                  </div>
                  <StatusBadge status={item.status} />
                </div>
                <div className="mt-3 grid gap-2 text-xs text-(--theme-fg-muted)">
                  <div className="flex justify-between gap-3">
                    <span>{t('admin_alerts_events_col_metric')}</span>
                    <span className="font-medium text-(--theme-fg-default)">{metric}</span>
                  </div>
                  <div className="flex justify-between gap-3">
                    <span>{t('admin_alerts_events_col_last')}</span>
                    <span>{formatDate(item.closed_at || item.last_trigger_at, lang)}</span>
                  </div>
                  <div className="flex justify-between gap-3">
                    <span>{t('admin_alerts_events_col_duration')}</span>
                    <span>
                      {formatDuration(item.first_trigger_at, item.closed_at, item.status, lang)}
                    </span>
                  </div>
                  <div className="flex justify-between gap-3">
                    <span>{t('admin_alerts_events_col_value')}</span>
                    <span className="font-mono">
                      {numberText(item.current_value)} / {numberText(item.effective_threshold)}
                    </span>
                  </div>
                </div>
                <div className="mt-3 border-t border-(--theme-border-muted) pt-3">
                  <div className="text-sm font-semibold text-(--theme-fg-default)">{title}</div>
                  {item.message && (
                    <div className="mt-1 text-xs text-(--theme-fg-muted)">{item.message}</div>
                  )}
                </div>
              </Card>
            );
          })
        )}
      </div>
    </>
  );
};
