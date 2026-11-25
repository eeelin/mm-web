import { Activity, BatteryMedium, Bell, CheckCircle2, ChevronLeft, ChevronRight, CircleHelp, Code2, Copy, ExternalLink, Globe, Info, LockKeyhole, MessageSquare, Phone, Radio, RefreshCw, Server, Settings, ShieldCheck, Signal, Smartphone, Wifi, XCircle } from 'lucide-react';
import { useEffect, useState } from 'react';
import { LOCALES, useI18n, dateTime, formatDayHeader, formatUptime, modemStates, timeOfDay, translate, type Locale } from './i18n';
import { Messages } from './Messages';
import { Phone as PhoneApp } from './Phone';
import { clearUnread, emptyUnreadState, readUnreadState, type UnreadState } from './unread';

type Modem = { id:string; name:string; model:string; state:string; network:string; tech:string; signal:number; sim:string; imei:string; firmware:string; port:string; manufacturer:string; deviceId:string; device:string; drivers:string[]; plugin:string; ports:string[]; ownNumbers:string[]; powerState:string; capabilities:string; supportedModes:string; currentModes:string; ipFamilies:string; operatorCode:string; registration:string; packetService:string; unlockRequired:string; unlockRetries:string[] };

export function App() {
  const { t, locale } = useI18n();
  const [screen, setScreen] = useState<'home'|'settings'|'modems'|'detail'|'messages'|'phone'|'about'|'diagnostics'>(() => {const requested=new URLSearchParams(location.search).get('screen');return requested==='messages'||requested==='phone'||requested==='diagnostics'?requested:'home'});
  const [modems, setModems] = useState<Modem[]>([]);
  const [selected, setSelected] = useState<Modem | null>(null);
  const [error, setError] = useState('');
  const [time, setTime] = useState('09:41');
  const [unread,setUnread]=useState<UnreadState>(emptyUnreadState);
  useEffect(()=>{let alive=true;const refresh=()=>void readUnreadState().then(state=>{if(alive)setUnread(state)});const changed=(event:Event)=>{const state=(event as CustomEvent<UnreadState>).detail;if(state)setUnread(state);else refresh()};refresh();window.addEventListener('mmos-unread-change',changed);navigator.serviceWorker?.addEventListener('message',changed);return()=>{alive=false;window.removeEventListener('mmos-unread-change',changed);navigator.serviceWorker?.removeEventListener('message',changed)}},[]);
  useEffect(() => { const tick=()=>setTime(timeOfDay(new Date(), locale)); tick(); const id=setInterval(tick,30000); return()=>clearInterval(id)},[locale]);
  useEffect(() => {
    let alive = true;
    const load = async () => {
      try {
        const response = await fetch('/api/modems', {cache: 'no-store'});
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const data = await response.json() as {modems: Modem[]};
        if (!alive) return;
        setModems(data.modems);
        setSelected(current => data.modems.find(m => m.id === current?.id) ?? data.modems[0] ?? null);
        setError('');
      } catch {
        if (alive) setError(t('connectRetrying'));
      }
    };
    load();
    const id = window.setInterval(load, 5000);
    return () => { alive = false; window.clearInterval(id); };
  }, [t]);
  useEffect(()=>{
    let alive=true;
    const check=async()=>{try{const response=await fetch('/api/calls',{cache:'no-store'});if(!response.ok)return;const data=await response.json() as {calls:{state:string}[]};if(alive&&data.calls.some(call=>call.state==='ringing-in'))setScreen('phone')}catch{}};
    check();const events=new EventSource('/api/calls/events');events.addEventListener('call-state',check);const fallback=window.setInterval(check,3000);return()=>{alive=false;events.close();window.clearInterval(fallback)};
  },[]);
  const goHome=()=>setScreen('home');
  // Canonical comparison value: ModemManager state strings come from the backend (modemState()).
  const active = modems.find(m=>m.state==='已连接') ?? modems[0] ?? null;
  const showModem=(m:Modem)=>{setSelected(m);setScreen('detail')};
  return <main className="stage">
    <section className="phone" aria-label={t('appAria')}>
      <div className="statusbar"><strong>{time}</strong><div className="island"/><div className="status-icons"><Signal size={15}/><Wifi size={15}/><BatteryMedium size={18}/></div></div>
      <div className="screen">
        {screen==='home' && <Home onOpen={()=>setScreen('settings')} onPhone={()=>setScreen('phone')} onMessages={()=>setScreen('messages')} onAbout={()=>setScreen('about')} onDiagnostics={()=>setScreen('diagnostics')} active={active} error={error} unread={unread}/>}
        {screen==='settings' && <SettingsRoot onBack={goHome} onModems={()=>setScreen('modems')} onAbout={()=>setScreen('about')} active={active} count={modems.length}/>}
        {screen==='modems' && <ModemList active={active?.id ?? ''} modems={modems} onBack={()=>setScreen('settings')} onDetail={showModem}/>}
        {screen==='detail' && selected && <Detail modem={selected} active={active?.id===selected.id} onBack={()=>setScreen('modems')}/>}
        {screen==='messages' && <Messages onClose={goHome}/>}
        {screen==='phone' && <PhoneApp onClose={goHome}/>}
        {screen==='about' && <About onBack={()=>setScreen('settings')}/>}
        {screen==='diagnostics' && <Diagnostics onBack={goHome}/>}
      </div>
      {screen!=='home' && <button className="home-indicator" aria-label={t('backToDesktop')} onClick={goHome}/>}
      {screen==='home' && <div className="home-indicator decorative"/>}
    </section>
    <aside className="product-note"><span>mmOS</span><strong>ModemManager<br/>{t('productTitle')}</strong><p>{t('productNote')}</p><div className="live"><i/> {t('systemRunning')}</div></aside>
  </main>
}

function Home({onOpen,onPhone,onMessages,onAbout,onDiagnostics,active,error,unread}:{onOpen:()=>void;onPhone:()=>void;onMessages:()=>void;onAbout:()=>void;onDiagnostics:()=>void;active:Modem|null;error:string;unread:UnreadState}) {
  const { t, locale } = useI18n();
  return <div className="wallpaper">
  <header className="home-head"><p>{formatDayHeader(new Date(), locale)}</p><h1>{t('homeGreeting')}</h1></header>
  <section className="network-widget"><div><small>{t('homeMobileNetwork')}</small><h2>{active?.network ?? (error ? t('serviceUnavailable') : t('readingDevice'))}</h2><p><span className="dot"/> {error || (active ? `${active.tech} · ${t('signalPercent', {n: active.signal})}` : t('noModem'))}</p></div><SignalBars value={active?.signal ?? 0}/></section>
  <div className="app-grid"><button className="app" onClick={onOpen}><span className="app-icon settings-icon"><Settings/></span><b>{t('homeSettings')}</b></button><button className="app" onClick={onPhone}><span className="app-icon phone-icon"><Phone/>{unread.phone>0&&<i className="app-badge"/>}</span><b>{t('homePhone')}</b></button><button className="app" onClick={onMessages}><span className="app-icon message-icon"><MessageSquare/>{unread.messages>0&&<i className="app-badge"/>}</span><b>{t('homeMessages')}</b></button><button className="app" onClick={onDiagnostics}><span className="app-icon diagnostics-icon"><Activity/>{unread.diagnostics>0&&<i className="app-badge"/>}</span><b>{t('homeDiagnostics')}</b></button><button className="app" onClick={onAbout}><span className="app-icon"><Info/></span><b>{t('homeAbout')}</b></button></div>
  <div className="dock"><button className="dock-icon phone-icon" onClick={onPhone}><Phone/>{unread.phone>0&&<i className="app-badge"/>}</button><button className="dock-icon" onClick={onMessages}><MessageSquare/>{unread.messages>0&&<i className="app-badge"/>}</button><button className="dock-icon settings-icon" onClick={onOpen}><Settings/></button></div>
</div>}

type HealthState = {dbusAvailable:boolean;modemsOnline:boolean;atAvailable:boolean;modemIds:string[];checkedAt:string};

function Diagnostics({onBack}:{onBack:()=>void}) {
  const { t, locale } = useI18n();
  const [health,setHealth]=useState<HealthState|null>(null); const [error,setError]=useState(''); const [loading,setLoading]=useState(true);
  const [interval,setIntervalSeconds]=useState(120); const [savingInterval,setSavingInterval]=useState(false);
  const load=async()=>{setLoading(true);try{const response=await fetch('/api/health',{cache:'no-store'});const data=await response.json() as {modemManager?:HealthState};if(!data.modemManager)throw new Error();setHealth(data.modemManager);setError('');void clearUnread('diagnostics')}catch{setError(t('backendUnreachable'))}finally{setLoading(false)}};
  useEffect(()=>{void load();fetch('/api/settings/health',{cache:'no-store'}).then(response=>response.json()).then((data:{checkIntervalSeconds:number})=>setIntervalSeconds(data.checkIntervalSeconds)).catch(()=>{});const timer=window.setInterval(()=>void load(),10000);return()=>window.clearInterval(timer)},[t]);
  const changeInterval=async(seconds:number)=>{setSavingInterval(true);try{const response=await fetch('/api/settings/health',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({checkIntervalSeconds:seconds})});if(!response.ok)throw new Error();setIntervalSeconds(seconds)}catch{setError(t('diagnosticsIntervalSaveFailed'))}finally{setSavingInterval(false)}};
  const healthy=Boolean(health?.dbusAvailable&&health.modemsOnline&&health.atAvailable);
  return <div className="page diagnostics-page"><Top title={t('diagnosticsTitle')} onBack={onBack}/><div className="content">
    <section className={`health-hero ${healthy?'ok':'bad'}`}>{healthy?<CheckCircle2/>:<XCircle/>}<div><h2>{loading&&!health?t('diagnosticsChecking'):healthy?t('diagnosticsHealthy'):t('diagnosticsAbnormal')}</h2><p>{healthy?t('diagnosticsNormalBody'):t('diagnosticsPartial')}</p></div></section>
    {error&&<p className="messages-error">{error}</p>}
    <SectionLabel text={t('diagnosticsSectionChecks')}/><div className="health-list">
      <HealthRow label={t('diagnosticsDbusLabel')} detail={t('diagnosticsDbusDetail')} ok={health?.dbusAvailable}/>
      <HealthRow label={t('diagnosticsModemLabel')} detail={health?.modemIds.length?t('diagnosticsModemsFound',{n:health.modemIds.length,ids:health.modemIds.join('、')}):t('diagnosticsModemsNone')} ok={health?.modemsOnline}/>
      <HealthRow label={t('diagnosticsAtLabel')} detail={t('diagnosticsAtDetail')} ok={health?.atAvailable}/>
    </div>
    <p className="health-time">{health?.checkedAt?t('diagnosticsCheckedAt',{time:timeOfDay(new Date(health.checkedAt), locale)}):t('diagnosticsAwaitingResult')}</p>
    <button className="primary diagnostics-refresh" onClick={()=>void load()} disabled={loading}><RefreshCw className={loading?'spinning':''}/>{loading?t('diagnosticsChecking'):t('diagnosticsRefresh')}</button>
    <SectionLabel text={t('diagnosticsSectionSettings')}/><label className="health-config"><span><b>{t('diagnosticsAutoInterval')}</b><small>{t('diagnosticsDefaultIntervalNote')}</small></span><select value={interval} disabled={savingInterval} onChange={event=>void changeInterval(Number(event.target.value))}><option value="30">{t('diagnosticsOpt30s')}</option><option value="60">{t('diagnosticsOpt1m')}</option><option value="120">{t('diagnosticsOpt2m')}</option><option value="300">{t('diagnosticsOpt5m')}</option><option value="600">{t('diagnosticsOpt10m')}</option></select></label>
    <div className="info-box"><Info/><p><b>{t('diagnosticsContinuousLabel')}</b><br/>{t('diagnosticsContinuousBody')}</p></div>
  </div></div>
}

function HealthRow({label,detail,ok}:{label:string;detail:string;ok:boolean|undefined}) {
  const { t } = useI18n();
  return <div className="health-row"><span className={ok?'ok':ok===false?'bad':'pending'}>{ok?<CheckCircle2/>:ok===false?<XCircle/>:<Activity/>}</span><div><b>{label}</b><small>{detail}</small></div><em>{ok?t('stateOk'):ok===false?t('stateBad'):t('stateChecking')}</em></div>
}

function Top({title,onBack,eyebrow}:{title:string;onBack:()=>void;eyebrow?:string}) {
  const { t } = useI18n();
  return <header className="appbar"><button onClick={onBack} aria-label={t('back')}><ChevronLeft/></button><div>{eyebrow&&<small>{eyebrow}</small>}<h1>{title}</h1></div><span/></header>
}
function SettingsRoot({onBack,onModems,onAbout,active,count}:{onBack:()=>void;onModems:()=>void;onAbout:()=>void;active:Modem|null;count:number}) {
  const { t } = useI18n();
  return <div className="page"><Top title={t('settingsTitle')} onBack={onBack}/><div className="content"><section className="device-card"><div className="device-avatar"><Smartphone/></div><div><h2>{t('settingsConsoleTitle')}</h2><p>{t('settingsLocalOnline')}</p></div><span className="online-dot"/></section><SectionLabel text={t('settingsSectionNetworkDevices')}/><button className="setting-row" onClick={onModems}><span className="row-icon blue"><Radio/></span><span><b>{t('settingsModemRow')}</b><small>{active?.name ?? t('settingsNoDevice')}</small></span><em>{t('settingsCountSuffix', {n: count})}</em><ChevronRight/></button><div className="setting-row disabled"><span className="row-icon green"><Wifi/></span><span><b>{t('settingsSectionWireless')}</b><small>{t('settingsManagedByHost')}</small></span><em>{t('settingsUnavailable')}</em></div><SectionLabel text={t('settingsSectionNotifications')}/><PushPreviewSetting/><SectionLabel text={t('settingsSectionSystem')}/><div className="setting-row disabled"><span className="row-icon purple"><ShieldCheck/></span><span><b>ModemManager</b><small>{t('settingsSystemStatus')}</small></span><em className="success">{t('settingsConnected')}</em></div><button className="setting-row" onClick={onAbout}><span className="row-icon gray"><CircleHelp/></span><span><b>{t('settingsAboutTitle')}</b><small>{t('settingsVersionLabel', {v:'0.2.0'})}</small></span><ChevronRight/></button><SectionLabel text={t('settingsSectionLanguage')}/><LanguageSetting/></div></div>}

function PushPreviewSetting() {
  const { t } = useI18n();
  const [enabled,setEnabled]=useState(false); const [loading,setLoading]=useState(true); const [error,setError]=useState('');
  useEffect(()=>{fetch('/api/settings/push',{cache:'no-store'}).then(async response=>{if(!response.ok)throw new Error();return response.json() as Promise<{showMessageContent:boolean}>}).then(data=>{setEnabled(data.showMessageContent);setError('')}).catch(()=>setError(t('settingsReadSettingsFailed'))).finally(()=>setLoading(false))},[t]);
  const toggle=async()=>{if(loading)return;const next=!enabled;setLoading(true);setError('');try{const response=await fetch('/api/settings/push',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({showMessageContent:next})});if(!response.ok)throw new Error();setEnabled(next)}catch{setError(t('saveFailed'))}finally{setLoading(false)}};
  return <button className="setting-row" onClick={toggle} disabled={loading} role="switch" aria-checked={enabled}><span className="row-icon red"><Bell/></span><span><b>{t('settingsShowContentTitle')}</b><small>{error || (enabled?t('settingsShowContentBody'):t('settingsShowContentHint'))}</small></span><i className={`toggle ${enabled?'on':''}`}><i/></i></button>
}

function LanguageSetting() {
  const { t, locale, setLocale } = useI18n();
  return <div className="setting-row language-row">
    <span className="row-icon gray"><Globe/></span>
    <span><b>{t('settingsLanguageTitle')}</b><small>{t('settingsLanguageNote')}</small></span>
    <select className="language-select" value={locale} aria-label={t('settingsLanguageLabel')} onChange={event=>setLocale(event.target.value as Locale)}>{LOCALES.map(entry=>(<option key={entry.locale} value={entry.locale}>{entry.name}</option>))}</select>
    <ChevronRight/>
  </div>
}

type AboutInfo = {version:string;commit:string;buildTime:string;goVersion:string;os:string;arch:string;uptimeSeconds:number;serverTime:string;modemManager:string;modemManagerVersion:string;pushSubscriptions:number;showMessageContent:boolean};

function About({onBack}:{onBack:()=>void}) {
  const { t, locale } = useI18n();
  const [info,setInfo]=useState<AboutInfo|null>(null); const [error,setError]=useState(''); const [copied,setCopied]=useState(false); const [subscribed,setSubscribed]=useState(false);
  const standalone=window.matchMedia('(display-mode: standalone)').matches; const permission='Notification' in window?Notification.permission:'unsupported';
  useEffect(()=>{fetch('/api/about',{cache:'no-store'}).then(async response=>{if(!response.ok)throw new Error();return response.json() as Promise<AboutInfo>}).then(setInfo).catch(()=>setError(t('backendUnreachable')));if('serviceWorker'in navigator&&'PushManager'in window)navigator.serviceWorker.ready.then(registration=>registration.pushManager.getSubscription()).then(value=>setSubscribed(Boolean(value))).catch(()=>{})},[t]);
  const diagnostics=()=>[`mmOS ${info?.version??'unknown'}`,`Commit: ${info?.commit||'development'}`,`Platform: ${info?.os??'unknown'}/${info?.arch??'unknown'}`,`ModemManager: ${info?.modemManager??'unknown'}${info?.modemManagerVersion?` (${info.modemManagerVersion})`:''}`,`Web Push: ${subscribed?'subscribed':'not subscribed'}`,`PWA: ${standalone?'standalone':'browser'}`,`Notification: ${permission}`].join('\n');
  const copy=async()=>{await navigator.clipboard.writeText(diagnostics());setCopied(true);window.setTimeout(()=>setCopied(false),1800)};
  return <div className="page about-page"><Top title={t('aboutTitle')} onBack={onBack}/><div className="content"><section className="about-hero"><span><Smartphone/></span><h2>mmOS</h2><p>ModemManager</p><em>{t('settingsVersionLabel', {v: info?.version??'—'})}</em></section>{error&&<p className="messages-error">{error}</p>}<SectionLabel text={t('aboutStatusSection')}/><div className="about-list"><AboutRow icon={<Server/>} label={t('settingsBackendName')} value={info?t('aboutBackendOnline'):t('aboutBackendLoading')} ok={Boolean(info)}/><AboutRow icon={<Radio/>} label="ModemManager" value={info?.modemManager==='connected'?t('settingsConnected'):t('settingsUnavailable')} ok={info?.modemManager==='connected'}/><AboutRow icon={<Bell/>} label="Web Push" value={subscribed?t('aboutPushSubscribed'):t('aboutPushCount',{n:info?.pushSubscriptions??0})} ok={subscribed}/><AboutRow icon={<Smartphone/>} label={t('aboutPWAName')} value={standalone?t('aboutInstalled'):t('aboutBrowser')} ok={standalone}/></div><SectionLabel text={t('aboutEnvironmentSection')}/><dl className="specs about-specs"><div><dt>{t('aboutSpecsBuild')}</dt><dd>{info?.commit||'development'}</dd></div><div><dt>{t('aboutSpecsModemVersion')}</dt><dd>{info?.modemManagerVersion||'—'}</dd></div><div><dt>{t('aboutSpecsPlatform')}</dt><dd>{info?`${info.os}/${info.arch}`:'—'}</dd></div><div><dt>{t('aboutSpecsGoVersion')}</dt><dd>{info?.goVersion||'—'}</dd></div><div><dt>{t('aboutSpecsUptime')}</dt><dd>{formatUptime(info?.uptimeSeconds??0, locale)}</dd></div><div><dt>{t('aboutSpecsServerTime')}</dt><dd>{info?dateTime(new Date(info.serverTime), locale):'—'}</dd></div></dl><SectionLabel text={t('aboutPrivacySection')}/><div className="privacy-card"><LockKeyhole/><p><b>{t('aboutDataLocal')}</b><span>{t('aboutSmsLocal')}{info?.showMessageContent?t('aboutPushWithContent'):t('aboutPushWithoutContent')}</span></p></div><div className="about-actions"><button onClick={copy}>{copied?<CheckCircle2/>:<Copy/>}{copied?t('aboutCopied'):t('aboutCopyDiagnostics')}</button><a href="https://github.com/eeelin/mm-web" target="_blank" rel="noreferrer"><Code2/>GitHub<ExternalLink/></a></div></div></div>
}

function AboutRow({icon,label,value,ok}:{icon:React.ReactNode;label:string;value:string;ok:boolean}) {return <div className="about-row"><span>{icon}</span><b>{label}</b><em className={ok?'success':''}>{value}</em></div>}
function SectionLabel({text}:{text:string}) {return <h3 className="section-label">{text}</h3>}

function ModemList({active,modems,onBack,onDetail}:{active:string;modems:Modem[];onBack:()=>void;onDetail:(m:Modem)=>void}) {
  const { t } = useI18n();
  return <div className="page"><Top title={t('modemsTitle')} eyebrow={t('modemsEyebrow')} onBack={onBack}/><div className="content"><p className="lead">{t('modemsLead')}</p><div className="modem-list">{modems.map(m=>(<article className={`modem-card ${active===m.id?'active':''}`} key={m.id}><button className="modem-main" onClick={()=>onDetail(m)}><span className="radio-check">{active===m.id&&<i/>}</span><span className="modem-copy"><span className="modem-title"><b>{m.name}</b>{active===m.id&&<em>{t('modemsCurrentDevice')}</em>}</span><small>{m.model} · {m.port}</small><span className="network-line"><i className={m.state==='已连接'?'connected':''}/>{m.network} · {m.tech}<SignalBars value={m.signal}/></span></span></button><button className="details" onClick={()=>onDetail(m)} aria-label={t('modemsViewDetails',{name:m.name})}><ChevronRight/></button></article>))}</div>{modems.length===0&&<div className="info-box"><Info/><p><b>{t('modemsNone')}</b><br/>{t('modemsCheckRunning')}</p></div>}</div></div>
}

function Detail({modem,active,onBack}:{modem:Modem;active:boolean;onBack:()=>void}) {
  const { t, locale } = useI18n();
  const st = (value:string) => translate(modemStates, value, locale);
  return <div className="page"><Top title={t('detailTitle')} onBack={onBack}/><div className="content"><div className="detail-hero"><div className="big-device"><Radio/></div><h2>{modem.name}</h2><p><i/> {st(modem.state)} · {modem.network}</p></div><SectionLabel text={t('detailSectionStatusNetwork')}/><SpecList rows={[[t('detailSpecPower'),st(modem.powerState)],[t('detailSpecRegistration'),st(modem.registration)],[t('detailSpecPacketService'),st(modem.packetService)],[t('detailSpecOperator'),modem.network],[t('detailSpecOperatorCode'),modem.operatorCode],[t('detailSpecAccessTech'),modem.tech],[t('detailSpecSignalQuality'),`${modem.signal}%`],[t('detailSpecIPSupport'),modem.ipFamilies],[t('detailSpecOwnNumbers'),modem.ownNumbers.join('、')]]}/><SectionLabel text={t('detailSectionModesCapabilities')}/><SpecList rows={[[t('detailSpecCurrentCapabilities'),modem.capabilities],[t('detailSpecCurrentModes'),modem.currentModes],[t('detailSpecSupportedModes'),modem.supportedModes]]}/><SectionLabel text={t('detailSectionHardware')}/><SpecList rows={[[t('detailSpecManufacturer'),modem.manufacturer],[t('detailSpecModel'),modem.model],[t('detailSpecFirmware'),modem.firmware],[t('detailSpecImei'),modem.imei],[t('detailSpecDeviceId'),modem.deviceId]]}/><SectionLabel text={t('detailSectionSystemIntegration')}/><SpecList rows={[[t('detailSpecMainPort'),modem.port],[t('detailSpecAllPorts'),modem.ports.join('、')],[t('detailSpecDrivers'),modem.drivers.join('、')],[t('detailSpecPlugin'),modem.plugin],[t('detailSpecSystemDevice'),modem.device]]}/><SectionLabel text={t('detailSectionSIMSecurity')}/><SpecList rows={[[t('detailSpecSIMCard'),modem.sim],[t('detailSpecUnlock'),modem.unlockRequired],[t('detailSpecRetries'),modem.unlockRetries.join('、')]]}/>{active&&<button className="primary active-btn" disabled><ShieldCheck/>{t('detailActiveDevice')}</button>}</div></div>
}
function SpecList({rows}:{rows:[string,string][]}) {return <dl className="specs">{rows.filter(([,value])=>value).map(([label,value])=><div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>}
function SignalBars({value}:{value:number}) {
  const { t } = useI18n();
  return <span className="bars" aria-label={t('signalPercent', {n: value})}>{[25,45,65,80].map((n,i)=><i key={n} className={value>=n?'on':''} style={{height:5+i*3}}/>)}</span>
}

