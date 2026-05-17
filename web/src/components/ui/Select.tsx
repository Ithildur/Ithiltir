import React from 'react';

interface Props extends React.SelectHTMLAttributes<HTMLSelectElement> {
  className?: string;
  width?: 'auto' | 'full';
}

const Select: React.FC<Props> = ({ className = '', width = 'full', children, ...props }) => (
  <select
    className={`${width === 'full' ? 'w-full' : 'w-auto'} px-3 py-1.25 rounded-md border border-(--theme-border-subtle) dark:border-(--theme-border-default) bg-(--theme-bg-default) dark:bg-(--theme-bg-inset) text-sm/5 text-(--theme-fg-default) dark:text-(--theme-fg-default) focus:ring-1 focus:ring-(--theme-bg-accent-emphasis) focus:border-(--theme-bg-accent-emphasis) outline-none transition-[background-color,border-color,box-shadow] ${className}`}
    {...props}
  >
    {children}
  </select>
);

export default Select;
