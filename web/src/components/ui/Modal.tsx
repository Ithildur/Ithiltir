import React from 'react';
import { createPortal } from 'react-dom';
import X from 'lucide-react/dist/esm/icons/x';
import { useI18n } from '@i18n';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  children: React.ReactNode;
  className?: string;
  maxWidth?: string;
  zIndex?: number;
  ariaLabelledby?: string;
  ariaLabel?: string;
}

export const Modal: React.FC<ModalProps> = ({
  isOpen,
  onClose,
  children,
  className = '',
  maxWidth = 'max-w-2xl',
  zIndex = 50,
  ariaLabelledby,
  ariaLabel,
}) => {
  const dialogRef = React.useRef<HTMLDivElement>(null);
  const lastActiveRef = React.useRef<HTMLElement | null>(null);

  React.useEffect(() => {
    if (!isOpen) return;
    lastActiveRef.current = document.activeElement as HTMLElement | null;
    const dialog = dialogRef.current;
    if (!dialog) return;

    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    const focusableSelector =
      'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';
    const focusables = Array.from(dialog.querySelectorAll<HTMLElement>(focusableSelector));
    const target = focusables[0] ?? dialog;
    target.focus();

    return () => {
      document.body.style.overflow = prevOverflow;
      lastActiveRef.current?.focus?.();
    };
  }, [isOpen]);

  if (!isOpen) return null;

  const keyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.stopPropagation();
      onClose();
      return;
    }
    if (event.key !== 'Tab') return;

    const dialog = dialogRef.current;
    if (!dialog) return;
    const focusableSelector =
      'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';
    const focusables = Array.from(dialog.querySelectorAll<HTMLElement>(focusableSelector)).filter(
      (el) => !el.hasAttribute('disabled') && !el.getAttribute('aria-hidden'),
    );
    if (focusables.length === 0) {
      event.preventDefault();
      dialog.focus();
      return;
    }
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    const isShift = event.shiftKey;
    const active = document.activeElement as HTMLElement | null;

    if (isShift && (active === first || !dialog.contains(active))) {
      event.preventDefault();
      last.focus();
      return;
    }
    if (!isShift && active === last) {
      event.preventDefault();
      first.focus();
    }
  };

  return createPortal(
    <div
      className="fixed inset-0 flex items-center justify-center p-4 sm:p-6 animate-in fade-in duration-200 motion-reduce:animate-none"
      style={{ zIndex }}
    >
      <div
        className="absolute inset-0 bg-(--theme-overlay-scrim) backdrop-blur-sm transition-opacity motion-reduce:transition-none"
        onClick={onClose}
      />
      <div
        ref={dialogRef}
        className={`relative w-full ${maxWidth} bg-(--theme-bg-default) dark:bg-(--theme-bg-inset) rounded-xl shadow-2xl ring-1 ring-(--theme-border-subtle) dark:ring-(--theme-border-default) overflow-hidden flex flex-col max-h-[90vh] animate-in zoom-in-95 duration-200 motion-reduce:animate-none ${className}`}
        role="dialog"
        aria-modal="true"
        aria-labelledby={ariaLabelledby}
        aria-label={ariaLabel}
        tabIndex={-1}
        onKeyDown={keyDown}
      >
        {children}
      </div>
    </div>,
    document.body,
  );
};

interface ModalHeaderProps {
  title: React.ReactNode;
  onClose?: () => void;
  icon?: React.ReactNode;
  className?: string;
  id?: string;
}

export const ModalHeader: React.FC<ModalHeaderProps> = ({
  title,
  onClose,
  icon,
  className = '',
  id,
}) => {
  const { t } = useI18n();

  return (
    <div
      className={`flex items-center justify-between px-4 py-3 border-b border-(--theme-border-muted) dark:border-(--theme-border-default) bg-(--theme-bg-default) dark:bg-(--theme-canvas-subtle) ${className}`}
    >
      <h2
        id={id}
        className="text-lg font-bold text-(--theme-fg-default) dark:text-(--theme-fg-strong) flex items-center gap-2"
      >
        {icon}
        {title}
      </h2>
      {onClose && (
        <button
          type="button"
          onClick={onClose}
          className="p-1.5 text-(--theme-fg-subtle) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-control-hover) hover:bg-(--theme-surface-control-hover) dark:hover:bg-(--theme-border-default) rounded-md transition-colors"
          aria-label={t('common_close')}
        >
          <X size={18} />
        </button>
      )}
    </div>
  );
};

export const ModalBody: React.FC<{ children: React.ReactNode; className?: string }> = ({
  children,
  className = '',
}) => {
  return <div className={`flex-1 overflow-y-auto p-4 ${className}`}>{children}</div>;
};

export const ModalFooter: React.FC<{ children: React.ReactNode; className?: string }> = ({
  children,
  className = '',
}) => {
  return (
    <div
      className={`px-4 py-3 border-t border-(--theme-border-muted) dark:border-(--theme-canvas-muted) bg-(--theme-bg-muted) dark:bg-(--theme-bg-inset) flex justify-end gap-2 ${className}`}
    >
      {children}
    </div>
  );
};
