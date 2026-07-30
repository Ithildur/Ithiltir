import React from 'react';
import Check from 'lucide-react/dist/esm/icons/check';
import ChevronDown from 'lucide-react/dist/esm/icons/chevron-down';
import Filter from 'lucide-react/dist/esm/icons/filter';
import X from 'lucide-react/dist/esm/icons/x';

interface MultiSelectFilterItem {
  id: number;
  label: string;
  trailing?: React.ReactNode;
}

interface Props {
  items: MultiSelectFilterItem[];
  selectedIds: number[];
  onChange: (ids: number[]) => void;
  label: string;
  title: string;
  emptyLabel: string;
  clearLabel: string;
  closeLabel?: string;
  customTrigger?: React.ReactNode;
  align?: 'left' | 'right';
  direction?: 'up' | 'down';
  variant?: 'default' | 'fab';
}

const MultiSelectFilter: React.FC<Props> = ({
  items,
  selectedIds,
  onChange,
  label,
  title,
  emptyLabel,
  clearLabel,
  closeLabel = clearLabel,
  customTrigger,
  align = 'left',
  direction = 'down',
  variant = 'default',
}) => {
  const popupId = React.useId();
  const listboxId = React.useId();
  const titleId = React.useId();
  const [isOpen, setIsOpen] = React.useState(false);
  const containerRef = React.useRef<HTMLDivElement>(null);
  const popupRef = React.useRef<HTMLDivElement>(null);
  const closeButtonRef = React.useRef<HTMLButtonElement>(null);
  const optionFocusRequestedRef = React.useRef(false);
  const controlledId = variant === 'fab' ? popupId : listboxId;

  const trigger = React.useCallback(
    () => containerRef.current?.querySelector<HTMLElement>('[aria-haspopup]') ?? null,
    [],
  );

  const close = React.useCallback(
    (restoreFocus: boolean) => {
      const target = restoreFocus ? trigger() : null;
      setIsOpen(false);
      if (target) {
        requestAnimationFrame(() => target.focus());
      }
    },
    [trigger],
  );

  const focusOption = React.useCallback((position: 'first' | 'last') => {
    optionFocusRequestedRef.current = true;
    requestAnimationFrame(() => {
      const options = popupRef.current?.querySelectorAll<HTMLElement>('[data-filter-option]');
      if (options && options.length > 0) {
        options[position === 'first' ? 0 : options.length - 1].focus();
      }
      optionFocusRequestedRef.current = false;
    });
  }, []);

  React.useEffect(() => {
    if (!isOpen) return;
    const closeOnOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', closeOnOutside);
    return () => document.removeEventListener('mousedown', closeOnOutside);
  }, [isOpen]);

  React.useEffect(() => {
    if (!isOpen) return;
    const keyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      close(true);
    };
    document.addEventListener('keydown', keyDown);
    return () => document.removeEventListener('keydown', keyDown);
  }, [close, isOpen]);

  React.useEffect(() => {
    if (!isOpen || variant !== 'fab') return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    if (!optionFocusRequestedRef.current) {
      requestAnimationFrame(() => (closeButtonRef.current ?? popupRef.current)?.focus());
    }
    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, [isOpen, variant]);

  const toggleItem = (id: number) => {
    if (selectedIds.includes(id)) {
      onChange(selectedIds.filter((i) => i !== id));
    } else {
      onChange([...selectedIds, id]);
    }
  };

  const hasSelection = selectedIds.length > 0;

  const checkboxClass = (selected: boolean) =>
    `flex h-5 w-5 shrink-0 items-center justify-center rounded-[3px] border transition-[background-color,border-color] ${
      selected
        ? 'border-(--theme-bg-accent-emphasis) bg-(--theme-bg-accent-emphasis) text-(--theme-fg-on-emphasis)'
        : 'border-(--theme-border-default) bg-(--theme-bg-default) dark:bg-(--theme-bg-inset)'
    }`;

  const menuPosClass =
    direction === 'up'
      ? align === 'right'
        ? 'menu-pop-bottom-right'
        : 'menu-pop-bottom-left'
      : align === 'right'
        ? 'menu-pop-top-right'
        : 'menu-pop-top-left';

  const triggerClassName =
    'cursor-pointer select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--theme-focus-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--theme-bg-default) dark:focus-visible:ring-offset-(--theme-bg-default)';

  const handleTriggerKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      setIsOpen(true);
      focusOption(event.key === 'ArrowDown' ? 'first' : 'last');
      return;
    }
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      setIsOpen((current) => !current);
    }
  };

  const handlePopupKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.stopPropagation();
      close(true);
      return;
    }
    if (event.key === 'Tab' && variant === 'fab') {
      const popup = popupRef.current;
      if (!popup) return;
      const focusables = Array.from(
        popup.querySelectorAll<HTMLElement>(
          'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])',
        ),
      );
      if (focusables.length === 0) {
        event.preventDefault();
        popup.focus();
        return;
      }
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      const active = document.activeElement;
      if (event.shiftKey && (active === first || !popup.contains(active))) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
      return;
    }
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;

    const options = Array.from(
      popupRef.current?.querySelectorAll<HTMLElement>('[data-filter-option]') ?? [],
    );
    if (options.length === 0) return;
    event.preventDefault();
    const current = options.indexOf(document.activeElement as HTMLElement);
    let next = current;
    if (event.key === 'Home') next = 0;
    if (event.key === 'End') next = options.length - 1;
    if (event.key === 'ArrowDown') next = current < 0 ? 0 : (current + 1) % options.length;
    if (event.key === 'ArrowUp') {
      next = current < 0 ? options.length - 1 : (current - 1 + options.length) % options.length;
    }
    options[next].focus();
  };

  return (
    <div className="relative" ref={containerRef}>
      {customTrigger ? (
        React.isValidElement<React.HTMLAttributes<Element>>(customTrigger) ? (
          React.cloneElement(customTrigger, {
            onClick: (event: React.MouseEvent) => {
              customTrigger.props.onClick?.(event);
              if (!event.defaultPrevented) {
                setIsOpen((current) => !current);
              }
            },
            onKeyDown: (event: React.KeyboardEvent) => {
              customTrigger.props.onKeyDown?.(event);
              if (event.defaultPrevented) return;
              handleTriggerKeyDown(event);
            },
            'aria-label': customTrigger.props['aria-label'] ?? label,
            'aria-controls': controlledId,
            'aria-expanded': isOpen,
            'aria-haspopup': variant === 'fab' ? 'dialog' : 'listbox',
            className: `${customTrigger.props.className ?? ''} ${triggerClassName}`.trim(),
            role:
              customTrigger.props.role ??
              (typeof customTrigger.type === 'string' && customTrigger.type !== 'button'
                ? 'button'
                : undefined),
            tabIndex:
              customTrigger.props.tabIndex ??
              (typeof customTrigger.type === 'string' && customTrigger.type !== 'button'
                ? 0
                : undefined),
          })
        ) : (
          <div
            onClick={() => setIsOpen((current) => !current)}
            onKeyDown={handleTriggerKeyDown}
            role="button"
            tabIndex={0}
            aria-label={label}
            aria-controls={controlledId}
            aria-expanded={isOpen}
            aria-haspopup={variant === 'fab' ? 'dialog' : 'listbox'}
            className={triggerClassName}
          >
            {customTrigger}
          </div>
        )
      ) : (
        <button
          type="button"
          onClick={() => setIsOpen((current) => !current)}
          onKeyDown={handleTriggerKeyDown}
          aria-controls={controlledId}
          aria-expanded={isOpen}
          aria-haspopup={variant === 'fab' ? 'dialog' : 'listbox'}
          className={`
            flex items-center gap-2 px-3 py-1.5 text-sm font-medium rounded-md border transition-[background-color,border-color,box-shadow] shadow-sm
            ${
              isOpen || hasSelection
                ? 'bg-(--theme-bg-muted) dark:bg-(--theme-canvas-selected) border-(--theme-fg-accent)/50 text-(--theme-fg-default) dark:text-(--theme-fg-default) ring-1 ring-(--theme-fg-accent)/20'
                : 'bg-(--theme-bg-default) dark:bg-(--theme-canvas-muted) border-(--theme-border-default) dark:border-(--theme-border-default) text-(--theme-fg-default) dark:text-(--theme-fg-default) hover:bg-(--theme-bg-muted) dark:hover:bg-(--theme-border-default)'
            }
          `}
        >
          <Filter
            size={14}
            className={hasSelection ? 'text-(--theme-fg-accent)' : 'text-(--theme-fg-muted)'}
          />
          <span>{label}</span>
          {hasSelection && (
            <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-(--theme-bg-accent-muted) px-1 text-xs font-bold text-(--theme-fg-accent)">
              {selectedIds.length}
            </span>
          )}
          <ChevronDown size={12} className="text-(--theme-fg-muted) opacity-75" />
        </button>
      )}

      {isOpen && (
        <>
          {variant === 'fab' && (
            <div
              className="fixed inset-0 bg-(--theme-overlay-scrim-muted) backdrop-blur-sm z-40"
              onClick={(e) => {
                e.stopPropagation();
                close(true);
              }}
              onMouseDown={(e) => e.stopPropagation()}
              onTouchStart={(e) => e.stopPropagation()}
              aria-hidden="true"
            />
          )}
          <div
            ref={popupRef}
            id={variant === 'fab' ? popupId : undefined}
            role={variant === 'fab' ? 'dialog' : undefined}
            aria-modal={variant === 'fab' ? 'true' : undefined}
            aria-labelledby={variant === 'fab' ? titleId : undefined}
            tabIndex={variant === 'fab' ? -1 : undefined}
            className={`
            ${variant !== 'fab' ? `menu-pop menu-pop-anim ${menuPosClass}` : 'absolute z-50 bottom-full mb-4 right-0 w-80 max-w-[calc(100vw-1rem)] origin-bottom-right'}
            ${
              variant !== 'fab'
                ? ''
                : 'border border-(--theme-border-default) bg-(--theme-bg-default) dark:bg-(--theme-bg-inset)'
            }
            ${variant === 'fab' ? 'rounded-xl shadow-2xl touch-auto' : ''}
            flex flex-col overflow-hidden text-left
            `}
            onMouseDown={(e) => e.stopPropagation()}
            onTouchStart={(e) => e.stopPropagation()}
            onKeyDown={handlePopupKeyDown}
          >
            <div className="menu-head">
              <span
                id={titleId}
                className="text-sm font-bold text-(--theme-fg-default) dark:text-(--theme-fg-default)"
              >
                {title}
              </span>
              <button
                ref={closeButtonRef}
                type="button"
                onClick={() => close(true)}
                className="icon-ghost"
                aria-label={closeLabel}
                title={closeLabel}
              >
                <X size={16} strokeWidth={2.5} aria-hidden="true" />
              </button>
            </div>
            <div
              id={listboxId}
              role="listbox"
              aria-label={title}
              aria-multiselectable="true"
              className="max-h-[60vh] overflow-y-auto p-2 custom-scrollbar"
            >
              {items.length === 0 ? (
                <div className="px-3 py-4 text-sm text-(--theme-fg-muted)">{emptyLabel}</div>
              ) : (
                items.map((item) => {
                  const isSelected = selectedIds.includes(item.id);
                  return (
                    <button
                      type="button"
                      role="option"
                      data-filter-option
                      key={item.id}
                      aria-selected={isSelected}
                      onClick={() => toggleItem(item.id)}
                      className={isSelected ? 'menu-item gap-3' : 'menu-item menu-item-hover gap-3'}
                    >
                      <span className={checkboxClass(isSelected)} aria-hidden="true">
                        {isSelected && <Check size={14} strokeWidth={3} />}
                      </span>
                      <span className="min-w-0 flex-1 truncate">{item.label}</span>
                      {item.trailing}
                    </button>
                  );
                })
              )}
            </div>
          </div>
        </>
      )}
    </div>
  );
};

export default MultiSelectFilter;
