import React from 'react';
import { Translations } from './translations';

export type TranslationKey = keyof typeof Translations.en;
export type Lang = keyof typeof Translations;

const _zhMustCoverEn: Record<TranslationKey, string> = Translations.zh;
void _zhMustCoverEn;

export const translate = (
  lang: Lang,
  key: TranslationKey,
  vars?: Record<string, string | number>,
): string => {
  const dict = Translations[lang];
  const template = (dict as Record<TranslationKey, string>)[key] ?? key;
  if (!vars) return template;
  return template.replace(/\{\{(\w+)\}\}/g, (match, name: string) => {
    const value = vars[name];
    return value === undefined || value === null ? match : String(value);
  });
};

export interface I18nContextValue {
  lang: Lang;
  setLang: React.Dispatch<React.SetStateAction<Lang>>;
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string;
}

export const I18nContext = React.createContext<I18nContextValue>({
  lang: 'zh',
  setLang: () => {},
  t: (key) => (Translations.zh as Record<TranslationKey, string>)[key] || key,
});

const LANG_STORAGE_KEY = 'lang';

const readStoredLang = (): Lang | null => {
  if (typeof window === 'undefined') return null;
  try {
    const raw = window.localStorage.getItem(LANG_STORAGE_KEY);
    return raw === 'en' || raw === 'zh' ? raw : null;
  } catch {
    return null;
  }
};

const readBrowserLang = (): Lang => {
  if (typeof navigator === 'undefined') return 'zh';

  const items =
    Array.isArray(navigator.languages) && navigator.languages.length > 0
      ? navigator.languages
      : [navigator.language];

  for (const item of items) {
    const lang = item.toLowerCase();
    if (lang.startsWith('zh')) return 'zh';
    if (lang.startsWith('en')) return 'en';
  }

  return 'zh';
};

const readInitialLang = (): Lang => readStoredLang() ?? readBrowserLang();

export const I18nProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [lang, setLang] = React.useState<Lang>(() => readInitialLang());

  React.useEffect(() => {
    try {
      window.localStorage.setItem(LANG_STORAGE_KEY, lang);
    } catch {
      // ignore storage errors
    }
  }, [lang]);

  const t = React.useCallback(
    (key: TranslationKey, vars?: Record<string, string | number>) => translate(lang, key, vars),
    [lang],
  );

  const value = React.useMemo(() => ({ lang, setLang, t }), [lang, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
};

export const useI18n = () => React.useContext(I18nContext);
