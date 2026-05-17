import React from 'react';
import ArrowDown from 'lucide-react/dist/esm/icons/arrow-down';
import ArrowUp from 'lucide-react/dist/esm/icons/arrow-up';
import ArrowUpDown from 'lucide-react/dist/esm/icons/arrow-up-down';
import Gauge from 'lucide-react/dist/esm/icons/gauge';

interface NetworkRowFullProps {
  label: string;
  up: string;
  down: string;
  isTotal?: boolean;
}

const NetworkRowFull: React.FC<NetworkRowFullProps> = ({ label, up, down, isTotal }) => (
  <div className="flex items-center justify-between bg-(--theme-bg-default) dark:bg-(--theme-bg-default) px-3 py-1.5 rounded-lg border border-(--theme-border-muted) dark:border-(--theme-border-default) w-full">
    <div className="flex items-center gap-2">
      <span
        className={`p-1 rounded bg-(--theme-bg-interactive-muted) dark:bg-(--theme-canvas-muted)/70 ${isTotal ? 'text-(--theme-fg-info)' : 'text-(--theme-fg-interactive)'}`}
      >
        {isTotal ? <ArrowUpDown size={10} /> : <Gauge size={10} />}
      </span>
      <span className="text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) font-medium">
        {label}
      </span>
    </div>
    <div className="flex items-center gap-3">
      <div className="flex items-center gap-1.5 min-w-17.5 justify-end">
        <ArrowDown
          size={12}
          className={isTotal ? 'text-(--theme-fg-info)' : 'text-(--theme-fg-success-muted)'}
        />
        <span className="text-xs font-mono text-(--theme-fg-default) dark:text-(--theme-fg-default)">
          {down}
        </span>
      </div>
      <div className="w-px h-3 bg-(--theme-border-default) dark:bg-(--theme-border-default)/70" />
      <div className="flex items-center gap-1.5 min-w-17.5 justify-end">
        <ArrowUp
          size={12}
          className={isTotal ? 'text-(--theme-fg-interactive)' : 'text-(--theme-fg-warning-muted)'}
        />
        <span className="text-xs font-mono text-(--theme-fg-default) dark:text-(--theme-fg-default)">
          {up}
        </span>
      </div>
    </div>
  </div>
);

interface CompactDetailRowProps {
  label: string;
  value: string | number;
  sub?: string;
}

const BACK_DETAIL_LABEL_CLASS =
  'text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) font-medium';
const BACK_DETAIL_VALUE_CLASS =
  'text-xs font-mono text-(--theme-fg-default) dark:text-(--theme-fg-default) text-right';

const CompactDetailRow: React.FC<CompactDetailRowProps> = ({ label, value, sub }) => (
  <div className="flex items-center h-6 border-b border-(--theme-border-muted) dark:border-(--theme-border-default) last:border-0 text-xs">
    <span className={`${BACK_DETAIL_LABEL_CLASS} flex-1`}>{label}</span>
    {sub && (
      <span className="mx-2 text-[10px] text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted) font-mono tabular-nums min-w-14 text-right">
        {sub}
      </span>
    )}
    <span className={BACK_DETAIL_VALUE_CLASS}>{value}</span>
  </div>
);

const DetailPair: React.FC<{ label: string; value: string | number }> = ({ label, value }) => (
  <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center h-6 gap-2 text-xs whitespace-nowrap">
    <span className={`${BACK_DETAIL_LABEL_CLASS} text-left`}>{label}</span>
    <span className={`${BACK_DETAIL_VALUE_CLASS} tabular-nums`}>{value}</span>
  </div>
);

interface ConnectionsRowProps {
  tcpLabel: string;
  udpLabel: string;
  tcp: number;
  udp: number;
}

const ConnectionsRow: React.FC<ConnectionsRowProps> = ({ tcpLabel, udpLabel, tcp, udp }) => (
  <div className="grid grid-cols-2 gap-4 h-6 border-b border-(--theme-border-muted) dark:border-(--theme-border-default) last:border-0 text-xs">
    <DetailPair label={tcpLabel} value={tcp} />
    <DetailPair label={udpLabel} value={udp} />
  </div>
);

interface BackReadWriteRowProps {
  label: string;
  sub?: string;
  readTag: string;
  writeTag: string;
  read: string | number;
  write: string | number;
}

const BackReadWriteRow: React.FC<BackReadWriteRowProps> = ({
  label,
  sub,
  readTag,
  writeTag,
  read,
  write,
}) => (
  <div className="flex items-center h-6 border-b border-(--theme-border-muted) dark:border-(--theme-border-default) last:border-0 text-xs">
    <div className="flex items-center gap-2 flex-1 min-w-0">
      <span className={BACK_DETAIL_LABEL_CLASS}>{label}</span>
      {sub && (
        <span className="text-[10px] text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted) font-mono tabular-nums truncate">
          {sub}
        </span>
      )}
    </div>
    <div className="flex items-center gap-3">
      <div className="flex items-center gap-1.5 min-w-20 justify-end">
        <span className="text-[12px] font-bold font-mono tabular-nums text-(--theme-fg-success-strong) dark:text-(--theme-fg-success-muted)">
          {readTag}
        </span>
        <span className="text-xs font-mono text-(--theme-fg-default) dark:text-(--theme-fg-default)">
          {read}
        </span>
      </div>
      <div className="w-px h-3 bg-(--theme-border-default) dark:bg-(--theme-border-default)/70" />
      <div className="flex items-center gap-1.5 min-w-20 justify-end">
        <span className="text-[12px] font-bold font-mono tabular-nums text-(--theme-fg-warning) dark:text-(--theme-fg-warning)">
          {writeTag}
        </span>
        <span className="text-xs font-mono text-(--theme-fg-default) dark:text-(--theme-fg-default)">
          {write}
        </span>
      </div>
    </div>
  </div>
);

export { NetworkRowFull, CompactDetailRow, ConnectionsRow, BackReadWriteRow };
