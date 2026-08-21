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
import IOSSwitch from '@components/ui/IOSSwitch';
import SearchInput from '@components/ui/SearchInput';
import type {
  AlertChannel,
  AlertChannelDeliveryStatus,
  AlertChannelType,
  AlertSettings,
  ValidAlertChannel,
} from '@app-types/admin';
import { useI18n } from '@i18n';
import { formatLocalDateTime, formatLocalTimestamp, formatTimeAgo } from '@utils/time';

interface Props {
  channels: AlertChannel[];
  loading: boolean;
  settings: AlertSettings | null;
  loadingSettings: boolean;
  savingSettings: boolean;
  togglingIds: number[];
  testingIds: number[];
  onToggleSettingsEnabled: () => void;
  onToggleSettingsChannel: (id: number) => void;
  onToggleEnabled: (channel: AlertChannel) => void;
  onEdit: (channel: ValidAlertChannel) => void;
  onDelete: (channel: AlertChannel) => void;
  onTest: (channel: AlertChannel) => void;
}

type AlertChannelFilter = 'all' | 'active' | 'paused';

const AlertChannelsPanel: React.FC<Props> = ({
  channels,
  loading,
  settings,
  loadingSettings,
  savingSettings,
  togglingIds,
  testingIds,
  onToggleSettingsEnabled,
  onToggleSettingsChannel,
  onToggleEnabled,
  onEdit,
  onDelete,
  onTest,
}) => {
  const { t, lang } = useI18n();
  const [search, setSearch] = React.useState('');
  const [activeFilter, setActiveFilter] = React.useState<AlertChannelFilter>('all');

  const channelTypeMeta: Record<
    AlertChannelType,
    { label: string; color: 'indigo' | 'amber' | 'slate'; icon: LucideIcon }
  > = {
    telegram: { label: t('admin_alerts_channels_tab_telegram'), color: 'indigo', icon: Send },
    email: { label: t('admin_alerts_channels_tab_email'), color: 'amber', icon: Mail },
    webhook: { label: t('admin_alerts_channels_tab_webhook'), color: 'slate', icon: Webhook },
  };
  const deliveryMeta: Record<
    AlertChannelDeliveryStatus,
    { label: string; color: 'emerald' | 'amber' | 'slate' | 'indigo' }
  > = {
    healthy: {
      label: t('admin_alerts_channels_delivery_healthy'),
      color: 'emerald',
    },
    degraded: {
      label: t('admin_alerts_channels_delivery_degraded'),
      color: 'amber',
    },
    disabled: {
      label: t('admin_alerts_channels_delivery_disabled'),
      color: 'slate',
    },
    unknown: {
      label: t('admin_alerts_channels_delivery_unknown'),
      color: 'indigo',
    },
  };

  const normalizeSearch = (value: string) => value.trim().toLowerCase();

  const formatSummary = React.useCallback(
    (channel: AlertChannel): string => {
      if (channel.config === null) {
        return t('admin_alerts_channels_config_invalid_summary');
      }
      if (channel.type === 'telegram') {
        const config = channel.config;
        if (config.mode === 'mtproto') {
          return t('admin_alerts_channels_summary_mtproto', {
            phone: config.phone,
            chat: config.chat_id,
          });
        }
        return t('admin_alerts_channels_summary_bot', {
          chat: config.chat_id,
        });
      }
      if (channel.type === 'email') {
        const config = channel.config;
        return t('admin_alerts_channels_summary_email', {
          from: config.from,
          count: String(config.to.length),
        });
      }
      return t('admin_alerts_channels_summary_webhook', { url: channel.config.url });
    },
    [t],
  );

  const filteredChannels = React.useMemo(() => {
    const keyword = normalizeSearch(search);
    return channels.filter((channel) => {
      if (activeFilter === 'active' && (!channel.enabled || channel.config === null)) return false;
      if (activeFilter === 'paused' && channel.enabled) return false;
      if (!keyword) return true;
      const summary = formatSummary(channel);
      const haystack = [channel.name, channel.type, summary].join(' ').toLowerCase();
      return haystack.includes(keyword);
    });
  }, [activeFilter, channels, formatSummary, search]);

  const filters: Array<{ key: AlertChannelFilter; label: string }> = [
    { key: 'all', label: t('admin_alerts_channels_filter_all') },
    { key: 'active', label: t('admin_alerts_channels_filter_active') },
    { key: 'paused', label: t('admin_alerts_channels_filter_paused') },
  ];
  const selectedChannelIds = React.useMemo(
    () => new Set(settings?.channel_ids ?? []),
    [settings?.channel_ids],
  );
  const selectedActiveCount = React.useMemo(() => {
    if (!settings?.enabled) return 0;
    let count = 0;
    for (const channel of channels) {
      if (channel.config !== null && channel.enabled && selectedChannelIds.has(channel.id)) {
        count += 1;
      }
    }
    return count;
  }, [channels, selectedChannelIds, settings?.enabled]);
  const settingsDisabled = !settings || loadingSettings || savingSettings;

  return (
    <div className="space-y-4 md:space-y-6">
      <Card className="p-4">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="text-sm font-semibold text-(--theme-fg-default)">
                {t('admin_alerts_settings_title')}
              </h3>
              {loadingSettings ? (
                <Badge color="slate">{t('loading')}</Badge>
              ) : selectedActiveCount > 0 ? (
                <Badge color="emerald">
                  {t('admin_alerts_settings_active_count', {
                    count: String(selectedActiveCount),
                  })}
                </Badge>
              ) : (
                <Badge color="amber">{t('admin_alerts_settings_no_target')}</Badge>
              )}
            </div>
            <p className="mt-1 text-xs/5 text-(--theme-fg-muted)">
              {t('admin_alerts_settings_desc')}
            </p>
          </div>
          <IOSSwitch
            checked={settings?.enabled ?? false}
            disabled={settingsDisabled}
            ariaLabel={t('admin_alerts_settings_enabled')}
            onChange={onToggleSettingsEnabled}
          />
        </div>

        <div className="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {channels.length === 0 ? (
            <div className="rounded-lg border border-dashed border-(--theme-border-subtle) p-4 text-sm text-(--theme-fg-muted) dark:border-(--theme-border-default)">
              {t('admin_alerts_settings_empty')}
            </div>
          ) : (
            channels.map((channel) => {
              const selected = selectedChannelIds.has(channel.id);
              const invalidConfig = channel.config === null;
              const selectionDisabled = settingsDisabled || (invalidConfig && !selected);
              return (
                <label
                  key={channel.id}
                  className={`flex min-w-0 items-start gap-3 rounded-lg border p-3 text-sm transition-colors ${
                    selected
                      ? 'border-(--theme-border-interactive-muted) bg-(--theme-bg-interactive-muted)'
                      : 'border-(--theme-border-subtle) bg-(--theme-bg-default) dark:border-(--theme-border-default)'
                  } ${selectionDisabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer hover:bg-(--theme-surface-row-hover)'}`}
                >
                  <input
                    type="checkbox"
                    className="mt-0.5 size-4 accent-(--theme-fg-interactive)"
                    checked={selected}
                    disabled={selectionDisabled}
                    onChange={() => onToggleSettingsChannel(channel.id)}
                  />
                  <span className="min-w-0">
                    <span className="block truncate font-semibold text-(--theme-fg-default)">
                      {channel.name}
                    </span>
                    <span className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-(--theme-fg-muted)">
                      <span>{channelTypeMeta[channel.type].label}</span>
                      {!channel.enabled && (
                        <Badge color="amber">{t('admin_alerts_settings_channel_paused')}</Badge>
                      )}
                      {invalidConfig ? (
                        <Badge color="rose">{t('admin_alerts_channels_config_invalid')}</Badge>
                      ) : channel.delivery_status === 'degraded' ? (
                        <Badge color="amber">{t('admin_alerts_channels_delivery_degraded')}</Badge>
                      ) : null}
                    </span>
                  </span>
                </label>
              );
            })
          )}
        </div>
      </Card>

      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-3">
        <SearchInput
          icon={Search}
          placeholder={t('admin_alerts_channels_search_placeholder')}
          aria-label={t('admin_alerts_channels_search_placeholder')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
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
          <table className="min-w-6xl w-full text-left text-sm bg-(--theme-bg-default) dark:bg-(--theme-bg-default)">
            <thead className="bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle) text-(--theme-fg-default) dark:text-(--theme-fg-default) text-xs font-semibold border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
              <tr>
                <th scope="col" className="min-w-52 px-4 py-3 align-middle">
                  {t('admin_alerts_channels_col_name')}
                </th>
                <th scope="col" className="min-w-44 px-4 py-3 align-middle">
                  {t('admin_alerts_channels_col_type')}
                </th>
                <th scope="col" className="min-w-56 px-4 py-3 align-middle">
                  {t('admin_alerts_channels_col_summary')}
                </th>
                <th scope="col" className="min-w-64 px-4 py-3 align-middle">
                  {t('admin_alerts_channels_col_status')}
                </th>
                <th scope="col" className="w-24 px-4 py-3 text-center align-middle">
                  {t('admin_alerts_channels_col_enabled')}
                </th>
                <th scope="col" className="min-w-32 px-4 py-3 align-middle">
                  {t('admin_alerts_channels_col_updated')}
                </th>
                <th scope="col" className="w-32 px-4 py-3 text-right align-middle">
                  {t('common_actions')}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-(--theme-border-muted) dark:divide-(--theme-canvas-muted)">
              {loading && filteredChannels.length === 0 ? (
                <tr>
                  <td
                    colSpan={7}
                    className="px-4 py-12 text-center text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)"
                  >
                    {t('loading')}
                  </td>
                </tr>
              ) : filteredChannels.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-12">
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
                  const delivery = deliveryMeta[channel.delivery_status];
                  const summary = formatSummary(channel);
                  const invalidConfig = channel.config === null;
                  return (
                    <tr
                      key={channel.id}
                      className="hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle) transition-colors"
                    >
                      <td className="px-4 py-3 align-middle">
                        <div className="flex flex-col">
                          <span className="font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-default)">
                            {channel.name}
                          </span>
                          <span className="text-[11px] text-(--theme-fg-muted-alt) font-mono">
                            ID: {channel.id}
                          </span>
                        </div>
                      </td>
                      <td className="px-4 py-3 align-middle">
                        <div className="flex items-center gap-2">
                          <meta.icon
                            className="size-4 text-(--theme-fg-muted-alt)"
                            aria-hidden="true"
                          />
                          <Badge color={meta.color}>{meta.label}</Badge>
                        </div>
                      </td>
                      <td className="px-4 py-3 align-middle text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-muted) font-mono">
                        {summary}
                      </td>
                      <td className="px-4 py-3 align-middle">
                        <div className="min-w-0 space-y-1.5">
                          {invalidConfig ? (
                            <Badge color="rose">{t('admin_alerts_channels_config_invalid')}</Badge>
                          ) : (
                            <Badge color={delivery.color}>{delivery.label}</Badge>
                          )}
                          {channel.consecutive_failures > 0 ? (
                            <p className="text-[11px] text-(--theme-fg-danger-muted)">
                              {t('admin_alerts_channels_delivery_failures', {
                                count: String(channel.consecutive_failures),
                              })}
                            </p>
                          ) : null}
                          {channel.last_error ? (
                            <p
                              className="max-w-64 truncate text-[11px] text-(--theme-fg-danger-muted)"
                              title={channel.last_error}
                            >
                              {channel.last_error}
                            </p>
                          ) : channel.pending_count > 0 ? (
                            <p className="text-[11px] text-(--theme-fg-muted)">
                              {t('admin_alerts_channels_delivery_pending', {
                                count: String(channel.pending_count),
                              })}
                            </p>
                          ) : null}
                          {channel.next_retry_at ? (
                            <p className="text-[11px] text-(--theme-fg-muted)">
                              {t('admin_alerts_channels_delivery_next_retry', {
                                time: formatLocalTimestamp(channel.next_retry_at),
                              })}
                            </p>
                          ) : null}
                          {channel.next_probe_at ? (
                            <p className="text-[11px] text-(--theme-fg-muted)">
                              {t('admin_alerts_channels_delivery_next_probe', {
                                time: formatLocalDateTime(channel.next_probe_at, lang, {
                                  dateStyle: 'short',
                                  timeStyle: 'short',
                                }),
                              })}
                            </p>
                          ) : null}
                          {channel.last_success_at ? (
                            <p className="text-[11px] text-(--theme-fg-muted)">
                              {t('admin_alerts_channels_delivery_last_success', {
                                time: formatLocalDateTime(channel.last_success_at, lang, {
                                  dateStyle: 'short',
                                  timeStyle: 'short',
                                }),
                              })}
                            </p>
                          ) : null}
                        </div>
                      </td>
                      <td className="px-4 py-3 text-center align-middle">
                        <div className="inline-flex items-center justify-center">
                          <IOSSwitch
                            size="sm"
                            checked={channel.enabled}
                            disabled={
                              togglingIds.includes(channel.id) ||
                              (invalidConfig && !channel.enabled)
                            }
                            ariaLabel={t('admin_alerts_channels_enabled_toggle', {
                              name: channel.name,
                            })}
                            onChange={() => onToggleEnabled(channel)}
                          />
                        </div>
                      </td>
                      <td className="px-4 py-3 align-middle text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-muted) font-mono whitespace-nowrap">
                        {formatTimeAgo(channel.updated_at, lang)}
                      </td>
                      <td className="px-4 py-3 text-right align-middle">
                        <div className="flex items-center justify-end gap-1">
                          <button
                            type="button"
                            onClick={() => {
                              if (!invalidConfig) onEdit(channel);
                            }}
                            className="ui-focus-ring inline-flex size-8 items-center justify-center rounded-md text-(--theme-fg-subtle) transition-colors hover:bg-(--theme-bg-interactive-hover) hover:text-(--theme-fg-interactive) disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-(--theme-bg-interactive-hover) dark:hover:text-(--theme-fg-interactive-hover)"
                            aria-label={t('common_edit')}
                            title={
                              invalidConfig
                                ? t('admin_alerts_channels_config_invalid_summary')
                                : t('common_edit')
                            }
                            disabled={invalidConfig}
                          >
                            <Edit2 className="size-4.5" aria-hidden="true" />
                          </button>
                          <button
                            type="button"
                            onClick={() => onTest(channel)}
                            className="ui-focus-ring inline-flex size-8 items-center justify-center rounded-md text-(--theme-fg-subtle) transition-colors hover:bg-(--theme-bg-interactive-hover) hover:text-(--theme-fg-interactive) disabled:cursor-not-allowed disabled:opacity-40 dark:hover:bg-(--theme-bg-interactive-hover) dark:hover:text-(--theme-fg-interactive-hover)"
                            aria-label={t('admin_alerts_channels_action_test')}
                            title={
                              invalidConfig
                                ? t('admin_alerts_channels_config_invalid_summary')
                                : t('admin_alerts_channels_action_test')
                            }
                            disabled={invalidConfig || testingIds.includes(channel.id)}
                          >
                            <FlaskConical className="size-4.5" aria-hidden="true" />
                          </button>
                          <button
                            type="button"
                            onClick={() => onDelete(channel)}
                            className="ui-focus-ring inline-flex size-8 items-center justify-center rounded-md text-(--theme-fg-danger-muted) transition-colors hover:bg-(--theme-bg-danger-muted) hover:text-(--theme-fg-danger) dark:text-(--theme-fg-danger) dark:hover:bg-(--theme-bg-danger-subtle) dark:hover:text-(--theme-fg-danger-soft)"
                            aria-label={t('common_delete')}
                          >
                            <Trash2 className="size-4.5" aria-hidden="true" />
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
