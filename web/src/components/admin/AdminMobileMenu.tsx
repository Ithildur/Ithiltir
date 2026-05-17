import React from 'react';
import Globe from 'lucide-react/dist/esm/icons/globe';
import LayoutDashboard from 'lucide-react/dist/esm/icons/layout-dashboard';
import LogOut from 'lucide-react/dist/esm/icons/log-out';
import ThemeToggle from '@components/ui/ThemeToggle';
import SidebarItem from '@components/admin/SidebarItem';
import { useAuth } from '@context/AuthContext';
import { useI18n } from '@i18n';
import type { AdminNavItem } from '@components/admin/AdminSidebar';
import type { DashboardTab } from '@app-types/admin';

interface Props {
  isOpen: boolean;
  tabs: AdminNavItem[];
  activeTab: DashboardTab;
  onTabChange: (tab: DashboardTab) => void;
  onClose: () => void;
}

const AdminMobileMenu: React.FC<Props> = ({ isOpen, tabs, activeTab, onTabChange, onClose }) => {
  const { logout } = useAuth();
  const { lang, setLang, t } = useI18n();
  const visibleTabs = React.useMemo(() => tabs.filter((tab) => !tab.hidden), [tabs]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 md:hidden flex">
      <div
        className="absolute inset-0 bg-(--theme-fg-strong)/20 dark:bg-(--theme-overlay-scrim) backdrop-blur-sm"
        onClick={onClose}
      />
      <div className="relative w-72 bg-(--theme-bg-default) dark:bg-(--theme-bg-inset) h-full shadow-2xl theme-shadow-float flex flex-col animate-in fade-in slide-in-from-left-5 border-r border-(--theme-border-subtle) dark:border-(--theme-border-default)">
        <div className="h-22 flex items-center justify-between px-6 border-b border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-overlay) dark:bg-(--theme-bg-inset)">
          <span className="font-bold text-2xl text-(--theme-fg-strong) dark:text-(--theme-fg-strong)">
            {t('admin_menu')}
          </span>
          <button
            type="button"
            onClick={onClose}
            className="text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)"
            aria-label={t('common_close')}
          >
            <span aria-hidden="true">×</span>
          </button>
        </div>
        <div className="p-4 pt-8 space-y-1 flex-1 overflow-y-auto">
          {visibleTabs.map((tab) => (
            <SidebarItem
              key={tab.key}
              icon={tab.icon}
              label={t(tab.labelKey)}
              active={activeTab === tab.key}
              onClick={() => {
                onTabChange(tab.key);
                onClose();
              }}
            />
          ))}
        </div>
        <div className="px-4 pb-2">
          <SidebarItem icon={LayoutDashboard} label={t('common_back_to_dashboard')} to="/" />
        </div>
        <div className="p-4 border-t border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-surface-control-strong) dark:bg-transparent">
          <div className="grid grid-cols-1 gap-2">
            <button
              className="flex items-center gap-2 text-sm text-(--theme-bg-danger-emphasis) w-full px-3 py-2 hover:bg-(--theme-bg-danger-subtle) dark:hover:bg-(--theme-bg-danger-soft) rounded-lg"
              type="button"
              onClick={logout}
            >
              <LogOut size={16} /> {t('admin_sign_out')}
            </button>
            <button
              className="flex items-center gap-2 text-sm text-(--theme-fg-default) dark:text-(--theme-fg-default) w-full px-3 py-2 hover:bg-(--theme-bg-interactive-muted) dark:hover:bg-(--theme-bg-interactive-soft) rounded-lg"
              type="button"
              onClick={() => setLang((value) => (value === 'zh' ? 'en' : 'zh'))}
            >
              <Globe size={16} /> {t('admin_change_lang')}{' '}
              <span className="ml-auto font-mono text-xs opacity-70">{lang.toUpperCase()}</span>
            </button>
            <ThemeToggle
              size="sm"
              variant="soft"
              showLabel
              labelMode="action"
              actionLabel={t('admin_change_theme')}
              className="w-full justify-start px-3 rounded-lg"
            />
          </div>
        </div>
      </div>
    </div>
  );
};

export default AdminMobileMenu;
