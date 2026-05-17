import React from 'react';
import Globe from 'lucide-react/dist/esm/icons/globe';
import LayoutDashboard from 'lucide-react/dist/esm/icons/layout-dashboard';
import LogOut from 'lucide-react/dist/esm/icons/log-out';
import type { LucideIcon } from 'lucide-react';
import BrandLogo from '@components/BrandLogo';
import ServerTimeCard from '@components/admin/ServerTimeCard';
import ThemeToggle from '@components/ui/ThemeToggle';
import { useAuth } from '@context/AuthContext';
import { useSiteBrand } from '@context/SiteBrandContext';
import { useI18n, type TranslationKey } from '@i18n';
import type { DashboardTab } from '@app-types/admin';
import { Link } from 'react-router-dom';

type AdminNavItem = {
  key: DashboardTab;
  labelKey: TranslationKey;
  icon: LucideIcon;
  hidden?: boolean;
};

interface Props {
  tabs: AdminNavItem[];
  activeTab: DashboardTab;
  onTabChange: (tab: DashboardTab) => void;
  versionLabel: string;
}

const navItemClass = (active: boolean) =>
  active
    ? 'inline-flex items-center gap-2 rounded-xl border border-(--theme-border-default) bg-(--theme-bg-accent-muted) px-3 py-2 text-sm font-semibold text-(--theme-fg-accent) shadow-sm'
    : 'inline-flex items-center gap-2 rounded-xl border border-transparent px-3 py-2 text-sm font-medium text-(--theme-fg-muted) transition-[color,background-color,border-color] hover:border-(--theme-border-default) hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-default)';

const actionClass =
  'inline-flex h-10 items-center justify-center rounded-xl border border-transparent px-3 text-sm text-(--theme-fg-muted) transition-[color,background-color,border-color] hover:border-(--theme-border-default) hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-default)';

const AdminTopbar: React.FC<Props> = ({ tabs, activeTab, onTabChange, versionLabel }) => {
  const { logout } = useAuth();
  const { brand } = useSiteBrand();
  const { lang, setLang, t } = useI18n();
  const visibleTabs = React.useMemo(() => tabs.filter((tab) => !tab.hidden), [tabs]);

  return (
    <header className="relative z-20 hidden shrink-0 border-b border-(--theme-border-subtle)/80 bg-(--theme-surface-overlay) backdrop-blur-xl dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)/88 md:block">
      <div className="mx-auto flex max-w-screen-2xl items-center gap-4 px-6 py-4">
        <div className="flex shrink-0 items-center gap-3 pr-2">
          <div className="flex size-10 items-center justify-center rounded-2xl border border-(--theme-border-default) bg-(--theme-bg-default)">
            <BrandLogo />
          </div>
          <div className="min-w-0">
            <div className="truncate font-mono text-[11px] tracking-[0.18em] text-(--theme-fg-subtle)">
              {versionLabel}
            </div>
            <div className="truncate text-lg font-semibold tracking-tight text-(--theme-fg-default)">
              {brand.topbar_text}
            </div>
          </div>
        </div>

        <nav className="custom-scrollbar flex min-w-0 flex-1 items-center gap-2 overflow-x-auto">
          {visibleTabs.map((tab) => (
            <button
              key={tab.key}
              type="button"
              onClick={() => onTabChange(tab.key)}
              className={navItemClass(activeTab === tab.key)}
              aria-current={activeTab === tab.key ? 'page' : undefined}
            >
              <tab.icon size={16} />
              <span className="whitespace-nowrap">{t(tab.labelKey)}</span>
            </button>
          ))}
        </nav>

        <div className="flex shrink-0 items-center gap-2">
          <ServerTimeCard className="hidden xl:flex" />

          <button
            type="button"
            onClick={() => setLang((value) => (value === 'zh' ? 'en' : 'zh'))}
            className={actionClass}
            title={t('admin_change_lang')}
            aria-label={t('admin_change_lang')}
          >
            <Globe size={16} />
            <span className="ml-2 font-mono text-xs uppercase">{lang}</span>
          </button>

          <ThemeToggle
            size="sm"
            variant="plain"
            titleOverride={t('admin_change_theme')}
            className="h-10 rounded-xl border border-transparent px-3 text-(--theme-fg-muted) hover:border-(--theme-border-default) hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-default)"
          />

          <Link
            to="/"
            className={actionClass}
            title={t('common_back_to_dashboard')}
            aria-label={t('common_back_to_dashboard')}
          >
            <LayoutDashboard size={16} />
          </Link>

          <button
            type="button"
            onClick={logout}
            className={`${actionClass} text-(--theme-bg-danger-emphasis) hover:border-(--theme-border-danger-hover) hover:bg-(--theme-bg-danger-muted) hover:text-(--theme-bg-danger-emphasis) dark:hover:border-(--theme-border-danger-hover) dark:hover:bg-(--theme-bg-danger-muted)`}
            title={t('admin_logout')}
            aria-label={t('admin_logout')}
          >
            <LogOut size={16} />
          </button>
        </div>
      </div>
    </header>
  );
};

export default AdminTopbar;
