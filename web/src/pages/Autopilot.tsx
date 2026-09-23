import {useState} from 'react';
import {Sparkles,Square} from 'lucide-react';
import {AutopilotDetails} from '../components/AutopilotDetails';
import {ErrorBox,Loading} from '../components/common';
import type {AutopilotRun} from '../types';

const suggestions=['Максимально улучшить качество жизни в пределах бюджета','Помочь Нуре и убрать критические показатели','Снизить число критических показателей, потратив не больше 90'];
export function Autopilot({run,error,notice,pending,starting,cancelling,mock,ready,onStart,onCancel,onRefresh,onHistory}:{run:AutopilotRun|null;error:string;notice:string;pending:boolean;starting:boolean;cancelling:boolean;mock:boolean;ready:boolean;onStart:(goal:string)=>void;onCancel:()=>void;onRefresh:()=>void;onHistory:()=>void}) {
  const [goal,setGoal]=useState(run?.goal||'');
  return <>
    <div className="page-title"><div><p className="eyebrow">ЦЕЛЬ → ПЛАН → ПРОВЕРКА</p><h1>AI-автопилот</h1><p className="subtitle">Опишите задачу города. Агенты сами подберут пять решений, проверят расчёты и объяснят компромиссы.</p></div></div>
    <section className="panel autopilot-form">
      <form onSubmit={e=>{e.preventDefault();if(!pending&&!mock&&ready)onStart(goal);}}>
        <label htmlFor="autopilot-goal">Что нужно улучшить?<textarea id="autopilot-goal" value={goal} onChange={e=>setGoal(e.target.value)} maxLength={2000} rows={3} disabled={pending} placeholder="Например: помоги Нуре, не снижай итоговый Score остальных районов и уложись в 90 единиц"/></label>
        <p className="help">Можно оставить поле пустым — цель будет максимизировать качество жизни в рамках бюджета. Бюджет города и лимит расходов AI проверяются отдельно.</p>
        <div className="goal-suggestions" aria-label="Примеры целей">{suggestions.map(text=><button key={text} type="button" className="secondary" disabled={pending} onClick={()=>setGoal(text)}>{text}</button>)}</div>
        <div className="autopilot-actions"><button className="cta" type="submit" disabled={pending||mock||!ready}><Sparkles size={18}/>{starting?'Запускаем агентов…':'Создать план с AI'}</button>{pending&&!starting&&<button className="secondary" type="button" disabled={cancelling||mock} onClick={onCancel}><Square size={16}/>{cancelling?'Останавливаем…':'Остановить запуск'}</button>}</div>
        {mock&&<p className="notice warning">В mock-режиме автопилот отключён. Подключите API для работы агентов.</p>}
      </form>
    </section>
    {pending&&!error&&<Loading>Координатор → поиск → планировщик → проверяющий. Можно перейти на другую страницу; работа продолжится.</Loading>}
    {error&&<ErrorBox message={error} retry={pending?onRefresh:undefined}/>}
    {notice&&<div className="notice warning">{notice}{run?.status==='completed'&&<button className="text-button" onClick={onHistory}>Открыть историю</button>}</div>}
    {run&&<AutopilotDetails run={run}/>}
    {run?.status==='completed'&&!pending&&<button className="secondary" onClick={onHistory}>Посмотреть готовый план в истории</button>}
  </>;
}
