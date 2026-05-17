import React from 'react';
import Check from 'lucide-react/dist/esm/icons/check';

interface Props extends Omit<
  React.InputHTMLAttributes<HTMLInputElement>,
  'type' | 'className' | 'size'
> {
  children?: React.ReactNode;
  className?: string;
  controlClassName?: string;
  indeterminate?: boolean;
  size?: 'sm' | 'md';
}

const Checkbox: React.FC<Props> = ({
  children,
  className = '',
  controlClassName = '',
  indeterminate = false,
  size = 'sm',
  disabled,
  ...props
}) => {
  const inputRef = React.useRef<HTMLInputElement>(null);
  const boxSize = size === 'md' ? 'h-5 w-5' : 'h-4 w-4';
  const iconSize = size === 'md' ? 14 : 12;

  React.useEffect(() => {
    if (inputRef.current) {
      inputRef.current.indeterminate = indeterminate;
    }
  }, [indeterminate]);

  return (
    <label
      className={`inline-flex items-center gap-2 align-middle ${
        disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'
      } ${className}`}
    >
      <input
        {...props}
        ref={inputRef}
        type="checkbox"
        disabled={disabled}
        aria-checked={indeterminate ? 'mixed' : props['aria-checked']}
        className="peer sr-only"
      />
      <span
        aria-hidden="true"
        className={`
          pointer-events-none flex shrink-0 items-center justify-center rounded-[3px] border border-(--theme-border-default)
          bg-(--theme-bg-default) text-(--theme-fg-on-emphasis) shadow-[inset_0_1px_0_var(--theme-control-inset-strong)]
          transition-[background-color,border-color,box-shadow]
          peer-checked:border-(--theme-bg-accent-emphasis) peer-checked:bg-(--theme-bg-accent-emphasis)
          peer-checked:[&_svg]:opacity-100 peer-focus-visible:ring-2 peer-focus-visible:ring-(--theme-bg-accent-emphasis)/30
          dark:bg-(--theme-bg-inset)
          ${indeterminate ? 'border-(--theme-bg-accent-emphasis) bg-(--theme-bg-accent-emphasis)' : ''}
          ${boxSize} ${controlClassName}
        `}
      >
        {indeterminate ? (
          <span className="h-0.5 w-2 rounded-full bg-(--theme-bg-default)" />
        ) : (
          <Check size={iconSize} strokeWidth={3} className="opacity-0 transition-opacity" />
        )}
      </span>
      {children}
    </label>
  );
};

export default Checkbox;
