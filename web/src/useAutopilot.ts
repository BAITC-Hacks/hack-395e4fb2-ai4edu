import {useEffect,useRef,useState} from 'react';
import {ApiError,cancelAutopilot,getAutopilot,getScenario,startAutopilot} from './api';
import {isScenario,resultMatchesScenario} from './contract';
import {datasetKey} from './validation';
import type {AutopilotRun,Scenario} from './types';

export const ACTIVE_AUTOPILOT_KEY='akim.autopilot.active.v1';
interface Tracker {id:string; scenario:Scenario; draftKey:string; suppressApply:boolean}
const active=(r:AutopilotRun)=>r.status==='queued'||r.status==='running';
function readActive():Tracker|null {
  try {
    const v=JSON.parse(localStorage.getItem(ACTIVE_AUTOPILOT_KEY)||'null');
    return v&&typeof v.id==='string'&&v.id&&isScenario(v.scenario)&&typeof v.draftKey==='string'&&typeof v.suppressApply==='boolean'?v:null;
  } catch {return null;}
}
export function useAutopilot({scenario,draftKey,mock,onComplete}:{scenario?:Scenario;draftKey:string;mock:boolean;onComplete:(run:AutopilotRun,scenario:Scenario,draftKey:string,suppressApply:boolean)=>boolean}) {
  const [tracker,setTracker]=useState<Tracker|null>(readActive);
  const [run,setRun]=useState<AutopilotRun|null>(null);
  const [error,setError]=useState('');
  const [notice,setNotice]=useState('');
  const [starting,setStarting]=useState(false);
  const [cancelling,setCancelling]=useState(false);
  const [retry,setRetry]=useState(0);
  const locked=useRef(false);
  const cancelRequested=useRef(false);
  const callback=useRef(onComplete);callback.current=onComplete;
  const completed=useRef(new Set<string>());
  const mounted=useRef(true);
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;};},[]);
  function remember(next:Tracker|null) {
    setTracker(next);
    try {if(next)localStorage.setItem(ACTIVE_AUTOPILOT_KEY,JSON.stringify(next));else localStorage.removeItem(ACTIVE_AUTOPILOT_KEY);}
    catch {setNotice('Браузер не сохранил активный запуск. Не закрывайте страницу до завершения.');}
  }
  useEffect(()=>{
    if(!tracker||mock)return;
    let alive=true;
    let timer:ReturnType<typeof setTimeout>|undefined;
    const controller=new AbortController();
    async function poll() {
      try {
        const next=await getAutopilot(tracker!.id,controller.signal);
        if(!alive)return;
        if(next.id!==tracker!.id)throw new Error('Сервер вернул другой запуск. Текущий план сохранён.');
        setRun(next);setError('');
        if(active(next)){timer=setTimeout(poll,1000);return;}
        if(next.status==='completed'&&!completed.current.has(next.id)) {
          const latest=await getScenario();
          if(!alive)return;
          if(datasetKey(latest)!==datasetKey(tracker!.scenario)||!next.result||!resultMatchesScenario(next.result,latest)) {
            setNotice('Каталог или правила изменились во время запуска. Готовый вариант не применён; обновите каталог и запустите автопилот заново.');
          } else {
            completed.current.add(next.id);
            const applied=callback.current(next,latest,tracker!.draftKey,tracker!.suppressApply||cancelRequested.current);
            if(!applied)setNotice('План был изменён или запрошена остановка. Готовый вариант сохранён в истории; текущие решения не заменены.');
          }
        }
        if(alive)remember(null);
      } catch(e) {
        if(!alive)return;
        setError(e instanceof Error?e.message:'Не удалось получить состояние автопилота.');
        if(e instanceof ApiError&&e.status===404)remember(null);
      }
    }
    void poll();
    return()=>{alive=false;controller.abort();clearTimeout(timer);};
  },[tracker,retry,mock]);
  async function start(goal:string) {
    if(locked.current||tracker||mock||!scenario)return;
    locked.current=true;setStarting(true);setError('');setNotice('');setRun(null);cancelRequested.current=false;
    const snapshot=scenario, initialDraft=draftKey;
    try {
      const latest=await getScenario();
      if(datasetKey(latest)!==datasetKey(snapshot))throw new Error('Каталог изменился. Обновите страницу перед новым запуском.');
      const next=await startAutopilot(goal.trim());
      if(!mounted.current)return;
      setRun(next);
      remember({id:next.id,scenario:snapshot,draftKey:initialDraft,suppressApply:false});
    } catch(e) {if(mounted.current)setError(e instanceof Error?e.message:'Не удалось запустить автопилот.');}
    finally {locked.current=false;if(mounted.current)setStarting(false);}
  }
  async function cancel() {
    if(!tracker||cancelling||mock)return;
    // A response already in flight must never apply a plan after the user stops it.
    cancelRequested.current=true;setCancelling(true);setError('');
    const stopped={...tracker,suppressApply:true};remember(stopped);
    try {
      const next=await cancelAutopilot(tracker.id);
      if(!mounted.current)return;
      if(next.id!==tracker.id)throw new Error('Сервер вернул другой запуск при остановке.');
      setRun(next);
      // Refresh authoritative terminal state through the same reconciliation path.
      setRetry(v=>v+1);
    } catch(e){if(mounted.current)setError(e instanceof Error?e.message:'Не удалось остановить запуск. Проверьте его состояние.');}
    finally{if(mounted.current)setCancelling(false);}
  }
  return {run,error,notice,starting,cancelling,pending:!!tracker||starting,start,cancel,refresh:()=>{setError('');setRetry(v=>v+1);}};
}
