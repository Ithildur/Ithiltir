import { translate, type Lang } from '@i18n';

type DateLike = Date | number | string;

const localeByLang: Record<Lang, string> = {
  zh: 'zh-CN',
  en: 'en-US',
};

const toDate = (value: DateLike): Date | null => {
  const parsed = value instanceof Date ? new Date(value.getTime()) : new Date(value);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
};

const getDateTimeLocale = (lang: Lang): string => localeByLang[lang] ?? localeByLang.en;

const formatDateTime = (
  value: DateLike,
  locale: string,
  options: Intl.DateTimeFormatOptions = {},
): string => {
  const parsed = toDate(value);
  if (!parsed) return '';

  return new Intl.DateTimeFormat(locale, options).format(parsed);
};

export const formatLocalDateTime = (
  value: DateLike,
  lang: Lang,
  options: Intl.DateTimeFormatOptions = {},
): string => formatDateTime(value, getDateTimeLocale(lang), options);

export const formatTimeInTimeZone = (
  value: DateLike,
  lang: Lang,
  timeZone: string,
  withSeconds = false,
): string =>
  formatDateTime(value, getDateTimeLocale(lang), {
    hour: '2-digit',
    minute: '2-digit',
    ...(withSeconds ? { second: '2-digit' } : {}),
    hour12: false,
    timeZone,
  });

export const formatUTCOffsetInTimeZone = (
  value: DateLike = new Date(),
  timeZone: string,
): string => {
  const parsed = toDate(value);
  if (!parsed) return '';

  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone,
    timeZoneName: 'shortOffset',
  }).formatToParts(parsed);
  const offset = parts.find((part) => part.type === 'timeZoneName')?.value;
  if (!offset) {
    throw new Error(`missing UTC offset for time zone ${timeZone}`);
  }
  return offset.replace(/^GMT/, 'UTC');
};

export const formatTimeAgo = (isoString: string, lang: Lang = 'en'): string => {
  const parsed = new Date(isoString);
  if (Number.isNaN(parsed.getTime())) return translate(lang, 'dashboard_time_na');

  const diffMs = Date.now() - parsed.getTime();
  const minutes = Math.floor(diffMs / 60_000);

  if (minutes < 1) return translate(lang, 'dashboard_time_just_now');
  if (minutes < 60) return translate(lang, 'dashboard_time_minutes_ago', { count: minutes });

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return translate(lang, 'dashboard_time_hours_ago', { count: hours });

  const days = Math.floor(hours / 24);
  return translate(lang, 'dashboard_time_days_ago', { count: days });
};
