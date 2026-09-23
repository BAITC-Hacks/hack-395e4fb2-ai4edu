import {useEffect,useState} from 'react';
import {assessPlan} from './api';
import {datasetKey,planKey} from './validation';
import type {Decision,PlanAssessment,Scenario} from './types';

export function usePlanAssessment(plan:Decision[],scenario:Scenario,mock:boolean) {
  const key=datasetKey(scenario)+planKey(plan);
  const [state,setState]=useState<{key:string;answer?:PlanAssessment;error?:string}>();
  const [attempt,setAttempt]=useState(0);
  useEffect(()=>{
    if(mock)return;
    const controller=new AbortController();
    setState(undefined);
    const timer=setTimeout(()=>{
      assessPlan(plan,controller.signal).then(answer=>{
        if(!controller.signal.aborted)setState({key,answer});
      }).catch(error=>{
        if(!controller.signal.aborted)setState({key,error:error instanceof Error?error.message:'Проверка плана недоступна.'});
      });
    },250);
    return ()=>{clearTimeout(timer);controller.abort();};
  },[key,mock,attempt]);
  const current=!mock&&state?.key===key?state:undefined;
  return {answer:current?.answer,error:current?.error,checking:!mock&&!current,retry:()=>setAttempt(n=>n+1)};
}
