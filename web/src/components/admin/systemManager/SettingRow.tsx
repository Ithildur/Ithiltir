import React from 'react';

type SettingRowProps = {
  title: React.ReactNode;
  description?: React.ReactNode;
  disabled?: boolean;
  children: React.ReactNode;
};

const SettingRow: React.FC<SettingRowProps> = ({ title, description, disabled, children }) => (
  <div
    className={`grid gap-5 rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-default) p-5 shadow-sm transition-[border-color,background-color] md:grid-cols-[minmax(0,1fr)_auto] md:items-center hover:border-(--theme-border-hover) hover:bg-(--theme-surface-row-hover) dark:border-(--theme-border-default) dark:bg-(--theme-bg-default) dark:hover:bg-(--theme-canvas-subtle) ${
      disabled ? 'opacity-70' : ''
    }`}
  >
    <div className="min-w-0">
      <div className="text-sm font-semibold text-(--theme-fg-default)">{title}</div>
      {description ? (
        <div className="mt-1 max-w-160 text-xs/5 text-(--theme-fg-muted)">{description}</div>
      ) : null}
    </div>
    <div className="flex w-full justify-start md:justify-end">{children}</div>
  </div>
);

export default SettingRow;
