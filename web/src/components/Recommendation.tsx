import {useEffect,useRef,useState} from 'react';
import {ApiError,recommendBest} from '../api';
import {validatePlan} from '../validation';
import type {Candidate,Decision,Scenario} from '../types';
import {ErrorBox,format,Loading} from './common';

export function Recommendation({scenario,mock,busy,onApply}:{scenario:Scenario;mock:boolean;busy:boolean;onApply:(plan:Decision[])=>void}) {
  const [best,setBest]=useState<Candidate>();
  const [loading,setLoading]=useState(false);
  const [error,setError]=useState('');
  const control=useRef<AbortController|null>(null);
  const timer=useRef<ReturnType<typeof setTimeout>|undefined>(undefined);
  function cancel(){control.current?.abort();control.current=null;clearTimeout(timer.current);setLoading(false);}
  useEffect(()=>()=>{control.current?.abort();clearTimeout(timer.current);},[]);
  useEffect(()=>{cancel();setBest(undefined);setError('');},[mock,scenario]);
  async function load(){
    if(mock||busy||control.current)return;
    const controller=new AbortController();control.current=controller;
    setLoading(true);setError('');setBest(undefined);
    async function poll(){
      try {
        const candidate=await recommendBest(controller.signal);
        if(controller.signal.aborted)return;
        if(!validatePlan(scenario,candidate.decisions).valid)throw new Error('Рекомендация не соответствует текущему каталогу. Обновите каталог.');
        setBest(candidate);setLoading(false);control.current=null;
      } catch(e) {
        if(controller.signal.aborted)return;
        if(e instanceof ApiError&&e.status===503&&e.validation_errors.some(v=>v.code==='not_ready')){
          timer.current=setTimeout(poll,2000);return;
        }
        setError(e instanceof Error?e.message:'Не удалось получить рекомендацию.');setLoading(false);control.current=null;
      }
    }
    await poll();
  }
  return <section className="panel recommendation"><h2>Лучший план по модели</h2><p className="help">Оптимизатор ищет лучший набор решений по модели города. Это расчётная рекомендация, а не ответ ИИ. Замена плана — только по вашему выбору.</p>
    {loading?<><Loading>Считается…</Loading><p className="help">Оптимизатор ещё работает. Запрос повторяется автоматически; ожидание можно отменить.</p><button className="secondary" onClick={cancel}>Отменить ожидание</button></>:<button className="secondary" disabled={mock||busy} onClick={load}>Показать лучший план</button>}
    {mock&&<p className="help">В учебном режиме (mock) рекомендации оптимизатора недоступны.</p>}{busy&&<p className="help">Дождитесь завершения расчёта.</p>}
    {error&&<ErrorBox message={error} retry={load}/>}
    {best&&<div className="recommendation-result"><p><strong>{format(best.final_score)} Score</strong> · стоимость {best.total_cost} · осталось {best.remaining_budget} · критических значений {best.critical_after}</p><ul>{best.decisions.map(d=><li key={d.measure_id}>{d.measure_id} · {scenario.measures.find(m=>m.id===d.measure_id)?.name} — {d.district_id?scenario.districts.find(x=>x.id===d.district_id)?.name:'Весь город'}</li>)}</ul><button className="secondary" disabled={busy} onClick={()=>{if(!busy)onApply(best.decisions.map(d=>({...d})));}}>Заменить текущий план рекомендацией</button><p className="help">Будут заменены все пять решений. Полные показатели появятся после запуска.</p></div>}
  </section>;
}
