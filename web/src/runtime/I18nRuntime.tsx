import React from 'react';
import { LANG_STORAGE_KEY, useI18nStore } from '@stores/i18nStore';

export const I18nRuntime: React.FC = () => {
  const lang = useI18nStore((state) => state.lang);

  React.useEffect(() => {
    try {
      window.localStorage.setItem(LANG_STORAGE_KEY, lang);
    } catch {
      // Language persistence is best-effort.
    }
  }, [lang]);

  return null;
};
