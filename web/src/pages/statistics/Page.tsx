import React from 'react';
import { useParams } from 'react-router-dom';
import Card from '@components/ui/Card';
import { useI18n } from '@i18n';
import { useBootstrapAuth } from '@hooks/useBootstrapAuth';
import { ratioToPercent } from '@pages/dashboard/viewModel';
import { buildMetricSections } from './config';
import { useStatisticsDetail } from './hooks/useStatisticsDetail';
import { useStatisticsOverview } from './hooks/useStatisticsOverview';
import {
  StatisticsDetailModal,
  StatisticsHeader,
  StatisticsMetricSection,
  StatisticsOverviewPanel,
} from './PageParts';

const Page = () => {
  useBootstrapAuth();
  const { t } = useI18n();
  const { serverId } = useParams();

  const numericServerId = serverId ? Number(serverId) : Number.NaN;
  const isValidServerId = Number.isFinite(numericServerId) && numericServerId > 0;

  const percentTransform = React.useCallback((value: number | null) => {
    if (value == null || !Number.isFinite(value)) return null;
    return ratioToPercent(value);
  }, []);

  const metricSections = React.useMemo(
    () => buildMetricSections(percentTransform),
    [percentTransform],
  );

  const overview = useStatisticsOverview({
    numericServerId,
    isValidServerId,
    metricSections,
  });

  const detail = useStatisticsDetail({
    numericServerId,
    isValidServerId,
    range: overview.range,
    ioDevice: overview.ioDevice,
    mountDevice: overview.mountDevice,
  });

  const serverLabel =
    overview.nodeView?.node.title || (isValidServerId ? `#${numericServerId}` : '');

  if (!isValidServerId) {
    return (
      <div className="min-h-screen bg-(--theme-page-bg) dark:bg-(--theme-bg-default) text-(--theme-fg-default) dark:text-(--theme-fg-strong)">
        <main className="max-w-410 mx-auto px-4 sm:px-6 lg:px-8 py-12">
          <Card variant="panel" className="p-6">
            <p className="text-sm text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
              {t('stats_no_server')}
            </p>
          </Card>
        </main>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-(--theme-page-bg) dark:bg-(--theme-bg-default) text-(--theme-fg-default) dark:text-(--theme-fg-strong)">
      <StatisticsHeader serverLabel={serverLabel} />

      <main className="max-w-410 mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
        <StatisticsOverviewPanel serverLabel={serverLabel} overview={overview} />

        <Card as="section" variant="panel" className="overflow-hidden">
          {overview.metricSections.map((section, sectionIndex) => (
            <StatisticsMetricSection
              key={section.key}
              section={section}
              sectionIndex={sectionIndex}
              overview={overview}
              onOpenDetail={detail.open}
            />
          ))}
        </Card>
      </main>

      <StatisticsDetailModal detail={detail} />
    </div>
  );
};

export default Page;
