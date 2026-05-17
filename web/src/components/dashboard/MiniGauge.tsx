import React from 'react';
import type { LucideIcon } from 'lucide-react';
import { getColorByPercent } from '@pages/dashboard/viewModel';

interface Props {
  value: number;
  label: string;
  icon: LucideIcon;
  detail?: React.ReactNode;
}

const MiniGauge: React.FC<Props> = ({ value, label, icon: Icon, detail }) => {
  const radius = 40;
  const circumference = 2 * Math.PI * radius;
  const clamped = Math.max(0, Math.min(100, value));
  const offset = circumference - (clamped / 100) * circumference;
  const colorClass = getColorByPercent(clamped);

  return (
    <div className="flex flex-col items-center justify-center">
      <div className="relative size-24 shrink-0">
        <svg className="size-full transform -rotate-90" viewBox="0 0 88 88">
          <circle
            cx="44"
            cy="44"
            r={radius}
            stroke="currentColor"
            strokeWidth="8"
            fill="transparent"
            className="text-(--theme-bg-muted)"
          />
          <circle
            cx="44"
            cy="44"
            r={radius}
            stroke="currentColor"
            strokeWidth="8"
            fill="transparent"
            strokeDasharray={circumference}
            strokeDashoffset={offset}
            strokeLinecap="round"
            className={`transition-[stroke-dashoffset,color] duration-1000 ease-out ${colorClass}`}
          />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-1.5">
          <Icon
            size={16}
            className="text-(--theme-fg-subtle) dark:text-(--theme-fg-control-muted)"
          />
          <span className="text-sm font-bold font-mono text-(--theme-fg-default) dark:text-(--theme-fg-default) leading-none">
            {Math.round(clamped)}%
          </span>
        </div>
      </div>
      <div className="flex flex-col items-center">
        {detail && <div>{detail}</div>}
        <span className="text-[11px] text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) font-medium uppercase tracking-tight">
          {label}
        </span>
      </div>
    </div>
  );
};

export default MiniGauge;
