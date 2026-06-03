import React from 'react';
import type { ConfirmDialogProps, ConfirmDialogState } from '@components/ui/ConfirmDialog';

export type ConfirmRequest = (state: ConfirmDialogState) => Promise<boolean>;
export type ConfirmAction = (
  state: ConfirmDialogState,
  action: () => Promise<void>,
) => Promise<void>;

interface UseConfirmDialogResult {
  dialogProps: ConfirmDialogProps;
  request: ConfirmRequest;
  run: ConfirmAction;
}

type PendingConfirm =
  | {
      kind: 'request';
      resolve: (ok: boolean) => void;
    }
  | {
      kind: 'action';
      action: () => Promise<void>;
      running: boolean;
      resolve: () => void;
      reject: (error: unknown) => void;
    };

const emptyDialogState: ConfirmDialogState = {
  title: '',
  message: '',
  confirmLabel: '',
  cancelLabel: '',
  tone: 'default',
};

const resolveRequest = (pending: PendingConfirm | null, result: boolean): void => {
  if (!pending) return;
  if (pending.kind === 'request') {
    pending.resolve(result);
  }
};

const cancelPending = (pending: PendingConfirm | null): void => {
  if (!pending) return;
  if (pending.kind === 'request') {
    pending.resolve(false);
    return;
  }
  if (!pending.running) {
    pending.resolve();
  }
};

export const useConfirmDialog = (): UseConfirmDialogResult => {
  const [dialog, setDialog] = React.useState<ConfirmDialogState | null>(null);
  const [isLoading, setIsLoading] = React.useState(false);
  const pendingRef = React.useRef<PendingConfirm | null>(null);
  const mountedRef = React.useRef(true);

  const reset = React.useCallback(() => {
    pendingRef.current = null;
    if (!mountedRef.current) return;
    setDialog(null);
    setIsLoading(false);
  }, []);

  React.useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      const pending = pendingRef.current;
      if (!pending) return;
      pendingRef.current = null;
      cancelPending(pending);
    };
  }, []);

  const request = React.useCallback<ConfirmRequest>((state) => {
    if (pendingRef.current) return Promise.resolve(false);
    return new Promise<boolean>((resolve) => {
      pendingRef.current = { kind: 'request', resolve };
      setDialog(state);
    });
  }, []);

  const run = React.useCallback<ConfirmAction>((state, action) => {
    if (pendingRef.current) return Promise.resolve();
    return new Promise<void>((resolve, reject) => {
      pendingRef.current = { kind: 'action', action, running: false, resolve, reject };
      setDialog(state);
    });
  }, []);

  const cancel = React.useCallback(() => {
    const pending = pendingRef.current;
    if (isLoading || (pending?.kind === 'action' && pending.running)) return;
    reset();
    cancelPending(pending);
  }, [isLoading, reset]);

  const confirm = React.useCallback(() => {
    const pending = pendingRef.current;
    if (!pending) {
      reset();
      return;
    }
    if (pending.kind === 'request') {
      reset();
      resolveRequest(pending, true);
      return;
    }

    if (pending.running) return;
    pending.running = true;
    setIsLoading(true);
    void (async () => {
      try {
        await pending.action();
        if (pendingRef.current === pending) {
          reset();
        }
        pending.resolve();
      } catch (error) {
        if (pendingRef.current === pending) {
          reset();
        }
        pending.reject(error);
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
    dialogProps,
    request,
    run,
  };
};
