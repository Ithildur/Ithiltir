import React, { useState, useRef } from 'react';
import { createPortal } from 'react-dom';

interface Props {
  content: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export const Tooltip: React.FC<Props> = ({ content, children, className }) => {
  const [isVisible, setIsVisible] = useState(false);
  const [coords, setCoords] = useState({ top: 0, left: 0 });
  const triggerRef = useRef<HTMLDivElement>(null);
  const frameRef = useRef<number | null>(null);
  const tooltipId = React.useId();

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
      positionFromPointer(event);
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
        aria-describedby={isVisible && content ? tooltipId : undefined}
      >
        {children}
      </div>
      {isVisible &&
        content &&
        createPortal(
          <div
            id={tooltipId}
            role="tooltip"
            className="fixed z-9999 px-3 py-2 text-xs font-medium text-(--theme-tooltip-fg) bg-(--theme-tooltip-bg) rounded-md shadow-xl pointer-events-none transform -translate-x-1/2 -translate-y-full whitespace-pre-line text-left border border-(--theme-tooltip-border)"
            style={{ top: coords.top, left: coords.left }}
          >
            {content}
            <div className="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-(--theme-tooltip-bg)" />
          </div>,
          document.body,
        )}
    </>
  );
};
