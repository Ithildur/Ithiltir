import React, { useState } from 'react';
import ArrowRight from 'lucide-react/dist/esm/icons/arrow-right';
import CheckCircle2 from 'lucide-react/dist/esm/icons/check-circle-2';
import Eye from 'lucide-react/dist/esm/icons/eye';
import EyeOff from 'lucide-react/dist/esm/icons/eye-off';
import Lock from 'lucide-react/dist/esm/icons/lock';
import { useLocation, useNavigate } from 'react-router-dom';
import BrandLogo from '@components/BrandLogo';
import Input from '@components/ui/Input';
import ThemeToggle from '@components/ui/ThemeToggle';
import { useTopBanner } from '@components/ui/TopBannerStack';
import { useAuth } from '@context/AuthContext';
import { useSiteBrand } from '@context/SiteBrandContext';
import { ApiError } from '@lib/api';
import { useI18n } from '@i18n';
import { useBootstrapAuth } from '@hooks/useBootstrapAuth';
import { readRememberLogin } from '@lib/authStore';

interface Props extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  loading?: boolean;
  loadingText?: React.ReactNode;
}

const GradientButton: React.FC<Props> = ({
  children,
  loading,
  loadingText,
  className = '',
  ...props
}) => (
  <button
    className={`
      relative w-full flex items-center justify-center gap-2 py-3 px-4 rounded-xl font-bold text-sm text-(--theme-fg-on-emphasis)
      theme-login-submit-gradient
      active:scale-[0.98] transition-[transform,background-color] duration-200 motion-reduce:transform-none motion-reduce:transition-none
      disabled:opacity-70 disabled:cursor-not-allowed
      ui-focus-ring
      overflow-hidden group
      ${className}
    `}
    disabled={loading || props.disabled}
    {...props}
  >
    <div className="theme-login-submit-shine absolute inset-0 -translate-x-full skew-x-12 transition-transform duration-700 group-hover:translate-x-full motion-reduce:hidden" />
    {loading ? (
      <>
        <div className="size-4 border-2 border-(--theme-fg-on-emphasis)/30 border-t-(--theme-fg-on-emphasis) rounded-full animate-spin motion-reduce:animate-none" />
        <span>{loadingText ?? children}</span>
      </>
    ) : (
      children
    )}
  </button>
);

type LoginRedirectState = {
  from?: string;
  denied?: {
    code?: number;
    reason?: string;
  };
};

const readRedirectState = (state: unknown): LoginRedirectState => {
  if (!state || typeof state !== 'object') {
    return {};
  }

  const value = state as LoginRedirectState;
  const from =
    typeof value.from === 'string' && value.from.startsWith('/') && value.from !== '/login'
      ? value.from
      : undefined;
  const denied =
    typeof value.denied?.code === 'number' && typeof value.denied?.reason === 'string'
      ? value.denied
      : undefined;

  return { from, denied };
};

const LoginPage: React.FC = () => {
  const [password, setPassword] = useState('');
  const [usernameTrap, setUsernameTrap] = useState('');
  const [remember, setRemember] = useState(() => readRememberLogin());
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  useBootstrapAuth();
  const { login, isAuthenticated } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const pushBanner = useTopBanner();
  const { t } = useI18n();
  const { brand } = useSiteBrand();
  const redirectState = React.useMemo(() => readRedirectState(location.state), [location.state]);
  const redirectTo = redirectState.from ?? '/admin';
  const deniedNoticeKeyRef = React.useRef<string | null>(null);

  React.useEffect(() => {
    if (redirectState.denied?.code !== 403 || redirectState.denied.reason !== 'statistics') {
      return;
    }

    const noticeKey = `${location.key}:statistics:403`;
    if (deniedNoticeKeyRef.current === noticeKey) {
      return;
    }
    deniedNoticeKeyRef.current = noticeKey;
    pushBanner(t('stats_auth_required'), { tone: 'error', durationMs: 4000 });
  }, [location.key, pushBanner, redirectState.denied, t]);

  React.useEffect(() => {
    if (isAuthenticated) {
      navigate(redirectTo, { replace: true });
    }
  }, [isAuthenticated, navigate, redirectTo]);

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (usernameTrap.trim() !== '') {
      setIsLoading(true);
      try {
        await new Promise((resolve) => window.setTimeout(resolve, 400));
        pushBanner(t('login_failed'), { tone: 'error' });
      } finally {
        setIsLoading(false);
      }
      return;
    }
    if (!password.trim()) {
      pushBanner(t('login_password_required'), { tone: 'warning' });
      return;
    }
    setIsLoading(true);
    try {
      await login(password.trim(), remember);
      pushBanner(t('login_success'), { tone: 'info' });
      navigate(redirectTo, { replace: true });
    } catch (error) {
      if (error instanceof ApiError) {
        pushBanner(error.message || t('login_failed'), { tone: 'error' });
      } else {
        pushBanner(t('login_failed_retry'), { tone: 'error' });
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-(--theme-page-bg) text-(--theme-fg-default) dark:bg-(--theme-bg-default) dark:text-(--theme-fg-strong) font-sans flex items-center justify-center p-4 relative overflow-hidden">
      <div className="fixed inset-0 z-0 pointer-events-none">
        <div className="theme-login-page-gradient-light absolute inset-0 dark:hidden" />

        <div className="theme-login-page-gradient-dark absolute inset-0 hidden dark:block" />

        <div className="theme-login-page-grid absolute inset-0 opacity-[0.4] dark:opacity-[0.15]" />
      </div>

      <div className="w-full max-w-lg z-10">
        <div className="bg-(--theme-surface-overlay) backdrop-blur-xl border border-(--theme-border-subtle) dark:border-(--theme-border-default) rounded-2xl shadow-2xl overflow-hidden relative group">
          <div className="theme-login-card-edge absolute inset-x-0 top-0 h-px transition-all duration-500 motion-reduce:transition-none" />
          <div className="absolute top-4 right-4">
            <ThemeToggle size="sm" variant="soft" />
          </div>

          <div className="p-8">
            <div className="text-center mb-8">
              <div className="inline-flex items-center justify-center size-14 rounded-2xl mb-5">
                <BrandLogo />
              </div>
              <h1 className="wrap-break-word text-2xl font-bold tracking-tight text-(--theme-fg-default) dark:text-(--theme-fg-strong) mb-2">
                {brand.topbar_text}
              </h1>
              <p className="text-(--theme-fg-muted) dark:text-(--theme-fg-neutral) text-sm">
                {t('login_subtitle')}
              </p>
            </div>

            <form onSubmit={submit} className="space-y-5">
              <div className="sr-only" aria-hidden="true">
                <label htmlFor="login-username-trap" className="sr-only">
                  {t('login_username')}
                </label>
                <input
                  id="login-username-trap"
                  name="username"
                  type="text"
                  value={usernameTrap}
                  onChange={(event) => setUsernameTrap(event.target.value)}
                  autoComplete="off"
                  tabIndex={-1}
                />
              </div>

              <div className="space-y-2">
                <div className="space-y-1.5 group/field">
                  <label
                    htmlFor="login-password"
                    className="text-xs font-bold uppercase tracking-wider transition-colors duration-200 text-(--theme-fg-muted) dark:text-(--theme-fg-control-muted) group-focus-within/field:text-(--theme-bg-accent-emphasis) dark:group-focus-within/field:text-(--theme-fg-accent)"
                  >
                    {t('login_password')}
                  </label>
                  <Input
                    id="login-password"
                    name="password"
                    type={showPassword ? 'text' : 'password'}
                    placeholder="••••••••••••"
                    icon={Lock}
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    autoComplete="current-password"
                    className="py-3 rounded-xl bg-(--theme-surface-control-strong)"
                    rightElement={
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="ui-focus-ring flex items-center rounded text-(--theme-fg-muted) dark:text-(--theme-fg-control-muted) hover:text-(--theme-fg-default) dark:hover:text-(--theme-fg-control-hover) transition-colors"
                        aria-label={t('login_toggle_password_visibility')}
                      >
                        {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                      </button>
                    }
                  />
                </div>

                <div className="flex justify-start items-center px-1">
                  <label className="flex items-center gap-2 cursor-pointer group/check">
                    <div className="relative flex items-center">
                      <input
                        type="checkbox"
                        checked={remember}
                        onChange={(event) => setRemember(event.target.checked)}
                        className="ui-focus-ring peer size-4 cursor-pointer appearance-none rounded border border-(--theme-border-control) bg-(--theme-surface-control) transition-all checked:border-(--theme-bg-interactive-emphasis) checked:bg-(--theme-bg-interactive-emphasis) motion-reduce:transition-none"
                      />
                      <CheckCircle2
                        size={10}
                        className="absolute inset-0 m-auto text-(--theme-fg-on-emphasis) opacity-0 peer-checked:opacity-100 pointer-events-none transition-opacity"
                      />
                    </div>
                    <span className="text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-control-muted) group-hover/check:text-(--theme-fg-default) dark:group-hover/check:text-(--theme-fg-control-hover) transition-colors select-none">
                      {t('login_remember_me')}
                    </span>
                  </label>
                </div>
              </div>

              <div className="pt-2">
                <GradientButton
                  loading={isLoading}
                  loadingText={t('login_authenticating')}
                  type="submit"
                >
                  <span>{t('login_sign_in')}</span>
                  <ArrowRight
                    size={16}
                    className="group-hover:translate-x-0.5 transition-transform motion-reduce:transform-none motion-reduce:transition-none"
                  />
                </GradientButton>
              </div>
            </form>
          </div>
        </div>

        <div className="text-center mt-6">
          <p className="text-xs text-(--theme-fg-muted) dark:text-(--theme-fg-footer)">
            &copy; Powered by Ithiltir
          </p>
        </div>
      </div>
    </div>
  );
};

export default LoginPage;
