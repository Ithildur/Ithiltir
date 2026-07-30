import React from 'react';
import { useI18nStore } from '@stores/i18nStore';
import type { Lang, LangUpdate } from '@stores/i18nStore';
import { Translations } from './translations';

export type TranslationKey = keyof typeof Translations.en;
export type { Lang };

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

export interface I18nValue {
  lang: Lang;
  setLang: (next: LangUpdate) => void;
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string;
}

export const useI18n = (): I18nValue => {
  const lang = useI18nStore((state) => state.lang);
  const setLang = useI18nStore((state) => state.setLang);

  const t = React.useCallback(
    (key: TranslationKey, vars?: Record<string, string | number>) => translate(lang, key, vars),
    [lang],
  );

  return React.useMemo(() => ({ lang, setLang, t }), [lang, setLang, t]);
};
