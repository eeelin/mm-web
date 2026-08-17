import { useEffect, useMemo, useState } from 'react';
import { Bell, BellRing, ChevronLeft, Edit3, MessageSquare, Search, SendHorizontal, Trash2 } from 'lucide-react';

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
      if (!configResponse.ok) throw new Error('服务端推送尚未就绪');
      const {publicKey} = await configResponse.json() as {publicKey:string};
      const subscription = await registration.pushManager.subscribe({userVisibleOnly:true,applicationServerKey:urlBase64ToUint8Array(publicKey)});
      const response = await fetch('/api/push/subscriptions', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(subscription)});
      if (!response.ok) { await subscription.unsubscribe(); throw new Error('保存推送订阅失败'); }
      setPushState('on');
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '启用消息通知失败');
    }
  };

  const load = async () => {
    try {
      const response = await fetch('/api/messages', {cache:'no-store'});
      if (!response.ok) throw new Error();
      const data = await response.json() as {messages:Message[]};
      setMessages(data.messages);
      setError('');
    } catch {
      setError('无法读取信息，正在重试');
    }
  };

  useEffect(() => {
    load();
    const id = window.setInterval(load, 5000);
    return () => window.clearInterval(id);
  }, []);

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
      if (!response.ok) throw new Error(data.error || '发送失败');
      setDraft(''); setView('thread'); await load();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : '发送失败');
    } finally { setPending(false); }
  };
  const removeConversation = async () => {
    if (pending || !window.confirm(`删除与 ${number} 的全部信息？此操作无法撤销。`)) return;
    setPending(true); setError('');
    try {
      for (const item of thread) {
        const response = await fetch(`/api/messages/${item.id}`, {method:'DELETE'});
        if (!response.ok) throw new Error('删除失败');
      }
      await load(); setView('list'); setNumber('');
    } catch (reason) { setError(reason instanceof Error ? reason.message : '删除失败'); }
    finally { setPending(false); }
  };

  if (view === 'list') return <div className="messages-page">
    <MessagesBar title="信息" onBack={onClose} action={<div className="messages-actions"><button aria-label={pushState==='on'?'关闭消息通知':'开启消息通知'} onClick={togglePush} disabled={pushState==='loading'||pushState==='unsupported'}>{pushState==='on'?<BellRing/>:<Bell/>}</button><button aria-label="新信息" onClick={()=>{setView('compose');setNumber('');setDraft('')}}><Edit3/></button></div>}/>
    <div className="messages-content">
      {pushState==='off' && <button className="push-tip" onClick={togglePush}><Bell/><span><b>开启新信息通知</b><small>前端关闭后也能收到提醒，不显示短信正文。</small></span></button>}
      {pushState==='denied' && <p className="push-note">通知权限已关闭，请在系统设置中允许 mmOS 通知。</p>}
      {pushState==='unsupported' && <p className="push-note">iPhone 请先将 mmOS 添加到主屏幕，再从主屏幕打开并开启通知。</p>}
      <label className="message-search"><Search/><input value={query} onChange={event=>setQuery(event.target.value)} placeholder="搜索"/></label>
      {error && <p className="messages-error">{error}</p>}
      <div className="conversation-list">{visible.map(item=><button key={item.number} className="conversation" onClick={()=>openThread(item.number)}>
        <span className="contact-avatar">{avatarText(item.number)}</span><span className="conversation-copy"><span><b>{displayNumber(item.number)}</b><time>{formatTime(item.latest.timestamp)}</time></span><small>{item.latest.direction==='sent'?'你：':''}{item.latest.text}</small></span>
      </button>)}</div>
      {!error && visible.length===0 && <div className="messages-empty"><MessageSquare/><b>{query?'未找到信息':'暂无信息'}</b><p>{query?'尝试搜索其他号码或内容。':'收到或发送的信息会显示在这里。'}</p></div>}
    </div>
  </div>;

  if (view === 'compose') return <div className="messages-page">
    <MessagesBar title="新信息" onBack={()=>setView('list')} />
    <div className="recipient"><span>收件人：</span><input autoFocus inputMode="tel" value={number} onChange={event=>setNumber(event.target.value)} placeholder="电话号码"/></div>
    <div className="new-message-space"/>
    {error && <p className="messages-error compose-error">{error}</p>}
    <Composer value={draft} onChange={setDraft} onSend={send} disabled={pending || !number.trim()}/>
  </div>;

  return <div className="messages-page">
    <MessagesBar title={displayNumber(number)} subtitle={number} onBack={()=>setView('list')} action={<button aria-label="删除对话" onClick={removeConversation} disabled={pending}><Trash2/></button>}/>
    <div className="message-thread">{thread.map(item=><div key={item.id} className={`bubble-row ${item.direction}`}><div className="bubble"><p>{item.text}</p><small>{formatTime(item.timestamp)}{item.direction==='sent'&&` · ${stateLabel(item.state)}`}</small></div></div>)}</div>
    {error && <p className="messages-error thread-error">{error}</p>}
    <Composer value={draft} onChange={setDraft} onSend={send} disabled={pending}/>
  </div>;
}

function MessagesBar({title,subtitle,onBack,action}:{title:string;subtitle?:string;onBack:()=>void;action?:React.ReactNode}) { return <header className="messages-bar"><button aria-label="返回" onClick={onBack}><ChevronLeft/></button><div><h1>{title}</h1>{subtitle&&subtitle!==title&&<small>{subtitle}</small>}</div><span className="messages-action-slot">{action}</span></header> }
function Composer({value,onChange,onSend,disabled}:{value:string;onChange:(value:string)=>void;onSend:()=>void;disabled:boolean}) { return <div className="composer"><textarea rows={1} value={value} onChange={event=>onChange(event.target.value)} placeholder="信息" onKeyDown={event=>{if(event.key==='Enter'&&!event.shiftKey){event.preventDefault();onSend()}}}/><button aria-label="发送" onClick={onSend} disabled={disabled||!value.trim()}><SendHorizontal/></button></div> }
function avatarText(number:string) { return number.replace(/^\+86/,'').slice(0,2) || '?' }
function displayNumber(number:string) { return ({'10086':'中国移动','10086100':'中国移动服务'} as Record<string,string>)[number] ?? number }
function stateLabel(state:string) { return ({sent:'已发送',sending:'发送中',stored:'待发送'} as Record<string,string>)[state] ?? state }
function formatTime(value:string) { if(!value)return ''; const date=new Date(value.replace(/([+-]\d{2})$/,'$1:00')); if(Number.isNaN(date.getTime()))return value; return date.toLocaleString('zh-CN',{month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit',hour12:false}) }
function urlBase64ToUint8Array(value:string) { const padding='='.repeat((4-value.length%4)%4); const raw=atob((value+padding).replace(/-/g,'+').replace(/_/g,'/')); return Uint8Array.from([...raw].map(char=>char.charCodeAt(0))); }
