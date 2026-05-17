import React from 'react';

type CardVariant = 'default' | 'panel' | 'summaryStrip';
type CardElement = 'div' | 'section';

interface Props extends React.HTMLAttributes<HTMLElement> {
  children: React.ReactNode;
  className?: string;
  variant?: CardVariant;
  as?: CardElement;
}

const variantClass: Record<CardVariant, string> = {
  default:
    'bg-(--theme-bg-default) backdrop-blur-md rounded-sm border border-(--theme-border-subtle) dark:border-(--theme-border-default) shadow-sm',
  panel:
    'bg-(--theme-surface-panel) border border-(--theme-border-subtle) dark:border-(--theme-border-default) rounded-xl shadow-sm',
  summaryStrip:
    'overflow-hidden rounded-2xl border border-(--theme-border-subtle) bg-(--theme-surface-summary) shadow-sm dark:border-(--theme-border-default)',
};

const Card: React.FC<Props> = ({
  children,
  className = '',
  variant = 'default',
  as: Component = 'div',
  ...props
}) => (
  <Component className={`${variantClass[variant]} ${className}`} {...props}>
    {children}
  </Component>
);

export default Card;
