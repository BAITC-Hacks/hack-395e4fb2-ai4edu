import {useCallback,useEffect,useRef,useState,type RefObject} from 'react';

const preference=()=>typeof window!=='undefined'&&typeof window.matchMedia==='function'
  ?window.matchMedia('(prefers-reduced-motion: reduce)'):undefined;

// CSS owns the finite construction timeline. This state only controls user intent;
// no wall-clock timer can run ahead of a paused or backgrounded animation.
export function useSceneMotion(sceneRef?:RefObject<SVGSVGElement|null>) {
  const [reduced,setReduced]=useState(()=>preference()?.matches??false);
  const [awaitingEntry,setAwaitingEntry]=useState(()=>!reduced&&!!sceneRef&&typeof IntersectionObserver==='function');
  const [motion,setMotion]=useState<'running'|'paused'|'complete'>(()=>reduced?'complete':awaitingEntry?'paused':'running');
  const [run,setRun]=useState(0);
  const waiting=useRef(awaitingEntry),observer=useRef<IntersectionObserver|null>(null);
  const stopWaiting=useCallback(()=>{
    waiting.current=false;setAwaitingEntry(false);
    observer.current?.disconnect();observer.current=null;
  },[]);
  useEffect(()=>{
    const media=preference();if(!media)return;
    const sync=()=>{setReduced(media.matches);if(media.matches){stopWaiting();setMotion('complete');}};
    sync();media.addEventListener('change',sync);
    return ()=>media.removeEventListener('change',sync);
  },[stopWaiting]);
  useEffect(()=>{
    if(!waiting.current||reduced)return;
    const target=sceneRef?.current;
    if(!target){stopWaiting();setMotion('running');return;}
    const entryObserver=new IntersectionObserver(entries=>{
      // Disconnected observers may still have queued entries. Explicit controls,
      // reduced motion and the first entry permanently retire this auto-start.
      if(observer.current!==entryObserver||!waiting.current)return;
      if(entries.some(entry=>entry.target===target&&entry.isIntersecting&&entry.intersectionRatio>=.15)){
        stopWaiting();setMotion('running');
      }
    },{threshold:.15});
    observer.current=entryObserver;entryObserver.observe(target);
    return ()=>{entryObserver.disconnect();if(observer.current===entryObserver)observer.current=null;};
  },[sceneRef,reduced,stopWaiting]);
  return {
    motion:reduced?'reduced' as const:motion,run,awaitingEntry,
    toggle:()=>{if(!reduced){stopWaiting();setMotion(value=>value==='running'?'paused':value==='paused'?'running':value);}},
    replay:()=>{if(!reduced){stopWaiting();setRun(value=>value+1);setMotion('running');}},
    complete:()=>{stopWaiting();setMotion('complete');},
  };
}
