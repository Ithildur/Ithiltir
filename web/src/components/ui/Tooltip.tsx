import React, { useState, useRef } from 'react';
import { createPortal } from 'react-dom';

export interface TooltipHandle {
  show: () => void;
}

interface Props {
  ref?: React.Ref<TooltipHandle>;
  content: React.ReactNode | (() => React.ReactNode);
  children: React.ReactNode;
  className?: string;
  variant?: 'default' | 'surface';
}

export const Tooltip: React.FC<Props> = ({
  ref,
  content,
  children,
  className,
  variant = 'default',
}) => {
  const [isVisible, setIsVisible] = useState(false);
  const [coords, setCoords] = useState({ top: 0, left: 0 });
  const triggerRef = useRef<HTMLDivElement>(null);
  const tooltipRef = useRef<HTMLDivElement>(null);
  const arrowRef = useRef<HTMLDivElement>(null);
  const frameRef = useRef<number | null>(null);
  const tooltipId = React.useId();
  const renderedContent = typeof content === 'function' ? (isVisible ? content() : null) : content;

  React.useLayoutEffect(() => {
    const tooltip = tooltipRef.current;
    const arrow = arrowRef.current;
    if (!isVisible || !tooltip || !arrow) return;
    const { width, height } = tooltip.getBoundingClientRect();
    const left = Math.max(8, Math.min(coords.left - width / 2, window.innerWidth - width - 8));
    const above = coords.top - height >= 8;
    const top = above
      ? coords.top - height
      : Math.min(coords.top + 20, window.innerHeight - height - 8);
    tooltip.style.left = `${left}px`;
    tooltip.style.top = `${Math.max(8, top)}px`;
    arrow.style.left = `${Math.max(8, Math.min(coords.left - left, width - 8))}px`;
    arrow.style.top = above ? '100%' : 'auto';
    arrow.style.bottom = above ? 'auto' : '100%';
    const background =
      variant === 'surface' ? 'var(--theme-bg-default)' : 'var(--theme-tooltip-bg)';
    arrow.style.borderTopColor = above ? background : 'transparent';
    arrow.style.borderBottomColor = above ? 'transparent' : background;
  }, [coords, isVisible, renderedContent, variant]);

  React.useLayoutEffect(() => {
    if (!renderedContent) return;
    const focused = document.activeElement;
    const target =
      focused instanceof HTMLElement && triggerRef.current?.contains(focused)
        ? focused
        : triggerRef.current?.querySelector<HTMLElement>(
            'button:not([disabled]), input:not([disabled]), a[href], select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
          );
    if (!target) return;

    const describedBy = new Set(
      (target.getAttribute('aria-describedby') ?? '').split(/\s+/).filter(Boolean),
    );
    describedBy.add(tooltipId);
    target.setAttribute('aria-describedby', [...describedBy].join(' '));

    return () => {
      const remaining = (target.getAttribute('aria-describedby') ?? '')
        .split(/\s+/)
        .filter((id) => id && id !== tooltipId);
      if (remaining.length > 0) {
        target.setAttribute('aria-describedby', remaining.join(' '));
      } else {
        target.removeAttribute('aria-describedby');
      }
    };
  }, [children, renderedContent, tooltipId]);

  React.useEffect(() => {
    if (!isVisible) return;
    const dismiss = () => setIsVisible(false);
    window.addEventListener('resize', dismiss);
    window.addEventListener('scroll', dismiss, true);
    return () => {
      window.removeEventListener('resize', dismiss);
      window.removeEventListener('scroll', dismiss, true);
    };
  }, [isVisible]);

  React.useEffect(
    () => () => {
      if (frameRef.current !== null) {
        window.cancelAnimationFrame(frameRef.current);
      }
    },
    [],
  );

  const positionFromRect = () => {
    if (frameRef.current !== null) {
      window.cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    }
    const rect = triggerRef.current?.getBoundingClientRect();
    if (!rect) return;
    setCoords({
      top: rect.top - 8,
      left: rect.left + rect.width / 2,
    });
  };

  const positionFromPointer = (event: React.PointerEvent<HTMLDivElement>) => {
    const next = {
      top: event.clientY - 10,
      left: event.clientX,
    };
    if (frameRef.current !== null) {
      window.cancelAnimationFrame(frameRef.current);
    }
    frameRef.current = window.requestAnimationFrame(() => {
      frameRef.current = null;
      setCoords(next);
    });
  };

  const show = (event?: React.PointerEvent<HTMLDivElement>) => {
    if (!content) return;
    if (event) {
      setCoords({ top: event.clientY - 10, left: event.clientX });
    } else {
      positionFromRect();
    }
    setIsVisible(true);
  };

  const hide = () => {
    if (frameRef.current !== null) {
      window.cancelAnimationFrame(frameRef.current);
      frameRef.current = null;
    }
    setIsVisible(false);
  };

  React.useImperativeHandle(ref, () => ({ show: () => show() }));

  return (
    <>
      <div
        ref={triggerRef}
        className={className}
        onPointerEnter={(event) => show(event)}
        onPointerMove={(event) => {
          if (isVisible) positionFromPointer(event);
        }}
        onPointerLeave={hide}
        onFocus={() => show()}
        onBlur={hide}
        onKeyDown={(event) => {
          if (event.key === 'Escape') hide();
        }}
      >
        {children}
      </div>
      {!isVisible && renderedContent ? (
        <span id={tooltipId} hidden>
          {renderedContent}
        </span>
      ) : null}
      {isVisible &&
        renderedContent &&
        createPortal(
          <div
            ref={tooltipRef}
            id={tooltipId}
            role="tooltip"
            className={`fixed z-9999 w-max max-w-[calc(100vw-1rem)] px-3 py-2 text-xs font-medium rounded-md pointer-events-none whitespace-pre-line text-left border ${variant === 'surface' ? 'text-(--theme-fg-default) bg-(--theme-bg-default) border-(--theme-border-default) shadow-md' : 'text-(--theme-tooltip-fg) bg-(--theme-tooltip-bg) border-(--theme-tooltip-border) shadow-xl'}`}
            style={{ top: coords.top, left: coords.left }}
          >
            {renderedContent}
            <div
              ref={arrowRef}
              className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-(--theme-tooltip-bg)"
            />
          </div>,
          document.body,
        )}
    </>
  );
};
