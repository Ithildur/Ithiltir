import React from 'react';
import { createPortal } from 'react-dom';
import LogIn from 'lucide-react/dist/esm/icons/log-in';
import LogOut from 'lucide-react/dist/esm/icons/log-out';
import Search from 'lucide-react/dist/esm/icons/search';
import SlidersHorizontal from 'lucide-react/dist/esm/icons/sliders-horizontal';
import UserCog from 'lucide-react/dist/esm/icons/user-cog';
import X from 'lucide-react/dist/esm/icons/x';
import { Link } from 'react-router';
import BrandLogo from '@components/BrandLogo';
import { useI18n } from '@i18n';
import Button from '@components/ui/Button';
import SearchInput from '@components/ui/SearchInput';
import ThemeToggle from '@components/ui/ThemeToggle';
import { logout, useAuthStore } from '@stores/authStore';
import { useSiteBrandStore } from '@stores/siteBrandStore';

interface Props {
  searchTerm: string;
  setSearchTerm: (searchTerm: string) => void;
}

const Header: React.FC<Props> = ({ searchTerm, setSearchTerm }) => {
  const { lang, setLang, t } = useI18n();
  const isAuthenticated = useAuthStore(
    (state) => state.status === 'authenticated' && Boolean(state.accessToken),
  );
  const brand = useSiteBrandStore((state) => state.brand);
  const [isMobileSearchOpen, setIsMobileSearchOpen] = React.useState(false);
  const [isPreferencesOpen, setIsPreferencesOpen] = React.useState(false);
  const preferencesRef = React.useRef<HTMLDivElement>(null);
  const desktopSearchInputRef = React.useRef<HTMLInputElement>(null);
  const mobileSearchDialogRef = React.useRef<HTMLDivElement>(null);
  const mobileSearchInputRef = React.useRef<HTMLInputElement>(null);
  const mobileSearchReturnFocusRef = React.useRef<HTMLElement | null>(null);

  const openMobileSearch = React.useCallback(() => {
    mobileSearchReturnFocusRef.current =
      document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setIsMobileSearchOpen(true);
  }, []);

  const closeMobileSearch = React.useCallback(() => {
    setIsMobileSearchOpen(false);
    const returnFocus = mobileSearchReturnFocusRef.current;
    mobileSearchReturnFocusRef.current = null;
    window.requestAnimationFrame(() => {
      if (returnFocus?.isConnected) returnFocus.focus();
    });
  }, []);

  React.useEffect(() => {
    if (!isMobileSearchOpen) return;
    const frame = window.requestAnimationFrame(() => mobileSearchInputRef.current?.focus());
    return () => window.cancelAnimationFrame(frame);
  }, [isMobileSearchOpen]);

  React.useEffect(() => {
    if (!isMobileSearchOpen) return;
    const desktop = window.matchMedia('(min-width: 768px)');
    const closeAtDesktop = () => {
      if (!desktop.matches) return;
      setIsMobileSearchOpen(false);
      mobileSearchReturnFocusRef.current = null;
      window.requestAnimationFrame(() => desktopSearchInputRef.current?.focus());
    };
    desktop.addEventListener('change', closeAtDesktop);
    closeAtDesktop();
    return () => desktop.removeEventListener('change', closeAtDesktop);
  }, [isMobileSearchOpen]);

  React.useEffect(() => {
    if (!isPreferencesOpen) return;
    const closeOnOutside = (event: MouseEvent) => {
      if (preferencesRef.current && !preferencesRef.current.contains(event.target as Node)) {
        setIsPreferencesOpen(false);
      }
    };
    document.addEventListener('mousedown', closeOnOutside);
    return () => document.removeEventListener('mousedown', closeOnOutside);
  }, [isPreferencesOpen]);

  return (
    <header className="sticky top-0 z-40 backdrop-blur-md bg-(--theme-surface-control) dark:bg-(--theme-bg-inset)/90 border-b border-(--theme-border-subtle) dark:border-(--theme-border-default)">
      <div className="mx-auto flex h-16 max-w-410 items-center justify-between px-4 sm:px-6 lg:px-8">
        <div className="flex min-w-0 items-center gap-2">
          <div className="size-8 shrink-0 rounded-lg flex items-center justify-center">
            <BrandLogo />
          </div>
          <span className="max-w-[52vw] truncate text-xl font-semibold text-(--theme-fg-default) sm:block md:max-w-md">
            {brand.topbar_text}
          </span>
        </div>

        <div className="flex items-center gap-3">
          <div className="hidden md:flex">
            <SearchInput
              ref={desktopSearchInputRef}
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder={t('search_placeholder')}
              aria-label={t('search_placeholder')}
              icon={Search}
              className="py-1.5 w-64 rounded-full bg-(--theme-bg-muted) dark:bg-(--theme-canvas-subtle)"
              wrapperClassName="w-64"
            />
          </div>
          <Button
            type="button"
            onClick={openMobileSearch}
            variant="icon"
            className="md:hidden"
            aria-label={t('search_placeholder')}
            data-search-trigger="true"
            icon={Search}
          />

          <div className="relative group md:hidden" ref={preferencesRef}>
            <Button
              type="button"
              onClick={() => setIsPreferencesOpen((prev) => !prev)}
              variant="icon"
              aria-label={t('theme')}
              icon={SlidersHorizontal}
            />
            {isPreferencesOpen && (
              <div
                className="menu-pop menu-pop-top-right menu-pop-anim"
                role="dialog"
                aria-label={t('settings')}
              >
                <div className="menu-head">
                  <span className="text-sm font-bold text-(--theme-fg-default)">
                    {t('settings')}
                  </span>
                  <button
                    type="button"
                    onClick={() => setIsPreferencesOpen(false)}
                    className="icon-ghost"
                    aria-label={t('common_close')}
                    title={t('common_close')}
                  >
                    <X size={16} strokeWidth={2.5} />
                  </button>
                </div>
                <div className="p-2">
                  <button
                    type="button"
                    onClick={() => setLang((l) => (l === 'zh' ? 'en' : 'zh'))}
                    className="menu-item menu-item-hover"
                  >
                    {t('admin_change_lang')}: {lang === 'zh' ? t('language_zh') : t('language_en')}
                  </button>
                  <ThemeToggle
                    size="md"
                    variant="plain"
                    showLabel
                    labelMode="action"
                    actionLabel={t('admin_change_theme')}
                    className="menu-item menu-item-hover justify-start"
                  />
                </div>
              </div>
            )}
          </div>

          <div className="hidden md:flex items-center gap-3">
            <button
              onClick={() => setLang((l) => (l === 'zh' ? 'en' : 'zh'))}
              className="p-2 rounded-full hover:bg-(--theme-bg-muted) dark:hover:bg-(--theme-bg-default)/30 text-(--theme-fg-default) dark:text-(--theme-fg-control-hover) transition-colors font-mono text-xs font-bold border border-transparent hover:border-(--theme-border-subtle) dark:hover:border-(--theme-border-default)/60"
            >
              {lang.toUpperCase()}
            </button>
            <ThemeToggle size="sm" variant="soft" />
          </div>

          {isAuthenticated ? (
            <div className="flex items-center gap-2">
              <div className="relative group">
                <Link
                  to="/admin"
                  className="btn-icon"
                  aria-label={t('admin_console')}
                  title={t('admin_console')}
                >
                  <UserCog size={18} />
                </Link>
                <div className="hidden md:block absolute left-1/2 -translate-x-1/2 top-full mt-0 opacity-0 invisible group-hover:opacity-100 group-hover:visible group-focus-within:opacity-100 group-focus-within:visible transition-all duration-200 motion-reduce:transition-none transform origin-top z-50 pt-1.5">
                  <div className="bg-(--theme-bg-default) dark:bg-(--theme-canvas-subtle) rounded-full shadow-xl border border-(--theme-border-subtle) dark:border-(--theme-border-default) p-1 overflow-hidden">
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        logout();
                      }}
                      className="flex items-center justify-center size-8 text-(--theme-bg-danger-emphasis) hover:bg-(--theme-bg-danger-muted) dark:hover:bg-(--theme-bg-danger-muted) rounded-full transition-colors"
                      title={t('admin_logout')}
                      aria-label={t('admin_logout')}
                    >
                      <LogOut size={16} />
                    </button>
                  </div>
                </div>
              </div>
              <Button
                type="button"
                onClick={logout}
                variant="iconDanger"
                className="md:hidden"
                aria-label={t('admin_logout')}
                title={t('admin_logout')}
                icon={LogOut}
              />
            </div>
          ) : (
            <Link
              to="/login"
              className="btn-icon"
              aria-label={t('login_sign_in')}
              title={t('login_sign_in')}
            >
              <LogIn size={16} />
            </Link>
          )}
        </div>
      </div>
      {isMobileSearchOpen &&
        typeof document !== 'undefined' &&
        createPortal(
          <div
            className="fixed inset-0 z-60 flex items-center justify-center px-6 md:hidden"
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                event.preventDefault();
                closeMobileSearch();
                return;
              }
              if (event.key !== 'Tab') return;

              const dialog = mobileSearchDialogRef.current;
              if (!dialog) return;
              const focusable = Array.from(
                dialog.querySelectorAll<HTMLElement>(
                  'button:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])',
                ),
              );
              if (focusable.length === 0) {
                event.preventDefault();
                return;
              }

              const first = focusable[0];
              const last = focusable[focusable.length - 1];
              if (event.shiftKey && document.activeElement === first) {
                event.preventDefault();
                last.focus();
              } else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault();
                first.focus();
              }
            }}
          >
            <div
              className="absolute inset-0 bg-(--theme-overlay-scrim) backdrop-blur-sm"
              aria-hidden="true"
              onClick={closeMobileSearch}
            />
            <div
              ref={mobileSearchDialogRef}
              role="dialog"
              aria-modal="true"
              aria-label={t('search_placeholder')}
              className="relative w-full max-w-xs rounded-full border border-(--theme-border-default) bg-(--theme-bg-default) shadow-xl dark:border-(--theme-border-default) dark:bg-(--theme-canvas-subtle)"
            >
              <SearchInput
                ref={mobileSearchInputRef}
                type="text"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                placeholder={t('search_placeholder')}
                aria-label={t('search_placeholder')}
                icon={Search}
                className="py-1.5 rounded-full bg-(--theme-bg-muted) dark:bg-(--theme-bg-default)"
                wrapperClassName="w-full"
                rightElement={
                  <button
                    type="button"
                    className="grid size-7 place-items-center rounded-full text-(--theme-fg-muted) transition-colors hover:bg-(--theme-bg-muted) hover:text-(--theme-fg-default) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--theme-focus-ring)"
                    aria-label={t('common_close')}
                    title={t('common_close')}
                    onClick={closeMobileSearch}
                  >
                    <X size={16} aria-hidden="true" />
                  </button>
                }
              />
            </div>
          </div>,
          document.body,
        )}
    </header>
  );
};

export default Header;
