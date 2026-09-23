import {useId,useRef,useState,type KeyboardEvent} from 'react';
import type {Decision,Measure,Metric,Scenario,SimulationResult} from '../types';
import {format} from './common';
import {shapesForDistricts,schematicViewBox} from './districtGeometry';
import {CityGround,CityArchitecture} from './CityScene';
import {useSceneMotion} from './useSceneMotion';
import './district-visualization.css';

type ViewMode='before'|'after'|'change';
interface Props {
  scenario:Scenario; selectedId:string; onSelect:(id:string)=>void;
  decisions?:Decision[]; result?:SimulationResult;
  onOpenMeasures?:(districtId:string,metric:Metric)=>void;
}
const levels=[[244,228,207],[213,225,195],[153,185,155]];
// Color interpolation encodes an existing value only; it creates no new metric.
export function indicatorColor(value:number) {
  const position=Math.max(0,Math.min(100,value))/50;
  const index=position<1?0:1, ratio=position-index;
  return `rgb(${levels[index].map((v,i)=>Math.round(v+(levels[index+1][i]-v)*ratio)).join(', ')})`;
}
const changeColor=(value:number)=>value<0?'#eed0bd':value>0?'#bfd9bd':'#e6eae2';
const direction=(value:number)=>value<0?'↓ Снижение':value>0?'↑ Рост':'= Без изменений';
const display=(value:number|undefined,change=false)=>value===undefined?'Нет данных':`${change&&value>0?'+':''}${format(value,6)}`;

export function DistrictVisualization({scenario:s,selectedId,onSelect,decisions=[],result,onOpenMeasures}:Props) {
  const uid=useId();
  const sceneRef=useRef<SVGSVGElement>(null);
  const animation=useSceneMotion(sceneRef);
  const [indicator,setIndicator]=useState<Metric>(s.indicator_order.includes('S1')?'S1':s.indicator_order[0]);
  const [mode,setMode]=useState<ViewMode>(result?'after':'before');
  const metric=s.indicator_order.includes(indicator)?indicator:s.indicator_order[0];
  const view=result?mode:'before';
  const selected=s.districts.find(d=>d.id===selectedId)||s.districts[0];
  const nodes=useRef<Record<string,SVGGElement|null>>({});
  const threshold=s.scoring.critical_threshold;
  const shapes=shapesForDistricts(s.districts.map(d=>d.id));
  const measure=(d:Decision)=>s.measures.find(m=>m.id===d.measure_id);
  // On results the badges always describe that exact calculated plan, including history.
  const shownDecisions=result?result.decisions:decisions;
  const localMeasures=(id:string)=>shownDecisions.filter(d=>d.district_id===id&&measure(d)?.scope==='District').map(d=>measure(d)!);
  const cityMeasures=shownDecisions.filter(d=>measure(d)?.scope==='City').map(d=>measure(d)!);
  const affectedIds=result?s.districts.filter(d=>cityMeasures.length>0||localMeasures(d.id).length>0).map(d=>d.id):[];
  const affectedSlots=affectedIds.map(id=>shapes.get(id)?.sceneSlot).filter((slot):slot is string=>!!slot);
  function values(id:string) {
    const calculated=result?.district_before_after.find(d=>d.district_id===id);
    const before=result?calculated?.before[metric]:s.districts.find(d=>d.id===id)?.indicators[metric];
    const after=calculated?.after[metric];
    const delta=result?.indicator_deltas[id]?.[metric];
    const value=view==='change'?delta:view==='after'?after:before;
    // In change mode the critical marker refers to the final indicator, not its delta.
    const criticalValue=view==='change'?after:value;
    return {before,after,delta,value,critical:criticalValue!==undefined&&criticalValue<threshold};
  }
  const detail=values(selected.id);
  function handleKey(event:KeyboardEvent<SVGGElement>,id:string) {
    if(event.key==='Enter'||event.key===' '){event.preventDefault();onSelect(id);return;}
    const index=s.districts.findIndex(d=>d.id===id);
    let next=index;
    if(event.key==='ArrowRight'||event.key==='ArrowDown')next=(index+1)%s.districts.length;
    else if(event.key==='ArrowLeft'||event.key==='ArrowUp')next=(index-1+s.districts.length)%s.districts.length;
    else if(event.key==='Home')next=0;
    else if(event.key==='End')next=s.districts.length-1;
    else return;
    event.preventDefault();const target=s.districts[next].id;onSelect(target);nodes.current[target]?.focus();
  }
  const badge=(m:Measure)=><li key={m.id}><span className="district-measure-badge">{m.id}</span><span>{m.name}</span></li>;
  return <section className="panel district-visualization" aria-labelledby={`${uid}-title`}>
    <div className="district-view-header">
      <div><p className="eyebrow">ПЯТЬ РАЙОНОВ · ОДИН ГОРОД</p><h2 id={`${uid}-title`}>Город по районам</h2><p className="schematic-label">Условная визуализация · синтетические данные</p></div>
      <label className="district-indicator" htmlFor={`${uid}-indicator`}>Показатель на схеме<select id={`${uid}-indicator`} value={metric} onChange={e=>setIndicator(e.target.value as Metric)}>{s.indicator_order.map(k=><option key={k} value={k}>{s.indicator_names[k]}</option>)}</select></label>
    </div>
    {result&&<div className="district-view-modes" role="group" aria-label="Режим схемы">{([['before','До'],['after','После'],['change','Изменение']] as const).map(([id,label])=><button key={id} aria-pressed={view===id} onClick={()=>setMode(id)}>{label}</button>)}</div>}
    <div className="district-view-layout">
      <div className="district-canvas">
        <div className="district-canvas-caption"><span>{s.indicator_names[metric]}</span><strong>{view==='change'?'Изменение, пункты':view==='after'?'После решений':'Исходные значения'}</strong></div>
        <div className="city-motion-toolbar" role="group" aria-label="Управление анимацией города">
          <p role="status">{animation.motion==='reduced'?'Статичная сцена · уменьшение движения':animation.awaitingEntry?'Сцена готова к появлению':animation.motion==='paused'?'Анимация на паузе':animation.motion==='complete'?'Город собран':'Город постепенно появляется'}</p>
          <div><button className="city-motion-button" disabled={animation.motion==='reduced'||animation.motion==='complete'} onClick={animation.toggle}>{animation.motion==='paused'?'Продолжить анимацию':'Пауза анимации'}</button><button className="city-motion-button" disabled={animation.motion==='reduced'} onClick={animation.replay}>Повторить анимацию</button></div>
        </div>
        <svg ref={sceneRef} viewBox={schematicViewBox} className="district-svg city-scene" data-motion={animation.motion} role="group" aria-label="Схема пяти районов" aria-describedby={`${uid}-keyboard ${uid}-legend ${uid}-illustration`}>
          <CityGround/>
          {s.districts.map(d=>{
            const shape=shapes.get(d.id);
            if(!shape)return null;
            const data=values(d.id), chosen=selected.id===d.id, count=localMeasures(d.id).length;
            const status=data.value===undefined?'Нет данных':view==='change'?direction(data.value):data.critical?'Критическое значение':'Вне критической зоны';
            return <g key={d.id} ref={node=>{nodes.current[d.id]=node;}} className={`district-cell ${chosen?'is-selected':''}`} role="button" tabIndex={chosen?0:-1} aria-pressed={chosen} aria-controls={`${uid}-detail`} aria-label={`${d.name}: ${display(data.value,view==='change')}. ${status}${view==='change'&&data.critical?'. Критическое значение после решений':''}. Районных мер: ${count}`} onClick={()=>onSelect(d.id)} onKeyDown={event=>handleKey(event,d.id)}>
              <path d={shape.path} fill={data.value===undefined?'#edf0eb':view==='change'?changeColor(data.value):indicatorColor(data.value)} className="district-cell-shape"/>
              <defs><g id={`${uid}-label-${d.id}`} transform={`translate(${shape.label[0]} ${shape.label[1]})`} className={`district-cell-label ${chosen?'is-selected':''}`} aria-hidden="true">
                <rect x="-83" y="-28" width="166" height="69" rx="12"/>
                <text className="district-cell-name" x="-68" y="-4">{chosen?'✓ ':''}{d.name}</text>
                <text className="district-cell-value" x="-68" y="24">{data.critical?'! ':''}{data.value===undefined?'—':`${view==='change'&&data.value>0?'+':''}${format(data.value,2)}`}</text>
                {count>0&&<text className="district-cell-count" x="69" y="-4">({count})</text>}
                {view==='change'&&data.value!==undefined&&<text className="district-cell-count" x="69" y="24">{data.value>0?'↑':data.value<0?'↓':'='}</text>}
              </g></defs>
            </g>;
          })}
          <g key={animation.run} className="city-construction" data-motion={animation.motion} data-run={animation.run} aria-hidden="true" pointerEvents="none">
            <CityArchitecture affectedSlots={affectedSlots}/>
            <g className="city-animation-end" data-testid="city-animation-end" onAnimationEnd={event=>{if(event.target===event.currentTarget)animation.complete();}}/>
          </g>
          <g className="city-label-layer" aria-hidden="true">{s.districts.map(d=><use key={d.id} href={`#${uid}-label-${d.id}`} data-district-label={d.id} onClick={()=>{onSelect(d.id);nodes.current[d.id]?.focus();}}/>)}</g>
        </svg>
        <div className="city-district-picker" role="group" aria-label="Выбор района">{s.districts.map(d=>{
          const data=values(d.id),chosen=selected.id===d.id;
          return <button key={d.id} aria-pressed={chosen} aria-controls={`${uid}-detail`} onClick={()=>onSelect(d.id)}><span>{chosen?'✓ ':''}{d.name}</span><strong>{data.critical?'! ':''}{display(data.value,view==='change')}</strong><small>{view==='change'&&data.value!==undefined?direction(data.value):data.critical?'Критическое значение':'из 100'}{view==='change'&&data.critical?' · ! Критическое после решений':''}</small></button>;
        })}</div>
        <p className="city-animation-note" id={`${uid}-illustration`}>Здания и их появление — художественная иллюстрация, не прогноз количества объектов, мест или сроков строительства.{result?' Золотые акценты отмечают районы, затронутые решениями этого расчёта.':''}</p>
        <div className="district-scale" id={`${uid}-legend`}>
          {view==='change'?<><p>Изменение показателя, пункты</p><div className="change-legend"><span><i style={{background:changeColor(-1)}}/>− Снижение (&lt; 0)</span><span><i style={{background:changeColor(0)}}/>= Без изменений (0)</span><span><i style={{background:changeColor(1)}}/>+ Рост (&gt; 0)</span></div></>:<><p>Единая шкала 0–100 · чем выше, тем лучше</p><div className="indicator-legend-track" aria-hidden="true" style={{background:`linear-gradient(to right, ${indicatorColor(0)}, ${indicatorColor(50)}, ${indicatorColor(100)})`}}/><div className="indicator-legend-labels"><span>0</span><span>50</span><span>100</span></div></>}
          <p className="district-critical-key">! {view==='change'?'Показатель после решений':'Значение'} строго ниже {threshold} — критическое.</p>
        </div>
        <p className="help district-keyboard" id={`${uid}-keyboard`}>Выберите район нажатием или клавишами Tab и Enter / Пробел. Стрелки перебирают районы в порядке таблицы.</p>
      </div>
      <section className="district-detail-panel" id={`${uid}-detail`} aria-label="Выбранный район">
        <p className="eyebrow">ВЫБРАННЫЙ РАЙОН</p><h3>Район {selected.name}</h3>
        <p className="district-detail-metric">{s.indicator_names[metric]}</p>
        <div className={`district-detail-value ${detail.critical?'danger':''}`}>{display(detail.value,view==='change')}<small>{view==='change'?'п.':'из 100'}</small></div>
        <p className="district-detail-status" role="status">{view==='change'&&detail.value!==undefined?`${direction(detail.value)}. `:''}{detail.critical?`! Критическое значение${view==='change'?' после решений':''}: ниже ${threshold}.`:detail.value===undefined?'Для выбранного режима нет данных.':'Вне критической зоны.'}</p>
        {result&&<dl className="district-result-values"><div><dt>До</dt><dd>{display(detail.before)}</dd></div><div><dt>После</dt><dd>{display(detail.after)}</dd></div><div><dt>Изменение</dt><dd>{display(detail.delta,true)}</dd></div></dl>}
        <div className="district-selected-measures"><h4>{result?'Решения этого расчёта':'Районные решения в плане'}</h4>{localMeasures(selected.id).length?<ul>{localMeasures(selected.id).map(badge)}</ul>:<p className="help">Районные меры не выбраны.</p>}<p className="help">Метки относятся ко всему району и не обозначают места строительства.</p></div>
        {onOpenMeasures&&<><button className="cta" onClick={()=>onOpenMeasures(selected.id,metric)}>Показать подходящие меры <span aria-hidden="true">→</span></button><p className="help">Откроется каталог по этому показателю. Решения добавляются только по вашему выбору.</p></>}
      </section>
    </div>
    <section className="district-city-measures" aria-label="Меры для всего города"><div><h3>Весь город</h3><p className="help">Эти решения действуют на все {s.districts.length} районов.</p></div>{cityMeasures.length?<ul>{cityMeasures.map(badge)}</ul>:<p className="help">Городские меры не выбраны.</p>}</section>
    <p className="district-schematic-note">Формы, размеры и взаимное расположение условны. Схема не показывает официальные границы или адреса работ. Все районы также доступны в таблице ниже.</p>
  </section>;
}
