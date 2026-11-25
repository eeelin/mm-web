import type { Locale } from './types';

/**
 * Display labels for values the backend returns verbatim (ModemManager
 * state names, call states, call reasons and SMS states). Keys are the
 * canonical values the backend uses — see modemState() and callState()
 * in internal/server/modems.go and calls.go, and smsState() in
 * internal/server/messages.go. Values that are already plain identifiers
 * (e.g. "SIM PIN", "2G", "IPv4") intentionally stay the same in every locale.
 */
export type StateTable = Record<string, Partial<Record<Locale, string>>>

export const modemStates: StateTable = {
  // States from modemState() in internal/server/modems.go.
  '已连接': { 'zh-CN': '已连接', en: 'Connected', sv: 'Ansluten' },
  '已注册': { 'zh-CN': '已注册', en: 'Registered', sv: 'Registrerat' },
  '已启用': { 'zh-CN': '已启用', en: 'Enabled', sv: 'Aktiverat' },
  '已禁用': { 'zh-CN': '已禁用', en: 'Disabled', sv: 'Inaktiverat' },
  '正在启用': { 'zh-CN': '正在启用', en: 'Enabling', sv: 'Aktiverar' },
  '正在禁用': { 'zh-CN': '正在禁用', en: 'Disabling', sv: 'Inaktiverar' },
  '正在连接': { 'zh-CN': '正在连接', en: 'Connecting', sv: 'Ansluter' },
  '正在搜索网络': { 'zh-CN': '正在搜索网络', en: 'Searching for network', sv: 'Söker nätverk' },
  '正在断开': { 'zh-CN': '正在断开', en: 'Disconnecting', sv: 'Kopplar ner' },
  '未知状态': { 'zh-CN': '未知状态', en: 'Unknown status', sv: 'Okänd status' },
  '未注册网络': { 'zh-CN': '未注册网络', en: 'Network not registered', sv: 'Nätverk ej registrerat' },
  '蜂窛调制解调器': { 'zh-CN': '蜂窛调制解调器', en: 'Cellular modem', sv: 'Mobilmodem' },
  // Power status (modems.go).
  '关闭': { 'zh-CN': '关闭', en: 'Off', sv: 'Av' },
  '低功耗': { 'zh-CN': '低功耗', en: 'Low power', sv: 'Strömsparläge' },
  '开启': { 'zh-CN': '开启', en: 'On', sv: 'På' },
  // Registration status (modems.go).
  '空闲': { 'zh-CN': '空闲', en: 'Idle', sv: 'Fria' },
  '本地网络': { 'zh-CN': '本地网络', en: 'Local network', sv: 'Lokalt nätverk' },
  '搜索中': { 'zh-CN': '搜索中', en: 'Searching', sv: 'Söker' },
  '已拒绝': { 'zh-CN': '已拒绝', en: 'Rejected', sv: 'Avvisat' },
  '漫游': { 'zh-CN': '漫游', en: 'Roaming', sv: 'Roaming' },
  '未附着': { 'zh-CN': '未附着', en: 'Not attached', sv: 'Inte ansluten' },
  '已附着': { 'zh-CN': '已附着', en: 'Attached', sv: 'Ansluten' },
};

// States from callState() in internal/server/calls.go.
export const phoneCallStates: StateTable = {
  unknown: { 'zh-CN': '正在读取', en: 'Reading', sv: 'Läser in' },
  dialing: { 'zh-CN': '正在呼叫…', en: 'Dialing…', sv: 'Ringer…' },
  'ringing-out': { 'zh-CN': '对方响铃中', en: 'Ringing out', sv: 'Utringsning' },
  'ringing-in': { 'zh-CN': '来电', en: 'Incoming call', sv: 'Inkommande samtal' },
  active: { 'zh-CN': '通话中', en: 'Active', sv: 'Aktivt' },
  held: { 'zh-CN': '通话保持中', en: 'On hold', sv: 'Väntar' },
  waiting: { 'zh-CN': '呼叫等待', en: 'Call waiting', sv: 'Samtal väntar' },
  terminated: { 'zh-CN': '通话已结束', en: 'Call ended', sv: 'Samtalet avslutats' },
};

// Reasons from callReason() in internal/server/calls.go.
export const phoneCallReasons: StateTable = {
  'refused-or-busy': { 'zh-CN': '对方忙或已拒绝', en: 'Busy or rejected', sv: 'Upptaget eller avslått' },
  error: { 'zh-CN': '呼叫失败', en: 'Call error', sv: 'Samtalssystemfel' },
  'audio-setup-failed': { 'zh-CN': '音频链路设置失败', en: 'Audio link setup failed', sv: 'Ljudlänkstjänsten misslyckades' },
  terminated: { 'zh-CN': '通话已结束', en: 'Call ended', sv: 'Samtalet avslutats' },
};

// States from smsState() in internal/server/messages.go (displayed labels).
export const messageStates: StateTable = {
  sent: { 'zh-CN': '已发送', en: 'Sent', sv: 'Skickat' },
  sending: { 'zh-CN': '发送中', en: 'Sending', sv: 'Skickar' },
  stored: { 'zh-CN': '待发送', en: 'Queued', sv: 'Väntar' },
};

export function translate<T extends StateTable>(table: T, canonical: string, locale: Locale): string {
  return table[canonical]?.[locale] ?? canonical;
}
