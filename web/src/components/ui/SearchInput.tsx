import React from 'react';
import Input, { type InputProps } from './Input';

const searchShortcutHint = (
  <kbd className="hidden sm:inline-flex items-center h-5 rounded border border-(--theme-border-subtle) bg-(--theme-surface-control-strong) px-1.5 text-[10px] font-mono leading-none text-(--theme-fg-muted) dark:border-(--theme-border-default) dark:bg-(--theme-bg-default) dark:text-(--theme-fg-neutral)">
    /
  </kbd>
);

const SearchInput = React.forwardRef<HTMLInputElement, InputProps>(
  ({ rightElement, rightElementClassName = '', ...props }, ref) => {
    const hasCustomRightElement = rightElement !== undefined;

    return (
      <Input
        ref={ref}
        {...props}
        data-search-input="true"
        rightElement={rightElement ?? searchShortcutHint}
        rightElementClassName={
          hasCustomRightElement
            ? rightElementClassName
            : `pointer-events-none ${rightElementClassName}`.trim()
        }
      />
    );
  },
);

SearchInput.displayName = 'SearchInput';

export default SearchInput;
