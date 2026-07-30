import React from 'react';
import { useParams } from 'react-router';
import {
  trafficCoverageWarningThreshold,
  trafficDirectionLabelKey,
} from '@lib/trafficSettingsModel';
import { useConfirmDialog } from '@hooks/useConfirmDialog';
import { useTrafficRebuild } from '@hooks/useTrafficRebuild';
import { useTrafficRebuildBanner } from '@hooks/useTrafficRebuildBanner';
import { useAuthStore } from '@stores/authStore';
import { TRAFFIC_MONTHLY_HISTORY_MONTHS } from '@config/traffic';
import { useI18n } from '@i18n';
import {
  buildTrafficCurrentStatGroups,
  buildTrafficHeroStats,
  coverageStatsFrom,
  dailyPointFrom,
  localeFor,
  monthlyPointFrom,
  type TrafficChartMode,
} from './pageModel';
import { useTrafficData } from './useTrafficData';

export const useTrafficPage = () => {
  const { t, lang } = useI18n();
  const token = useAuthStore((state) => state.accessToken);
  const { dialogProps: confirmDialogProps, request: requestConfirm } = useConfirmDialog();
  const { serverId } = useParams();
  const numericServerId = serverId ? Number(serverId) : Number.NaN;
  const isValidServerId = Number.isFinite(numericServerId) && numericServerId > 0;
  const locale = localeFor(lang);
  const {
    ifaces,
    iface,
    setIface,
    summary,
    currentDaily,
    previousDaily,
    monthly,
    loading,
    errorKey,
    loadTraffic,
  } = useTrafficData({
    serverId: numericServerId,
    isValidServerId,
  });
  const [preferredChartMode, setChartMode] = React.useState<TrafficChartMode>('current_daily');
  const {
    busy: trafficRebuildBusy,
    nodeRebuildActive: nodeTrafficRebuildActive,
    startBusy: trafficRebuildStartBusy,
    finishedKey: trafficRebuildFinishedKey,
    start: startTrafficRebuild,
  } = useTrafficRebuild({
    nodeId: isValidServerId ? numericServerId : null,
  });
  const showTrafficRebuildOutcome = useTrafficRebuildBanner();

  React.useEffect(() => {
    if (!trafficRebuildFinishedKey) return;
    const controller = new AbortController();
    void loadTraffic(controller.signal);
    return () => controller.abort();
  }, [loadTraffic, trafficRebuildFinishedKey]);

  const serverLabel =
    summary?.server_name?.trim() || (isValidServerId ? `#${numericServerId}` : '');
  const showP95 = Boolean(summary?.stats.p95_enabled);
  const showCoverage = summary?.usage_mode === 'billing';
  const summaryCoverageWarning = Boolean(
    summary && summary.stats.coverage_ratio < trafficCoverageWarningThreshold,
  );
  const rebuildNodeTraffic = React.useCallback(async () => {
    if (!token || summary?.usage_mode !== 'billing' || !isValidServerId || trafficRebuildBusy)
      return;

    const ok = await requestConfirm({
      title: t('common_confirm'),
      message: t('admin_confirm_rebuild_node_traffic', { name: serverLabel }),
      confirmLabel: t('admin_node_traffic_rebuild'),
      cancelLabel: t('common_cancel'),
      tone: 'default',
    });
    if (!ok) return;

    const outcome = await startTrafficRebuild(numericServerId);
    showTrafficRebuildOutcome(outcome);
  }, [
    isValidServerId,
    numericServerId,
    requestConfirm,
    serverLabel,
    showTrafficRebuildOutcome,
    startTrafficRebuild,
    summary?.usage_mode,
    t,
    token,
    trafficRebuildBusy,
  ]);
  const directionLabel = summary ? t(trafficDirectionLabelKey[summary.direction_mode]) : '';

  const currentDailyPoints = React.useMemo(() => {
    const timezone = summary?.cycle.timezone || '';
    return currentDaily.items.map((item) => dailyPointFrom(item, locale, timezone));
  }, [currentDaily.items, locale, summary?.cycle.timezone]);
  const previousDailyPoints = React.useMemo(() => {
    const timezone = summary?.cycle.timezone || '';
    return previousDaily.items.map((item) => dailyPointFrom(item, locale, timezone));
  }, [locale, previousDaily.items, summary?.cycle.timezone]);
  const monthlyPoints = React.useMemo(
    () =>
      monthly.items
        .slice(0, TRAFFIC_MONTHLY_HISTORY_MONTHS)
        .reverse()
        .map((item) => monthlyPointFrom(item, locale)),
    [locale, monthly.items],
  );
  const dailyAvailable = summary?.usage_mode === 'billing';
  const chartMode =
    summary && !dailyAvailable && preferredChartMode !== 'monthly' ? 'monthly' : preferredChartMode;
  const chartPoints =
    chartMode === 'current_daily'
      ? currentDailyPoints
      : chartMode === 'previous_daily'
        ? previousDailyPoints
        : monthlyPoints;
  const chartCoverageStats = React.useMemo(() => {
    if (!showCoverage) return null;
    if (chartMode === 'current_daily') {
      return coverageStatsFrom(currentDaily.items.map((item) => item.stats));
    }
    if (chartMode === 'previous_daily') {
      return coverageStatsFrom(previousDaily.items.map((item) => item.stats));
    }
    return coverageStatsFrom(
      monthly.items.slice(0, TRAFFIC_MONTHLY_HISTORY_MONTHS).map((item) => item.stats),
    );
  }, [chartMode, currentDaily.items, monthly.items, previousDaily.items, showCoverage]);
  const chartCoverageWarning = Boolean(
    chartCoverageStats && chartCoverageStats.coverageRatio < trafficCoverageWarningThreshold,
  );

  const statItems = React.useMemo(
    () =>
      buildTrafficHeroStats({
        summary,
        showCoverage,
        coverageWarning: summaryCoverageWarning,
        t,
      }),
    [showCoverage, summary, summaryCoverageWarning, t],
  );

  const currentStatGroups = React.useMemo(
    () => buildTrafficCurrentStatGroups(summary, t),
    [summary, t],
  );

  return {
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
  };
};
