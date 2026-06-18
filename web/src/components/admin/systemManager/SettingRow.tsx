import React from 'react';

type SettingRowProps = {
  title: React.ReactNode;
  description?: React.ReactNode;
  disabled?: boolean;
  controlClassName?: string;
  children: React.ReactNode;
};

type SettingPanelProps = {
  title?: React.ReactNode;
  description?: React.ReactNode;
  children: React.ReactNode;
};

export const SettingPanel: React.FC<SettingPanelProps> = ({ title, description, children }) => (
  <section className="overflow-hidden rounded-lg border border-(--theme-border-subtle) bg-(--theme-bg-default) shadow-sm dark:border-(--theme-border-default) dark:bg-(--theme-bg-default)">
    {title || description ? (
      <div className="border-b border-(--theme-border-subtle) px-5 py-4 dark:border-(--theme-border-default)">
        {title ? (
          <h2 className="text-base/6 font-semibold tracking-tight text-(--theme-fg-default)">
            {title}
          </h2>
        ) : null}
        {description ? (
          <div className="mt-1 max-w-160 text-xs/5 text-(--theme-fg-muted)">{description}</div>
        ) : null}
      </div>
    ) : null}
    {children}
  </section>
);

export const SettingPanelFooter: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div className="flex justify-end bg-(--theme-bg-muted)/40 px-5 py-3 dark:bg-(--theme-canvas-subtle)">
    {children}
  </div>
);

const SettingRow: React.FC<SettingRowProps> = ({
  title,
  description,
  disabled,
  controlClassName = '',
  children,
}) => (
  <div
    className={`grid gap-4 border-b border-(--theme-border-subtle) px-5 py-4 transition-[background-color] last:border-b-0 md:grid-cols-[minmax(220px,360px)_minmax(0,1fr)] md:items-center hover:bg-(--theme-surface-row-hover) dark:border-(--theme-border-default) dark:hover:bg-(--theme-canvas-subtle) ${
      disabled ? 'opacity-70' : ''
    }`}
  >
    <div className="min-w-0">
      <div className="text-sm font-semibold text-(--theme-fg-default)">{title}</div>
      {description ? (
        <div className="mt-1 max-w-160 text-xs/5 text-(--theme-fg-muted)">{description}</div>
      ) : null}
    </div>
    <div className={`flex min-w-0 w-full justify-start md:justify-end ${controlClassName}`}>
      {children}
    </div>
  </div>
);

export default SettingRow;
