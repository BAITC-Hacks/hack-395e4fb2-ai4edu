import {useEffect,useRef,useState} from 'react';
import {recommendImprove} from '../api';
import {datasetKey,planKey,validatePlan} from '../validation';
import type {Decision,ImproveResponse,PlanAssessment,Scenario} from '../types';
import {ErrorBox,format,Loading} from './common';

interface Props {
  scenario:Scenario; plan:Decision[]; assessment?:PlanAssessment;
  ready:boolean; busy:boolean; mock:boolean; onApply:(plan:Decision[])=>void;
}
const sameDecision=(a:Decision,b:Decision)=>a.measure_id===b.measure_id&&a.district_id===b.district_id;
export function changedDecision(before:Decision[],after:Decision[]) {
  const removed=before.filter(a=>!after.some(b=>sameDecision(a,b)));
  const added=after.filter(a=>!before.some(b=>sameDecision(a,b)));
  return removed.length===1&&added.length===1?{before:removed[0],after:added[0]}:undefined;
}

export function ImprovePlan(props:Props) {
  // A new request lifecycle for every edit, including A → B → A and mode changes.
  // Leaving this page unmounts the request and aborts it as well.
  return <PlanRequest key={datasetKey(props.scenario)+planKey(props.plan)+props.mock+props.ready} {...props}/>;
}
function PlanRequest({scenario,plan,assessment,ready,busy,mock,onApply}:Props) {
  const [answer,setAnswer]=useState<ImproveResponse>();
  const [loading,setLoading]=useState(false);
  const [error,setError]=useState('');
  const control=useRef<AbortController|null>(null);
  useEffect(()=>()=>{control.current?.abort();},[]);
  function cancel(){control.current?.abort();control.current=null;setLoading(false);}
  async function load(){
    if(!ready||mock||busy||control.current)return;
    const controller=new AbortController();control.current=controller;
    setLoading(true);setError('');setAnswer(undefined);
    try {
      const response=await recommendImprove(plan,controller.signal);
      if(controller.signal.aborted)return;
      if(response.improvements.some(c=>!validatePlan(scenario,c.decisions).valid||!changedDecision(plan,c.decisions)))
        throw new Error('Рекомендация не соответствует текущему плану или каталогу. Повторите запрос.');
      setAnswer(response);
    } catch(e){
      if(!controller.signal.aborted)setError(e instanceof Error?e.message:'Не удалось найти улучшения. Повторите запрос.');
    } finally {
      if(!controller.signal.aborted){control.current=null;setLoading(false);}
    }
  }
  const describe=(d:Decision)=>`${d.measure_id} · ${scenario.measures.find(m=>m.id===d.measure_id)?.name} — ${d.district_id?scenario.districts.find(x=>x.id===d.district_id)?.name:'Весь город'}`;
  return <section className="panel improve-plan" aria-labelledby="improve-title">
    <div className="improve-heading"><div><p className="eyebrow">РЕКОМЕНДАЦИИ ОПТИМИЗАТОРА</p><h2 id="improve-title">Одна замена — новые возможности</h2></div>
      <button className="secondary" disabled={!ready||busy||mock||loading} aria-describedby="improve-reason" onClick={load}>Улучшить мой план</button></div>
    <p className="help" id="improve-reason">{mock?'В учебном режиме (mock) поиск недоступен. Подключите сервер.':busy?'Дождитесь завершения расчёта.':!ready?'Соберите пять решений и дождитесь проверки плана сервером.':'Оптимизатор проверит замену одной меры или её района. Это расчётная рекомендация, не ответ ИИ. Ваш план изменится только по вашему выбору.'}</p>
    {loading&&<><Loading>Ищем улучшения вашего плана…</Loading><button className="text-button" onClick={cancel}>Отменить поиск</button></>}
    {error&&<ErrorBox message={error} retry={load}/>}
    {answer&&<div className="improve-results">
      <p role="status">{answer.improvements.length?`Найдено вариантов: ${answer.improvements.length}. Выберите замену или оставьте свой план.`:'Улучшений одной заменой не найдено. Это не означает, что план лучший среди всех возможных.'}</p>
      <p className="help">Оценка текущего плана: <strong>{format(answer.current_score,6)} Score</strong>. Прирост каждого варианта указан относительно этого плана.</p>
      {answer.improvements.map((candidate,i)=>{
        const change=changedDecision(plan,candidate.decisions)!;
        return <article className="improvement" key={planKey(candidate.decisions)} aria-label={`Вариант ${i+1}`}>
          <h3>Вариант {i+1} · {change.before.measure_id===change.after.measure_id?'Изменить район':'Заменить мероприятие'}</h3>
          <dl className="decision-change"><div><dt>Сейчас</dt><dd>{describe(change.before)}</dd></div><div><dt>Предложение</dt><dd>{describe(change.after)}</dd></div></dl>
          <p className="help">Остальные четыре решения сохраняются.</p>
          <dl className="improve-metrics">
            <div><dt>Score: сейчас → предложение</dt><dd>{format(answer.current_score,6)} → <strong>{format(candidate.final_score,6)}</strong> <span className="tag">+{format(candidate.score_delta,6)}</span></dd></div>
            <div><dt>Потрачено: сейчас → предложение</dt><dd>{assessment?.total_cost??'Нет данных'} → <strong>{candidate.total_cost}</strong> ед.</dd></div>
            <div><dt>Остаток: сейчас → предложение</dt><dd>{assessment?.remaining_budget??'Нет данных'} → <strong>{candidate.remaining_budget}</strong> ед.</dd></div>
            <div><dt>Критические значения: сейчас → предложение</dt><dd>{assessment?.critical_after??'Нет данных'} → <strong>{candidate.critical_after}</strong></dd></div>
          </dl>
          <button className="secondary" disabled={!ready||busy||mock} onClick={()=>{if(ready&&!busy&&!mock)onApply(candidate.decisions.map(d=>({...d})));}}>Применить вариант {i+1}</button>
        </article>;
      })}
    </div>}
  </section>;
}
