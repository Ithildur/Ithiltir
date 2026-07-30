import React from 'react';
import ChevronDown from 'lucide-react/dist/esm/icons/chevron-down';
import RotateCcw from 'lucide-react/dist/esm/icons/rotate-ccw';
import Search from 'lucide-react/dist/esm/icons/search';
import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import ComboboxSelect, { type ComboboxOption } from '@components/ui/ComboboxSelect';
import Input from '@components/ui/Input';
import type { AlertEvent, AlertEventServer, AlertEventStatusFilter } from '@app-types/admin';
import { fetchAlertEvents, fetchAlertEventServers } from '@lib/adminApi';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import { useI18n } from '@i18n';
import { isCanceledRequestError } from '@utils/errors';
import { alertMetricName, alertMetricValues } from './alertLabels';
import {
  alertEventCustomRangeLabel,
  alertEventFilterReady,
  alertEventFilterFromParams,
  alertEventFilterKey,
  alertEventRequestBounds,
  defaultAlertEventFilter,
  type AlertEventFilter,
  type AlertEventRange,
  urlParamsForAlertEventFilter,
} from './alertEventsModel';
import { AlertEventsTable } from './AlertEventsTable';

interface Props {
  searchParams: URLSearchParams;
  setSearchParams: (next: URLSearchParams) => void;
}

const eventPageLimit = 200;
type AlertEventBounds = ReturnType<typeof alertEventRequestBounds>;

export const AlertEventsPanel: React.FC<Props> = ({ searchParams, setSearchParams }) => {
  const { lang, t } = useI18n();
  const apiError = useApiErrorHandler();
  const [draft, setDraft] = React.useState<AlertEventFilter>(() =>
    alertEventFilterFromParams(searchParams),
  );
  const [applied, setApplied] = React.useState<AlertEventFilter>(() =>
    alertEventFilterFromParams(searchParams),
  );
  const [servers, setServers] = React.useState<AlertEventServer[]>([]);
  const [items, setItems] = React.useState<AlertEvent[]>([]);
  const [nextCursor, setNextCursor] = React.useState<string | null>(null);
  const [hasMore, setHasMore] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const moreController = React.useRef<AbortController | null>(null);
  const pageBounds = React.useRef<AlertEventBounds>({});

  React.useEffect(() => {
    const controller = new AbortController();
    fetchAlertEventServers({ signal: controller.signal })
      .then((res) => {
        if (controller.signal.aborted) return;
        setServers(res.items);
      })
      .catch((error) => {
        if (isCanceledRequestError(error)) return;
        setServers([]);
        apiError(error, { key: 'admin_alerts_events_servers_fetch_failed' });
      });
    return () => {
      controller.abort();
    };
  }, [apiError]);

  React.useEffect(() => {
    const next = alertEventFilterFromParams(searchParams);
    const nextKey = alertEventFilterKey(next);
    setDraft((current) => (alertEventFilterKey(current) === nextKey ? current : next));
    setApplied((current) => (alertEventFilterKey(current) === nextKey ? current : next));
  }, [searchParams]);

  React.useEffect(() => {
    if (!alertEventFilterReady(applied)) {
      moreController.current?.abort();
      pageBounds.current = {};
      setLoading(false);
      setLoadingMore(false);
      setItems([]);
      setNextCursor(null);
      setHasMore(false);
      return;
    }

    const controller = new AbortController();
    const bounds = alertEventRequestBounds(applied);
    pageBounds.current = bounds;
    moreController.current?.abort();
    setLoading(true);
    setLoadingMore(false);
    setItems([]);
    setNextCursor(null);
    setHasMore(false);
    fetchAlertEvents({
      serverId: applied.serverId,
      status: applied.status,
      metric: applied.metric,
      from: bounds.from,
      to: bounds.to,
      limit: eventPageLimit,
      signal: controller.signal,
    })
      .then((res) => {
        if (controller.signal.aborted) return;
        setItems(res.items);
        setNextCursor(res.next_cursor);
        setHasMore(res.has_more);
      })
      .catch((error) => {
        if (isCanceledRequestError(error)) return;
        setNextCursor(null);
        setHasMore(false);
        apiError(error, { key: 'admin_alerts_events_fetch_failed' });
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => {
      controller.abort();
    };
  }, [apiError, applied]);

  React.useEffect(
    () => () => {
      moreController.current?.abort();
    },
    [],
  );

  const apply = React.useCallback(() => {
    setSearchParams(urlParamsForAlertEventFilter(searchParams, draft));
  }, [draft, searchParams, setSearchParams]);

  const reset = React.useCallback(() => {
    setSearchParams(urlParamsForAlertEventFilter(searchParams, defaultAlertEventFilter));
  }, [searchParams, setSearchParams]);

  const loadMore = React.useCallback(() => {
    if (loading || loadingMore || !hasMore || !nextCursor || !alertEventFilterReady(applied)) {
      return;
    }

    moreController.current?.abort();
    const controller = new AbortController();
    moreController.current = controller;
    const bounds = pageBounds.current;
    setLoadingMore(true);
    fetchAlertEvents({
      serverId: applied.serverId,
      status: applied.status,
      metric: applied.metric,
      from: bounds.from,
      to: bounds.to,
      cursor: nextCursor,
      limit: eventPageLimit,
      signal: controller.signal,
    })
      .then((res) => {
        if (controller.signal.aborted) return;
        setItems((current) => {
          const seen = new Set(current.map((item) => item.id));
          const next = [...current];
          for (const item of res.items) {
            if (seen.has(item.id)) continue;
            seen.add(item.id);
            next.push(item);
          }
          return next;
        });
        setNextCursor(res.next_cursor);
        setHasMore(res.has_more);
      })
      .catch((error) => {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_alerts_events_fetch_failed' });
      })
      .finally(() => {
        if (moreController.current === controller) {
          moreController.current = null;
        }
        if (!controller.signal.aborted) {
          setLoadingMore(false);
        }
      });
  }, [apiError, applied, hasMore, loading, loadingMore, nextCursor]);

  const customRangeLabel = alertEventCustomRangeLabel(applied, lang);
  const canSearch = alertEventFilterReady(draft);
  const serverOptions = React.useMemo<ComboboxOption[]>(() => {
    const options: ComboboxOption[] = [{ value: '0', label: t('admin_alerts_events_all_servers') }];
    if (draft.serverId > 0 && !servers.some((server) => server.id === draft.serverId)) {
      options.push({ value: String(draft.serverId), label: `#${draft.serverId}` });
    }
    for (const server of servers) {
      const id = String(server.id);
      options.push({
        value: id,
        label: server.name || `#${id}`,
        keywords: [id, `#${id}`],
      });
    }
    return options;
  }, [draft.serverId, servers, t]);
  const metricOptions = React.useMemo<ComboboxOption[]>(
    () => [
      { value: '', label: t('admin_alerts_events_all_metrics') },
      ...alertMetricValues.map((metric) => ({
        value: metric,
        label: alertMetricName(metric, t),
        keywords: [metric],
      })),
    ],
    [t],
  );
  const statusOptions = React.useMemo<ComboboxOption[]>(
    () => [
      { value: 'open', label: t('admin_alerts_events_status_open') },
      { value: 'closed', label: t('admin_alerts_events_status_closed') },
      { value: 'all', label: t('admin_alerts_events_status_all') },
    ],
    [t],
  );
  const rangeOptions = React.useMemo<ComboboxOption[]>(
    () => [
      { value: '24h', label: t('admin_alerts_events_range_24h') },
      { value: '7d', label: t('admin_alerts_events_range_7d') },
      { value: '30d', label: t('admin_alerts_events_range_30d') },
      { value: 'all', label: t('admin_alerts_events_range_all') },
      { value: 'custom', label: t('admin_alerts_events_range_custom') },
    ],
    [t],
  );

  return (
    <div className="space-y-4 md:space-y-6">
      <Card className="p-3 md:p-4">
        <div className="flex flex-col gap-3">
          <div className="grid gap-2 md:grid-cols-[minmax(12rem,1.1fr)_9rem_minmax(11rem,1fr)_9rem_auto_auto] md:items-end">
            <label className="grid gap-1 text-xs font-semibold text-(--theme-fg-muted)">
              {t('admin_alerts_events_filter_server')}
              <ComboboxSelect
                value={String(draft.serverId)}
                options={serverOptions}
                ariaLabel={t('admin_alerts_events_filter_server')}
                clearLabel={t('common_clear')}
                emptyLabel={t('admin_alerts_events_server_filter_empty')}
                onChange={(value) =>
                  setDraft((current) => ({
                    ...current,
                    serverId: Number.parseInt(value, 10) || 0,
                  }))
                }
              />
            </label>
            <label className="grid gap-1 text-xs font-semibold text-(--theme-fg-muted)">
              {t('admin_alerts_events_filter_status')}
              <ComboboxSelect
                value={draft.status}
                options={statusOptions}
                ariaLabel={t('admin_alerts_events_filter_status')}
                clearLabel={t('common_clear')}
                emptyLabel={t('common_unknown')}
                searchable={false}
                onChange={(value) =>
                  setDraft((current) => ({
                    ...current,
                    status: value as AlertEventStatusFilter,
                  }))
                }
              />
            </label>
            <label className="grid gap-1 text-xs font-semibold text-(--theme-fg-muted)">
              {t('admin_alerts_events_filter_metric')}
              <ComboboxSelect
                value={draft.metric}
                options={metricOptions}
                ariaLabel={t('admin_alerts_events_filter_metric')}
                clearLabel={t('common_clear')}
                emptyLabel={t('admin_alerts_events_metric_filter_empty')}
                onChange={(value) => setDraft((current) => ({ ...current, metric: value }))}
              />
            </label>
            <label className="grid gap-1 text-xs font-semibold text-(--theme-fg-muted)">
              {t('admin_alerts_events_filter_range')}
              <ComboboxSelect
                value={draft.range}
                options={rangeOptions}
                ariaLabel={t('admin_alerts_events_filter_range')}
                clearLabel={t('common_clear')}
                emptyLabel={t('common_unknown')}
                searchable={false}
                onChange={(value) =>
                  setDraft((current) => ({
                    ...current,
                    range: value as AlertEventRange,
                  }))
                }
              />
            </label>
            <Button
              icon={Search}
              onClick={apply}
              disabled={!canSearch}
              className="w-full md:w-auto"
            >
              {t('admin_alerts_events_search')}
            </Button>
            <Button
              variant="secondary"
              icon={RotateCcw}
              onClick={reset}
              className="w-full md:w-auto"
            >
              {t('admin_alerts_events_reset')}
            </Button>
          </div>

          {draft.range === 'custom' && (
            <div className="grid gap-2 md:grid-cols-2">
              <label className="grid gap-1 text-xs font-semibold text-(--theme-fg-muted)">
                {t('admin_alerts_events_filter_from')}
                <Input
                  type="datetime-local"
                  value={draft.fromLocal}
                  onChange={(event) =>
                    setDraft((current) => ({ ...current, fromLocal: event.target.value }))
                  }
                />
              </label>
              <label className="grid gap-1 text-xs font-semibold text-(--theme-fg-muted)">
                {t('admin_alerts_events_filter_to')}
                <Input
                  type="datetime-local"
                  value={draft.toLocal}
                  onChange={(event) =>
                    setDraft((current) => ({ ...current, toLocal: event.target.value }))
                  }
                />
              </label>
            </div>
          )}
        </div>
      </Card>

      <AlertEventsTable items={items} loading={loading} customRangeLabel={customRangeLabel} />

      {hasMore && (
        <div className="flex justify-center">
          <Button
            variant="secondary"
            icon={ChevronDown}
            onClick={loadMore}
            disabled={loading || loadingMore || !nextCursor}
          >
            {loadingMore
              ? t('admin_alerts_events_loading_more')
              : t('admin_alerts_events_load_more')}
          </Button>
        </div>
      )}

      {!loading && !hasMore && items.length > 0 && (
        <div className="text-center text-xs text-(--theme-fg-muted)">
          {t('admin_alerts_events_all_loaded')}
        </div>
      )}
    </div>
  );
};
