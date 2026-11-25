import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import type { Translations } from './locales/zhCN';
import type { Locale } from './types';
import { localeEntry, readStoredLocale, storeLocale, type LocaleEntry } from './registry';

export type T = (key: keyof Translations, vars?: Record<string, string | number>) => string;

function makeT(translations: Translations): T {
  return (key, vars) => {
    const text = translations[key] ?? key;
    if (!vars) return text;
    return text.replace(/\{(\w+)\}/g, (match, name: string) => (vars[name] !== undefined ? String(vars[name]) : match));
  };
}

export interface I18nContextValue {
  locale: Locale;
  localeEntry: LocaleEntry;
  setLocale: (locale: Locale) => void;
  t: T;
}

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(readStoredLocale);
  const entry = localeEntry(locale);
  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    storeLocale(next);
  }, []);
  const value = useMemo(
    () => ({ locale, localeEntry: entry, setLocale, t: makeT(entry.translations) }),
    [locale, entry, setLocale],
  );
  useEffect(() => {
    document.documentElement.lang = entry.locale;
  }, [entry.locale]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const value = useContext(I18nContext);
  if (!value) throw new Error('useI18n() must be used inside <I18nProvider>');
  return value;
}

export function useT(): T {
  return useI18n().t;
}
