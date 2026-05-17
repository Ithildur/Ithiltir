import React from 'react';
import Globe from 'lucide-react/dist/esm/icons/globe';
import LayoutDashboard from 'lucide-react/dist/esm/icons/layout-dashboard';
import LogOut from 'lucide-react/dist/esm/icons/log-out';
import type { LucideIcon } from 'lucide-react';
import BrandLogo from '@components/BrandLogo';
import ThemeToggle from '@components/ui/ThemeToggle';
import SidebarItem from '@components/admin/SidebarItem';
import { useAuth } from '@context/AuthContext';
import { useSiteBrand } from '@context/SiteBrandContext';
import { useI18n, type TranslationKey } from '@i18n';
import type { DashboardTab } from '@app-types/admin';

export interface AdminNavItem {
  key: DashboardTab;
  labelKey: TranslationKey;
  icon: LucideIcon;
  hidden?: boolean;
}

interface Props {
  tabs: AdminNavItem[];
  activeTab: DashboardTab;
  onTabChange: (tab: DashboardTab) => void;
  versionLabel: string;
}

const AdminSidebar: React.FC<Props> = ({ tabs, activeTab, onTabChange, versionLabel }) => {
  const [isLangMenuOpen, setIsLangMenuOpen] = React.useState(false);
  const langMenuRef = React.useRef<HTMLDivElement | null>(null);
  const { logout } = useAuth();
  const { brand } = useSiteBrand();
  const { lang, setLang, t } = useI18n();
  const visibleTabs = React.useMemo(() => tabs.filter((tab) => !tab.hidden), [tabs]);
  const langOptionClass = (selected: boolean) =>
    `w-full rounded-lg border-l-2 px-3 py-2 text-left text-sm transition-[color,background-color,border-color] duration-200 motion-reduce:transition-none focus:outline-none focus-visible:ring-2 focus-visible:ring-(--theme-focus-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--theme-bg-default) ${
      selected
        ? 'border-(--theme-border-underline-nav-active) bg-(--theme-bg-accent-muted) font-semibold text-(--theme-fg-accent)'
        : 'border-transparent text-(--theme-fg-default) hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-accent)'
    }`;

  React.useEffect(() => {
    if (!isLangMenuOpen) return;
    const onPointerDown = (event: PointerEvent) => {
      if (!langMenuRef.current) return;
      if (langMenuRef.current.contains(event.target as Node)) return;
      setIsLangMenuOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setIsLangMenuOpen(false);
    };
    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('pointerdown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [isLangMenuOpen]);

  return (
    <aside className="w-72 bg-(--theme-surface-control-strong) dark:bg-(--theme-bg-inset) backdrop-blur-xl border-r border-(--theme-border-subtle) dark:border-(--theme-border-default) shrink-0 hidden md:flex flex-col z-20 relative shadow-xl theme-shadow-soft dark:shadow-none">
      <div className="min-h-26 pt-5 flex items-center gap-3 px-6 border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
        <div className="size-9 rounded-xl flex items-center justify-center">
          <BrandLogo />
        </div>
        <div className="min-w-0">
          <span className="block truncate font-bold text-lg tracking-tight leading-none">
            {brand.topbar_text}
          </span>
          <span className="text-[10px] text-(--theme-fg-muted) font-mono tracking-widest">
            {versionLabel}
          </span>
        </div>
      </div>

      <div className="p-4 pt-8 space-y-1 flex-1 overflow-y-auto custom-scrollbar">
        {visibleTabs.map((tab) => (
          <SidebarItem
            key={tab.key}
            icon={tab.icon}
            label={t(tab.labelKey)}
            active={activeTab === tab.key}
            onClick={() => onTabChange(tab.key)}
          />
        ))}
      </div>

      <div className="px-4 pb-2">
        <SidebarItem icon={LayoutDashboard} label={t('common_back_to_dashboard')} to="/" />
      </div>

      <div className="p-4 border-t border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-control) dark:bg-transparent">
        <div className="grid grid-cols-5 gap-2">
          <div
            className="group/lang relative col-span-3"
            ref={langMenuRef}
            onMouseEnter={() => setIsLangMenuOpen(true)}
            onMouseLeave={() => setIsLangMenuOpen(false)}
          >
            <button
              type="button"
              onClick={() => setIsLangMenuOpen(true)}
              className="inline-flex h-10 w-full items-center justify-center gap-2.5 rounded-xl text-(--theme-fg-default) transition-[color,background-color] duration-300 hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-accent) focus:outline-none focus-visible:ring-2 focus-visible:ring-(--theme-focus-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--theme-bg-default) motion-reduce:transition-none"
              title={t('admin_change_lang')}
              aria-label={t('admin_change_lang')}
              aria-haspopup="listbox"
              aria-expanded={isLangMenuOpen}
            >
              <Globe
                size={16}
                className={`transition-[color,rotate] duration-500 group-hover/lang:rotate-180 group-hover/lang:text-(--theme-fg-accent) motion-reduce:rotate-0 motion-reduce:transition-none ${
                  isLangMenuOpen ? 'rotate-180 text-(--theme-fg-accent)' : 'text-(--theme-fg-muted)'
                }`}
              />
              <span className="relative text-sm font-medium">
                {lang === 'zh' ? t('language_zh') : t('language_en')}
                <span
                  className={`absolute -bottom-1 left-1/2 h-0.5 -translate-x-1/2 rounded-full bg-(--theme-border-underline-nav-active) transition-all duration-300 group-hover/lang:w-full group-hover/lang:opacity-100 motion-reduce:transition-none ${
                    isLangMenuOpen ? 'w-full opacity-100' : 'w-0 opacity-0'
                  }`}
                />
              </span>
            </button>

            <div
              className={`absolute bottom-full left-0 z-50 w-full origin-bottom pb-2 transition-all duration-200 motion-reduce:transition-none ${
                isLangMenuOpen
                  ? 'opacity-100 translate-y-0 scale-100'
                  : 'opacity-0 translate-y-2 scale-95 pointer-events-none'
              }`}
              role="listbox"
              aria-label={t('admin_change_lang')}
            >
              <div className="w-full overflow-hidden rounded-xl border border-(--theme-border-default) bg-(--theme-bg-default) p-1 theme-shadow-float dark:bg-(--theme-bg-default)">
                <button
                  type="button"
                  onClick={() => {
                    setLang('zh');
                    setIsLangMenuOpen(false);
                  }}
                  role="option"
                  aria-selected={lang === 'zh'}
                  className={langOptionClass(lang === 'zh')}
                >
                  <span className="relative z-10">{t('language_zh')}</span>
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setLang('en');
                    setIsLangMenuOpen(false);
                  }}
                  role="option"
                  aria-selected={lang === 'en'}
                  className={langOptionClass(lang === 'en')}
                >
                  <span className="relative z-10">{t('language_en')}</span>
                </button>
              </div>
            </div>
          </div>

          <ThemeToggle
            size="sm"
            variant="plain"
            titleOverride={t('admin_change_theme')}
            className="h-10 w-full justify-center rounded-xl border-0 bg-transparent text-(--theme-fg-muted) transition-all hover:bg-(--theme-surface-control-hover)/80 hover:text-(--theme-fg-accent) dark:text-(--theme-fg-neutral) dark:hover:bg-(--theme-bg-default)/25 dark:hover:text-(--theme-fg-accent)"
          />

          <button
            type="button"
            onClick={logout}
            className="h-10 w-full inline-flex items-center justify-center rounded-xl text-(--theme-bg-danger-emphasis) transition-all duration-300 hover:bg-(--theme-bg-danger-muted) hover:text-(--theme-bg-danger-emphasis) dark:hover:bg-(--theme-bg-danger-muted)"
            title={t('admin_logout')}
            aria-label={t('admin_logout')}
          >
            <LogOut size={18} />
          </button>
        </div>
      </div>
    </aside>
  );
};

export default AdminSidebar;
