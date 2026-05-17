import React from 'react';
import { useI18n } from '@i18n';
import type { AlertChannelType, AlertTelegramMode } from '@app-types/admin';
import * as adminApi from '@lib/adminApi';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { useApiErrorHandler } from '@hooks/useApiErrorHandler';

type ConnectionStatus = 'unknown' | 'valid' | 'invalid';

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
  const pushBanner = useTopBanner();
  const apiError = useApiErrorHandler();
  const [connectionStatus, setConnectionStatus] = React.useState<ConnectionStatus>('unknown');
  const [connectionReason, setConnectionReason] = React.useState<string | null>(null);
  const [loginId, setLoginId] = React.useState<string | null>(null);
  const [loginTimeout, setLoginTimeout] = React.useState<number | null>(null);
  const [loginCode, setLoginCode] = React.useState('');
  const [twoFactorPassword, setTwoFactorPassword] = React.useState('');
  const [passwordRequired, setPasswordRequired] = React.useState(false);
  const [isRequestingCode, setIsRequestingCode] = React.useState(false);
  const [isVerifyingCode, setIsVerifyingCode] = React.useState(false);
  const [isSubmittingPassword, setIsSubmittingPassword] = React.useState(false);
  const [isTesting, setIsTesting] = React.useState(false);

  React.useEffect(() => {
    if (!isOpen) return;
    setConnectionStatus('unknown');
    setConnectionReason(null);
    setLoginId(null);
    setLoginTimeout(null);
    setLoginCode('');
    setTwoFactorPassword('');
    setPasswordRequired(false);
  }, [isOpen]);

  const requireChannelId = React.useCallback((): number | null => {
    if (!channelId) {
      pushBanner(t('admin_alerts_channels_need_save'), { tone: 'warning' });
      return null;
    }
    return channelId;
  }, [channelId, pushBanner, t]);

  const requestCode = React.useCallback(async () => {
    const id = requireChannelId();
    if (!id || isRequestingCode) return;
    setIsRequestingCode(true);
    try {
      const data = await adminApi.requestAlertMtprotoCode(id);
      setLoginId(data.login_id);
      setLoginTimeout(data.timeout);
      setPasswordRequired(false);
      pushBanner(t('admin_alerts_channels_request_code_success'), { tone: 'info' });
    } catch (error) {
      apiError(error, t('admin_alerts_channels_request_code_failed'));
    } finally {
      setIsRequestingCode(false);
    }
  }, [apiError, isRequestingCode, pushBanner, requireChannelId, t]);

  const verifyCode = React.useCallback(async () => {
    const code = loginCode.trim();
    if (!loginId || !code || isVerifyingCode) return;
    setIsVerifyingCode(true);
    try {
      const response = await adminApi.verifyAlertMtprotoCode({
        login_id: loginId,
        code,
      });
      if (response && 'password_required' in response && response.password_required) {
        setPasswordRequired(true);
        pushBanner(t('admin_alerts_channels_password_required'), { tone: 'warning' });
      } else {
        setPasswordRequired(false);
        pushBanner(t('admin_alerts_channels_verify_code_success'), { tone: 'info' });
      }
    } catch (error) {
      apiError(error, t('admin_alerts_channels_verify_code_failed'));
    } finally {
      setIsVerifyingCode(false);
    }
  }, [apiError, isVerifyingCode, loginCode, loginId, pushBanner, t]);

  const submitPassword = React.useCallback(async () => {
    const password = twoFactorPassword.trim();
    if (!loginId || !password || isSubmittingPassword) return;
    setIsSubmittingPassword(true);
    try {
      await adminApi.submitAlertMtprotoPassword({
        login_id: loginId,
        password,
      });
      setPasswordRequired(false);
      pushBanner(t('admin_alerts_channels_password_submit_success'), { tone: 'info' });
    } catch (error) {
      apiError(error, t('admin_alerts_channels_password_submit_failed'));
    } finally {
      setIsSubmittingPassword(false);
    }
  }, [apiError, isSubmittingPassword, loginId, pushBanner, t, twoFactorPassword]);

  const testConnection = React.useCallback(async () => {
    const id = requireChannelId();
    if (!id || isTesting) return;
    setIsTesting(true);
    try {
      if (channelType === 'telegram' && telegramMode === 'mtproto') {
        const result = await adminApi.pingAlertMtproto(id);
        if (result.valid) {
          setConnectionStatus('valid');
          setConnectionReason(null);
          pushBanner(t('admin_alerts_channels_test_success'), { tone: 'info' });
        } else {
          setConnectionStatus('invalid');
          setConnectionReason(result.reason ?? null);
          pushBanner(t('admin_alerts_channels_test_failed'), { tone: 'error' });
        }
      } else {
        await adminApi.testAlertChannel(id, {
          title: t('admin_alerts_channels_test_title'),
          message: t('admin_alerts_channels_test_message'),
        });
        pushBanner(t('admin_alerts_channels_test_success'), { tone: 'info' });
      }
    } catch (error) {
      apiError(error, t('admin_alerts_channels_test_failed'));
    } finally {
      setIsTesting(false);
    }
  }, [channelType, apiError, isTesting, pushBanner, requireChannelId, t, telegramMode]);

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
    isRequestingCode,
    isVerifyingCode,
    isSubmittingPassword,
    isTesting,
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
