import type { PVEVM } from '@app-types/virt';
import type { NodeVMSettings } from '@app-types/api';
import { useI18n } from '@i18n';
import { formatBytes } from '@pages/dashboard/viewModel';
import VMHistory from './VMHistory';
import VMIPs from './VMIPs';

const VMDetails = ({
  serverId,
  vm,
  settings,
  observedAt,
}: {
  serverId: number;
  vm: PVEVM;
  settings: NodeVMSettings;
  observedAt: number;
}) => {
  const { t } = useI18n();
  const ipStale =
    vm.ips_collected_at != null &&
    vm.ips_ttl_seconds != null &&
    observedAt > Date.parse(vm.ips_collected_at) + vm.ips_ttl_seconds * 1000;

  return (
    <div className="space-y-3 p-4">
      <dl className="flex flex-wrap gap-x-6 gap-y-2 text-xs">
        <div className="flex items-baseline gap-2">
          <dt className="text-(--theme-fg-muted)">{t('pve_disk_capacity')}</dt>
          <dd className="font-mono">
            {vm.disk_capacity_bytes == null ? '—' : formatBytes(vm.disk_capacity_bytes)}
          </dd>
        </div>
        <div className="flex items-baseline gap-2">
          <dt className="text-(--theme-fg-muted)">vCPU</dt>
          <dd className="font-mono">{vm.cpus ?? '—'}</dd>
        </div>
        <div className="flex items-baseline gap-2">
          <dt className="text-(--theme-fg-muted)">{t('uptime')}</dt>
          <dd className="font-mono">
            {vm.uptime_seconds == null
              ? '—'
              : t('pve_uptime', {
                  days: Math.floor(vm.uptime_seconds / 86400),
                  hours: Math.floor((vm.uptime_seconds % 86400) / 3600),
                  minutes: Math.floor((vm.uptime_seconds % 3600) / 60),
                })}
          </dd>
        </div>
        <div className="flex w-full flex-wrap items-baseline gap-x-3 gap-y-1">
          <dt className="text-(--theme-fg-muted)">{t('pve_ips')}</dt>
          <dd className="min-w-0 max-w-full font-mono">
            <VMIPs vm={vm} />
          </dd>
          {ipStale && <dd className="text-(--theme-fg-warning-strong)">{t('pve_ip_stale')}</dd>}
        </div>
      </dl>
      {!vm.template && <VMHistory serverId={serverId} vmId={vm.id} settings={settings} />}
    </div>
  );
};

export default VMDetails;
