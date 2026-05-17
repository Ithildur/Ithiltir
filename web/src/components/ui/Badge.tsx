import React from 'react';

export type BadgeColor = 'slate' | 'emerald' | 'rose' | 'amber' | 'indigo';

interface Props {
  children: React.ReactNode;
  color?: BadgeColor;
  glow?: boolean;
}

const colorClasses: Record<BadgeColor, string> = {
  slate:
    'bg-(--theme-bg-muted) text-(--theme-fg-strong) dark:bg-(--theme-fg-strong) dark:text-(--theme-fg-neutral) border-(--theme-border-subtle) dark:border-(--theme-border-default)',
  emerald:
    'bg-(--theme-bg-success-subtle) text-(--theme-fg-success-on-muted) dark:bg-(--theme-fg-success-muted)/10 dark:text-(--theme-fg-success-muted) border-(--theme-border-success-muted) dark:border-(--theme-border-success-muted)',
  rose: 'bg-(--theme-bg-danger-subtle) text-(--theme-fg-danger) dark:bg-(--theme-bg-danger-soft) dark:text-(--theme-fg-danger) border-(--theme-border-danger-muted) dark:border-(--theme-border-danger-muted)',
  amber:
    'bg-(--theme-bg-warning-subtle) text-(--theme-fg-warning-strong) dark:bg-(--theme-bg-warning-soft) dark:text-(--theme-fg-warning) border-(--theme-border-warning-muted) dark:border-(--theme-border-warning-soft)',
  indigo:
    'bg-(--theme-bg-interactive-muted) text-(--theme-fg-interactive-strong) dark:bg-(--theme-bg-interactive-soft) dark:text-(--theme-fg-interactive-hover) border-(--theme-border-interactive-muted) dark:border-(--theme-border-interactive-muted)',
};

const Badge: React.FC<Props> = ({ children, color = 'slate', glow = false }) => {
  const glowClass = glow ? 'dark:shadow-[0_0_10px_-2px_currentColor]' : '';

  return (
    <span
      className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider border ${
        colorClasses[color]
      } ${glowClass}`}
    >
      {children}
    </span>
  );
};

export default Badge;
