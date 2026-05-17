import React from 'react';
import Menu from 'lucide-react/dist/esm/icons/menu';
import Server from 'lucide-react/dist/esm/icons/server';
import Users from 'lucide-react/dist/esm/icons/users';
import LayoutDashboard from 'lucide-react/dist/esm/icons/layout-dashboard';
import Bell from 'lucide-react/dist/esm/icons/bell';
import Settings2 from 'lucide-react/dist/esm/icons/settings-2';
import { Link } from 'react-router-dom';
import AdminMobileMenu from '@components/admin/AdminMobileMenu';
import AdminSidebar from '@components/admin/AdminSidebar';
import type { AdminNavItem } from '@components/admin/AdminSidebar';
import AdminTopbar from '@components/admin/AdminTopbar';
import NodeManager from '@components/admin/nodeManager/NodeManager';
import ServerTimeCard from '@components/admin/ServerTimeCard';
import FullScreenLoader from '@components/ui/FullScreenLoader';
import ThemeToggle from '@components/ui/ThemeToggle';
import type { DashboardTab } from '@app-types/admin';
import { useSiteBrand } from '@context/SiteBrandContext';
import { useTheme } from '@context/ThemeContext';
import { useI18n } from '@i18n';
import { fetchAppVersion } from '@lib/versionApi';

const GroupManager = React.lazy(() => import('@components/admin/GroupManager'));
const AlertManager = React.lazy(() => import('@components/admin/alertManager/AlertManager'));
const SystemSettings = React.lazy(() => import('@components/admin/systemManager/SystemSettings'));

const tabs: AdminNavItem[] = [
  { key: 'nodes', labelKey: 'admin_tab_nodes', icon: Server },
  { key: 'groups', labelKey: 'admin_tab_groups', icon: Users },
  { key: 'alerts', labelKey: 'admin_tab_alerts', icon: Bell },
  { key: 'system', labelKey: 'admin_tab_system', icon: Settings2 },
];

const visibleTabs = tabs.filter((tab) => !tab.hidden);

const tabComponents = {
  nodes: NodeManager,
  groups: GroupManager,
  alerts: AlertManager,
  system: SystemSettings,
} satisfies Record<DashboardTab, React.ElementType>;

const AdminConsolePage: React.FC = () => {
  const [activeTab, setActiveTab] = React.useState<DashboardTab>('nodes');
  const [isMobileMenuOpen, setIsMobileMenuOpen] = React.useState(false);
  const [dashVersion, setDashVersion] = React.useState('');
  const { t } = useI18n();
  const { brand } = useSiteBrand();
  const {
    manifest: { skin: theme },
  } = useTheme();

  React.useEffect(() => {
    if (!visibleTabs.some((tab) => tab.key === activeTab)) {
      setActiveTab(visibleTabs[0]?.key ?? 'nodes');
    }
  }, [activeTab]);

  const activeTabMeta = tabs.find((tab) => tab.key === activeTab) ?? visibleTabs[0] ?? tabs[0];
  const ActiveTabComponent = tabComponents[activeTab] ?? NodeManager;
  const topbarShell = theme.admin.shell === 'topbar';
  const flatFrame = theme.admin.frame === 'flat';
  const versionLabel = dashVersion ? `Dash ${dashVersion}` : 'Dash';

  React.useEffect(() => {
    const controller = new AbortController();
    fetchAppVersion({ signal: controller.signal })
      .then((res) => setDashVersion(res.version?.trim() ?? ''))
      .catch((error) => {
        if (error instanceof DOMException && error.name === 'AbortError') return;
        setDashVersion('');
      });
    return () => {
      controller.abort();
    };
  }, []);

  return (
    <div
      className={`min-h-screen text-(--theme-fg-default) dark:text-(--theme-fg-strong) font-sans flex overflow-hidden ${
        flatFrame ? 'bg-(--theme-bg-muted)' : 'bg-(--theme-bg-muted) dark:bg-(--theme-bg-default)'
      }`}
    >
      {!flatFrame && (
        <div className="fixed inset-0 z-0 pointer-events-none">
          <div className="absolute inset-0 theme-admin-page-gradient-light dark:hidden" />

          <div className="absolute inset-0 hidden dark:block theme-admin-page-gradient-dark" />

          <div className="absolute inset-0 theme-admin-page-grid opacity-[0.4] dark:opacity-[0.15]" />
        </div>
      )}

      {!topbarShell && (
        <AdminSidebar
          tabs={tabs}
          activeTab={activeTab}
          onTabChange={setActiveTab}
          versionLabel={versionLabel}
        />
      )}

      <AdminMobileMenu
        isOpen={isMobileMenuOpen}
        tabs={tabs}
        activeTab={activeTab}
        onTabChange={setActiveTab}
        onClose={() => setIsMobileMenuOpen(false)}
      />

      <div className="flex-1 flex flex-col min-w-0 h-screen overflow-hidden relative z-10">
        {topbarShell && (
          <AdminTopbar
            tabs={tabs}
            activeTab={activeTab}
            onTabChange={setActiveTab}
            versionLabel={versionLabel}
          />
        )}

        <header className="md:hidden h-16 bg-(--theme-surface-overlay) dark:bg-(--theme-bg-inset)/90 backdrop-blur-md border-b border-(--theme-border-subtle) dark:border-(--theme-border-default) flex items-center justify-between px-4 z-10 shrink-0">
          <div className="flex min-w-0 items-center gap-3">
            <button
              onClick={() => setIsMobileMenuOpen(true)}
              className="p-1 text-(--theme-fg-muted) dark:text-(--theme-fg-control-hover)"
              type="button"
              aria-label={t('admin_menu')}
            >
              <Menu size={24} />
            </button>
            <span className="truncate font-bold text-lg text-(--theme-fg-strong) dark:text-(--theme-fg-strong)">
              {brand.topbar_text}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Link
              to="/"
              className="rounded-lg p-2 text-(--theme-fg-muted) transition-colors hover:bg-(--theme-bg-accent-muted) hover:text-(--theme-fg-accent) dark:text-(--theme-fg-control-hover) dark:hover:bg-(--theme-bg-default)/5 dark:hover:text-(--theme-fg-accent)"
              aria-label={t('common_back_to_dashboard')}
            >
              <LayoutDashboard size={20} />
            </Link>
            <ThemeToggle size="sm" variant="soft" />
          </div>
        </header>

        <div
          className={`flex-1 overflow-auto scroll-smooth custom-scrollbar ${
            flatFrame ? 'px-4 pb-8 pt-5 md:px-6 md:pb-10 md:pt-6' : 'p-4 md:p-8'
          }`}
        >
          <div className="mx-auto max-w-screen-2xl pb-10">
            <div
              className={`flex flex-col justify-between gap-4 sm:flex-row sm:items-center ${
                flatFrame
                  ? 'mb-4 rounded-2xl border border-(--theme-border-default) bg-(--theme-bg-default)/72 px-5 py-4 shadow-sm dark:bg-(--theme-bg-default)/72'
                  : 'mb-6 border-b border-(--theme-border-subtle) pb-4 dark:border-(--theme-border-default)'
              }`}
            >
              <div>
                <h1 className="text-2xl font-semibold text-(--theme-fg-strong) dark:text-(--theme-fg-strong) tracking-tight capitalize flex items-center gap-3">
                  {activeTabMeta.icon && (
                    <activeTabMeta.icon
                      className="text-(--theme-fg-muted) dark:text-(--theme-fg-neutral)"
                      size={24}
                    />
                  )}
                  {t(activeTabMeta.labelKey)}
                </h1>
              </div>
              <div className="hidden sm:flex gap-3">{!topbarShell && <ServerTimeCard />}</div>
            </div>

            <React.Suspense fallback={<FullScreenLoader />}>
              <ActiveTabComponent />
            </React.Suspense>
          </div>
        </div>
      </div>
    </div>
  );
};

export default AdminConsolePage;
