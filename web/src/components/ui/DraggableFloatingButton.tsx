import React, { useState, useEffect, useRef, useCallback } from 'react';

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
  const positionRef = useRef<Position | null>(null);
  const dragDistanceRef = useRef(0);

  useEffect(() => {
    positionRef.current = position;
  }, [position]);

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
      return {
        x: prev.x + dx,
        y: prev.y + dy,
      };
    });

    dragStartPos.current = { x: clientX, y: clientY };
  }, []);

  const endDrag = useCallback(() => {
    setIsDragging(false);
    dragStartPos.current = null;

    if (positionRef.current && buttonRef.current) {
      const rect = buttonRef.current.getBoundingClientRect();
      const winW = window.innerWidth;
      const winH = window.innerHeight;

      const clampedPosition = {
        x: Math.max(0, Math.min(winW - rect.width, positionRef.current.x)),
        y: Math.max(0, Math.min(winH - rect.height, positionRef.current.y)),
      };

      if (
        clampedPosition.x !== positionRef.current.x ||
        clampedPosition.y !== positionRef.current.y
      ) {
        setPosition(clampedPosition);
      }

      writeStoredPosition(storageKey, clampedPosition);
    }
  }, [storageKey]);

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
    if (dragDistanceRef.current < 5 && onClick) {
      onClick();
    }
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
      onClick={click}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          click();
        }
      }}
      role="button"
      tabIndex={0}
      aria-label={ariaLabel}
    >
      {children}
    </div>
  );
};
