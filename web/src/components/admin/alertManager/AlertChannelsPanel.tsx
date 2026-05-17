import React from 'react';
import Edit2 from 'lucide-react/dist/esm/icons/edit-2';
import FlaskConical from 'lucide-react/dist/esm/icons/flask-conical';
import Mail from 'lucide-react/dist/esm/icons/mail';
import Search from 'lucide-react/dist/esm/icons/search';
import Send from 'lucide-react/dist/esm/icons/send';
import Trash2 from 'lucide-react/dist/esm/icons/trash-2';
import Webhook from 'lucide-react/dist/esm/icons/webhook';
import type { LucideIcon } from 'lucide-react';
import Badge from '@components/ui/Badge';
import Card from '@components/ui/Card';
import Input from '@components/ui/Input';
import IOSSwitch from '@components/ui/IOSSwitch';
import type { AlertChannel, AlertChannelType } from '@app-types/admin';
import { useI18n } from '@i18n';
import { formatTimeAgo } from '@utils/time';

type FilterKey = 'all' | 'active' | 'paused';

interface Props {
  channels: AlertChannel[];
  loading: boolean;
  togglingId: number | null;
  testingId: number | null;
  onToggleEnabled: (channel: AlertChannel) => void;
  onEdit: (channel: AlertChannel) => void;
  onDelete: (channel: AlertChannel) => void;
  onTest: (channel: AlertChannel) => void;
}

const AlertChannelsPanel: React.FC<Props> = ({
  channels,
  loading,
  togglingId,
  testingId,
  onToggleEnabled,
  onEdit,
  onDelete,
  onTest,
}) => {
  const { t, lang } = useI18n();
  const [search, setSearch] = React.useState('');
  const [activeFilter, setActiveFilter] = React.useState<FilterKey>('all');

  const channelTypeMeta: Record<
    AlertChannelType,
    { label: string; color: 'indigo' | 'amber' | 'slate'; icon: LucideIcon }
  > = {
    telegram: { label: t('admin_alerts_channels_tab_telegram'), color: 'indigo', icon: Send },
    email: { label: t('admin_alerts_channels_tab_email'), color: 'amber', icon: Mail },
    webhook: { label: t('admin_alerts_channels_tab_webhook'), color: 'slate', icon: Webhook },
  };

  const normalizeSearch = (value: string) => value.trim().toLowerCase();

  const formatSummary = React.useCallback(
    (channel: AlertChannel): string => {
      if (channel.type === 'telegram') {
        const config = channel.config as { mode?: string; chat_id?: string; phone?: string };
        if (config.mode === 'mtproto') {
          return t('admin_alerts_channels_summary_mtproto', {
            phone: config.phone ?? '-'.toString(),
            chat: config.chat_id ?? '-'.toString(),
          });
        }
        return t('admin_alerts_channels_summary_bot', {
          chat: config.chat_id ?? '-'.toString(),
        });
      }
      if (channel.type === 'email') {
        const config = channel.config as { from?: string; to?: string[] };
        return t('admin_alerts_channels_summary_email', {
          from: config.from ?? '-',
          count: String(config.to?.length ?? 0),
        });
      }
      const config = channel.config as { url?: string };
      return t('admin_alerts_channels_summary_webhook', { url: config.url ?? '-' });
    },
    [t],
  );

  const filteredChannels = React.useMemo(() => {
    const keyword = normalizeSearch(search);
    return channels.filter((channel) => {
      if (activeFilter === 'active' && !channel.enabled) return false;
      if (activeFilter === 'paused' && channel.enabled) return false;
      if (!keyword) return true;
      const summary = formatSummary(channel);
      const haystack = [channel.name, channel.type, summary].join(' ').toLowerCase();
      return haystack.includes(keyword);
    });
  }, [activeFilter, channels, formatSummary, search]);

  const filters: Array<{ key: FilterKey; label: string }> = [
    { key: 'all', label: t('admin_alerts_channels_filter_all') },
    { key: 'active', label: t('admin_alerts_channels_filter_active') },
    { key: 'paused', label: t('admin_alerts_channels_filter_paused') },
  ];

  return (
    <div className="space-y-4 md:space-y-6">
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-3">
        <Input
          icon={Search}
          placeholder={t('admin_alerts_channels_search_placeholder')}
          aria-label={t('admin_alerts_channels_search_placeholder')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          data-search-input="true"
          wrapperClassName="w-full lg:max-w-md"
        />
        <div className="inline-flex rounded-lg border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-control) dark:bg-(--theme-bg-default) p-1">
          {filters.map((filter) => {
            const isActive = filter.key === activeFilter;
            return (
              <button
                key={filter.key}
                type="button"
                aria-pressed={isActive}
                onClick={() => setActiveFilter(filter.key)}
                className={`px-3 py-1 text-xs font-semibold rounded-md transition-colors ${
                  isActive
                    ? 'bg-(--theme-bg-inverse) text-(--theme-fg-inverse) shadow-sm'
                    : 'text-(--theme-fg-muted) hover:text-(--theme-fg-strong) dark:text-(--theme-fg-neutral) dark:hover:text-(--theme-fg-strong) hover:bg-(--theme-surface-control-hover) dark:hover:bg-(--theme-canvas-subtle)'
                }`}
              >
                {filter.label}
              </button>
            );
          })}
        </div>
      </div>

      <Card className="overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
            <thead className="bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-default) text-xs font-semibold border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
              <tr>
                <th className="px-4 py-3">{t('admin_alerts_channels_col_name')}</th>
                <th className="px-4 py-3">{t('admin_alerts_channels_col_type')}</th>
                <th className="px-4 py-3">{t('admin_alerts_channels_col_summary')}</th>
                <th className="px-4 py-3">{t('admin_alerts_channels_col_status')}</th>
                <th className="px-4 py-3">{t('admin_alerts_channels_col_updated')}</th>
                <th className="px-4 py-3 text-right">{t('common_actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
              {loading && filteredChannels.length === 0 ? (
                <tr>
                  <td
                    colSpan={6}
                    className="px-4 py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)"
                  >
                    {t('loading')}
                  </td>
                </tr>
              ) : filteredChannels.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12">
                    <div className="flex flex-col items-center text-center gap-2">
                      <div className="size-12 rounded-full bg-(--theme-bg-interactive-muted) dark:bg-(--theme-bg-interactive-soft) flex items-center justify-center text-(--theme-fg-interactive-strong) dark:text-(--theme-fg-interactive-hover)">
                        <Send className="size-5" aria-hidden="true" />
                      </div>
                      <p className="text-sm font-semibold text-(--theme-fg-strong) dark:text-(--theme-fg-strong)">
                        {t('admin_alerts_channels_empty_title')}
                      </p>
                      <p className="text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) max-w-md">
                        {t('admin_alerts_channels_empty_description')}
                      </p>
                    </div>
                  </td>
                </tr>
              ) : (
                filteredChannels.map((channel) => {
                  const meta = channelTypeMeta[channel.type];
                  const summary = formatSummary(channel);
                  return (
                    <tr
                      key={channel.id}
                      className="hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle) transition-colors"
                    >
                      <td className="px-4 py-3">
                        <div className="flex flex-col">
                          <span className="font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-default)">
                            {channel.name}
                          </span>
                          <span className="text-[11px] text-(--theme-fg-muted-alt) font-mono">
                            ID: {channel.id}
                          </span>
                        </div>
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <meta.icon
                            className="size-4 text-(--theme-fg-muted-alt)"
                            aria-hidden="true"
                          />
                          <Badge color={meta.color}>{meta.label}</Badge>
                        </div>
                      </td>
                      <td className="px-4 py-3 text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-muted) font-mono">
                        {summary}
                      </td>
                      <td className="px-4 py-3">
                        <IOSSwitch
                          size="sm"
                          checked={channel.enabled}
                          disabled={togglingId === channel.id}
                          onChange={() => onToggleEnabled(channel)}
                        />
                      </td>
                      <td className="px-4 py-3 text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-muted) font-mono">
                        {formatTimeAgo(channel.updated_at, lang)}
                      </td>
                      <td className="px-4 py-3 text-right">
                        <div className="flex items-center justify-end gap-1">
                          <button
                            type="button"
                            onClick={() => onEdit(channel)}
                            className="p-1.5 text-(--theme-fg-subtle) hover:text-(--theme-fg-interactive) dark:hover:text-(--theme-fg-interactive-hover) hover:bg-(--theme-bg-interactive-hover) dark:hover:bg-(--theme-bg-interactive-hover) rounded transition-colors"
                            aria-label={t('common_edit')}
                          >
                            <Edit2 className="size-4" />
                          </button>
                          <button
                            type="button"
                            onClick={() => onTest(channel)}
                            className="p-1.5 text-(--theme-fg-subtle) hover:text-(--theme-fg-interactive) dark:hover:text-(--theme-fg-interactive-hover) hover:bg-(--theme-bg-interactive-hover) dark:hover:bg-(--theme-bg-interactive-hover) rounded transition-colors"
                            aria-label={t('admin_alerts_channels_action_test')}
                            title={t('admin_alerts_channels_action_test')}
                            disabled={testingId === channel.id}
                          >
                            <FlaskConical className="size-4" />
                          </button>
                          <button
                            type="button"
                            onClick={() => onDelete(channel)}
                            className="p-1.5 text-(--theme-fg-danger-muted) hover:text-(--theme-fg-danger) dark:text-(--theme-fg-danger) dark:hover:text-(--theme-fg-danger-soft) hover:bg-(--theme-bg-danger-muted) dark:hover:bg-(--theme-bg-danger-subtle) rounded transition-colors"
                            aria-label={t('common_delete')}
                          >
                            <Trash2 className="size-4" />
                          </button>
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
    </div>
  );
};

export default AlertChannelsPanel;
