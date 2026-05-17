import React from 'react';
import Trash2 from 'lucide-react/dist/esm/icons/trash-2';
import Button from '@components/ui/Button';
import type { ThemePackage } from '@app-types/admin';
import { useI18n } from '@i18n';
import ThemePreview, { FallbackThemePreview } from '@components/admin/systemManager/ThemePreview';
import {
  chipClass,
  formatTimestamp,
  statusBadgeClass,
  themeBadgeLabels,
  type DefaultThemeOption,
} from '@components/admin/systemManager/themeManagerModel';

export const ThemeRow: React.FC<{
  item: ThemePackage;
  busy: boolean;
  applying: boolean;
  onApply: () => void;
  onDelete: () => void;
}> = ({ item, busy, applying, onApply, onDelete }) => {
  const { t } = useI18n();
  const badges = themeBadgeLabels(item, t);
  const missing = Boolean(item.missing);
  const broken = Boolean(item.broken);
  const unavailable = missing || broken;
  const version = unavailable ? '—' : item.version || '—';
  const author = unavailable ? '—' : item.author || '—';
  const unavailableLabel = broken ? t('admin_theme_broken_badge') : t('admin_theme_missing_badge');
  const unavailableDesc = broken ? t('admin_theme_broken_desc') : t('admin_theme_missing_desc');
  const applyDisabled = item.active || busy || unavailable;

  return (
    <tr
      className={`transition-colors ${
        unavailable
          ? 'bg-(--theme-bg-warning-muted)/60 hover:bg-(--theme-bg-warning-muted)/80 dark:bg-(--theme-bg-warning-muted) dark:hover:bg-(--theme-bg-warning-muted)'
          : item.active
            ? 'bg-(--theme-bg-accent-muted)/30 hover:bg-(--theme-bg-accent-muted)/45'
            : 'hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)'
      }`}
    >
      <td className="px-4 py-3">
        <ThemePreview item={item} />
      </td>

      <td className="px-4 py-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong)">
              {item.name}
            </span>
            {item.active && (
              <span className={statusBadgeClass('accent')}>{t('admin_theme_active_badge')}</span>
            )}
            {item.built_in && (
              <span className={statusBadgeClass('neutral')}>{t('admin_theme_builtin_badge')}</span>
            )}
            {missing && (
              <span className={statusBadgeClass('warning')}>{t('admin_theme_missing_badge')}</span>
            )}
            {broken && (
              <span className={statusBadgeClass('warning')}>{t('admin_theme_broken_badge')}</span>
            )}
          </div>
          <div className="mt-1 text-[11px] font-mono text-(--theme-fg-muted)">{item.id}</div>
          <div className="mt-1 text-xs/5 text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
            {unavailable ? unavailableDesc : item.description || t('admin_theme_no_description')}
          </div>
        </div>
      </td>

      <td className="px-4 py-3 text-xs text-(--theme-fg-subtle)">
        <div>
          {t('admin_theme_meta_version')}:{' '}
          <span className="text-(--theme-fg-default) dark:text-(--theme-fg-strong)">{version}</span>
        </div>
        <div className="mt-1">
          {t('admin_theme_meta_author')}:{' '}
          <span className="text-(--theme-fg-default) dark:text-(--theme-fg-strong)">{author}</span>
        </div>
      </td>

      <td className="px-4 py-3 text-xs text-(--theme-fg-subtle)">
        {missing
          ? t('admin_theme_meta_deleted')
          : broken
            ? t('admin_theme_meta_broken')
            : t('admin_theme_meta_updated', { updated: formatTimestamp(item.updated_at) })}
      </td>

      <td className="px-4 py-3">
        <div className="flex flex-wrap gap-2">
          {badges.map((badge) => (
            <span key={badge} className={chipClass}>
              {badge}
            </span>
          ))}
        </div>
      </td>

      <td className="px-4 py-3 text-right">
        <div className="flex items-center justify-end gap-2">
          <Button
            variant={item.active ? 'secondary' : 'primary'}
            className="h-9 min-w-24 rounded-lg"
            onClick={onApply}
            disabled={applyDisabled}
          >
            {unavailable
              ? unavailableLabel
              : item.active
                ? t('admin_theme_applied')
                : applying
                  ? t('admin_theme_applying')
                  : t('admin_theme_apply')}
          </Button>
          {item.deletable && (
            <Button
              variant="danger"
              className="h-9 rounded-lg px-3"
              onClick={onDelete}
              disabled={busy}
              aria-label={t('common_delete')}
              title={t('common_delete')}
            >
              <Trash2 size={16} />
            </Button>
          )}
        </div>
      </td>
    </tr>
  );
};

export const DefaultThemeRow: React.FC<{
  item: DefaultThemeOption;
  busy: boolean;
  applying: boolean;
  onApply: () => void;
}> = ({ item, busy, applying, onApply }) => {
  const { t } = useI18n();
  const badges = themeBadgeLabels(item, t);
  const applyDisabled = item.active || busy;
  const description = item.description || t('admin_theme_default_desc');

  return (
    <tr
      className={`transition-colors ${
        item.active
          ? 'bg-(--theme-bg-accent-muted)/30 hover:bg-(--theme-bg-accent-muted)/45'
          : 'hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)'
      }`}
    >
      <td className="px-4 py-3">
        <FallbackThemePreview item={item} />
      </td>

      <td className="px-4 py-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold text-(--theme-fg-default) dark:text-(--theme-fg-strong)">
              {item.name}
            </span>
            {item.active && (
              <span className={statusBadgeClass('accent')}>{t('admin_theme_active_badge')}</span>
            )}
            <span className={statusBadgeClass('neutral')}>{t('admin_theme_builtin_badge')}</span>
          </div>
          <div className="mt-1 text-[11px] font-mono text-(--theme-fg-muted)">{item.id}</div>
          <div className="mt-1 text-xs/5 text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)">
            {description}
          </div>
        </div>
      </td>

      <td className="px-4 py-3 text-xs text-(--theme-fg-subtle)">
        <div>
          {t('admin_theme_meta_version')}:{' '}
          <span className="text-(--theme-fg-default) dark:text-(--theme-fg-strong)">
            {item.version || '—'}
          </span>
        </div>
        <div className="mt-1">
          {t('admin_theme_meta_author')}:{' '}
          <span className="text-(--theme-fg-default) dark:text-(--theme-fg-strong)">
            {item.author || '—'}
          </span>
        </div>
      </td>

      <td className="px-4 py-3 text-xs text-(--theme-fg-subtle)">—</td>

      <td className="px-4 py-3">
        <div className="flex flex-wrap gap-2">
          {badges.map((badge) => (
            <span key={badge} className={chipClass}>
              {badge}
            </span>
          ))}
        </div>
      </td>

      <td className="px-4 py-3 text-right">
        <div className="flex items-center justify-end gap-2">
          <Button
            variant={item.active ? 'secondary' : 'primary'}
            className="h-9 min-w-24 rounded-lg"
            onClick={onApply}
            disabled={applyDisabled}
          >
            {item.active
              ? t('admin_theme_applied')
              : applying
                ? t('admin_theme_applying')
                : t('admin_theme_apply')}
          </Button>
        </div>
      </td>
    </tr>
  );
};
