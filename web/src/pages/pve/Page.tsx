import React from 'react';
import { Link, useParams, useSearchParams } from 'react-router';
import ArrowLeft from 'lucide-react/dist/esm/icons/arrow-left';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import ChevronRight from 'lucide-react/dist/esm/icons/chevron-right';
import Button from '@components/ui/Button';
import Card from '@components/ui/Card';
import ThemeToggle from '@components/ui/ThemeToggle';
import { useI18n, type TranslationKey } from '@i18n';
import { ApiError } from '@lib/api';
import { fetchPVE } from '@lib/virtApi';
import type { PVEView, PVEVM } from '@app-types/virt';
import { formatBytes } from '@pages/dashboard/viewModel';
import { isCanceledRequestError } from '@utils/errors';
import { formatLocalDateTime } from '@utils/time';
import { useAuthStore } from '@stores/authStore';
import VMDetails from './VMDetails';
import VMIPs from './VMIPs';

const bytes = (value?: number) => (value == null ? '—' : formatBytes(value));
const percent = (value?: number) => (value == null ? '—' : `${(value * 100).toFixed(1)}%`);
const memoryPercent = (vm: PVEVM) =>
  vm.memory_bytes != null && vm.memory_limit_bytes != null && vm.memory_limit_bytes > 0
    ? percent(vm.memory_bytes / vm.memory_limit_bytes)
    : null;
const vmState = (vm: PVEVM) =>
  vm.template ? 'template' : vm.qmp_status === 'paused' ? 'paused' : vm.status;

const VMStatus = ({ vm }: { vm: PVEVM }) => {
  const { t } = useI18n();
  const status = vmState(vm);
  const label = vm.template
    ? t('pve_template')
    : status === 'running'
      ? t('pve_running')
      : status === 'stopped'
        ? t('pve_stopped')
        : status === 'paused'
          ? t('pve_paused')
          : status;
  return (
    <span
      className={
        status === 'running' && !vm.template
          ? 'text-(--theme-fg-success)'
          : status === 'paused'
            ? 'text-(--theme-fg-warning-strong)'
            : 'text-(--theme-fg-muted)'
      }
    >
      {label}
    </span>
  );
};

const PVEHost = ({ serverId, isAdmin }: { serverId: number; isAdmin: boolean }) => {
  const { t, lang } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();
  const [view, setView] = React.useState<PVEView | null>(null);
  const [error, setError] = React.useState<TranslationKey | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [revision, setRevision] = React.useState(0);
  const [observedAt, setObservedAt] = React.useState(0);
  const validID = Number.isSafeInteger(serverId) && serverId > 0;

  React.useEffect(() => {
    if (!validID) return;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    const load = async () => {
      setLoading(true);
      try {
        const next = await fetchPVE(serverId, controller.signal);
        if (controller.signal.aborted) return;
        setView(next);
        setObservedAt(Date.now());
        setError(null);
      } catch (failure) {
        if (controller.signal.aborted || isCanceledRequestError(failure)) return;
        // A revoked permission must also remove the previously rendered VM data.
        setView(null);
        setError(
          failure instanceof ApiError && failure.status === 403
            ? 'pve_forbidden'
            : failure instanceof ApiError && failure.status === 404
              ? 'pve_host_missing'
              : 'pve_error',
        );
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
          timer = setTimeout(() => void load(), 15_000);
        }
      }
    };
    void load();
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [serverId, validID, revision]);

  const snapshot = view?.snapshot;
  const vms = snapshot?.vms ?? [];
  const statusCounts = new Map<string, number>();
  for (const vm of vms) {
    const status = vmState(vm);
    statusCounts.set(status, (statusCounts.get(status) ?? 0) + 1);
  }
  const expandedVMs = new Set(isAdmin ? searchParams.getAll('vm') : []);
  const allExpanded = vms.length > 0 && vms.every((vm) => expandedVMs.has(String(vm.id)));
  const anyExpanded = vms.some((vm) => expandedVMs.has(String(vm.id)));
  const collectedAt = snapshot?.last_success_at ?? snapshot?.collected_at;
  const date = (timestamp: string) =>
    formatLocalDateTime(timestamp, lang, { dateStyle: 'medium', timeStyle: 'medium' });
  const errorMessage = validID ? error : 'pve_host_missing';
  const toggleVM = (vmId: number) =>
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      const id = String(vmId);
      if (next.getAll('vm').includes(id)) next.delete('vm', id);
      else next.append('vm', id);
      return next;
    });
  const expandAll = (expand: boolean) =>
    setSearchParams((current) => {
      const next = new URLSearchParams(current);
      next.delete('vm');
      if (expand) for (const vm of vms) next.append('vm', String(vm.id));
      return next;
    });

  return (
    <div className="min-h-screen bg-(--theme-page-bg) text-(--theme-fg-default) dark:bg-(--theme-bg-default)">
      <header className="sticky top-0 z-40 border-b border-(--theme-border-subtle) bg-(--theme-surface-control) backdrop-blur-md dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)/90">
        <div className="mx-auto flex h-16 max-w-410 items-center justify-between gap-3 px-4 sm:px-6 lg:px-8">
          <Link
            to="/"
            className="ui-focus-ring inline-flex items-center gap-2 rounded text-xs font-semibold text-(--theme-fg-muted) hover:text-(--theme-fg-default)"
          >
            <ArrowLeft size={14} />
            {t('common_back_to_dashboard')}
          </Link>
          <ThemeToggle size="sm" variant="soft" />
        </div>
      </header>
      <main
        id="main-content"
        tabIndex={-1}
        className="mx-auto max-w-410 space-y-4 px-4 py-5 sm:px-6 lg:px-8"
      >
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0 space-y-1">
            <h1 className="wrap-anywhere text-xl font-semibold">
              {snapshot?.host || `#${serverId}`} · {t('pve_title')}
            </h1>
            {collectedAt && (
              <p className="text-xs text-(--theme-fg-muted)">
                {t('pve_last_collected', { time: date(collectedAt) })}
              </p>
            )}
          </div>
          <Button
            variant="secondary"
            icon={RefreshCw}
            disabled={loading || !validID}
            onClick={() => setRevision((value) => value + 1)}
          >
            {t('stats_refresh')}
          </Button>
        </div>
        {errorMessage ? (
          <p role="alert" className="text-sm text-(--theme-fg-danger)">
            {t(errorMessage)}
          </p>
        ) : !view ? (
          <p role="status" className="text-sm text-(--theme-fg-muted)">
            {t('pve_loading')}
          </p>
        ) : !snapshot ? (
          <Card variant="panel" className="p-6 text-sm text-(--theme-fg-muted)">
            {t('pve_no_snapshot')}
          </Card>
        ) : (
          <>
            {view.stale && snapshot.status === 'ok' && (
              <p role="status" className="text-sm text-(--theme-fg-warning)">
                {t('pve_stale')}
              </p>
            )}
            {snapshot.status !== 'ok' && (
              <p role="status" className="text-sm text-(--theme-fg-danger)">
                {t('pve_collection_error')}
              </p>
            )}
            <Card as="section" variant="panel" className="overflow-hidden">
              <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2 border-b border-(--theme-border-subtle) px-4 py-3">
                <h2 className="text-sm font-semibold">
                  {snapshot.vms ? t('pve_vm_count', { count: vms.length }) : t('pve_title')}
                </h2>
                <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-(--theme-fg-muted)">
                  {(['running', 'paused', 'stopped', 'template'] as const).map((status) =>
                    statusCounts.has(status) ? (
                      <span key={status}>
                        {t(`pve_${status}`)}{' '}
                        <span className="font-mono text-(--theme-fg-default)">
                          {statusCounts.get(status)}
                        </span>
                      </span>
                    ) : null,
                  )}
                </div>
              </div>
              {isAdmin && vms.length > 0 && (
                <div className="flex flex-wrap justify-end gap-2 border-b border-(--theme-border-subtle) px-4 py-2">
                  <Button
                    variant="secondary"
                    disabled={allExpanded}
                    onClick={() => expandAll(true)}
                  >
                    {t('pve_expand_all')}
                  </Button>
                  <Button
                    variant="secondary"
                    disabled={!anyExpanded}
                    onClick={() => expandAll(false)}
                  >
                    {t('pve_collapse_all')}
                  </Button>
                </div>
              )}
              {vms.length === 0 ? (
                <p className="p-6 text-sm text-(--theme-fg-muted)">
                  {t(snapshot.vms ? 'pve_empty' : 'pve_inventory_unknown')}
                </p>
              ) : (
                <table className="block w-full text-left text-sm md:table">
                  <thead className="hidden text-xs text-(--theme-fg-muted) md:table-header-group">
                    <tr>
                      {[
                        t('pve_vm'),
                        t('pve_status'),
                        t('cpu'),
                        t('memory'),
                        ...(isAdmin ? [t('pve_ips')] : []),
                      ].map((label, index) => (
                        <th
                          key={label}
                          scope="col"
                          className={`px-4 py-2 font-medium ${index === 4 ? 'hidden lg:table-cell' : ''}`}
                        >
                          {label}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody className="block md:table-row-group">
                    {vms.map((vm) => {
                      const expanded = expandedVMs.has(String(vm.id));
                      const label = (
                        <span className="flex min-w-0 flex-wrap items-baseline gap-x-2">
                          <span className="wrap-anywhere font-medium">
                            {vm.name || `VM ${vm.id}`}
                          </span>
                          {vm.name && (
                            <span className="font-mono text-xs text-(--theme-fg-muted)">
                              #{vm.id}
                            </span>
                          )}
                        </span>
                      );
                      return (
                        <React.Fragment key={vm.id}>
                          <tr
                            className={`grid grid-cols-2 items-center border-t border-(--theme-border-subtle) md:table-row ${expanded ? 'bg-(--theme-bg-muted)' : ''}`}
                          >
                            <td className="min-w-0 px-4 pt-3 pb-1 md:py-2.5">
                              {isAdmin ? (
                                <button
                                  id={`vm-toggle-${vm.id}`}
                                  type="button"
                                  aria-expanded={expanded}
                                  aria-controls={`vm-details-${vm.id}`}
                                  className="ui-focus-ring flex max-w-64 items-center gap-2 rounded text-left hover:text-(--theme-fg-interactive)"
                                  onClick={() => toggleVM(vm.id)}
                                >
                                  <ChevronRight
                                    aria-hidden="true"
                                    size={14}
                                    className={`shrink-0 ${expanded ? 'rotate-90' : ''}`}
                                  />
                                  {label}
                                </button>
                              ) : (
                                label
                              )}
                            </td>
                            <td className="px-4 pt-3 pb-1 text-right text-xs md:py-2.5 md:text-left md:text-sm">
                              <VMStatus vm={vm} />
                            </td>
                            <td className="px-4 pt-1 pb-3 font-mono text-xs md:py-2.5">
                              <span className="mr-2 text-(--theme-fg-muted) md:hidden">CPU</span>
                              {percent(vm.cpu_ratio)}
                            </td>
                            <td className="min-w-0 px-4 pt-1 pb-3 text-right font-mono text-xs md:py-2.5 md:text-left">
                              <span className="mr-2 text-(--theme-fg-muted) md:hidden">
                                {t('memory')}
                              </span>
                              {memoryPercent(vm) && (
                                <span className="mr-3">{memoryPercent(vm)}</span>
                              )}
                              <span className="inline-block text-(--theme-fg-muted)">
                                {bytes(vm.memory_bytes)} / {bytes(vm.memory_limit_bytes)}
                              </span>
                            </td>
                            {isAdmin && (
                              <td className="hidden max-w-64 px-4 py-2.5 font-mono text-xs lg:table-cell">
                                <VMIPs vm={vm} />
                                {vm.ips_collected_at &&
                                vm.ips_ttl_seconds != null &&
                                observedAt >
                                  Date.parse(vm.ips_collected_at) + vm.ips_ttl_seconds * 1000 ? (
                                  <span className="block text-(--theme-fg-warning-strong)">
                                    {t('pve_ip_stale')}
                                  </span>
                                ) : null}
                              </td>
                            )}
                          </tr>
                          {isAdmin && (
                            <tr
                              hidden={!expanded}
                              className={expanded ? 'block md:table-row' : undefined}
                            >
                              <td colSpan={5} className="block min-w-0 p-0 md:table-cell">
                                <section
                                  id={`vm-details-${vm.id}`}
                                  aria-labelledby={`vm-toggle-${vm.id}`}
                                >
                                  {expanded && (
                                    <VMDetails
                                      serverId={serverId}
                                      vm={vm}
                                      settings={view}
                                      observedAt={observedAt}
                                    />
                                  )}
                                </section>
                              </td>
                            </tr>
                          )}
                        </React.Fragment>
                      );
                    })}
                  </tbody>
                </table>
              )}
            </Card>
            {expandedVMs.size > 0 && !anyExpanded && vms.length > 0 && (
              <p className="text-sm text-(--theme-fg-muted)">{t('pve_vm_missing')}</p>
            )}
          </>
        )}
      </main>
    </div>
  );
};

export default function Page() {
  const { serverId } = useParams();
  const isAdmin = useAuthStore((state) => state.status === 'authenticated');
  return <PVEHost key={`${serverId}:${isAdmin}`} serverId={Number(serverId)} isAdmin={isAdmin} />;
}
