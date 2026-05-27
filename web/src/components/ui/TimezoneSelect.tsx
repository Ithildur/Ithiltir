import React from 'react';
import Check from 'lucide-react/dist/esm/icons/check';
import ChevronDown from 'lucide-react/dist/esm/icons/chevron-down';

const fallbackTimeZones = [
  'UTC',
  'Asia/Shanghai',
  'Asia/Hong_Kong',
  'Asia/Taipei',
  'Asia/Singapore',
  'Asia/Tokyo',
  'Europe/London',
  'Europe/Berlin',
  'America/New_York',
  'America/Los_Angeles',
];

const getTimeZones = (): string[] => {
  const supportedValuesOf = (
    Intl as typeof Intl & { supportedValuesOf?: (input: string) => string[] }
  ).supportedValuesOf;
  const supported = supportedValuesOf ? supportedValuesOf('timeZone') : fallbackTimeZones;
  return Array.from(new Set([...fallbackTimeZones, ...supported]));
};

interface Props {
  value: string;
  disabled?: boolean;
  ariaLabel: string;
  placeholder: string;
  systemLabel: string;
  emptyLabel: string;
  onChange: (value: string) => void;
}

const TimezoneSelect: React.FC<Props> = ({
  value,
  disabled,
  ariaLabel,
  placeholder,
  systemLabel,
  emptyLabel,
  onChange,
}) => {
  const listboxId = React.useId();
  const containerRef = React.useRef<HTMLDivElement>(null);
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState(value);

  React.useEffect(() => {
    if (!open) setQuery(value);
  }, [open, value]);

  const options = React.useMemo(() => {
    const trimmed = value.trim();
    const zones = getTimeZones();
    if (trimmed && !zones.includes(trimmed)) return [trimmed, ...zones];
    return zones;
  }, [value]);

  const normalizedQuery = query.trim().toLowerCase();
  const systemMatched = !normalizedQuery || systemLabel.toLowerCase().includes(normalizedQuery);
  const filteredOptions = React.useMemo(() => {
    if (!normalizedQuery) return options.slice(0, 80);
    return options
      .filter((timeZone) => timeZone.toLowerCase().includes(normalizedQuery))
      .slice(0, 80);
  }, [normalizedQuery, options]);

  const selectValue = React.useCallback(
    (nextValue: string) => {
      onChange(nextValue);
      setQuery(nextValue);
      setOpen(false);
    },
    [onChange],
  );

  return (
    <div
      ref={containerRef}
      className="relative"
      onBlur={(event) => {
        const nextTarget = event.relatedTarget;
        if (!(nextTarget instanceof Node) || !containerRef.current?.contains(nextTarget)) {
          setOpen(false);
        }
      }}
    >
      <div className="relative">
        <input
          value={open ? query : value}
          disabled={disabled}
          aria-label={ariaLabel}
          aria-autocomplete="list"
          aria-controls={listboxId}
          aria-expanded={open}
          role="combobox"
          placeholder={placeholder}
          className="w-full rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-default) px-3 py-1.25 pr-9 text-sm/5 text-(--theme-fg-default) outline-none transition-[background-color,border-color,box-shadow] placeholder:text-(--theme-fg-subtle) focus:border-(--theme-bg-accent-emphasis) focus:ring-1 focus:ring-(--theme-bg-accent-emphasis) disabled:cursor-not-allowed disabled:opacity-60 dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)"
          onFocus={() => {
            setQuery(value);
            setOpen(true);
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
              const nextValue = systemMatched ? '' : filteredOptions[0];
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
          onClick={() => setOpen((current) => !current)}
        >
          <ChevronDown className="size-4" aria-hidden="true" />
        </button>
      </div>

      {open && !disabled ? (
        <div
          id={listboxId}
          role="listbox"
          className="absolute z-30 mt-1 max-h-64 w-full min-w-64 overflow-y-auto rounded-md border border-(--theme-border-subtle) bg-(--theme-bg-default) py-1 text-sm shadow-lg dark:border-(--theme-border-default) dark:bg-(--theme-bg-inset)"
        >
          {systemMatched ? (
            <button
              type="button"
              role="option"
              aria-selected={!value}
              className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-(--theme-fg-default) hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => selectValue('')}
            >
              <span>{systemLabel}</span>
              {!value ? (
                <Check className="size-4 text-(--theme-bg-accent-emphasis)" aria-hidden="true" />
              ) : null}
            </button>
          ) : null}
          {filteredOptions.map((timeZone) => (
            <button
              key={timeZone}
              type="button"
              role="option"
              aria-selected={value === timeZone}
              className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-(--theme-fg-default) hover:bg-(--theme-surface-row-hover) dark:hover:bg-(--theme-canvas-subtle)"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => selectValue(timeZone)}
            >
              <span className="truncate">{timeZone}</span>
              {value === timeZone ? (
                <Check
                  className="size-4 shrink-0 text-(--theme-bg-accent-emphasis)"
                  aria-hidden="true"
                />
              ) : null}
            </button>
          ))}
          {!systemMatched && filteredOptions.length === 0 ? (
            <div className="px-3 py-2 text-(--theme-fg-muted)">{emptyLabel}</div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
};

export default TimezoneSelect;
