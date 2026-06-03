import React from 'react';
import type { LucideIcon } from 'lucide-react';

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  icon?: LucideIcon;
  wrapperClassName?: string;
  rightElement?: React.ReactNode;
  rightElementClassName?: string;
}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  (
    {
      icon: Icon,
      wrapperClassName = '',
      className = '',
      rightElement,
      rightElementClassName = '',
      ...props
    },
    ref,
  ) => {
    return (
      <div className={`relative group/input ${wrapperClassName}`}>
        {Icon && (
          <Icon
            size={16}
            className="absolute left-3 top-1/2 -translate-y-1/2 text-(--theme-fg-subtle) group-focus-within/input:text-(--theme-fg-accent) transition-colors z-10 pointer-events-none"
          />
        )}
        <input
          ref={ref}
          className={`w-full bg-(--theme-surface-info)/70 backdrop-blur-sm border border-(--theme-border-subtle) dark:border-(--theme-border-default) rounded-md py-1.25 ${
            Icon ? 'pl-9' : 'pl-3'
          } ${rightElement ? 'pr-9' : 'pr-3'} dark:bg-(--theme-bg-default) text-sm/5 text-(--theme-fg-default) dark:text-(--theme-fg-default) placeholder-(--theme-fg-subtle) focus:outline-none focus:ring-1 focus:ring-(--theme-bg-accent-emphasis) focus:border-(--theme-bg-accent-emphasis) focus:bg-(--theme-bg-default) dark:focus:bg-(--theme-bg-default)/25 transition-[background-color,border-color,box-shadow] shadow-sm ${className}`}
          {...props}
        />
        {rightElement && (
          <div className={`absolute right-3 inset-y-0 flex items-center ${rightElementClassName}`}>
            {rightElement}
          </div>
        )}
      </div>
    );
  },
);

Input.displayName = 'Input';

export default Input;
