import { en } from './en';
import { zh } from './zh';

export const Translations = {
  en,
  zh,
} as const satisfies Record<string, Record<keyof typeof en, string>>;
