import React, { useState, useEffect, useLayoutEffect, useRef, useCallback } from 'react';

interface Position {
  x: number;
  y: number;
}

function isValidPosition(value: unknown): value is Position {
  return (
    typeof value === 'object' &&
    value !== null &&
    'x' in value &&
    'y' in value &&
    typeof (value as Position).x === 'number' &&
    typeof (value as Position).y === 'number' &&
    !isNaN((value as Position).x) &&
    !isNaN((value as Position).y)
  );
}

function readStoredPosition(storageKey: string): Position | null {
  if (typeof window === 'undefined') return null;
  try {
    const saved = window.localStorage.getItem(storageKey);
    if (!saved) return null;
    const parsed: unknown = JSON.parse(saved);
    return isValidPosition(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function writeStoredPosition(storageKey: string, position: Position): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(position));
  } catch {
    // Storage may be disabled or full; dragging should still work.
  }
}

interface Props {
  children: React.ReactNode;
  onClick?: () => void;
  storageKey: string;
  className?: string;
  ariaLabel?: string;
}

export const DraggableFloatingButton: React.FC<Props> = ({
  children,
  onClick,
  storageKey,
  className = '',
  ariaLabel,
}) => {
  // null keeps the CSS default position until the first drag.
  const [position, setPosition] = useState<Position | null>(() => readStoredPosition(storageKey));

  const [isDragging, setIsDragging] = useState(false);
  const dragStartPos = useRef<Position | null>(null);
  const buttonRef = useRef<HTMLDivElement>(null);
  const positionRef = useRef<Position | null>(position);
  const dragDistanceRef = useRef(0);
  const suppressClickUntilRef = useRef(0);

  const clampCurrentPosition = useCallback(() => {
    const current = positionRef.current;
    const button = buttonRef.current;
    if (!current || !button) return;

    const rect = button.getBoundingClientRect();
    const clamped = {
      x: Math.max(0, Math.min(Math.max(0, window.innerWidth - rect.width), current.x)),
      y: Math.max(0, Math.min(Math.max(0, window.innerHeight - rect.height), current.y)),
    };
    positionRef.current = clamped;
    if (clamped.x !== current.x || clamped.y !== current.y) {
      setPosition(clamped);
    }
    writeStoredPosition(storageKey, clamped);
  }, [storageKey]);

  useLayoutEffect(() => {
    clampCurrentPosition();
  }, [clampCurrentPosition]);

  useEffect(() => {
    window.addEventListener('resize', clampCurrentPosition);
    return () => window.removeEventListener('resize', clampCurrentPosition);
  }, [clampCurrentPosition]);

  const startDrag = useCallback(
    (clientX: number, clientY: number) => {
      if (position === null && buttonRef.current) {
        const rect = buttonRef.current.getBoundingClientRect();
        setPosition({ x: rect.left, y: rect.top });
        positionRef.current = { x: rect.left, y: rect.top };
      }
      setIsDragging(true);
      dragStartPos.current = { x: clientX, y: clientY };
      dragDistanceRef.current = 0;
    },
    [position],
  );

  const moveDrag = useCallback((clientX: number, clientY: number) => {
    if (!dragStartPos.current) return;

    const dx = clientX - dragStartPos.current.x;
    const dy = clientY - dragStartPos.current.y;

    dragDistanceRef.current += Math.abs(dx) + Math.abs(dy);

    setPosition((prev) => {
      if (!prev) return null;
      const next = {
        x: prev.x + dx,
        y: prev.y + dy,
      };
      positionRef.current = next;
      return next;
    });

    dragStartPos.current = { x: clientX, y: clientY };
  }, []);

  const endDrag = useCallback(() => {
    if (dragDistanceRef.current >= 5) {
      suppressClickUntilRef.current = Date.now() + 500;
    }
    dragDistanceRef.current = 0;
    setIsDragging(false);
    dragStartPos.current = null;
    clampCurrentPosition();
  }, [clampCurrentPosition]);

  const onMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    startDrag(e.clientX, e.clientY);
  };

  useEffect(() => {
    if (!isDragging) return;

    const onMouseMove = (e: MouseEvent) => {
      e.preventDefault();
      moveDrag(e.clientX, e.clientY);
    };

    const onMouseUp = () => endDrag();

    window.addEventListener('mousemove', onMouseMove, { passive: false });
    window.addEventListener('mouseup', onMouseUp);

    return () => {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    };
  }, [isDragging, moveDrag, endDrag]);

  const onTouchStart = (e: React.TouchEvent) => {
    e.stopPropagation();
    startDrag(e.touches[0].clientX, e.touches[0].clientY);
  };

  useEffect(() => {
    if (!isDragging) return;

    const onTouchMove = (e: TouchEvent) => {
      e.preventDefault();
      if (e.touches.length > 0) {
        moveDrag(e.touches[0].clientX, e.touches[0].clientY);
      }
    };

    const onTouchEnd = () => endDrag();

    window.addEventListener('touchmove', onTouchMove, { passive: false });
    window.addEventListener('touchend', onTouchEnd);
    window.addEventListener('touchcancel', onTouchEnd);

    return () => {
      window.removeEventListener('touchmove', onTouchMove);
      window.removeEventListener('touchend', onTouchEnd);
      window.removeEventListener('touchcancel', onTouchEnd);
    };
  }, [isDragging, moveDrag, endDrag]);

  const click = () => {
    if (Date.now() > suppressClickUntilRef.current && onClick) {
      onClick();
    }
  };

  const suppressDraggedClick = (event: React.MouseEvent) => {
    if (Date.now() > suppressClickUntilRef.current) return;
    event.preventDefault();
    event.stopPropagation();
  };

  const positionStyle: React.CSSProperties = position
    ? { left: position.x, top: position.y }
    : { right: 4, bottom: 4 };

  return (
    <div
      ref={buttonRef}
      className={`fixed z-50 touch-none select-none ${className}`}
      style={{
        ...positionStyle,
        cursor: isDragging ? 'grabbing' : 'grab',
      }}
      onMouseDown={onMouseDown}
      onTouchStart={onTouchStart}
      onClickCapture={suppressDraggedClick}
      onClick={click}
    >
      {onClick ? (
        <button
          type="button"
          className="pointer-events-none absolute inset-0 rounded-full bg-transparent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--theme-focus-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--theme-bg-default)"
          aria-label={ariaLabel}
          onClick={(event) => {
            event.stopPropagation();
            click();
          }}
        />
      ) : null}
      {children}
    </div>
  );
};
