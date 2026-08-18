import { useCallback, useEffect, useState } from 'react';
import { ChevronLeft, Delete, Grid3X3, MicOff, Phone as PhoneIcon, PhoneOff } from 'lucide-react';

type Call = {id:string; modemId:string; number:string; direction:string; state:string; reason?:string};
type CallsResponse = {calls:Call[]; voiceAvailable:boolean};

const keys=[['1',''],['2','ABC'],['3','DEF'],['4','GHI'],['5','JKL'],['6','MNO'],['7','PQRS'],['8','TUV'],['9','WXYZ'],['*',''],['0','+'],['#','']];
const labels:Record<string,string>={unknown:'正在读取',dialing:'正在呼叫…','ringing-out':'对方响铃中','ringing-in':'来电',active:'通话中',held:'通话保持中',waiting:'呼叫等待',terminated:'通话已结束'};
const reasonLabels:Record<string,string>={'refused-or-busy':'对方忙或已拒绝',error:'呼叫失败','audio-setup-failed':'音频链路设置失败',terminated:'通话已结束'};

export function Phone({onClose}:{onClose:()=>void}) {
  const [number,setNumber]=useState(''); const [calls,setCalls]=useState<Call[]>([]); const [recentCallId,setRecentCallId]=useState(''); const [voiceAvailable,setVoiceAvailable]=useState<boolean|null>(null); const [loading,setLoading]=useState(false); const [error,setError]=useState(''); const [showKeys,setShowKeys]=useState(false);
  const load=useCallback(async()=>{try{const response=await fetch('/api/calls',{cache:'no-store'});const data=await response.json() as CallsResponse&{error?:string};if(!response.ok)throw new Error(data.error||'服务不可用');setCalls(data.calls);setVoiceAvailable(data.voiceAvailable)}catch(err){setError(err instanceof Error?err.message:'无法读取电话状态')}},[]);
  useEffect(()=>{load();const id=window.setInterval(load,1000);return()=>window.clearInterval(id)},[load]);
  const active=calls.find(call=>call.state!=='terminated')??calls.find(call=>call.id===recentCallId);
  const request=async(url:string,body?:unknown)=>{setLoading(true);setError('');try{const response=await fetch(url,{method:'POST',headers:body?{'Content-Type':'application/json'}:undefined,body:body?JSON.stringify(body):undefined});const data=response.status===204?null:await response.json();if(!response.ok)throw new Error(data?.error||'操作失败');await load();return data}catch(err){setError(err instanceof Error?err.message:'操作失败');return null}finally{setLoading(false)}};
  const dial=async()=>{if(!number)return;const data=await request('/api/calls',{number});if(data?.id)setRecentCallId(data.id)};
  const tone=(key:string)=>{if(active?.state==='active')void request(`/api/calls/${active.id}/dtmf`,{tones:key});else setNumber(value=>(value+key).slice(0,32))};
  if(active) return <CallScreen call={active} loading={loading} error={error} showKeys={showKeys} onKeys={()=>setShowKeys(value=>!value)} onTone={tone} onHangup={()=>void request(`/api/calls/${active.id}/hangup`)} onDone={()=>{setRecentCallId('');setNumber('');setError('')}} onClose={onClose}/>;
  return <div className="phone-page"><header className="phone-bar"><button onClick={onClose} aria-label="返回"><ChevronLeft/></button><h1>电话</h1><span/></header><main className="dialer"><div className="dial-number"><span>{number||'输入电话号码'}</span>{number&&<button onClick={()=>setNumber(value=>value.slice(0,-1))} aria-label="删除"><Delete/></button>}</div>{error&&<p className="call-error">{error}</p>}<div className="dial-keys">{keys.map(([key,letters])=><button key={key} onClick={()=>tone(key)} onContextMenu={event=>{event.preventDefault();if(key==='0')setNumber(value=>value+'+')}}><b>{key}</b><small>{letters}</small></button>)}</div><button className="dial-call" onClick={dial} disabled={loading||!number||voiceAvailable!==true} aria-label="呼叫"><PhoneIcon/></button><p className="voice-note">{voiceAvailable===false?'当前调制解调器不支持语音通话':voiceAvailable===null?'正在检测语音能力…':'呼叫声音由调制解调器的主机音频链路处理'}</p></main></div>
}

function CallScreen({call,loading,error,showKeys,onKeys,onTone,onHangup,onDone,onClose}:{call:Call;loading:boolean;error:string;showKeys:boolean;onKeys:()=>void;onTone:(key:string)=>void;onHangup:()=>void;onDone:()=>void;onClose:()=>void}) {
  const ended=call.state==='terminated';
  return <div className="active-call"><header><button onClick={onClose} aria-label="返回"><ChevronLeft/></button></header><main><div className="call-avatar">{call.number.slice(-2)||'?'}</div><h1>{call.number||'未知号码'}</h1><p>{ended?(reasonLabels[call.reason||'']||labels[call.state]):labels[call.state]}</p>{error&&<p className="call-error">{error}</p>}{showKeys&&<div className="call-keypad">{'123456789*0#'.split('').map(key=><button key={key} onClick={()=>onTone(key)}>{key}</button>)}</div>}<div className="call-controls"><button disabled title="浏览器无法控制主机麦克风"><MicOff/><span>静音</span></button><button onClick={onKeys} disabled={ended}><Grid3X3/><span>拨号键盘</span></button></div>{!ended&&<button className="hangup" onClick={onHangup} disabled={loading}><PhoneOff/></button>}{ended&&<button className="call-done" onClick={onDone}>完成</button>}<small className="audio-note">音频由主机与调制解调器的语音链路处理</small></main></div>
}
