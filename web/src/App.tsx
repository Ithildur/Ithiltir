import React from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import DashboardPage from '@pages/DashboardPage';
import LoginPage from '@pages/LoginPage';
import FullScreenLoader from '@components/ui/FullScreenLoader';
import { useI18n, type TranslationKey } from '@i18n';
import { useAuthStore } from '@stores/authStore';
import { refreshStatisticsAccess, useStatisticsAccessStore } from '@stores/statisticsAccessStore';
import { TrafficRebuildRuntime } from '@runtime/TrafficRebuildRuntime';
import { isCanceledRequestError } from '@utils/errors';

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
  const status = useAuthStore((state) => state.status);
  const isAuthenticated = useAuthStore(
    (state) => state.status === 'authenticated' && Boolean(state.accessToken),
  );
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
  const { t } = useI18n();
  const status = useAuthStore((state) => state.status);
  const isAuthenticated = useAuthStore(
    (state) => state.status === 'authenticated' && Boolean(state.accessToken),
  );
  const location = useLocation();
  const access = useStatisticsAccessStore((state) => state.load.access);
  const accessStatus = useStatisticsAccessStore((state) => state.load.status);
  const accessError = useStatisticsAccessStore((state) => state.load.error);

  React.useEffect(() => {
    if (isAuthenticated) return;
    if (status === 'unknown' || status === 'bootstrapping') return;
    const controller = new AbortController();
    void refreshStatisticsAccess({ signal: controller.signal }).catch((error) => {
      if (isCanceledRequestError(error)) return;
    });
    return () => {
      controller.abort();
    };
  }, [isAuthenticated, status]);

  if (status === 'unknown' || status === 'bootstrapping') {
    return <FullScreenLoader />;
  }
  if (isAuthenticated) return children;
  if (accessError !== null) {
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
  if (accessStatus !== 'ready' || !access) return <FullScreenLoader />;

  const allowed =
    kind === 'history'
      ? access.history_guest_access_mode === 'by_node'
      : access.traffic_guest_access_mode === 'by_node';
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

const App: React.FC = () => (
  <BrowserRouter>
    <TrafficRebuildRuntime />
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

export default App;
