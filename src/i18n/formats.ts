import type { Locale } from './types';
import { localeEntry } from './registry';

/** Short time-of-day string, e.g. "09:41". */
export function timeOfDay(date: Date, locale: Locale): string {
  return date.toLocaleTimeString(localeEntry(locale).bcp47, { hour: '2-digit', minute: '2-digit', hour12: false });
}

/** Date plus time, used where the backend reports a server timestamp. */
export function dateTime(date: Date, locale: Locale): string {
  return date.toLocaleString(localeEntry(locale).bcp47, {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  });
}

/** Locale-aware day header for the home screen ("8月17日 · 星期一" style). */
export function formatDayHeader(date: Date, locale: Locale): string {
  const m = String(date.getMonth() + 1);
  const d = String(date.getDate());
  const day = date.getDay();
  if (locale === 'zh-CN') {
    const weekdays = ['日', '一', '二', '三', '四', '五', '六'];
    return `${m}月${d}日 · 星期${weekdays[day]}`;
  }
  if (locale === 'en') {
    const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
    return `${weekdays[day]}, ${m}/${d}`;
  }
  const weekdays = ['söndag', 'måndag', 'tisdag', 'onsdag', 'torsdag', 'fredag', 'lördag'];
  return `${weekdays[day]}, ${d}/${m}`;
}

/** Conversation timestamp for the Messages screen. */
export function formatTime(value: string, locale: Locale): string {
  if (!value) return '';
  const date = new Date(value.replace(/([+-]\d{2})$/, '$1:00'));
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(localeEntry(locale).bcp47, {
    month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false,
  });
}

/** Timestamp for the Phone history list: time today, date otherwise. */
export function formatHistoryTime(value: string, locale: Locale): string {
  const date = new Date(value);
  const today = new Date();
  if (date.toDateString() === today.toDateString()) return timeOfDay(date, locale);
  return date.toLocaleDateString(localeEntry(locale).bcp47, { month: 'numeric', day: 'numeric' });
}

/** Uptime string for the About page. */
export function formatUptime(seconds: number, locale: Locale): string {
  const unit = (en: string, sv: string, zh: string): string => {
    if (locale === 'zh-CN') return zh;
    return locale === 'en' ? en : sv;
  };
  if (seconds < 60) return `${seconds} ${unit('sec', 'sek', '秒')}`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)} ${unit('min', 'min', '分钟')}`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} ${unit('hr', 't', '小时')}`;
  return `${Math.floor(seconds / 86400)} ${unit('d', 'dygn', '天')}`;
}
