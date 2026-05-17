import type { ThemeManifest } from '@app-types/admin';
import type { I18nContextValue } from '@i18n';

export type ThemeInfo = ThemeManifest & { broken?: boolean; missing?: boolean };
export type DefaultThemeOption = ThemeManifest & { active: boolean };

export const formatTimestamp = (value: string | null): string => {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date);
};

export const normalizeThemeSearch = (value: string) => value.trim().toLowerCase();

export const matchesThemeSearch = (item: ThemeInfo, keyword: string): boolean => {
  if (!keyword) return true;
  const capabilities = [
    item.skin.admin.shell,
    item.skin.admin.frame,
    item.skin.dashboard.summary,
    item.skin.dashboard.density,
  ];
  const haystack = [
    item.id,
    item.name,
    item.author,
    item.description,
    item.version,
    ...capabilities,
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
  return haystack.includes(keyword);
};

export const chipClass =
  'rounded-full border border-(--theme-border-subtle)/80 bg-(--theme-bg-muted) px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-(--theme-fg-subtle) dark:border-(--theme-border-default) dark:bg-(--theme-bg-default)/5';

export const statusBadgeClass = (tone: 'neutral' | 'accent' | 'warning') => {
  if (tone === 'accent') {
    return 'rounded-full border border-(--theme-border-success-muted) bg-(--theme-bg-success-muted) px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-(--theme-fg-success-on-muted) dark:border-(--theme-border-success-muted) dark:bg-(--theme-bg-success-muted) dark:text-(--theme-fg-success-on-muted)';
  }
  if (tone === 'warning') {
    return 'rounded-full border border-(--theme-border-warning-muted) bg-(--theme-bg-warning-muted) px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-(--theme-fg-warning-strong) dark:border-(--theme-border-warning-muted) dark:bg-(--theme-bg-warning-muted) dark:text-(--theme-fg-warning-strong)';
  }
  return chipClass;
};

export const themeBadgeLabels = (item: ThemeInfo, t: I18nContextValue['t']): string[] => {
  if (item.missing || item.broken) return [];
  return [
    item.skin.admin.shell === 'topbar'
      ? t('admin_theme_skin_shell_topbar')
      : t('admin_theme_skin_shell_sidebar'),
    item.skin.admin.frame === 'flat'
      ? t('admin_theme_skin_frame_flat')
      : t('admin_theme_skin_frame_layered'),
    item.skin.dashboard.summary === 'strip'
      ? t('admin_theme_skin_summary_strip')
      : t('admin_theme_skin_summary_cards'),
    item.skin.dashboard.density === 'compact'
      ? t('admin_theme_skin_density_compact')
      : t('admin_theme_skin_density_comfortable'),
  ];
};
