export type ActionOutcome<T = void> =
  | { status: 'ok'; value: T }
  | { status: 'busy' | 'stale' | 'noop' };

export function actionOk(): ActionOutcome<void>;
export function actionOk<T>(value: T): ActionOutcome<T>;
export function actionOk<T>(value?: T): ActionOutcome<T | void> {
  return {
    status: 'ok',
    value,
  };
}

export const actionBusy = { status: 'busy' } satisfies ActionOutcome<never>;
export const actionStale = { status: 'stale' } satisfies ActionOutcome<never>;
export const actionNoop = { status: 'noop' } satisfies ActionOutcome<never>;

export const isActionOk = <T>(outcome: ActionOutcome<T>): outcome is { status: 'ok'; value: T } =>
  outcome.status === 'ok';
