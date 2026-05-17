import React from 'react';
import Button, { type ButtonVariant } from './Button';
import { Modal, ModalHeader, ModalBody, ModalFooter } from './Modal';

export type ConfirmDialogTone = 'default' | 'danger';

export interface ConfirmDialogState {
  title: string;
  message: React.ReactNode;
  confirmLabel: string;
  cancelLabel: string;
  tone?: ConfirmDialogTone;
}

export interface ConfirmDialogProps extends ConfirmDialogState {
  isOpen: boolean;
  isLoading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  isOpen,
  title,
  message,
  confirmLabel,
  cancelLabel,
  tone = 'default',
  isLoading = false,
  onConfirm,
  onCancel,
}) => {
  const titleId = React.useId();
  const confirmVariant: ButtonVariant = tone === 'danger' ? 'danger' : 'primary';
  if (!isOpen) return null;

  return (
    <Modal
      isOpen={isOpen}
      onClose={isLoading ? () => {} : onCancel}
      maxWidth="max-w-md"
      zIndex={60}
      ariaLabelledby={titleId}
    >
      <ModalHeader
        title={title}
        onClose={isLoading ? undefined : onCancel}
        id={titleId}
        className="py-3"
      />

      <ModalBody className="text-sm/relaxed text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) dark:text-(--theme-fg-control-hover)">
        {message}
      </ModalBody>

      <ModalFooter>
        <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
          {cancelLabel}
        </Button>
        <Button variant={confirmVariant} onClick={onConfirm} disabled={isLoading}>
          {confirmLabel}
        </Button>
      </ModalFooter>
    </Modal>
  );
};

export default ConfirmDialog;
