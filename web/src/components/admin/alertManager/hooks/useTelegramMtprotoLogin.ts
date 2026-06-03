import React from 'react';
import { useI18n } from '@i18n';
import type { AlertChannelType, AlertTelegramMode } from '@app-types/admin';
import { pushTopBanner } from '@runtime/topBannerRuntime';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';
import {
  pingAlertMtproto,
  requestAlertMtprotoCode,
  submitAlertMtprotoPassword,
  verifyAlertMtprotoCode,
} from '@lib/adminApi';
import { testAlertChannel } from '@stores/alertChannelsStore';

type ConnectionStatus = 'unknown' | 'valid' | 'invalid';
type LoginScope = {
  key: string;
  version: number;
};
type ActiveLoginScope = LoginScope & {
  isOpen: boolean;
};

type PendingLogin = 'request_code' | 'verify_code' | 'submit_password' | 'test';

type LoginState = {
  connectionStatus: ConnectionStatus;
  connectionReason: string | null;
  loginId: string | null;
  loginTimeout: number | null;
  loginCode: string;
  twoFactorPassword: string;
  passwordRequired: boolean;
  pending: PendingLogin | null;
};

const initialLoginState: LoginState = {
  connectionStatus: 'unknown',
  connectionReason: null,
  loginId: null,
  loginTimeout: null,
  loginCode: '',
  twoFactorPassword: '',
  passwordRequired: false,
  pending: null,
};

export const useTelegramMtprotoLogin = ({
  isOpen,
  channelId,
  channelType,
  telegramMode,
}: {
  isOpen: boolean;
  channelId?: number;
  channelType: AlertChannelType;
  telegramMode: AlertTelegramMode;
}) => {
  const { t } = useI18n();
  const apiError = useApiErrorHandler();
  const [state, setState] = React.useState<LoginState>(initialLoginState);
  const {
    connectionStatus,
    connectionReason,
    loginId,
    loginTimeout,
    loginCode,
    twoFactorPassword,
    passwordRequired,
    pending,
  } = state;
  const scopeKey = `${isOpen ? 'open' : 'closed'}:${channelId ?? 'new'}:${channelType}:${telegramMode}`;
  const activeScopeRef = React.useRef<ActiveLoginScope>({ key: scopeKey, version: 0, isOpen });
  const pendingRef = React.useRef<PendingLogin | null>(null);

  React.useLayoutEffect(() => {
    activeScopeRef.current = {
      key: scopeKey,
      version:
        activeScopeRef.current.key === scopeKey
          ? activeScopeRef.current.version
          : activeScopeRef.current.version + 1,
      isOpen,
    };
    pendingRef.current = null;
    setState(initialLoginState);
  }, [isOpen, scopeKey]);

  const currentScope = React.useCallback((): LoginScope => {
    const { key, version } = activeScopeRef.current;
    return { key, version };
  }, []);

  const isCurrentScope = React.useCallback((scope: LoginScope): boolean => {
    const current = activeScopeRef.current;
    return current.isOpen && current.key === scope.key && current.version === scope.version;
  }, []);

  const beginPending = React.useCallback(
    (next: PendingLogin): LoginScope | null => {
      if (pendingRef.current) return null;
      pendingRef.current = next;
      setState((current) => (current.pending ? current : { ...current, pending: next }));
      return currentScope();
    },
    [currentScope],
  );

  const finishPending = React.useCallback(
    (scope: LoginScope, current: PendingLogin): void => {
      if (!isCurrentScope(scope)) return;
      if (pendingRef.current === current) pendingRef.current = null;
      setState((state) => (state.pending === current ? { ...state, pending: null } : state));
    },
    [isCurrentScope],
  );

  const requireChannelId = React.useCallback((): number | null => {
    if (!channelId) {
      pushTopBanner(t('admin_alerts_channels_need_save'), { tone: 'warning' });
      return null;
    }
    return channelId;
  }, [channelId, t]);

  const setLoginCode = React.useCallback((value: string) => {
    setState((current) => ({ ...current, loginCode: value }));
  }, []);

  const setTwoFactorPassword = React.useCallback((value: string) => {
    setState((current) => ({ ...current, twoFactorPassword: value }));
  }, []);

  const requestCode = React.useCallback(async () => {
    const id = requireChannelId();
    if (!id) return;
    const scope = beginPending('request_code');
    if (!scope) return;
    try {
      const data = await requestAlertMtprotoCode(id);
      if (!isCurrentScope(scope)) return;
      setState((current) => ({
        ...current,
        loginId: data.login_id,
        loginTimeout: data.timeout,
        passwordRequired: false,
      }));
      pushTopBanner(t('admin_alerts_channels_request_code_success'), { tone: 'info' });
    } catch (error) {
      if (isCurrentScope(scope)) {
        apiError(error, t('admin_alerts_channels_request_code_failed'));
      }
    } finally {
      finishPending(scope, 'request_code');
    }
  }, [apiError, beginPending, finishPending, isCurrentScope, requireChannelId, t]);

  const verifyCode = React.useCallback(async () => {
    const code = loginCode.trim();
    if (!loginId || !code) return;
    const scope = beginPending('verify_code');
    if (!scope) return;
    try {
      const response = await verifyAlertMtprotoCode({
        login_id: loginId,
        code,
      });
      if (!isCurrentScope(scope)) return;
      const nextPasswordRequired = response !== undefined && response.password_required === true;
      setState((current) => ({ ...current, passwordRequired: nextPasswordRequired }));
      if (nextPasswordRequired) {
        pushTopBanner(t('admin_alerts_channels_password_required'), { tone: 'warning' });
      } else {
        pushTopBanner(t('admin_alerts_channels_verify_code_success'), { tone: 'info' });
      }
    } catch (error) {
      if (isCurrentScope(scope)) {
        apiError(error, t('admin_alerts_channels_verify_code_failed'));
      }
    } finally {
      finishPending(scope, 'verify_code');
    }
  }, [apiError, beginPending, finishPending, isCurrentScope, loginCode, loginId, t]);

  const submitPassword = React.useCallback(async () => {
    const password = twoFactorPassword.trim();
    if (!loginId || !password) return;
    const scope = beginPending('submit_password');
    if (!scope) return;
    try {
      await submitAlertMtprotoPassword({
        login_id: loginId,
        password,
      });
      if (!isCurrentScope(scope)) return;
      setState((current) => ({ ...current, passwordRequired: false }));
      pushTopBanner(t('admin_alerts_channels_password_submit_success'), { tone: 'info' });
    } catch (error) {
      if (isCurrentScope(scope)) {
        apiError(error, t('admin_alerts_channels_password_submit_failed'));
      }
    } finally {
      finishPending(scope, 'submit_password');
    }
  }, [apiError, beginPending, finishPending, isCurrentScope, loginId, t, twoFactorPassword]);

  const testConnection = React.useCallback(async () => {
    const id = requireChannelId();
    if (!id) return;
    const scope = beginPending('test');
    if (!scope) return;
    try {
      if (channelType === 'telegram' && telegramMode === 'mtproto') {
        const result = await pingAlertMtproto(id);
        if (!isCurrentScope(scope)) return;
        setState((current) => ({
          ...current,
          connectionStatus: result.valid ? 'valid' : 'invalid',
          connectionReason: result.valid ? null : (result.reason ?? null),
        }));
        if (result.valid) {
          pushTopBanner(t('admin_alerts_channels_test_success'), { tone: 'info' });
        } else {
          pushTopBanner(t('admin_alerts_channels_test_failed'), { tone: 'error' });
        }
      } else {
        const didTest = await testAlertChannel(id);
        if (!isCurrentScope(scope)) return;
        if (!didTest) return;
        pushTopBanner(t('admin_alerts_channels_test_success'), { tone: 'info' });
      }
    } catch (error) {
      if (isCurrentScope(scope)) {
        apiError(error, t('admin_alerts_channels_test_failed'));
      }
    } finally {
      finishPending(scope, 'test');
    }
  }, [
    apiError,
    beginPending,
    channelType,
    finishPending,
    isCurrentScope,
    requireChannelId,
    t,
    telegramMode,
  ]);

  const connectionLabel =
    connectionStatus === 'valid'
      ? t('admin_alerts_channels_status_valid')
      : connectionStatus === 'invalid'
        ? t('admin_alerts_channels_status_invalid')
        : t('admin_alerts_channels_status_unknown');
  const connectionClass =
    connectionStatus === 'valid'
      ? 'text-(--theme-fg-success-strong) dark:text-(--theme-fg-success-muted)'
      : connectionStatus === 'invalid'
        ? 'text-(--theme-fg-danger) dark:text-(--theme-fg-danger)'
        : 'text-(--theme-fg-warning) dark:text-(--theme-fg-warning)';
  const connectionDotClass =
    connectionStatus === 'valid'
      ? 'bg-(--theme-fg-success-soft) shadow-[0_0_6px] shadow-(color:--theme-fg-success-soft)/40'
      : connectionStatus === 'invalid'
        ? 'bg-(--theme-fg-danger-soft) shadow-[0_0_6px] shadow-(color:--theme-border-underline-nav-active)/40'
        : 'bg-(--theme-fg-warning) shadow-[0_0_6px] shadow-(color:--theme-fg-warning)/60';

  return {
    loginId,
    loginTimeout,
    loginCode,
    setLoginCode,
    twoFactorPassword,
    setTwoFactorPassword,
    passwordRequired,
    isRequestingCode: pending === 'request_code',
    isVerifyingCode: pending === 'verify_code',
    isSubmittingPassword: pending === 'submit_password',
    isTesting: pending === 'test',
    connectionStatus,
    connectionReason,
    connectionLabel,
    connectionClass,
    connectionDotClass,
    requestCode,
    verifyCode,
    submitPassword,
    testConnection,
  };
};

export type TelegramMtprotoLoginState = ReturnType<typeof useTelegramMtprotoLogin>;
