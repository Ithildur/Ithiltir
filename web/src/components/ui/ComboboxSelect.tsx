import React from 'react';
import Check from 'lucide-react/dist/esm/icons/check';
import ChevronDown from 'lucide-react/dist/esm/icons/chevron-down';
import X from 'lucide-react/dist/esm/icons/x';
import { createPortal } from 'react-dom';

export interface ComboboxOption {
  value: string;
  label: string;
  keywords?: string[];
}

interface Props {
  value: string;
  options: ComboboxOption[];
  ariaLabel: string;
  clearLabel: string;
  emptyLabel: string;
  className?: string;
  disabled?: boolean;
  searchable?: boolean;
  onChange: (value: string) => void;
}

interface MenuRect {
  top: number;
  left: number;
  width: number;
  maxHeight: number;
}

const normalize = (value: string): string => value.trim().toLowerCase();

const optionMatches = (option: ComboboxOption, query: string): boolean => {
  if (!query) return true;
  if (normalize(option.label).includes(query)) return true;
  if (normalize(option.value).includes(query)) return true;
  return option.keywords?.some((keyword) => normalize(keyword).includes(query)) ?? false;
};

const ComboboxSelect: React.FC<Props> = ({
  value,
  options,
  ariaLabel,
  clearLabel,
  emptyLabel,
  className = '',
  disabled,
  searchable = true,
  onChange,
}) => {
  const listboxId = React.useId();
  const containerRef = React.useRef<HTMLDivElement>(null);
  const menuRef = React.useRef<HTMLDivElement>(null);
  const inputRef = React.useRef<HTMLInputElement>(null);
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState('');
  const [queryDirty, setQueryDirty] = React.useState(false);
  const [menuRect, setMenuRect] = React.useState<MenuRect | null>(null);

  const selected = React.useMemo(
    () => options.find((option) => option.value === value) ?? options[0],
    [options, value],
  );
  const selectedLabel = selected?.label ?? '';

  React.useEffect(() => {
    if (!open) {
      setQuery('');
      setQueryDirty(false);
    }
  }, [open]);

  const updateMenuRect = React.useCallback(() => {
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) return;

    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    const edgeGap = 8;
    const menuGap = 4;
    const width = Math.min(rect.width, viewportWidth - edgeGap * 2);
    const left = Math.min(
      Math.max(edgeGap, rect.left),
      Math.max(edgeGap, viewportWidth - width - edgeGap),
    );
    const spaceBelow = viewportHeight - rect.bottom - edgeGap;
    const spaceAbove = rect.top - edgeGap;
    const openUp = spaceBelow < 180 && spaceAbove > spaceBelow;
    const available = Math.max(120, (openUp ? spaceAbove : spaceBelow) - menuGap);
    const maxHeight = Math.min(256, available);
    setMenuRect({
      top: openUp ? Math.max(edgeGap, rect.top - menuGap - maxHeight) : rect.bottom + menuGap,
      left,
      width,
      maxHeight,
    });
  }, []);

  React.useLayoutEffect(() => {
    if (!open) {
      setMenuRect(null);
      return;
    }
    updateMenuRect();
  }, [open, updateMenuRect]);

  React.useEffect(() => {
    if (!open) return;

    const closeOnOutside = (event: MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (containerRef.current?.contains(target)) return;
      if (menuRef.current?.contains(target)) return;
      setOpen(false);
    };
    document.addEventListener('mousedown', closeOnOutside);
    return () => document.removeEventListener('mousedown', closeOnOutside);
  }, [open]);

  React.useEffect(() => {
    if (!open) return;

    window.addEventListener('resize', updateMenuRect);
    window.addEventListener('scroll', updateMenuRect, true);
    return () => {
      window.removeEventListener('resize', updateMenuRect);
      window.removeEventListener('scroll', updateMenuRect, true);
    };
  }, [open, updateMenuRect]);

  const filteredOptions = React.useMemo(() => {
    const normalizedQuery = searchable && queryDirty ? normalize(query) : '';
    return options.filter((option) => optionMatches(option, normalizedQuery)).slice(0, 80);
  }, [options, query, queryDirty, searchable]);

  const selectValue = React.useCallback(
    (nextValue: string) => {
      onChange(nextValue);
      setOpen(false);
      setQuery('');
      setQueryDirty(false);
    },
    [onChange],
  );

  const clearQuery = React.useCallback(() => {
    if (!searchable) return;
    setQuery('');
    setQueryDirty(true);
    setOpen(true);
    inputRef.current?.focus();
  }, [searchable]);

  const menu =
    open && !disabled && menuRect && typeof document !== 'undefined'
      ? createPortal(
          <div
            ref={menuRef}
            id={listboxId}
            role="listbox"
            className="fixed z-50 overflow-y-auto rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-default) py-1 text-sm shadow-lg dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)"
            style={{
              top: menuRect.top,
              left: menuRect.left,
              width: menuRect.width,
              maxHeight: menuRect.maxHeight,
            }}
          >
            {filteredOptions.length > 0 ? (
              filteredOptions.map((option) => {
                const isSelected = option.value === value;
                return (
                  <button
                    key={option.value}
                    type="button"
                    role="option"
                    aria-selected={isSelected}
                    className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-(--theme-fg-default) hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)"
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => selectValue(option.value)}
                  >
                    <span className="min-w-0 truncate">{option.label}</span>
                    {isSelected ? (
                      <Check
                        className="size-4 shrink-0 text-(--theme-bg-accent-emphasis)"
                        aria-hidden="true"
                      />
                    ) : null}
                  </button>
                );
              })
            ) : (
              <div className="px-3 py-2 text-(--theme-fg-muted)">{emptyLabel}</div>
            )}
          </div>,
          document.body,
        )
      : null;

  return (
    <div
      ref={containerRef}
      className={`relative ${className}`}
      onBlur={(event) => {
        const nextTarget = event.relatedTarget;
        if (!(nextTarget instanceof Node) || !containerRef.current?.contains(nextTarget)) {
          setOpen(false);
        }
      }}
    >
      <div className="relative">
        <input
          ref={inputRef}
          value={open ? query : selectedLabel}
          disabled={disabled}
          readOnly={!searchable}
          aria-label={ariaLabel}
          aria-autocomplete="list"
          aria-controls={listboxId}
          aria-expanded={open}
          role="combobox"
          className="w-full rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-default) px-3 py-1.25 pr-9 text-sm/5 text-(--theme-fg-default) outline-none transition-[background-color,border-color,box-shadow] placeholder:text-(--theme-fg-subtle) focus:border-(--theme-bg-accent-emphasis) focus:ring-1 focus:ring-(--theme-bg-accent-emphasis) disabled:cursor-not-allowed disabled:opacity-60 dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)"
          onFocus={(event) => {
            setQuery(selectedLabel);
            setQueryDirty(false);
            setOpen(true);
            event.currentTarget.setSelectionRange(0, event.currentTarget.value.length);
          }}
          onChange={(event) => {
            if (!searchable) return;
            setQuery(event.target.value);
            setQueryDirty(true);
            setOpen(true);
          }}
          onKeyDown={(event) => {
            if (event.key === 'Escape') {
              setOpen(false);
              return;
            }
            if (event.key === 'ArrowDown') {
              event.preventDefault();
              setOpen(true);
              return;
            }
            if (event.key === 'Enter' && open) {
              event.preventDefault();
              const nextValue = searchable && queryDirty ? filteredOptions[0]?.value : value;
              if (nextValue !== undefined) selectValue(nextValue);
            }
          }}
        />
        {searchable && open && queryDirty && query ? (
          <button
            type="button"
            tabIndex={-1}
            disabled={disabled}
            aria-label={clearLabel}
            className="absolute inset-y-0 right-1.5 grid w-7 place-items-center rounded text-(--theme-fg-muted) transition-colors hover:text-(--theme-fg-default) disabled:pointer-events-none"
            onMouseDown={(event) => event.preventDefault()}
            onClick={clearQuery}
          >
            <X className="size-4" aria-hidden="true" />
          </button>
        ) : (
          <button
            type="button"
            tabIndex={-1}
            disabled={disabled}
            aria-hidden="true"
            className="absolute inset-y-0 right-1.5 grid w-7 place-items-center rounded text-(--theme-fg-muted) transition-colors hover:text-(--theme-fg-default) disabled:pointer-events-none"
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              setQuery('');
              setQueryDirty(false);
              if (open) {
                setOpen(false);
                inputRef.current?.blur();
                return;
              }
              setOpen(true);
              inputRef.current?.focus();
            }}
          >
            <ChevronDown className="size-4" aria-hidden="true" />
          </button>
        )}
      </div>

      {menu}
    </div>
  );
};

export default ComboboxSelect;
