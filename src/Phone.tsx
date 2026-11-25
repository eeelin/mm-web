import { useCallback, useEffect, useState } from 'react';
import { ChevronLeft, Delete, Grid3X3, MicOff, Phone as PhoneIcon, PhoneIncoming, PhoneMissed, PhoneOff, PhoneOutgoing, Trash2 } from 'lucide-react';
import { formatHistoryTime, phoneCallReasons, phoneCallStates, translate, useI18n, type Locale } from './i18n';
import { clearUnread } from './unread';

type Call = {id:string; modemId:string; number:string; direction:string; state:string; reason?:string};
type CallsResponse = {calls:Call[]; voiceAvailable:boolean};
type HistoryEntry = {id:string; number:string; direction:string; outcome:string; startedAt:string; connectedAt?:string; endedAt?:string};

const keys=[['1',''],['2','ABC'],['3','DEF'],['4','GHI'],['5','JKL'],['6','MNO'],['7','PQRS'],['8','TUV'],['9','WXYZ'],['*',''],['0','+'],['#','']];

export function Phone({onClose}:{onClose:()=>void}) {
  const { t } = useI18n();
  const [tab,setTab]=useState<'recent'|'keypad'>('recent'); const [number,setNumber]=useState(''); const [calls,setCalls]=useState<Call[]>([]); const [history,setHistory]=useState<HistoryEntry[]>([]); const [recentCallId,setRecentCallId]=useState(''); const [voiceAvailable,setVoiceAvailable]=useState<boolean|null>(null); const [loading,setLoading]=useState(false); const [error,setError]=useState(''); const [showKeys,setShowKeys]=useState(false);
  const load=useCallback(async()=>{try{const response=await fetch('/api/calls',{cache:'no-store'});const data=await response.json() as CallsResponse&{error?:string};if(!response.ok)throw new Error(data.error||t('serviceUnavailable'));setCalls(data.calls);setVoiceAvailable(data.voiceAvailable);void clearUnread('phone')}catch(err){setError(err instanceof Error?err.message:t('phoneReadFailed'))}},[t]);
  const loadHistory=useCallback(async()=>{try{const response=await fetch('/api/call-history',{cache:'no-store'});if(response.ok)setHistory((await response.json() as {calls:HistoryEntry[]}).calls)}catch{}},[]);
  useEffect(()=>{load();loadHistory();const refresh=()=>{load();window.setTimeout(loadHistory,1200)};const events=new EventSource('/api/calls/events');events.addEventListener('call-state',refresh);const fallback=window.setInterval(refresh,5000);return()=>{events.close();window.clearInterval(fallback)}},[load,loadHistory]);
  const active=calls.find(call=>call.state!=='terminated')??calls.find(call=>call.id===recentCallId);
  const request=async(url:string,body?:unknown)=>{setLoading(true);setError('');try{const response=await fetch(url,{method:'POST',headers:body?{'Content-Type':'application/json'}:undefined,body:body?JSON.stringify(body):undefined});const data=response.status===204?null:await response.json();if(!response.ok)throw new Error(data?.error||t('actionFailed'));await load();return data}catch(err){setError(err instanceof Error?err.message:t('actionFailed'));return null}finally{setLoading(false)}};
  const dialNumber=async(value:string)=>{if(!value)return;const data=await request('/api/calls',{number:value});if(data?.id)setRecentCallId(data.id)};
  const dial=()=>dialNumber(number);
  const clearHistory=async()=>{if(!window.confirm(t('phoneClearHistoryConfirm')))return;const response=await fetch('/api/call-history',{method:'DELETE'});if(response.ok)setHistory([]);else setError(t('phoneClearHistoryFailed'))};
  const tone=(key:string)=>{if(active?.state==='active')void request(`/api/calls/${active.id}/dtmf`,{tones:key});else setNumber(value=>(value+key).slice(0,32))};
  if(active) return <CallScreen call={active} loading={loading} error={error} showKeys={showKeys} onKeys={()=>setShowKeys(value=>!value)} onTone={tone} onAccept={()=>void request(`/api/calls/${active.id}/accept`)} onHangup={()=>void request(`/api/calls/${active.id}/hangup`)} onDone={()=>{setRecentCallId('');setNumber('');setError('')}} onClose={onClose}/>;
  return <div className="phone-page"><header className="phone-bar"><button onClick={onClose} aria-label={t('back')}><ChevronLeft/></button><h1>{t('phoneTitle')}</h1>{tab==='recent'&&history.length>0?<button onClick={clearHistory} aria-label={t('phoneClearHistory')}><Trash2/></button>:<span/>}</header>{error&&<p className="call-error phone-error">{error}</p>}{tab==='recent'?<RecentCalls entries={history} onCall={value=>void dialNumber(value)}/>:<main className="dialer"><div className="dial-number"><span>{number||t('phoneNumberPlaceholder')}</span>{number&&<button onClick={()=>setNumber(value=>value.slice(0,-1))} aria-label={t('phoneBackspace')}><Delete/></button>}</div><div className="dial-keys">{keys.map(([key,letters])=><button key={key} onClick={()=>tone(key)} onContextMenu={event=>{event.preventDefault();if(key==='0')setNumber(value=>value+'+')}}><b>{key}</b><small>{letters}</small></button>)}</div><button className="dial-call" onClick={dial} disabled={loading||!number||voiceAvailable!==true} aria-label={t('phoneCallAria')}><PhoneIcon/></button><p className="voice-note">{voiceAvailable===false?t('phoneVoiceUnsupported'):voiceAvailable===null?t('phoneVoiceDetecting'):t('phoneVoiceNote')}</p></main>}<nav className="phone-tabs"><button className={tab==='recent'?'active':''} onClick={()=>setTab('recent')}>{t('phoneRecentTitle')}</button><button className={tab==='keypad'?'active':''} onClick={()=>setTab('keypad')}>{t('phoneKeypadTab')}</button></nav></div>
}

function RecentCalls({entries,onCall}:{entries:HistoryEntry[];onCall:(number:string)=>void}) {
  const { t, locale } = useI18n();
  if(entries.length===0)return <main className="recent-empty"><PhoneIcon/><b>{t('phoneRecentEmpty')}</b><p>{t('phoneRecentEmptyBody')}</p></main>;
  return <main className="recent-calls"><h2>{t('phoneRecentTitle')}</h2><div>{entries.map(entry=>{const missed=entry.outcome==='missed'||entry.outcome==='failed';const Icon=missed?PhoneMissed:entry.direction==='incoming'?PhoneIncoming:PhoneOutgoing;return <button key={entry.id} onClick={()=>onCall(entry.number)}><span className={`history-icon ${missed?'missed':''}`}><Icon/></span><span className="history-copy"><b className={missed?'missed':''}>{entry.number||t('phoneUnknownNumber')}</b><small>{historyLabel(entry,locale)}{durationLabel(entry)}</small></span><time>{formatHistoryTime(entry.startedAt, locale)}</time></button>})}</div></main>
}
function historyLabel(entry:HistoryEntry,locale:Locale){
  const { t } = useI18n();
  if(entry.outcome==='missed')return t('phoneOutcomeMissed');
  if(entry.outcome==='failed')return t('phoneOutcomeFailed');
  return entry.direction==='incoming'?t('phoneDirectionIncoming'):t('phoneDirectionOutgoing');
}
function durationLabel(entry:HistoryEntry){if(!entry.connectedAt||!entry.endedAt)return '';const seconds=Math.max(0,Math.floor((Date.parse(entry.endedAt)-Date.parse(entry.connectedAt))/1000));return ` · ${Math.floor(seconds/60)}:${String(seconds%60).padStart(2,'0')}`}

function CallScreen({call,loading,error,showKeys,onKeys,onTone,onAccept,onHangup,onDone,onClose}:{call:Call;loading:boolean;error:string;showKeys:boolean;onKeys:()=>void;onTone:(key:string)=>void;onAccept:()=>void;onHangup:()=>void;onDone:()=>void;onClose:()=>void}) {
  const { t, locale } = useI18n();
  const ended=call.state==='terminated';
  const incoming=call.state==='ringing-in';
  return <div className="active-call"><header><button onClick={onClose} aria-label={t('back')}><ChevronLeft/></button></header><main><div className="call-avatar">{call.number.slice(-2)||'?'}</div><h1>{call.number||t('phoneUnknownNumber')}</h1><p>{ended?translate(phoneCallReasons, call.reason||'', locale)||translate(phoneCallStates, call.state, locale):translate(phoneCallStates, call.state, locale)}</p>{error&&<p className="call-error">{error}</p>}{showKeys&&<div className="call-keypad">{'123456789*0#'.split('').map(key=><button key={key} onClick={()=>onTone(key)}>{key}</button>)}</div>}{!incoming&&<div className="call-controls"><button disabled title={t('phoneMuteTitle')}><MicOff/><span>{t('phoneMute')}</span></button><button onClick={onKeys} disabled={ended}><Grid3X3/><span>{t('phoneShowKeypad')}</span></button></div>}{incoming&&<div className="incoming-actions"><button className="reject" onClick={onHangup} disabled={loading}><PhoneOff/><span>{t('phoneReject')}</span></button><button className="accept" onClick={onAccept} disabled={loading}><PhoneIcon/><span>{t('phoneAccept')}</span></button></div>}{!ended&&!incoming&&<button className="hangup" onClick={onHangup} disabled={loading}><PhoneOff/></button>}{ended&&<button className="call-done" onClick={onDone}>{t('phoneDone')}</button>}<small className="audio-note">{t('phoneAudioNote')}</small></main></div>
}
