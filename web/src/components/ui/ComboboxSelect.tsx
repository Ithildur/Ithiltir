import React from 'react';
import Check from 'lucide-react/dist/esm/icons/check';
import ChevronDown from 'lucide-react/dist/esm/icons/chevron-down';

export interface ComboboxOption {
  value: string;
  label: string;
  keywords?: string[];
}

interface Props {
  value: string;
  options: ComboboxOption[];
  ariaLabel: string;
  emptyLabel: string;
  className?: string;
  disabled?: boolean;
  onChange: (value: string) => void;
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
  emptyLabel,
  className = '',
  disabled,
  onChange,
}) => {
  const listboxId = React.useId();
  const containerRef = React.useRef<HTMLDivElement>(null);
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState('');

  const selected = React.useMemo(
    () => options.find((option) => option.value === value) ?? options[0],
    [options, value],
  );
  const selectedLabel = selected?.label ?? '';

  React.useEffect(() => {
    if (!open) setQuery('');
  }, [open]);

  const filteredOptions = React.useMemo(() => {
    const normalizedQuery = normalize(query);
    return options.filter((option) => optionMatches(option, normalizedQuery)).slice(0, 80);
  }, [options, query]);

  const selectValue = React.useCallback(
    (nextValue: string) => {
      onChange(nextValue);
      setOpen(false);
      setQuery('');
    },
    [onChange],
  );

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
          value={open ? query : selectedLabel}
          disabled={disabled}
          aria-label={ariaLabel}
          aria-autocomplete="list"
          aria-controls={listboxId}
          aria-expanded={open}
          role="combobox"
          className="w-full rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-default) px-3 py-1.25 pr-9 text-sm/5 text-(--theme-fg-default) outline-none transition-[background-color,border-color,box-shadow] placeholder:text-(--theme-fg-subtle) focus:border-(--theme-bg-accent-emphasis) focus:ring-1 focus:ring-(--theme-bg-accent-emphasis) disabled:cursor-not-allowed disabled:opacity-60 dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)"
          onFocus={(event) => {
            setQuery(selectedLabel);
            setOpen(true);
            event.currentTarget.select();
          }}
          onChange={(event) => {
            setQuery(event.target.value);
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
              const nextValue = filteredOptions[0]?.value;
              if (nextValue !== undefined) selectValue(nextValue);
            }
          }}
        />
        <button
          type="button"
          tabIndex={-1}
          disabled={disabled}
          aria-hidden="true"
          className="absolute inset-y-0 right-1.5 grid w-7 place-items-center rounded text-(--theme-fg-muted) transition-colors hover:text-(--theme-fg-default) disabled:pointer-events-none"
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => {
            setQuery('');
            setOpen((current) => !current);
          }}
        >
          <ChevronDown className="size-4" aria-hidden="true" />
        </button>
      </div>

      {open && !disabled ? (
        <div
          id={listboxId}
          role="listbox"
          className="absolute z-30 mt-1 max-h-64 w-full min-w-56 overflow-y-auto rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-default) py-1 text-sm shadow-lg dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)"
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
        </div>
      ) : null}
    </div>
  );
};

export default ComboboxSelect;
