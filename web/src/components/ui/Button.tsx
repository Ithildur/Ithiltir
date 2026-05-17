import React from 'react';
import type { LucideIcon } from 'lucide-react';

export type ButtonVariant =
  | 'primary'
  | 'secondary'
  | 'danger'
  | 'ghost'
  | 'plain'
  | 'icon'
  | 'iconDanger';
export type ButtonSize = 'md' | 'none';

interface Props extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: LucideIcon;
}

const baseClass =
  'flex items-center justify-center gap-2 rounded-lg font-medium text-sm tracking-tight transition-[color,background-color,border-color,box-shadow,transform] duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-(--theme-focus-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--theme-focus-ring-offset) disabled:opacity-50 disabled:cursor-not-allowed active:scale-[0.98]';

const sizeClass: Record<ButtonSize, string> = {
  md: 'px-3 py-[5px] text-sm leading-5',
  none: '',
};

const variantClass: Record<ButtonVariant, string> = {
  primary:
    'bg-(--theme-bg-success-emphasis) text-(--theme-fg-on-emphasis) border border-(--theme-control-border) shadow-[0_1px_0_var(--theme-control-shadow),inset_0_1px_0_var(--theme-control-inset)] hover:bg-(--theme-fg-success) active:bg-(--theme-fg-success) active:shadow-none dark:border-(--theme-border-translucent) dark:shadow-[0_0_0_1px_var(--theme-border-translucent)] dark:active:bg-(--theme-bg-success-emphasis)',
  secondary:
    'bg-(--theme-bg-muted) text-(--theme-fg-default) border border-(--theme-control-border) shadow-[0_1px_0_var(--theme-control-shadow-subtle),inset_0_1px_0_var(--theme-control-inset-strong)] hover:bg-(--theme-bg-muted) hover:border-(--theme-control-border) active:bg-(--theme-bg-button-secondary-active) active:shadow-none dark:bg-(--theme-canvas-muted) dark:text-(--theme-fg-default) dark:border-(--theme-border-translucent) dark:shadow-[0_0_0_1px_var(--theme-border-translucent)] dark:hover:bg-(--theme-border-default) dark:hover:border-(--theme-border-translucent)',
  danger:
    'bg-(--theme-bg-danger-emphasis) text-(--theme-fg-on-emphasis) border border-(--theme-control-border) shadow-[0_1px_0_var(--theme-control-shadow),inset_0_1px_0_var(--theme-control-inset)] hover:bg-(--theme-bg-button-danger-hover) active:bg-(--theme-bg-button-danger-active) active:shadow-none dark:border-(--theme-border-translucent) dark:shadow-[0_1px_0_var(--theme-border-translucent),inset_0_1px_0_var(--theme-control-inset)]',
  ghost:
    'bg-(--theme-bg-muted) text-(--theme-fg-muted) dark:bg-(--theme-canvas-subtle) dark:text-(--theme-fg-muted)',
  plain: '',
  icon: 'btn-icon',
  iconDanger: 'btn-icon-danger',
};

const Button: React.FC<Props> = ({
  children,
  variant = 'primary',
  size = 'md',
  icon: Icon,
  className = '',
  type = 'button',
  ...props
}) => {
  const isIconVariant = variant === 'icon' || variant === 'iconDanger';
  const classes = isIconVariant
    ? [variantClass[variant], className].filter(Boolean).join(' ')
    : `${baseClass} ${sizeClass[size]} ${variantClass[variant]} ${className}`;

  return (
    <button type={type} className={classes} {...props}>
      {Icon && <Icon size={16} />}
      {children}
    </button>
  );
};

export default Button;
