import { create } from 'zustand';
import { Translations } from '@i18n/translations';

export type Lang = keyof typeof Translations;
export type LangUpdate = Lang | ((current: Lang) => Lang);

interface I18nState {
  lang: Lang;
  setLang: (next: LangUpdate) => void;
}

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

const writeStoredLang = (lang: Lang): void => {
  try {
    window.localStorage.setItem(LANG_STORAGE_KEY, lang);
  } catch {
    // Language persistence is best-effort.
  }
};

export const useI18nStore = create<I18nState>()((set, get) => {
  const lang = readStoredLang() ?? readBrowserLang();
  writeStoredLang(lang);
  return {
    lang,
    setLang: (next) => {
      const nextLang = typeof next === 'function' ? next(get().lang) : next;
      set({ lang: nextLang });
      writeStoredLang(nextLang);
    },
  };
});
