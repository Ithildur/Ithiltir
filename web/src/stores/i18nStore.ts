import { create } from 'zustand';
import { Translations } from '@i18n/translations';

export type Lang = keyof typeof Translations;
export type LangUpdate = Lang | ((current: Lang) => Lang);

export interface I18nState {
  lang: Lang;
  setLang: (next: LangUpdate) => void;
}

export const LANG_STORAGE_KEY = 'lang';

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

export const useI18nStore = create<I18nState>()((set) => ({
  lang: readInitialLang(),
  setLang: (next) =>
    set((state) => ({
      lang: typeof next === 'function' ? next(state.lang) : next,
    })),
}));
