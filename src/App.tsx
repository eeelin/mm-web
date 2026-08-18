import { useEffect, useState } from 'react';
import { BatteryMedium, Bell, CheckCircle2, ChevronLeft, ChevronRight, CircleHelp, Copy, Code2, ExternalLink, Info, LockKeyhole, MessageSquare, Phone, Radio, Server, Settings, ShieldCheck, Signal, Smartphone, Wifi } from 'lucide-react';
import { Messages } from './Messages';
import { Phone as PhoneApp } from './Phone';

type Modem = { id:string; name:string; model:string; state:string; network:string; tech:string; signal:number; sim:string; imei:string; firmware:string; port:string; manufacturer:string; deviceId:string; device:string; drivers:string[]; plugin:string; ports:string[]; ownNumbers:string[]; powerState:string; capabilities:string; supportedModes:string; currentModes:string; ipFamilies:string; operatorCode:string; registration:string; packetService:string; unlockRequired:string; unlockRetries:string[] };

export function App() {
  const [screen, setScreen] = useState<'home'|'settings'|'modems'|'detail'|'messages'|'phone'|'about'>(() => new URLSearchParams(location.search).get('screen') === 'messages' ? 'messages' : 'home');
  const [modems, setModems] = useState<Modem[]>([]);
  const [selected, setSelected] = useState<Modem | null>(null);
  const [error, setError] = useState('');
  const [time, setTime] = useState('09:41');
  useEffect(() => { const tick=()=>setTime(new Date().toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit',hour12:false})); tick(); const id=setInterval(tick,30000); return()=>clearInterval(id)},[]);
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
        if (alive) setError('无法连接 ModemManager，正在重试');
      }
    };
    load();
    const id = window.setInterval(load, 5000);
    return () => { alive = false; window.clearInterval(id); };
  }, []);
  const goHome=()=>setScreen('home');
  const active = modems.find(m=>m.state==='已连接') ?? modems[0] ?? null;
  const showModem=(m:Modem)=>{setSelected(m);setScreen('detail')};
  return <main className="stage">
    <section className="phone" aria-label="mmOS 虚拟手机">
      <div className="statusbar"><strong>{time}</strong><div className="island"/><div className="status-icons"><Signal size={15}/><Wifi size={15}/><BatteryMedium size={18}/></div></div>
      <div className="screen">
        {screen==='home' && <Home onOpen={()=>setScreen('settings')} onPhone={()=>setScreen('phone')} onMessages={()=>setScreen('messages')} onAbout={()=>setScreen('about')} active={active} error={error}/>}
        {screen==='settings' && <SettingsRoot onBack={goHome} onModems={()=>setScreen('modems')} onAbout={()=>setScreen('about')} active={active} count={modems.length}/>}
        {screen==='modems' && <ModemList active={active?.id ?? ''} modems={modems} onBack={()=>setScreen('settings')} onDetail={showModem}/>}
        {screen==='detail' && selected && <Detail modem={selected} active={active?.id===selected.id} onBack={()=>setScreen('modems')}/>}
        {screen==='messages' && <Messages onClose={goHome}/>}
        {screen==='phone' && <PhoneApp onClose={goHome}/>}
        {screen==='about' && <About onBack={()=>setScreen('settings')}/>}
      </div>
      {screen!=='home' && <button className="home-indicator" aria-label="返回桌面" onClick={goHome}/>}
      {screen==='home' && <div className="home-indicator decorative"/>}
    </section>
    <aside className="product-note"><span>mmOS</span><strong>ModemManager<br/>控制台</strong><p>在浏览器中，像使用手机一样管理网络设备。</p><div className="live"><i/> 系统运行正常</div></aside>
  </main>
}

function Home({onOpen,onPhone,onMessages,onAbout,active,error}:{onOpen:()=>void;onPhone:()=>void;onMessages:()=>void;onAbout:()=>void;active:Modem|null;error:string}) { return <div className="wallpaper">
  <header className="home-head"><p>8月17日 · 星期一</p><h1>早上好</h1></header>
  <section className="network-widget"><div><small>移动网络</small><h2>{active?.network ?? (error ? '服务不可用' : '正在读取设备')}</h2><p><span className="dot"/> {error || (active ? `${active.tech} · 信号 ${active.signal}%` : '未检测到调制解调器')}</p></div><SignalBars value={active?.signal ?? 0}/></section>
  <div className="app-grid"><button className="app" onClick={onOpen}><span className="app-icon settings-icon"><Settings/></span><b>系统设置</b></button><button className="app" onClick={onPhone}><span className="app-icon phone-icon"><Phone/></span><b>电话</b></button><button className="app" onClick={onMessages}><span className="app-icon message-icon"><MessageSquare/></span><b>信息</b></button><button className="app" onClick={onAbout}><span className="app-icon"><Info/></span><b>关于</b></button></div>
  <div className="dock"><button className="dock-icon phone-icon" onClick={onPhone}><Phone/></button><button className="dock-icon" onClick={onMessages}><MessageSquare/></button><button className="dock-icon" onClick={onOpen}><Settings/></button></div>
</div>}

function Top({title,onBack,eyebrow}:{title:string;onBack:()=>void;eyebrow?:string}) {return <header className="appbar"><button onClick={onBack} aria-label="返回"><ChevronLeft/></button><div>{eyebrow&&<small>{eyebrow}</small>}<h1>{title}</h1></div><span/></header>}
function SettingsRoot({onBack,onModems,onAbout,active,count}:{onBack:()=>void;onModems:()=>void;onAbout:()=>void;active:Modem|null;count:number}) {return <div className="page"><Top title="系统设置" onBack={onBack}/><div className="content"><section className="device-card"><div className="device-avatar"><Smartphone/></div><div><h2>mmOS 控制台</h2><p>本机 · 在线</p></div><span className="online-dot"/></section><SectionLabel text="网络与设备"/><button className="setting-row" onClick={onModems}><span className="row-icon blue"><Radio/></span><span><b>蜂窝调制解调器</b><small>{active?.name ?? '未检测到设备'}</small></span><em>{count} 台</em><ChevronRight/></button><div className="setting-row disabled"><span className="row-icon green"><Wifi/></span><span><b>无线网络</b><small>由主机系统管理</small></span><em>不可用</em></div><SectionLabel text="通知"/><PushPreviewSetting/><SectionLabel text="系统"/><div className="setting-row disabled"><span className="row-icon purple"><ShieldCheck/></span><span><b>ModemManager</b><small>服务运行正常</small></span><em className="success">已连接</em></div><button className="setting-row" onClick={onAbout}><span className="row-icon gray"><CircleHelp/></span><span><b>关于 mmOS</b><small>版本 0.2.0</small></span><ChevronRight/></button></div></div>}

function PushPreviewSetting() {
  const [enabled,setEnabled]=useState(false); const [loading,setLoading]=useState(true); const [error,setError]=useState('');
  useEffect(()=>{fetch('/api/settings/push',{cache:'no-store'}).then(async response=>{if(!response.ok)throw new Error();return response.json() as Promise<{showMessageContent:boolean}>}).then(data=>{setEnabled(data.showMessageContent);setError('')}).catch(()=>setError('读取设置失败')).finally(()=>setLoading(false))},[]);
  const toggle=async()=>{if(loading)return;const next=!enabled;setLoading(true);setError('');try{const response=await fetch('/api/settings/push',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({showMessageContent:next})});if(!response.ok)throw new Error();setEnabled(next)}catch{setError('保存失败')}finally{setLoading(false)}};
  return <button className="setting-row" onClick={toggle} disabled={loading} role="switch" aria-checked={enabled}><span className="row-icon red"><Bell/></span><span><b>显示信息内容</b><small>{error || (enabled?'通知显示号码和信息正文':'通知仅提示收到新信息')}</small></span><i className={`toggle ${enabled?'on':''}`}><i/></i></button>
}

type AboutInfo = {version:string;commit:string;buildTime:string;goVersion:string;os:string;arch:string;uptimeSeconds:number;serverTime:string;modemManager:string;modemManagerVersion:string;pushSubscriptions:number;showMessageContent:boolean};

function About({onBack}:{onBack:()=>void}) {
  const [info,setInfo]=useState<AboutInfo|null>(null); const [error,setError]=useState(''); const [copied,setCopied]=useState(false); const [subscribed,setSubscribed]=useState(false);
  const standalone=window.matchMedia('(display-mode: standalone)').matches; const permission='Notification' in window?Notification.permission:'unsupported';
  useEffect(()=>{fetch('/api/about',{cache:'no-store'}).then(async response=>{if(!response.ok)throw new Error();return response.json() as Promise<AboutInfo>}).then(setInfo).catch(()=>setError('无法读取系统信息'));if('serviceWorker'in navigator&&'PushManager'in window)navigator.serviceWorker.ready.then(registration=>registration.pushManager.getSubscription()).then(value=>setSubscribed(Boolean(value))).catch(()=>{})},[]);
  const diagnostics=()=>[`mmOS ${info?.version??'unknown'}`,`Commit: ${info?.commit||'development'}`,`Platform: ${info?.os??'unknown'}/${info?.arch??'unknown'}`,`ModemManager: ${info?.modemManager??'unknown'}${info?.modemManagerVersion?` (${info.modemManagerVersion})`:''}`,`Web Push: ${subscribed?'subscribed':'not subscribed'}`,`PWA: ${standalone?'standalone':'browser'}`,`Notification: ${permission}`].join('\n');
  const copy=async()=>{await navigator.clipboard.writeText(diagnostics());setCopied(true);window.setTimeout(()=>setCopied(false),1800)};
  return <div className="page about-page"><Top title="关于 mmOS" onBack={onBack}/><div className="content"><section className="about-hero"><span><Smartphone/></span><h2>mmOS</h2><p>ModemManager 控制台</p><em>版本 {info?.version??'—'}</em></section>{error&&<p className="messages-error">{error}</p>}<SectionLabel text="服务状态"/><div className="about-list"><AboutRow icon={<Server/>} label="mm-web 后端" value={info?'运行正常':'读取中'} ok={Boolean(info)}/><AboutRow icon={<Radio/>} label="ModemManager" value={info?.modemManager==='connected'?'已连接':'不可用'} ok={info?.modemManager==='connected'}/><AboutRow icon={<Bell/>} label="Web Push" value={subscribed?'本机已订阅':`${info?.pushSubscriptions??0} 个订阅`} ok={subscribed}/><AboutRow icon={<Smartphone/>} label="PWA 模式" value={standalone?'已安装':'浏览器访问'} ok={standalone}/></div><SectionLabel text="运行环境"/><dl className="specs about-specs"><div><dt>构建版本</dt><dd>{info?.commit||'development'}</dd></div><div><dt>ModemManager</dt><dd>{info?.modemManagerVersion||'—'}</dd></div><div><dt>运行平台</dt><dd>{info?`${info.os}/${info.arch}`:'—'}</dd></div><div><dt>Go 版本</dt><dd>{info?.goVersion||'—'}</dd></div><div><dt>运行时间</dt><dd>{formatUptime(info?.uptimeSeconds??0)}</dd></div><div><dt>服务器时间</dt><dd>{info?.serverTime?new Date(info.serverTime).toLocaleString('zh-CN'):'—'}</dd></div></dl><SectionLabel text="数据与隐私"/><div className="privacy-card"><LockKeyhole/><p><b>数据保留在本机</b><span>短信来自本机 ModemManager，不会由 mmOS 上传。{info?.showMessageContent?'通知当前会显示号码和短信正文，内容可能出现在锁屏上。':'通知当前不显示号码和短信正文。'}</span></p></div><div className="about-actions"><button onClick={copy}>{copied?<CheckCircle2/>:<Copy/>}{copied?'已复制':'复制诊断信息'}</button><a href="https://github.com/eeelin/mm-web" target="_blank" rel="noreferrer"><Code2/>GitHub<ExternalLink/></a></div></div></div>
}

function AboutRow({icon,label,value,ok}:{icon:React.ReactNode;label:string;value:string;ok:boolean}) {return <div className="about-row"><span>{icon}</span><b>{label}</b><em className={ok?'success':''}>{value}</em></div>}
function formatUptime(seconds:number){if(seconds<60)return `${seconds} 秒`;if(seconds<3600)return `${Math.floor(seconds/60)} 分钟`;if(seconds<86400)return `${Math.floor(seconds/3600)} 小时`;return `${Math.floor(seconds/86400)} 天`}
function SectionLabel({text}:{text:string}) {return <h3 className="section-label">{text}</h3>}

function ModemList({active,modems,onBack,onDetail}:{active:string;modems:Modem[];onBack:()=>void;onDetail:(m:Modem)=>void}) {return <div className="page"><Top title="调制解调器" eyebrow="网络与设备" onBack={onBack}/><div className="content"><p className="lead">来自本机 ModemManager 的实时设备。</p><div className="modem-list">{modems.map(m=><article className={`modem-card ${active===m.id?'active':''}`} key={m.id}><button className="modem-main" onClick={()=>onDetail(m)}><span className="radio-check">{active===m.id&&<i/>}</span><span className="modem-copy"><span className="modem-title"><b>{m.name}</b>{active===m.id&&<em>当前设备</em>}</span><small>{m.model} · {m.port}</small><span className="network-line"><i className={m.state==='已连接'?'connected':''}/>{m.network} · {m.tech}<SignalBars value={m.signal}/></span></span></button><button className="details" onClick={()=>onDetail(m)} aria-label={`查看${m.name}详情`}><ChevronRight/></button></article>)}</div>{modems.length===0&&<div className="info-box"><Info/><p><b>未检测到调制解调器</b><br/>请确认 ModemManager 正在运行并已识别设备。</p></div>}</div></div>}

function Detail({modem,active,onBack}:{modem:Modem;active:boolean;onBack:()=>void}) {return <div className="page"><Top title="设备详情" onBack={onBack}/><div className="content"><div className="detail-hero"><div className="big-device"><Radio/></div><h2>{modem.name}</h2><p><i/> {modem.state} · {modem.network}</p></div><SectionLabel text="状态与网络"/><SpecList rows={[["电源状态",modem.powerState],["注册状态",modem.registration],["分组服务",modem.packetService],["运营商",modem.network],["运营商代码",modem.operatorCode],["接入技术",modem.tech],["信号质量",`${modem.signal}%`],["IP 支持",modem.ipFamilies],["本机号码",modem.ownNumbers.join('、')]]}/><SectionLabel text="模式与能力"/><SpecList rows={[["当前能力",modem.capabilities],["当前模式",modem.currentModes],["支持模式",modem.supportedModes]]}/><SectionLabel text="硬件"/><SpecList rows={[["制造商",modem.manufacturer],["型号",modem.model],["固件版本",modem.firmware],["IMEI",modem.imei],["设备 ID",modem.deviceId]]}/><SectionLabel text="系统集成"/><SpecList rows={[["主端口",modem.port],["全部端口",modem.ports.join('、')],["驱动",modem.drivers.join('、')],["插件",modem.plugin],["系统设备",modem.device]]}/><SectionLabel text="SIM 与安全"/><SpecList rows={[["SIM 卡",modem.sim],["解锁要求",modem.unlockRequired],["剩余重试",modem.unlockRetries.join('、')]]}/>{active&&<button className="primary active-btn" disabled><ShieldCheck/>当前设备</button>}</div></div>}
function SpecList({rows}:{rows:[string,string][]}) {return <dl className="specs">{rows.filter(([,value])=>value).map(([label,value])=><div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>}
function SignalBars({value}:{value:number}) {return <span className="bars" aria-label={`信号 ${value}%`}>{[25,45,65,80].map((n,i)=><i key={n} className={value>=n?'on':''} style={{height:5+i*3}}/>)}</span>}
