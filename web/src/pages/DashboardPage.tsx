import React from 'react';
import ArrowDown from 'lucide-react/dist/esm/icons/arrow-down';
import ArrowUp from 'lucide-react/dist/esm/icons/arrow-up';
import AlertTriangle from 'lucide-react/dist/esm/icons/alert-triangle';
import CircleCheck from 'lucide-react/dist/esm/icons/circle-check';
import Cpu from 'lucide-react/dist/esm/icons/cpu';
import Filter from 'lucide-react/dist/esm/icons/filter';
import Network from 'lucide-react/dist/esm/icons/network';
import Server from 'lucide-react/dist/esm/icons/server';
import Shield from 'lucide-react/dist/esm/icons/shield';
import { useI18n } from '@i18n';
import { useFrontMetricsPolling } from '@pages/dashboard/useFrontMetricsPolling';
import { fetchFrontGroups } from '@lib/frontApi';
import { fetchStatisticsAccess } from '@lib/statisticsApi';
import type { GroupView } from '@app-types/frontMetrics';
import type { StatisticsAccess } from '@app-types/traffic';
import { useAuth } from '@context/AuthContext';
import { useBootstrapAuth } from '@hooks/useBootstrapAuth';
import { buildServerViewModel, formatBytes } from '@pages/dashboard/viewModel';

import { DraggableFloatingButton } from '@components/ui/DraggableFloatingButton';
import Card from '@components/ui/Card';
import GroupFilter from '@components/dashboard/GroupFilter';
import Header from '@components/dashboard/Header';
import ServerCard from '@components/dashboard/ServerCard';
import { useTheme } from '@context/ThemeContext';

const SUMMARY_CARD_BASE_CLASS =
  'bg-(--theme-bg-default) dark:bg-(--theme-bg-default) border border-(--theme-border-subtle) dark:border-(--theme-border-default) shadow-sm transition-[border-color] duration-200 dark:hover:border-(--theme-fg-accent)/40';

const DashboardPage: React.FC = () => {
  useBootstrapAuth();
  const { nodes, isLoading } = useFrontMetricsPolling();
  const [searchTerm, setSearchTerm] = React.useState('');
  const [groups, setGroups] = React.useState<GroupView[]>([]);
  const [statisticsAccess, setStatisticsAccess] = React.useState<StatisticsAccess>({
    history_guest_access_mode: 'disabled',
    traffic_guest_access_mode: 'disabled',
  });
  const [selectedGroupIds, setSelectedGroupIds] = React.useState<number[]>([]);
  const { t } = useI18n();
  const { isAuthenticated } = useAuth();
  const {
    manifest: { skin: theme },
  } = useTheme();

  React.useEffect(() => {
    const controller = new AbortController();

    Promise.allSettled([
      fetchFrontGroups({ signal: controller.signal }),
      fetchStatisticsAccess({ signal: controller.signal }),
    ]).then(([groupsResult, accessResult]) => {
      if (groupsResult.status === 'fulfilled') {
        setGroups(groupsResult.value);
      } else if (
        !(groupsResult.reason instanceof DOMException && groupsResult.reason.name === 'AbortError')
      ) {
        console.error('Failed to fetch groups', groupsResult.reason);
      }

      if (accessResult.status === 'fulfilled') {
        setStatisticsAccess(accessResult.value);
      } else if (
        !(accessResult.reason instanceof DOMException && accessResult.reason.name === 'AbortError')
      ) {
        console.error('Failed to fetch statistics access', accessResult.reason);
      }
    });

    return () => {
      controller.abort();
    };
  }, []);

  const groupById = React.useMemo(
    () => new Map(groups.map((group) => [group.id, group])),
    [groups],
  );
  const serverViews = React.useMemo(() => nodes.map(buildServerViewModel), [nodes]);

  const summary = React.useMemo(() => {
    const filteredServers: typeof serverViews = [];
    let healthyNodes = 0;
    let cpuTotal = 0;
    let alerts = 0;
    let throughputIn = 0;
    let throughputOut = 0;
    let allowedNodes: Set<string> | null = null;

    if (selectedGroupIds.length > 0) {
      const nextAllowed = new Set<string>();

      selectedGroupIds.forEach((gid) => {
        const group = groupById.get(gid);
        if (group) {
          group.node_ids.forEach((nid) => nextAllowed.add(String(nid)));
        }
      });

      allowedNodes = nextAllowed;
    }

    const tokens = searchTerm.trim().toLowerCase().split(/\s+/).filter(Boolean);

    serverViews.forEach((view) => {
      if (allowedNodes && !allowedNodes.has(view.id)) return;
      if (tokens.length > 0 && !tokens.every((token) => view.searchText.includes(token))) return;

      filteredServers.push(view);
      if (view.isAlive) healthyNodes += 1;
      cpuTotal += view.cpu.usagePercent;
      if (!view.isAlive) alerts += 1;
      if (view.raid.showAlert) alerts += 1;
      if (view.cpu.usagePercent > 85 || view.disk.usedPercent > 85) alerts += 1;
      throughputIn += view.network.rateIn;
      throughputOut += view.network.rateOut;
    });

    const totalNodes = filteredServers.length;

    return {
      alerts,
      avgCpu: totalNodes === 0 ? 0 : Math.round(cpuTotal / totalNodes),
      filteredServers,
      healthyNodes,
      throughputIn,
      throughputOut,
      totalNodes,
    };
  }, [groupById, serverViews, searchTerm, selectedGroupIds]);

  const { alerts, avgCpu, filteredServers, healthyNodes, throughputIn, throughputOut, totalNodes } =
    summary;
  const canOpenHistory =
    isAuthenticated || statisticsAccess.history_guest_access_mode === 'by_node';
  const canOpenTraffic =
    isAuthenticated || statisticsAccess.traffic_guest_access_mode === 'by_node';
  const compact = theme.dashboard.density === 'compact';
  const summaryStrip = theme.dashboard.summary === 'strip';
  const summaryCardClass = `${SUMMARY_CARD_BASE_CLASS} ${compact ? 'rounded-2xl p-3.5' : 'rounded-xl p-4'}`;
  const summaryGridClass = compact
    ? 'mb-3 grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-5'
    : 'mb-4 grid grid-cols-2 gap-4 md:grid-cols-4 xl:grid-cols-5';
  const serverGridClass = compact
    ? 'grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'
    : 'grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4';
  const summaryValueClass = compact
    ? 'text-xl font-bold font-mono'
    : 'text-2xl font-bold font-mono';
  const summaryThroughputClass = compact
    ? 'grid gap-0.5 text-sm font-semibold font-mono leading-tight'
    : 'grid gap-1 text-sm font-semibold font-mono leading-tight';
  const summaryIconClass = compact ? 'rounded-2xl p-2.5' : 'rounded-full p-3';
  const totalNodesLabel =
    selectedGroupIds.length === 0
      ? t('total_nodes')
      : selectedGroupIds.length === 1
        ? (groupById.get(selectedGroupIds[0])?.name ?? t('total_nodes'))
        : t('admin_nodes_filter');
  const throughputRows = (
    <span className={summaryThroughputClass}>
      <span className="inline-flex items-center gap-1.5" title={t('recv')} aria-label={t('recv')}>
        <ArrowDown size={12} className="text-(--theme-fg-success-muted)" aria-hidden="true" />
        {formatBytes(throughputIn)}/s
      </span>
      <span className="inline-flex items-center gap-1.5" title={t('sent')} aria-label={t('sent')}>
        <ArrowUp size={12} className="text-(--theme-fg-warning-muted)" aria-hidden="true" />
        {formatBytes(throughputOut)}/s
      </span>
    </span>
  );

  const desktopGroupFilter =
    groups.length > 0 ? (
      <GroupFilter
        groups={groups}
        selectedIds={selectedGroupIds}
        onChange={setSelectedGroupIds}
        align="right"
        customTrigger={
          <button
            className={`relative rounded-xl p-2 text-sm font-medium transition-colors ${
              selectedGroupIds.length > 0
                ? 'bg-(--theme-bg-accent-muted) text-(--theme-fg-accent)'
                : 'text-(--theme-fg-muted) hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-default) dark:border-(--theme-border-default)'
            }`}
          >
            <span className="inline-flex items-center gap-2">
              <Filter size={16} />
            </span>
            {selectedGroupIds.length > 0 && (
              <span className="absolute -right-1.5 -top-1.5 inline-flex min-h-5 min-w-5 items-center justify-center rounded-full bg-(--theme-bg-danger-emphasis) px-1 text-[10px] font-bold text-(--theme-fg-on-emphasis)">
                {selectedGroupIds.length}
              </span>
            )}
          </button>
        }
      />
    ) : null;

  const summaryItems = [
    {
      key: 'total',
      icon: Server,
      label: totalNodesLabel,
      value: totalNodes,
      iconClass: 'bg-(--theme-bg-accent-muted) text-(--theme-fg-accent)',
    },
    {
      key: 'healthy',
      icon: CircleCheck,
      label: t('healthy'),
      value: healthyNodes,
      iconClass: 'bg-(--theme-bg-success-emphasis)/10 text-(--theme-fg-success)',
    },
    {
      key: 'alerts',
      icon: AlertTriangle,
      label: t('alerts'),
      value: alerts,
      iconClass: 'bg-(--theme-bg-danger-emphasis)/10 text-(--theme-bg-danger-emphasis)',
    },
    {
      key: 'cpu',
      icon: Cpu,
      label: t('avg_cpu'),
      value: `${avgCpu}%`,
      iconClass: 'bg-(--theme-bg-muted) text-(--theme-fg-default)',
    },
    {
      key: 'throughput',
      icon: Network,
      label: t('dashboard_network_throughput'),
      value: throughputRows,
      iconClass: 'bg-(--theme-bg-muted) text-(--theme-fg-default)',
    },
  ];

  return (
    <>
      <div className="min-h-screen bg-(--theme-bg-muted) dark:bg-(--theme-bg-default) text-(--theme-fg-default) dark:text-(--theme-fg-strong) font-sans">
        <Header searchTerm={searchTerm} setSearchTerm={setSearchTerm} />

        <main
          className={`mx-auto max-w-410 px-4 sm:px-6 lg:px-8 ${compact ? 'py-5 lg:py-6' : 'py-8'}`}
        >
          {summaryStrip ? (
            <Card variant="summaryStrip" className="mb-2">
              <div
                className={`grid gap-px bg-(--theme-border-divider) dark:bg-(--theme-border-default) ${
                  desktopGroupFilter
                    ? 'md:grid-cols-[repeat(5,minmax(0,1fr))_auto]'
                    : 'md:grid-cols-4 xl:grid-cols-5'
                }`}
              >
                {summaryItems.map((item) => (
                  <div
                    key={item.key}
                    className="flex items-center gap-3 bg-(--theme-bg-default) px-4 py-3.5 dark:bg-(--theme-bg-default)"
                  >
                    <div className={`rounded-xl p-2.5 ${item.iconClass}`}>
                      <item.icon size={18} />
                    </div>
                    <div className="min-w-0">
                      <p className="truncate text-[11px] font-semibold uppercase tracking-[0.14em] text-(--theme-fg-subtle)">
                        {item.label}
                      </p>
                      <p className="mt-1 text-xl font-semibold tracking-tight text-(--theme-fg-default)">
                        {item.value}
                      </p>
                    </div>
                  </div>
                ))}

                {desktopGroupFilter && (
                  <div className="hidden items-center justify-end bg-(--theme-bg-default) px-4 md:flex">
                    {desktopGroupFilter}
                  </div>
                )}
              </div>
            </Card>
          ) : (
            <div className={summaryGridClass}>
              <div
                className={`${summaryCardClass} relative flex items-center justify-between overflow-visible`}
              >
                <div className={`flex items-center ${compact ? 'gap-3' : 'gap-4'}`}>
                  <div
                    className={`${summaryIconClass} bg-(--theme-bg-accent-muted) text-(--theme-fg-accent)`}
                  >
                    <Server size={compact ? 20 : 24} />
                  </div>
                  <div>
                    <p className="text-xs font-medium uppercase text-(--theme-fg-muted)">
                      {totalNodesLabel}
                    </p>
                    <p className={summaryValueClass}>{totalNodes}</p>
                  </div>
                </div>
                {desktopGroupFilter && <div className="hidden md:block">{desktopGroupFilter}</div>}
              </div>

              <div
                className={`${summaryCardClass} flex items-center ${compact ? 'gap-3' : 'gap-4'}`}
              >
                <div
                  className={`${summaryIconClass} bg-(--theme-bg-success-emphasis)/10 text-(--theme-fg-success)`}
                >
                  <CircleCheck size={compact ? 20 : 24} />
                </div>
                <div>
                  <p className="text-xs font-medium uppercase text-(--theme-fg-muted)">
                    {t('healthy')}
                  </p>
                  <p className={summaryValueClass}>{healthyNodes}</p>
                </div>
              </div>

              <div
                className={`${summaryCardClass} flex items-center ${compact ? 'gap-3' : 'gap-4'}`}
              >
                <div
                  className={`${summaryIconClass} bg-(--theme-bg-danger-emphasis)/10 text-(--theme-bg-danger-emphasis)`}
                >
                  <AlertTriangle size={compact ? 20 : 24} />
                </div>
                <div>
                  <p className="text-xs font-medium uppercase text-(--theme-fg-muted)">
                    {t('alerts')}
                  </p>
                  <p className={summaryValueClass}>{alerts}</p>
                </div>
              </div>

              <div
                className={`${summaryCardClass} flex items-center ${compact ? 'gap-3' : 'gap-4'}`}
              >
                <div
                  className={`${summaryIconClass} bg-(--theme-bg-muted) text-(--theme-fg-default)`}
                >
                  <Cpu size={compact ? 20 : 24} />
                </div>
                <div>
                  <p className="text-xs font-medium uppercase text-(--theme-fg-muted)">
                    {t('avg_cpu')}
                  </p>
                  <p className={summaryValueClass}>{avgCpu}%</p>
                </div>
              </div>

              <div
                className={`${summaryCardClass} flex items-center ${compact ? 'gap-3' : 'gap-4'}`}
              >
                <div
                  className={`${summaryIconClass} bg-(--theme-bg-muted) text-(--theme-fg-default)`}
                >
                  <Network size={compact ? 20 : 24} />
                </div>
                <div>
                  <p className="text-xs font-medium uppercase text-(--theme-fg-muted)">
                    {t('dashboard_network_throughput')}
                  </p>
                  <div className="mt-1">{throughputRows}</div>
                </div>
              </div>
            </div>
          )}

          {isLoading && (
            <div className="py-6 text-center text-sm text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
              {t('dashboard_loading_metrics')}
            </div>
          )}

          <div className={serverGridClass}>
            {filteredServers.map((view) => (
              <ServerCard
                key={view.id}
                view={view}
                canOpenHistory={canOpenHistory}
                canOpenTraffic={canOpenTraffic}
              />
            ))}

            {filteredServers.length === 0 && !isLoading && (
              <div className="col-span-full py-20 text-center text-(--theme-fg-subtle)">
                <Shield size={48} className="mx-auto mb-4 opacity-50" />
                <p>{t('no_servers')}</p>
              </div>
            )}
          </div>
        </main>

        {groups.length > 0 && (
          <div className="md:hidden">
            <DraggableFloatingButton
              storageKey="dashboard_filter_fab_pos"
              ariaLabel={t('admin_nodes_filter')}
            >
              <GroupFilter
                groups={groups}
                selectedIds={selectedGroupIds}
                onChange={setSelectedGroupIds}
                align="right"
                direction="up"
                variant="fab"
                customTrigger={
                  <div className="flex size-12 items-center justify-center rounded-full bg-(--theme-bg-accent-emphasis) text-(--theme-fg-on-emphasis) shadow-lg transition-colors hover:bg-(--theme-fg-accent)">
                    <Filter size={20} />
                    {selectedGroupIds.length > 0 && (
                      <span className="absolute -right-1 -top-1 flex size-5 items-center justify-center rounded-full bg-(--theme-bg-danger-emphasis) text-[10px] font-bold text-(--theme-fg-on-emphasis)">
                        {selectedGroupIds.length}
                      </span>
                    )}
                  </div>
                }
              />
            </DraggableFloatingButton>
          </div>
        )}
      </div>
    </>
  );
};

export default DashboardPage;
