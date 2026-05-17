import React from 'react';
import type { ConfirmDialogProps, ConfirmDialogState } from '@components/ui/ConfirmDialog';

export type ConfirmRequest = (state: ConfirmDialogState) => Promise<boolean>;
export type ConfirmAction = (
  state: ConfirmDialogState,
  action: () => Promise<void>,
) => Promise<void>;

interface UseConfirmDialogResult {
  dialog: ConfirmDialogState | null;
  isLoading: boolean;
  dialogProps: ConfirmDialogProps;
  request: ConfirmRequest;
  run: ConfirmAction;
  confirm: () => void;
  cancel: () => void;
}

const emptyDialogState: ConfirmDialogState = {
  title: '',
  message: '',
  confirmLabel: '',
  cancelLabel: '',
  tone: 'default',
};

export const useConfirmDialog = (): UseConfirmDialogResult => {
  const [dialog, setDialog] = React.useState<ConfirmDialogState | null>(null);
  const [isLoading, setIsLoading] = React.useState(false);
  const resolveRef = React.useRef<((ok: boolean) => void) | null>(null);
  const actionRef = React.useRef<(() => Promise<void>) | null>(null);
  const actionDoneRef = React.useRef<(() => void) | null>(null);

  const reset = React.useCallback(() => {
    resolveRef.current = null;
    actionRef.current = null;
    actionDoneRef.current = null;
    setDialog(null);
    setIsLoading(false);
  }, []);

  const request = React.useCallback<ConfirmRequest>(
    (state) =>
      new Promise<boolean>((resolve) => {
        resolveRef.current = resolve;
        setDialog(state);
      }),
    [],
  );

  const run = React.useCallback<ConfirmAction>(
    (state, action) =>
      new Promise<void>((resolve) => {
        actionRef.current = action;
        actionDoneRef.current = resolve;
        setDialog(state);
      }),
    [],
  );

  const cancel = React.useCallback(() => {
    if (isLoading) return;
    const resolve = resolveRef.current;
    const actionDone = actionDoneRef.current;
    reset();
    resolve?.(false);
    actionDone?.();
  }, [isLoading, reset]);

  const confirm = React.useCallback(() => {
    const action = actionRef.current;
    if (!action) {
      const resolve = resolveRef.current;
      reset();
      resolve?.(true);
      return;
    }

    setIsLoading(true);
    void (async () => {
      try {
        await action();
      } finally {
        const actionDone = actionDoneRef.current;
        reset();
        actionDone?.();
      }
    })();
  }, [reset]);

  const dialogProps = React.useMemo<ConfirmDialogProps>(
    () => ({
      isOpen: dialog !== null,
      title: dialog?.title ?? emptyDialogState.title,
      message: dialog?.message ?? emptyDialogState.message,
      confirmLabel: dialog?.confirmLabel ?? emptyDialogState.confirmLabel,
      cancelLabel: dialog?.cancelLabel ?? emptyDialogState.cancelLabel,
      tone: dialog?.tone ?? emptyDialogState.tone,
      isLoading,
      onConfirm: confirm,
      onCancel: cancel,
    }),
    [cancel, confirm, dialog, isLoading],
  );

  return {
    dialog,
    isLoading,
    dialogProps,
    request,
    run,
    confirm,
    cancel,
  };
};
