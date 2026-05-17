import React from 'react';
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw';
import Terminal from 'lucide-react/dist/esm/icons/terminal';
import { useI18n } from '@i18n';
import { formatBytes } from '@pages/dashboard/viewModel';
import type { ServerViewModel } from '@pages/dashboard/viewModel';
import {
  BackReadWriteRow,
  CompactDetailRow,
  ConnectionsRow,
} from '@components/dashboard/NetworkRow';

interface Props {
  view: ServerViewModel;
}

const ServerCardBack: React.FC<Props> = ({ view }) => {
  const { t } = useI18n();
  const raidArray = view.raid.primaryArray;
  const raidState = raidArray
    ? [raidArray.syncStatus, raidArray.syncProgress].filter(Boolean).join(' ')
    : '';
  const raidValue = raidArray ? raidArray.name || raidArray.status || raidArray.health : '';
  const raidSub = raidArray
    ? [raidArray.health || raidArray.status, raidArray.syncProgress ? raidState : '']
        .filter(Boolean)
        .join(' ')
    : '';
  const { readRate, writeRate, readIops, writeIops, deviceName } = view.diskIo;

  return (
    <div className="absolute inset-0 backface-hidden rotate-y-180 bg-(--theme-bg-default) dark:bg-(--theme-bg-default) rounded-xl border border-(--theme-border-subtle) dark:border-(--theme-border-default) shadow-sm cursor-pointer overflow-hidden flex flex-col transition-[border-color] duration-200 hover:border-(--theme-border-interactive-hover) dark:hover:border-(--theme-fg-accent)/40">
      <div className="flex justify-between items-center px-4 py-1.5 border-b border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-default) dark:bg-(--theme-bg-default) shrink-0">
        <span className="font-bold text-xs text-(--theme-fg-default) dark:text-(--theme-fg-default) flex items-center gap-2">
          <Terminal size={14} className="text-(--theme-fg-interactive)" />
          {view.hostname}
        </span>
        <span className="text-[10px] flex items-center gap-1 text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted) hover:text-(--theme-fg-interactive) transition-colors uppercase font-bold tracking-wider">
          <RefreshCw size={10} /> {t('back')}
        </span>
      </div>

      <div className="p-3 h-full overflow-hidden text-xs">
        <div className="bg-(--theme-bg-default) dark:bg-(--theme-bg-default) rounded px-3 py-1 border border-(--theme-border-muted) dark:border-(--theme-border-default) h-full flex flex-col gap-1">
          <CompactDetailRow label={t('cpu')} value={view.cpu.modelName} />
          <CompactDetailRow
            label={t('os_short')}
            value={
              `${view.system.platform} ${view.system.platformVersion}`.trim() +
              (view.system.arch && view.system.arch.trim().length > 0
                ? ` (${view.system.arch.trim().toUpperCase()})`
                : '')
            }
          />
          <CompactDetailRow label={t('kernel_short')} value={view.system.kernelVersion} />

          <CompactDetailRow
            label={t('mem_short')}
            value={`${formatBytes(view.memory.used)} / ${formatBytes(view.memory.total)}`}
            sub={`${view.memory.usedPercent.toFixed(0)}%`}
          />
          <CompactDetailRow
            label={t('swap_short')}
            value={`${formatBytes(view.memory.swapUsed)} / ${formatBytes(view.memory.swapTotal)}`}
          />

          <CompactDetailRow
            label={
              view.disk.fsType && view.disk.fsType.trim().length > 0
                ? `${view.disk.path} (${view.disk.fsType})`
                : view.disk.path
            }
            value={`${formatBytes(view.disk.used)} / ${formatBytes(view.disk.total)}`}
            sub={`${view.disk.usedPercent.toFixed(0)}%`}
          />

          <CompactDetailRow label={t('procs_short')} value={view.processes.count} />

          <ConnectionsRow
            tcpLabel={t('tcp_conn_count')}
            udpLabel={t('udp_conn_count')}
            tcp={view.connections.tcpCount}
            udp={view.connections.udpCount}
          />

          <BackReadWriteRow
            label={t('disk_io')}
            sub={deviceName}
            readTag={t('rw_read')}
            writeTag={t('rw_write')}
            read={`${formatBytes(readRate)}/s`}
            write={`${formatBytes(writeRate)}/s`}
          />
          <BackReadWriteRow
            label={t('iops_short')}
            sub={deviceName}
            readTag={t('rw_read')}
            writeTag={t('rw_write')}
            read={readIops ? readIops.toFixed(0) : 0}
            write={writeIops ? writeIops.toFixed(0) : 0}
          />

          {raidArray ? (
            <CompactDetailRow label={t('raid_short')} value={raidValue} sub={raidSub} />
          ) : (
            <></>
          )}
        </div>
      </div>
    </div>
  );
};

export default ServerCardBack;
