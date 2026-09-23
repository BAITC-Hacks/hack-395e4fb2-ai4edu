import {useRef, useState} from 'react';
import {ArrowRight, Check, Pencil, Plus, Trash2, X} from 'lucide-react';
import {categories, type Category, type Decision, type Measure, type Scenario} from '../types';
import {validatePlan} from '../validation';
import {CategoryIcon, EffectsList, ErrorBox, Loading} from '../components/common';

export function Decisions({scenario:s,plan,name,onName,onChange,onRun,busy,mock,error}:{scenario:Scenario;plan:Decision[];name:string;onName:(v:string)=>void;onChange:(v:Decision[])=>void;onRun:()=>void;busy:boolean;mock:boolean;error:string}) {
  const [filter,setFilter] = useState<Category|'all'>('all');
  const [targets,setTargets] = useState<Record<string,string>>({});
  const [editing,setEditing] = useState<number|null>(null);
  const [feedback,setFeedback] = useState('');
  const [editError,setEditError] = useState('');
  const catalog = useRef<HTMLElement>(null);
  const summary = validatePlan(s,plan);
  const candidate = (m:Measure):Decision => m.scope==='City' ? {measure_id:m.id} : {measure_id:m.id,district_id:targets[m.id] || ''};
  const proposed = (d:Decision) => editing===null ? [...plan,d] : plan.map((p,i)=>i===editing?d:p);
  function commit(m:Measure) {
    const next = proposed(candidate(m));
    const check = validatePlan(s,next,false);
    if (check.errors.length || busy) return;
    onChange(next); setEditing(null); setEditError('');
    setFeedback(`${m.id} ${editing===null?'добавлено в план':'сохранено в плане'}. Решений: ${next.length} из ${s.required_decisions}.`);
  }
  function changeDistrict(index:number,district_id:string) {
    const next = plan.map((d,i)=>i===index?{measure_id:d.measure_id,district_id}:d);
    const check = validatePlan(s,next,false);
    if (check.errors.length) { setEditError(check.errors.map(e=>e.message).join(' ')); return; }
    onChange(next); setEditError(''); setFeedback('Район изменён. План проверен.');
  }
  function remove(index:number) {
    onChange(plan.filter((_,i)=>i!==index)); setEditing(null); setEditError(''); setFeedback('Решение удалено. Освободился слот.');
    requestAnimationFrame(()=>document.getElementById('plan')?.focus());
  }
  const edit = (index:number) => {setEditing(index);setFilter('all');setFeedback('Выберите новую меру в каталоге.');catalog.current?.focus();};
  return <>
    <div className="page-title"><div><p className="eyebrow">01 / СОБЕРИТЕ ПЛАН</p><h1>Ваши решения</h1><p className="subtitle">{s.required_decisions} мер · бюджет {s.budget} · максимум {s.max_measures_per_category} меры одного направления</p></div><a className="secondary plan-anchor" href="#plan">К выбранным решениям ↓</a></div>
    <div className="planner"><section className="catalog" aria-label="Каталог мероприятий" ref={catalog} tabIndex={-1}>
      <div className="filters" aria-label="Фильтры по направлениям"><button aria-pressed={filter==='all'} onClick={()=>setFilter('all')}>Все · {s.measures.length}</button>{categories.map(c=><button key={c} aria-pressed={filter===c} onClick={()=>setFilter(c)}><CategoryIcon category={c}/>{s.category_names[c]}</button>)}</div>
      {editing!==null && <div className="notice edit-notice"><span>Замена решения {editing+1}. Выберите мероприятие и район.</span><button className="text-button" onClick={()=>setEditing(null)}><X size={16}/>Отменить замену</button></div>}
      <div className="catalog-grid">{s.measures.filter(m=>filter==='all'||m.category===filter).map(m=>{
        const chosen = plan.some((d,i)=>d.measure_id===m.id && i!==editing);
        const problems = validatePlan(s,proposed(candidate(m)),false).errors;
        const reason = chosen ? `${m.id} уже в плане. Можно изменить или удалить его в панели решений.` : problems.map(e=>e.message).join(' ');
        return <article key={m.id} className={`measure-card ${chosen?'chosen':''}`} aria-labelledby={`title-${m.id}`}>
          <div className="measure-top"><span className={`category category-${m.category}`}><CategoryIcon category={m.category}/>{s.category_names[m.category]}</span><span className="measure-id">{m.id}</span></div>
          <h2 id={`title-${m.id}`}>{m.name}</h2>
          <dl className="measure-facts"><div><dt>Стоимость</dt><dd>{m.cost} <small>ед.</small></dd></div><div><dt>Действует</dt><dd>{m.scope==='City'?'Весь город':'Один район'}</dd></div><div><dt>Лаг</dt><dd>{m.lag} <small>кв.</small></dd></div></dl>
          <details><summary>Полный эффект до учёта лага</summary><EffectsList effects={m.effects} scenario={s}/><p className="help">Реализованный эффект покажет Go после расчёта.</p></details>
          <div className="measure-action">{m.scope==='District' ? <label htmlFor={`district-${m.id}`}>Район мероприятия<select id={`district-${m.id}`} value={targets[m.id] || ''} disabled={busy||chosen} onChange={e=>setTargets({...targets,[m.id]:e.target.value})}><option value="">Выберите район</option>{s.districts.map(d=><option key={d.id} value={d.id}>{d.name}</option>)}</select></label> : <p className="city-scope">Применяется ко всем {s.districts.length} районам</p>}
            <button className={chosen?'secondary':'add-button'} aria-disabled={!!reason||busy} aria-describedby={`reason-${m.id}`} onClick={()=>!reason&&!busy&&commit(m)}>{chosen?<Check size={17}/>:<Plus size={17}/>} {chosen?'Уже в плане':editing===null?'Добавить решение':'Заменить этим решением'}</button>
            <p id={`reason-${m.id}`} className={`action-reason ${reason&&!chosen?'blocked':''}`}>{busy?'Дождитесь завершения расчёта.':reason||'Можно добавить в план.'}</p>
          </div>
        </article>;
      })}</div>
      <section className="panel synergy-catalog"><h2>Меры, которые усиливают друг друга</h2><p className="help">Синергии из каталога Go. Бонусы не зависят от лага.</p>{s.synergies.map(y=>{const active = y.measure_ids.every(id=>plan.some(d=>d.measure_id===id));const target=plan.find(d=>d.measure_id===y.district_measure_id)?.district_id;return <div className="synergy" key={y.measure_ids.join()}><strong>{y.measure_ids.join(' + ')} <span className="tag">{active?'Пара выбрана':'Доступная пара'}</span></strong><p>{y.measure_ids.map(id=>s.measures.find(m=>m.id===id)?.name).join(' + ')}</p><p>Район {active?s.districts.find(d=>d.id===target)?.name:`меры ${y.district_measure_id}`}</p><EffectsList effects={y.effects} scenario={s}/></div>})}</section>
    </section>
    <aside className="plan-panel" id="plan" tabIndex={-1} aria-label="Выбранный план"><div className="plan-heading"><p className="eyebrow">ВАШ ПЛАН</p><strong>{plan.length} / {s.required_decisions}</strong></div>
      <label className="name-label" htmlFor="plan-name">Название сценария<input id="plan-name" value={name} maxLength={120} onChange={e=>onName(e.target.value)} disabled={busy}/></label>
      <ol className="slots">{Array.from({length:Math.max(s.required_decisions,plan.length)},(_,i)=>{const d=plan[i];const m=s.measures.find(m=>m.id===d?.measure_id);return <li key={i} className={`${d?'filled':'vacant'} ${editing===i?'editing':''}`}><span className="slot-number">{i+1}</span>{d ? <div className="slot-content"><div className="slot-title"><strong>{m?.name || `Недоступная мера ${d.measure_id}`}</strong><span>{m?.cost ?? '—'} ед.</span></div>{m?.scope==='District'?<label className="slot-district">Район для {d.measure_id}<select aria-label={`Район выбранной меры ${d.measure_id}`} value={d.district_id || ''} disabled={busy} onChange={e=>changeDistrict(i,e.target.value)}><option value="">Выберите район</option>{!s.districts.some(x=>x.id===d.district_id)&&d.district_id&&<option value={d.district_id}>Недоступный район</option>}{s.districts.map(x=><option value={x.id} key={x.id}>{x.name}</option>)}</select></label>:<p className="help">{d.measure_id} · Весь город</p>}<div className="slot-actions"><button onClick={()=>edit(i)} disabled={busy} aria-label={`Заменить ${d.measure_id}`}><Pencil size={14}/>Заменить</button><button onClick={()=>remove(i)} disabled={busy} aria-label={`Удалить ${d.measure_id}`}><Trash2 size={14}/>Удалить</button></div></div>:<span>Выберите мероприятие</span>}</li>})}</ol>
      {editError && <ErrorBox message={editError}/>}
      <div className="budget"><div><span>Использовано</span><strong>{summary.cost} <small>/ {s.budget}</small></strong></div><progress max={s.budget} value={Math.min(summary.cost,s.budget)} aria-label="Использованный бюджет"/><div><span>Осталось</span><strong className={summary.remaining<0?'danger':''}>{summary.remaining} ед.</strong></div></div>
      <div className="plan-validation" id="run-reason">{mock?<p>В mock-режиме расчёт недоступен. Подключите Go API.</p>:summary.errors.length?<ul>{summary.errors.map((e,i)=><li key={i}>{e.message}</li>)}</ul>:<p className="ready"><Check size={17}/> План готов к расчёту</p>}</div>
      {error && <ErrorBox message={error}/>}
      <button className="cta" aria-disabled={!summary.valid||busy||mock} aria-describedby="run-reason" onClick={()=>summary.valid&&!busy&&!mock&&onRun()}>{busy?<Loading>Считаем в Go…</Loading>:<>Рассчитать результат <ArrowRight size={18}/></>}</button>
      <p className="help">Score и итоговые показатели рассчитывает Go. AI объясняет готовый результат.</p>
    </aside></div><p className="sr-only" role="status">{feedback}</p>
    <div className="plan-dock"><span><strong>{plan.length} / {s.required_decisions} решений · {summary.cost} ед.</strong>Осталось {summary.remaining} ед.</span><a href="#plan">План и запуск ↑</a></div>
  </>;
}
