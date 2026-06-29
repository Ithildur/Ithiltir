import React from 'react';
import RotateCcw from 'lucide-react/dist/esm/icons/rotate-ccw';
import Search from 'lucide-react/dist/esm/icons/search';
import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import Input from '@components/ui/Input';
import Select from '@components/ui/Select';
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

export const AlertEventsPanel: React.FC<Props> = ({
  searchParams,
  setSearchParams,
}) => {
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
  const [loading, setLoading] = React.useState(false);

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
      setLoading(false);
      setItems([]);
      return;
    }

    const controller = new AbortController();
    const bounds = alertEventRequestBounds(applied);
    setLoading(true);
    setItems([]);
    fetchAlertEvents({
      serverId: applied.serverId,
      status: applied.status,
      metric: applied.metric,
      from: bounds.from,
      to: bounds.to,
      limit: 200,
      signal: controller.signal,
    })
      .then((res) => {
        if (controller.signal.aborted) return;
        setItems(res.items);
      })
      .catch((error) => {
        if (isCanceledRequestError(error)) return;
        apiError(error, { key: 'admin_alerts_events_fetch_failed' });
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => {
      controller.abort();
    };
  }, [apiError, applied]);

  const apply = React.useCallback(() => {
    setSearchParams(urlParamsForAlertEventFilter(searchParams, draft));
  }, [draft, searchParams, setSearchParams]);

  const reset = React.useCallback(() => {
    setSearchParams(urlParamsForAlertEventFilter(searchParams, defaultAlertEventFilter));
  }, [searchParams, setSearchParams]);

  const customRangeLabel = alertEventCustomRangeLabel(applied, lang);
  const canSearch = alertEventFilterReady(draft);
  const hasSelectedServer = servers.some((server) => server.id === draft.serverId);

  return (
    <div className="space-y-4 md:space-y-6">
      <Card className="p-3 md:p-4">
        <div className="flex flex-col gap-3">
          <div className="grid gap-2 md:grid-cols-[minmax(12rem,1.1fr)_9rem_minmax(11rem,1fr)_9rem_auto_auto] md:items-end">
            <label className="grid gap-1 text-xs font-semibold text-(--theme-fg-muted)">
              {t('admin_alerts_events_filter_server')}
              <Select
                value={draft.serverId}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    serverId: Number.parseInt(event.target.value, 10) || 0,
                  }))
                }
              >
                <option value={0}>{t('admin_alerts_events_all_servers')}</option>
                {draft.serverId > 0 && !hasSelectedServer && (
                  <option value={draft.serverId}>{`#${draft.serverId}`}</option>
                )}
                {servers.map((server) => (
                  <option key={server.id} value={server.id}>
                    {server.name}
                  </option>
                ))}
              </Select>
            </label>
            <label className="grid gap-1 text-xs font-semibold text-(--theme-fg-muted)">
              {t('admin_alerts_events_filter_status')}
              <Select
                value={draft.status}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    status: event.target.value as AlertEventStatusFilter,
                  }))
                }
              >
                <option value="open">{t('admin_alerts_events_status_open')}</option>
                <option value="closed">{t('admin_alerts_events_status_closed')}</option>
                <option value="all">{t('admin_alerts_events_status_all')}</option>
              </Select>
            </label>
            <label className="grid gap-1 text-xs font-semibold text-(--theme-fg-muted)">
              {t('admin_alerts_events_filter_metric')}
              <Select
                value={draft.metric}
                onChange={(event) =>
                  setDraft((current) => ({ ...current, metric: event.target.value }))
                }
              >
                <option value="">{t('admin_alerts_events_all_metrics')}</option>
                {alertMetricValues.map((metric) => (
                  <option key={metric} value={metric}>
                    {alertMetricName(metric, t)}
                  </option>
                ))}
              </Select>
            </label>
            <label className="grid gap-1 text-xs font-semibold text-(--theme-fg-muted)">
              {t('admin_alerts_events_filter_range')}
              <Select
                value={draft.range}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    range: event.target.value as AlertEventRange,
                  }))
                }
              >
                <option value="24h">{t('admin_alerts_events_range_24h')}</option>
                <option value="7d">{t('admin_alerts_events_range_7d')}</option>
                <option value="30d">{t('admin_alerts_events_range_30d')}</option>
                <option value="all">{t('admin_alerts_events_range_all')}</option>
                <option value="custom">{t('admin_alerts_events_range_custom')}</option>
              </Select>
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

      <AlertEventsTable
        items={items}
        loading={loading}
        customRangeLabel={customRangeLabel}
      />
    </div>
  );
};
