import React from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import DashboardPage from '@pages/DashboardPage';
import LoginPage from '@pages/LoginPage';
import { useAuth } from '@context/AuthContext';
import FullScreenLoader from '@components/ui/FullScreenLoader';
import { useBootstrapAuth } from '@hooks/useBootstrapAuth';
import { fetchStatisticsAccess } from '@lib/statisticsApi';
import type { StatisticsAccess } from '@app-types/traffic';
import { useI18n, type TranslationKey } from '@i18n';

const AdminConsolePage = React.lazy(() => import('@pages/AdminConsolePage'));
const StatisticsPage = React.lazy(() => import('@pages/StatisticsPage'));
const TrafficPage = React.lazy(() => import('@pages/TrafficPage'));

type LoginRedirectState = {
  from?: string;
  denied?: {
    code: 403;
    reason: 'statistics';
  };
};

const LazyPage: React.FC<{ children: React.ReactElement }> = ({ children }) => (
  <React.Suspense fallback={<FullScreenLoader />}>{children}</React.Suspense>
);

const RequireAuth: React.FC<{
  children: React.ReactElement;
  redirectState?: Omit<LoginRedirectState, 'from'>;
}> = ({ children, redirectState }) => {
  useBootstrapAuth();
  const { isAuthenticated, status } = useAuth();
  const location = useLocation();

  if (status === 'unknown' || status === 'bootstrapping') {
    return <FullScreenLoader />;
  }
  if (!isAuthenticated) {
    const nextState = redirectState
      ? {
          ...redirectState,
          from: `${location.pathname}${location.search}${location.hash}`,
        }
      : undefined;
    return <Navigate to="/login" replace state={nextState} />;
  }
  return children;
};

const RequireStatisticsAccess: React.FC<{
  kind: 'history' | 'traffic';
  children: React.ReactElement;
}> = ({ kind, children }) => {
  useBootstrapAuth();
  const { t } = useI18n();
  const { isAuthenticated, status } = useAuth();
  const location = useLocation();
  const [access, setAccess] = React.useState<StatisticsAccess | null>(null);
  const [loaded, setLoaded] = React.useState(false);
  const [accessError, setAccessError] = React.useState(false);

  React.useEffect(() => {
    if (isAuthenticated) return;
    if (status === 'unknown' || status === 'bootstrapping') return;
    const controller = new AbortController();
    setLoaded(false);
    setAccessError(false);
    fetchStatisticsAccess({ signal: controller.signal })
      .then((next) => {
        setAccess(next);
        setAccessError(false);
        setLoaded(true);
      })
      .catch((error) => {
        if (error instanceof DOMException && error.name === 'AbortError') return;
        setAccess(null);
        setAccessError(true);
        setLoaded(true);
      });
    return () => controller.abort();
  }, [isAuthenticated, status]);

  if (status === 'unknown' || status === 'bootstrapping') {
    return <FullScreenLoader />;
  }
  if (isAuthenticated) return children;
  if (!loaded) return <FullScreenLoader />;
  if (accessError) {
    const messageKey: TranslationKey = kind === 'history' ? 'stats_error' : 'traffic_error';
    return (
      <div className="min-h-screen bg-(--theme-page-bg) text-(--theme-fg-default) dark:bg-(--theme-bg-default)">
        <main className="mx-auto max-w-410 px-4 py-12 sm:px-6 lg:px-8">
          <div className="rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-default) p-6 text-sm text-(--theme-fg-danger) dark:border-(--theme-border-default)">
            {t(messageKey)}
          </div>
        </main>
      </div>
    );
  }

  const allowed =
    kind === 'history'
      ? access?.history_guest_access_mode === 'by_node'
      : access?.traffic_guest_access_mode === 'by_node';
  if (!allowed) {
    return (
      <Navigate
        to="/login"
        replace
        state={{
          denied: { code: 403, reason: 'statistics' },
          from: `${location.pathname}${location.search}${location.hash}`,
        }}
      />
    );
  }
  return children;
};

const App: React.FC = () => {
  React.useEffect(() => {
    if (typeof window === 'undefined') return;
    const keyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.isComposing) return;
      if (event.key !== '/') return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;

      const activeElement = document.activeElement;
      if (activeElement instanceof HTMLElement) {
        const tagName = activeElement.tagName.toLowerCase();
        if (tagName === 'input' || tagName === 'textarea' || tagName === 'select') return;
        if (activeElement.isContentEditable) return;
      }

      const focusVisibleSearchInput = () => {
        const inputs = Array.from(
          document.querySelectorAll<HTMLInputElement>('[data-search-input="true"]'),
        );
        const target = inputs.find((input) => {
          if (input.disabled || input.readOnly) return false;
          return input.getClientRects().length > 0;
        });
        if (!target) return false;
        target.focus();
        target.select();
        return true;
      };

      if (focusVisibleSearchInput()) {
        event.preventDefault();
        return;
      }

      const triggers = Array.from(
        document.querySelectorAll<HTMLElement>('[data-search-trigger="true"]'),
      );
      const trigger = triggers.find((element) => element.getClientRects().length > 0);
      if (!trigger) return;
      event.preventDefault();
      trigger.click();
      window.setTimeout(() => {
        focusVisibleSearchInput();
      }, 0);
    };

    window.addEventListener('keydown', keyDown);
    return () => window.removeEventListener('keydown', keyDown);
  }, []);

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/" element={<DashboardPage />} />
        <Route
          path="/statistics/:serverId"
          element={
            <RequireStatisticsAccess kind="history">
              <LazyPage>
                <StatisticsPage />
              </LazyPage>
            </RequireStatisticsAccess>
          }
        />
        <Route
          path="/traffic/:serverId"
          element={
            <RequireStatisticsAccess kind="traffic">
              <LazyPage>
                <TrafficPage />
              </LazyPage>
            </RequireStatisticsAccess>
          }
        />
        <Route
          path="/admin"
          element={
            <RequireAuth>
              <LazyPage>
                <AdminConsolePage />
              </LazyPage>
            </RequireAuth>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
};

export default App;
