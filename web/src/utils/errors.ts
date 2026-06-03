import { isApiAuthStaleError } from '@lib/api';
import { isStaleLoadError } from './seqGate';

export const isAbortError = (error: unknown): boolean =>
  error instanceof DOMException && error.name === 'AbortError';

export const isCanceledRequestError = (error: unknown): boolean =>
  isAbortError(error) || isApiAuthStaleError(error) || isStaleLoadError(error);
