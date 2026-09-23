import {useState} from 'react';
import type {Metric, Scenario, SimulationResult} from '../types';
import {format} from './common';

// These are visual lengths, not a simulation or a Score calculation.
// docs/scoring.md bounds indicators and their weighted district average to 0–100.
export function BeforeAfterBars({label,before,after,threshold}:{label:string;before:number;after:number;threshold?:number}) {
  const direction=after>before?'↑ Рост':after<before?'↓ Снижение':'= Без изменений';
  return <li className="comparison-bars" aria-label={label}>
    <div className="bar-heading"><strong>{label}</strong><span className={after<before?'danger':'bar-direction'}>{direction}</span></div>
    {([['До',before,'before'],['После',after,'after']] as const).map(([name,value,phase])=>{
      const critical=threshold!==undefined&&value<threshold;
      return <div className={`bar-row ${phase}`} key={phase}>
        <span>{name}</span>
        <div className="bar-track" aria-hidden="true"><span style={{width:`${value}%`}}/>{threshold!==undefined&&<i className="threshold-mark" style={{left:`${threshold}%`}}/>}</div>
        <strong className={critical?'danger':''}>{format(value,6)}</strong>
        {critical&&<span className="bar-critical">! {name}: критическое значение</span>}
      </div>;
    })}
  </li>;
}

export function ResultCharts({result:r,scenario:s}:{result:SimulationResult;scenario:Scenario}) {
  const [selected,setSelected]=useState<Metric>(s.indicator_order.includes('S1')?'S1':s.indicator_order[0]);
  const metric=s.indicator_order.includes(selected)?selected:s.indicator_order[0];
  return <div className="result-charts">
    <section className="panel chart-panel" aria-labelledby="district-chart-title">
      <p className="eyebrow">ПЯТЬ РАЙОНОВ · ОДНА ШКАЛА</p><h2 id="district-chart-title">Оценки районов</h2>
      <p className="help">До и после решений, от 0 до 100. Чем выше, тем лучше.</p>
      <ul className="bar-list">{r.district_before_after.map(d=><BeforeAfterBars key={d.district_id} label={d.name} before={d.score_before} after={d.score_after}/>)}</ul>
    </section>
    <section className="panel chart-panel" aria-labelledby="indicator-chart-title">
      <p className="eyebrow">В ФОКУСЕ · ПОКАЗАТЕЛЬ</p><h2 id="indicator-chart-title">Где меняется жизнь</h2>
      <label className="chart-selector" htmlFor="chart-indicator">Показатель для сравнения<select id="chart-indicator" value={metric} onChange={e=>setSelected(e.target.value as Metric)}>{s.indicator_order.map(k=><option key={k} value={k}>{s.indicator_names[k]}</option>)}</select></label>
      <p className="help" id="chart-threshold">Шкала 0–100. ! Критическое значение — строго ниже {s.scoring.critical_threshold}. Пунктир отмечает порог.</p>
      <ul className="bar-list" aria-describedby="chart-threshold">{r.district_before_after.map(d=><BeforeAfterBars key={d.district_id} label={`${d.name} · ${s.indicator_names[metric]}`} before={d.before[metric]} after={d.after[metric]} threshold={s.scoring.critical_threshold}/>)}</ul>
    </section>
  </div>;
}
