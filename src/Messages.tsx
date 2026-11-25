import { useEffect, useMemo, useState } from 'react';
import { Bell, BellRing, ChevronLeft, Edit3, MessageSquare, Search, SendHorizontal, Trash2 } from 'lucide-react';
import { formatTime, messageStates, translate, useI18n } from './i18n';
import { clearUnread } from './unread';

type Message = {
  id: string;
  modemId: string;
  number: string;
  text: string;
  direction: 'received' | 'sent';
  state: string;
  timestamp: string;
};

type Conversation = { number: string; messages: Message[]; latest: Message };

export function Messages({onClose}:{onClose:()=>void}) {
  const { t, locale } = useI18n();
  const [messages, setMessages] = useState<Message[]>([]);
  const [view, setView] = useState<'list'|'thread'|'compose'>('list');
  const [number, setNumber] = useState('');
  const [draft, setDraft] = useState('');
  const [query, setQuery] = useState('');
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const [pushState, setPushState] = useState<'loading'|'unsupported'|'off'|'on'|'denied'>('loading');

  useEffect(() => {
    if (!('serviceWorker' in navigator) || !('PushManager' in window) || !('Notification' in window)) {
      setPushState('unsupported'); return;
    }
    navigator.serviceWorker.ready.then(registration => registration.pushManager.getSubscription())
      .then(subscription => setPushState(subscription ? 'on' : Notification.permission === 'denied' ? 'denied' : 'off'))
      .catch(() => setPushState('unsupported'));
  }, []);

  const togglePush = async () => {
    try {
      const registration = await navigator.serviceWorker.ready;
      const existing = await registration.pushManager.getSubscription();
      if (existing) {
        await fetch('/api/push/subscriptions', {method:'DELETE',headers:{'Content-Type':'application/json'},body:JSON.stringify({endpoint:existing.endpoint})});
        await existing.unsubscribe(); setPushState('off'); return;
      }
      const permission = await Notification.requestPermission();
      if (permission !== 'granted') { setPushState('denied'); return; }
      const configResponse = await fetch('/api/push', {cache:'no-store'});
      if (!configResponse.ok) throw new Error(t('messagesServicePushNotReady'));
      const {publicKey} = await configResponse.json() as {publicKey:string};
      const subscription = await registration.pushManager.subscribe({userVisibleOnly:true,applicationServerKey:urlBase64ToUint8Array(publicKey)});
      const response = await fetch('/api/push/subscriptions', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(subscription)});
      if (!response.ok) { await subscription.unsubscribe(); throw new Error(t('messagesSavePushSubscriptionFailed')); }
      setPushState('on');
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('messagesEnableNotifyFailed'));
    }
  };

  const load = async () => {
    try {
      const response = await fetch('/api/messages', {cache:'no-store'});
      if (!response.ok) throw new Error();
      const data = await response.json() as {messages:Message[]};
      setMessages(data.messages);
      setError('');
      void clearUnread('messages');
    } catch {
      setError(t('messagesReadMessagesFailed'));
    }
  };

  useEffect(() => {
    load();
    const id = window.setInterval(load, 5000);
    return () => window.clearInterval(id);
  }, [t]);

  const conversations = useMemo(() => {
    const grouped = new Map<string,Message[]>();
    for (const item of messages) grouped.set(item.number, [...(grouped.get(item.number) ?? []), item]);
    return [...grouped].map(([conversationNumber, items]) => ({number:conversationNumber,messages:items,latest:items[0]} satisfies Conversation));
  }, [messages]);
  const visible = conversations.filter(item => item.number.includes(query) || item.messages.some(message => message.text.toLowerCase().includes(query.toLowerCase())));
  const thread = [...(conversations.find(item => item.number === number)?.messages ?? [])].reverse();

  const openThread = (conversationNumber:string) => { setNumber(conversationNumber); setView('thread'); setDraft(''); setError(''); };
  const send = async () => {
    if (pending || !number.trim() || !draft.trim()) return;
    setPending(true); setError('');
    try {
      const response = await fetch('/api/messages', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({number:number.trim(),text:draft.trim()})});
      const data = await response.json().catch(()=>({})) as {error?:string};
      if (!response.ok) throw new Error(data.error || t('messagesSendFailed'));
      setDraft(''); setView('thread'); await load();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('messagesSendFailed'));
    } finally { setPending(false); }
  };
  const removeConversation = async () => {
    if (pending || !window.confirm(t('messagesDeleteConfirm', {number}))) return;
    setPending(true); setError('');
    try {
      for (const item of thread) {
        const response = await fetch(`/api/messages/${item.id}`, {method:'DELETE'});
        if (!response.ok) throw new Error(t('messagesDeleteFailed'));
      }
      await load(); setView('list'); setNumber('');
    } catch (reason) { setError(reason instanceof Error ? reason.message : t('messagesDeleteFailed')); }
    finally { setPending(false); }
  };

  if (view === 'list') return <div className="messages-page">
    <MessagesBar title={t('messagesTitle')} onBack={onClose} action={<div className="messages-actions"><button aria-label={pushState==='on'?t('messagesStopPush'):t('messagesStartPush')} onClick={togglePush} disabled={pushState==='loading'||pushState==='unsupported'}>{pushState==='on'?<BellRing/>:<Bell/>}</button><button aria-label={t('messagesComposeTitle')} onClick={()=>{setView('compose');setNumber('');setDraft('')}}><Edit3/></button></div>}/>
    <div className="messages-content">
      {pushState==='off' && <button className="push-tip" onClick={togglePush}><Bell/><span><b>{t('messagesPushTipTitle')}</b><small>{t('messagesPushTipBody')}</small></span></button>}
      {pushState==='denied' && <p className="push-note">{t('messagesPushDenied')}</p>}
      {pushState==='unsupported' && <p className="push-note">{t('messagesPushUnsupported')}</p>}
      <label className="message-search"><Search/><input value={query} onChange={event=>setQuery(event.target.value)} placeholder={t('messagesSearch')}/></label>
      {error && <p className="messages-error">{error}</p>}
      <div className="conversation-list">{visible.map(item=><button key={item.number} className="conversation" onClick={()=>openThread(item.number)}>
        <span className="contact-avatar">{avatarText(item.number)}</span><span className="conversation-copy"><span><b>{displayNumber(item.number)}</b><time>{formatTime(item.latest.timestamp, locale)}</time></span><small>{item.latest.direction==='sent'?t('messagesSentPrefix'):''}{item.latest.text}</small></span>
      </button>)}</div>
      {!error && visible.length===0 && <div className="messages-empty"><MessageSquare/><b>{query?t('messagesNoResults'):t('messagesNoMessages')}</b><p>{query?t('messagesSearchTip'):t('messagesEmptyTip')}</p></div>}
    </div>
  </div>;

  if (view === 'compose') return <div className="messages-page">
    <MessagesBar title={t('messagesComposeTitle')} onBack={()=>setView('list')} />
    <div className="recipient"><span>{t('messagesRecipientPrefix')}</span><input autoFocus inputMode="tel" value={number} onChange={event=>setNumber(event.target.value)} placeholder={t('messagesNumberPlaceholder')}/></div>
    <div className="new-message-space"/>
    {error && <p className="messages-error compose-error">{error}</p>}
    <Composer value={draft} onChange={setDraft} onSend={send} disabled={pending || !number.trim()}/>
  </div>;

  return <div className="messages-page">
    <MessagesBar title={displayNumber(number)} subtitle={number} onBack={()=>setView('list')} action={<button aria-label={t('messagesDeleteConversation')} onClick={removeConversation} disabled={pending}><Trash2/></button>}/>
    <div className="message-thread">{thread.map(item=><div key={item.id} className={`bubble-row ${item.direction}`}><div className="bubble"><p>{item.text}</p><small>{formatTime(item.timestamp, locale)}{item.direction==='sent'&&` · ${translate(messageStates, item.state, locale)}`}</small></div></div>)}</div>
    {error && <p className="messages-error thread-error">{error}</p>}
    <Composer value={draft} onChange={setDraft} onSend={send} disabled={pending}/>
  </div>;
}

function MessagesBar({title,subtitle,onBack,action}:{title:string;subtitle?:string;onBack:()=>void;action?:React.ReactNode}) {
  const { t } = useI18n();
  return <header className="messages-bar"><button aria-label={t('back')} onClick={onBack}><ChevronLeft/></button><div><h1>{title}</h1>{subtitle&&subtitle!==title&&<small>{subtitle}</small>}</div><span className="messages-action-slot">{action}</span></header>
}
function Composer({value,onChange,onSend,disabled}:{value:string;onChange:(value:string)=>void;onSend:()=>void;disabled:boolean}) {
  const { t } = useI18n();
  return <div className="composer"><textarea rows={1} value={value} onChange={event=>onChange(event.target.value)} placeholder={t('messagesMessagePlaceholder')} onKeyDown={event=>{if(event.key==='Enter'&&!event.shiftKey){event.preventDefault();onSend()}}}/><button aria-label={t('messagesSendAria')} onClick={onSend} disabled={disabled||!value.trim()}><SendHorizontal/></button></div>
}
function avatarText(number:string) { return number.replace(/^\+86/,'').slice(0,2) || '?' }
function displayNumber(number:string) { return ({'10086':'中国移动','10086100':'中国移动服务'} as Record<string,string>)[number] ?? number }
function urlBase64ToUint8Array(value:string) { const padding='='.repeat((4-value.length%4)%4); const raw=atob((value+padding).replace(/-/g,'+').replace(/_/g,'/')); return Uint8Array.from([...raw].map(char=>char.charCodeAt(0))); }
