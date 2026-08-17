import { useEffect, useState } from 'react';
import { Antenna, BatteryMedium, ChevronLeft, ChevronRight, CircleHelp, Info, Radio, Settings, ShieldCheck, Signal, Smartphone, Wifi, X } from 'lucide-react';

type Modem = { id: string; name: string; model: string; state: string; network: string; tech: string; signal: number; sim: string; imei: string; firmware: string; port: string };

const modems: Modem[] = [
  { id: '0', name: 'Quectel RM520N-GL', model: '5G Sub-6 GHz', state: '已连接', network: '中国移动', tech: '5G NSA', signal: 82, sim: 'SIM 1 · 就绪', imei: '86 80306 12048 217', firmware: 'RM520NGLAAR03A03M4G', port: 'cdc-wdm0' },
  { id: '1', name: 'Fibocom FM350-GL', model: '5G / LTE', state: '已注册', network: '中国联通', tech: '4G LTE', signal: 64, sim: 'SIM 2 · 就绪', imei: '86 90114 05231 694', firmware: '81600.0000.00.29.23.16', port: 'cdc-wdm1' },
];

export function App() {
  const [screen, setScreen] = useState<'home'|'settings'|'modems'|'detail'>('home');
  const [selected, setSelected] = useState(modems[0]);
  const [active, setActive] = useState(modems[0].id);
  const [toast, setToast] = useState('');
  const [time, setTime] = useState('09:41');
  useEffect(() => { const tick=()=>setTime(new Date().toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit',hour12:false})); tick(); const id=setInterval(tick,30000); return()=>clearInterval(id)},[]);
  const goHome=()=>setScreen('home');
  const activate=(m:Modem)=>{setActive(m.id);setSelected(m);setToast(`已切换到 ${m.name}`);setTimeout(()=>setToast(''),2200)};
  return <main className="stage">
    <section className="phone" aria-label="mmOS 虚拟手机">
      <div className="statusbar"><strong>{time}</strong><div className="island"/><div className="status-icons"><Signal size={15}/><Wifi size={15}/><BatteryMedium size={18}/></div></div>
      <div className="screen">
        {screen==='home' && <Home onOpen={()=>setScreen('settings')} active={modems.find(m=>m.id===active)!}/>} 
        {screen==='settings' && <SettingsRoot onBack={goHome} onModems={()=>setScreen('modems')} active={modems.find(m=>m.id===active)!}/>} 
        {screen==='modems' && <ModemList active={active} onBack={()=>setScreen('settings')} onSelect={activate} onDetail={(m)=>{setSelected(m);setScreen('detail')}}/>}
        {screen==='detail' && <Detail modem={selected} active={active===selected.id} onBack={()=>setScreen('modems')} onActivate={()=>activate(selected)}/>} 
      </div>
      {screen!=='home' && <button className="home-indicator" aria-label="返回桌面" onClick={goHome}/>} 
      {screen==='home' && <div className="home-indicator decorative"/>}
      {toast && <div className="toast"><ShieldCheck size={18}/>{toast}</div>}
    </section>
    <aside className="product-note"><span>mmOS</span><strong>ModemManager<br/>控制台</strong><p>在浏览器中，像使用手机一样管理网络设备。</p><div className="live"><i/> 系统运行正常</div></aside>
  </main>
}

function Home({onOpen,active}:{onOpen:()=>void;active:Modem}) { return <div className="wallpaper">
  <header className="home-head"><p>8月17日 · 星期一</p><h1>早上好</h1></header>
  <section className="network-widget"><div><small>移动网络</small><h2>{active.network}</h2><p><span className="dot"/> {active.tech} · 信号 {active.signal}%</p></div><SignalBars value={active.signal}/></section>
  <div className="app-grid"><button className="app" onClick={onOpen}><span className="app-icon settings-icon"><Settings/></span><b>系统设置</b></button><div className="app muted"><span className="app-icon"><Radio/></span><b>连接</b></div><div className="app muted"><span className="app-icon"><Antenna/></span><b>信号</b></div><div className="app muted"><span className="app-icon"><Info/></span><b>关于</b></div></div>
  <div className="dock"><div className="dock-icon"><Radio/></div><div className="dock-icon"><Signal/></div><button className="dock-icon" onClick={onOpen}><Settings/></button></div>
</div>}

function Top({title,onBack,eyebrow}:{title:string;onBack:()=>void;eyebrow?:string}) {return <header className="appbar"><button onClick={onBack} aria-label="返回"><ChevronLeft/></button><div>{eyebrow&&<small>{eyebrow}</small>}<h1>{title}</h1></div><span/></header>}
function SettingsRoot({onBack,onModems,active}:{onBack:()=>void;onModems:()=>void;active:Modem}) {return <div className="page"><Top title="系统设置" onBack={onBack}/><div className="content"><section className="device-card"><div className="device-avatar"><Smartphone/></div><div><h2>mmOS 控制台</h2><p>本机 · 在线</p></div><span className="online-dot"/></section><SectionLabel text="网络与设备"/><button className="setting-row" onClick={onModems}><span className="row-icon blue"><Radio/></span><span><b>蜂窝调制解调器</b><small>{active.name}</small></span><em>{modems.length} 台</em><ChevronRight/></button><div className="setting-row disabled"><span className="row-icon green"><Wifi/></span><span><b>无线网络</b><small>由主机系统管理</small></span><em>不可用</em></div><SectionLabel text="系统"/><div className="setting-row disabled"><span className="row-icon purple"><ShieldCheck/></span><span><b>ModemManager</b><small>服务运行正常</small></span><em className="success">已连接</em></div><div className="setting-row disabled"><span className="row-icon gray"><CircleHelp/></span><span><b>关于 mmOS</b><small>版本 0.1.0</small></span><ChevronRight/></div></div></div>}
function SectionLabel({text}:{text:string}) {return <h3 className="section-label">{text}</h3>}

function ModemList({active,onBack,onSelect,onDetail}:{active:string;onBack:()=>void;onSelect:(m:Modem)=>void;onDetail:(m:Modem)=>void}) {return <div className="page"><Top title="调制解调器" eyebrow="网络与设备" onBack={onBack}/><div className="content"><p className="lead">选择用于移动数据连接的调制解调器。</p><div className="modem-list">{modems.map(m=><article className={`modem-card ${active===m.id?'active':''}`} key={m.id}><button className="modem-main" onClick={()=>onSelect(m)}><span className="radio-check">{active===m.id&&<i/>}</span><span className="modem-copy"><span className="modem-title"><b>{m.name}</b>{active===m.id&&<em>当前使用</em>}</span><small>{m.model} · {m.port}</small><span className="network-line"><i className={m.state==='已连接'?'connected':''}/>{m.network} · {m.tech}<SignalBars value={m.signal}/></span></span></button><button className="details" onClick={()=>onDetail(m)} aria-label={`查看${m.name}详情`}><ChevronRight/></button></article>)}</div><div className="info-box"><Info/><p><b>切换调制解调器</b><br/>切换时当前数据连接可能会短暂中断。系统会保留原设备的配置。</p></div></div></div>}

function Detail({modem,active,onBack,onActivate}:{modem:Modem;active:boolean;onBack:()=>void;onActivate:()=>void}) {return <div className="page"><Top title="设备详情" onBack={onBack}/><div className="content"><div className="detail-hero"><div className="big-device"><Radio/></div><h2>{modem.name}</h2><p><i/> {modem.state} · {modem.network}</p></div><SectionLabel text="设备信息"/><dl className="specs"><div><dt>接入技术</dt><dd>{modem.tech}</dd></div><div><dt>信号质量</dt><dd>{modem.signal}%</dd></div><div><dt>SIM 卡</dt><dd>{modem.sim}</dd></div><div><dt>设备端口</dt><dd>{modem.port}</dd></div><div><dt>IMEI</dt><dd>{modem.imei}</dd></div><div><dt>固件版本</dt><dd>{modem.firmware}</dd></div></dl>{active?<button className="primary active-btn" disabled><ShieldCheck/>当前正在使用</button>:<button className="primary" onClick={onActivate}>切换到此调制解调器</button>}</div></div>}
function SignalBars({value}:{value:number}) {return <span className="bars" aria-label={`信号 ${value}%`}>{[25,45,65,80].map((n,i)=><i key={n} className={value>=n?'on':''} style={{height:5+i*3}}/>)}</span>}
