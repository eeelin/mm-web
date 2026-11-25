import type { Locale } from './types';
import type { Translations } from './locales/zhCN';
import * as zhCN from './locales/zhCN';
import * as en from './locales/en';
import * as sv from './locales/sv';

export const STORAGE_KEY = 'mmos-locale';
export const DEFAULT_LOCALE: Locale = 'zh-CN';

export interface LocaleEntry {
  locale: Locale;
  name: string;
  bcp47: string;
  translations: Translations;
}

export const LOCALES: LocaleEntry[] = [
  { locale: zhCN.locale, name: zhCN.name, bcp47: zhCN.bcp47, translations: zhCN.translations },
  { locale: en.locale, name: en.name, bcp47: en.bcp47, translations: en.translations },
  { locale: sv.locale, name: sv.name, bcp47: sv.bcp47, translations: sv.translations },
];

export function isLocale(value: unknown): value is Locale {
  return LOCALES.some(entry => entry.locale === value);
}

export function findLocale(value: unknown): LocaleEntry | undefined {
  return LOCALES.find(entry => entry.locale === value);
}

export function localeEntry(locale: Locale): LocaleEntry {
  return findLocale(locale) ?? LOCALES[0];
}

export function readStoredLocale(): Locale {
  try {
    const stored = typeof window !== 'undefined' ? window.localStorage.getItem(STORAGE_KEY) : null;
    return findLocale(stored)?.locale ?? DEFAULT_LOCALE;
  } catch {
    return DEFAULT_LOCALE;
  }
}

export function storeLocale(locale: Locale): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // Storage may be unavailable (private mode, disabled cookies); the
    // in-memory selection still works for the current session.
  }
}
