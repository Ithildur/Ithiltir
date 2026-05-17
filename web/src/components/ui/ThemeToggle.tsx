import React, { useEffect, useRef, useState, useCallback } from 'react';
import Laptop from 'lucide-react/dist/esm/icons/laptop';
import Moon from 'lucide-react/dist/esm/icons/moon';
import Sun from 'lucide-react/dist/esm/icons/sun';
import { useI18n } from '@i18n';

type ThemeMode = 'light' | 'dark' | 'system';

interface Props {
  className?: string;
  showLabel?: boolean;
  labelMode?: 'current' | 'action';
  actionLabel?: string;
  titleOverride?: string;
  size?: 'sm' | 'md' | 'icon';
  variant?: 'soft' | 'plain';
}

const fallbackGetTheme = (): ThemeMode => {
  try {
    const raw = globalThis.window?.__theme?.get?.();
    return raw === 'light' || raw === 'dark' || raw === 'system' ? raw : 'system';
  } catch {
    return 'system';
  }
};

const fallbackSetTheme = (theme: ThemeMode) => {
  try {
    globalThis.window?.__theme?.set?.(theme);
  } catch {
    // The inline theme bridge may be unavailable during early boot.
  }
};

const fallbackApplyTheme = (theme: ThemeMode) => {
  try {
    globalThis.window?.__theme?.apply?.(theme);
  } catch {
    // The inline theme bridge may be unavailable during early boot.
  }
};

const iconByTheme: Record<ThemeMode, React.ReactNode> = {
  light: <Sun size={16} />,
  dark: <Moon size={16} />,
  system: <Laptop size={16} />,
};

const ThemeToggle: React.FC<Props> = ({
  className = '',
  showLabel = false,
  labelMode = 'current',
  actionLabel,
  titleOverride,
  size = 'sm',
  variant = 'soft',
}) => {
  const { t } = useI18n();
  const [theme, setThemeState] = useState<ThemeMode>(() => fallbackGetTheme());
  const unsubscribeSystemRef = useRef<null | (() => void)>(null);

  useEffect(() => {
    fallbackApplyTheme(theme);
    fallbackSetTheme(theme);

    unsubscribeSystemRef.current?.();
    unsubscribeSystemRef.current = null;

    if (theme !== 'system') return;

    const onSystemChange = globalThis.window?.__theme?.onSystemChange;
    if (!onSystemChange) return;

    const unsubscribe = onSystemChange(() => fallbackApplyTheme('system'));
    unsubscribeSystemRef.current = unsubscribe;

    return () => {
      unsubscribeSystemRef.current?.();
      unsubscribeSystemRef.current = null;
    };
  }, [theme]);

  const setTheme = useCallback((newTheme: ThemeMode) => {
    setThemeState(newTheme);
  }, []);

  const nextTheme: ThemeMode = React.useMemo(() => {
    if (theme === 'light') return 'dark';
    if (theme === 'dark') return 'system';
    return 'light';
  }, [theme]);

  const labelByTheme = React.useMemo<Record<ThemeMode, string>>(
    () => ({
      light: t('theme_light'),
      dark: t('theme_dark'),
      system: t('theme_system'),
    }),
    [t],
  );

  const base =
    'inline-flex items-center gap-2 rounded-full font-medium transition-all motion-reduce:transition-none focus:outline-none focus-visible:ring-2 focus-visible:ring-(--theme-focus-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--theme-bg-default) dark:focus-visible:ring-offset-(--theme-bg-default)';

  const sizeClass =
    size === 'icon'
      ? 'h-8 w-8 p-0 text-xs'
      : size === 'sm'
        ? 'h-9 px-3 text-xs'
        : 'h-10 px-3.5 text-sm';
  const justifyClass = size === 'icon' ? 'justify-center' : '';

  const variantClass =
    variant === 'soft'
      ? 'bg-(--theme-surface-control-strong) dark:bg-(--theme-bg-inset) border border-(--theme-border-subtle) dark:border-(--theme-border-default) text-(--theme-fg-default) dark:text-(--theme-fg-default) shadow-sm hover:border-(--theme-border-interactive-hover) dark:hover:border-(--theme-border-interactive-hover) hover:text-(--theme-fg-interactive-strong) dark:hover:text-(--theme-fg-on-emphasis)'
      : '';

  return (
    <button
      type="button"
      onClick={() => setTheme(nextTheme)}
      className={`${base} ${sizeClass} ${justifyClass} ${variantClass} ${className}`}
      aria-label={t('theme_switch_aria', { current: labelByTheme[theme] })}
      title={
        titleOverride ??
        t('theme_switch_title', { current: labelByTheme[theme], next: labelByTheme[nextTheme] })
      }
    >
      {iconByTheme[theme]}
      {showLabel && (
        <span>{labelMode === 'action' ? actionLabel || t('theme') : labelByTheme[theme]}</span>
      )}
    </button>
  );
};

export default ThemeToggle;
