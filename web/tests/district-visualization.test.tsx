import {useState} from 'react';
import {render,screen,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {expect,it,vi} from 'vitest';
import {DistrictVisualization} from '../src/components/DistrictVisualization';
import {format} from '../src/components/common';
import type {Decision,Metric,Scenario,SimulationResult} from '../src/types';
import scenario from '../src/mock-scenario.json';
import fixture from './fixtures/golden-result.json';

const s=scenario as Scenario,r=fixture as SimulationResult;
const diagram=()=>screen.getByRole('group',{name:'Схема пяти районов'});
const district=(name:string)=>within(diagram()).getByRole('button',{name:new RegExp(`^${name}`)});
const details=()=>screen.getByRole('region',{name:'Выбранный район'});
const indicator=()=>screen.getByRole('combobox',{name:'Показатель на схеме'});
const mode=(name:string)=>within(screen.getByRole('group',{name:'Режим схемы'})).getByRole('button',{name,exact:true});

function Interactive({decisions=[],onSelect=vi.fn(),onOpenMeasures}:{decisions?:Decision[];onSelect?:(id:string)=>void;onOpenMeasures?:(districtId:string,metric:Metric)=>void}){
  const [selected,setSelected]=useState('esil');
  return <DistrictVisualization scenario={s} selectedId={selected} onSelect={id=>{setSelected(id);onSelect(id);}} decisions={decisions} onOpenMeasures={onOpenMeasures}/>;
}

it('shows exactly the five API districts, synthetic-data disclosure and every API indicator',()=>{
  render(<Interactive/>);
  expect(screen.getByText('Условная визуализация · синтетические данные')).toBeVisible();
  expect(within(diagram()).getAllByRole('button')).toHaveLength(s.districts.length);
  for(const d of s.districts)expect(district(d.name)).toBeInTheDocument();
  expect(within(indicator()).getAllByRole('option')).toHaveLength(s.indicator_order.length);
  for(const metric of s.indicator_order)expect(within(indicator()).getByRole('option',{name:new RegExp(s.indicator_names[metric])})).toHaveValue(metric);
});
it('keeps five distinct schematic shapes if an API district id is renamed or reordered',()=>{
  const changed=structuredClone(s);
  changed.districts[0].id='updated-id';
  changed.districts.reverse();
  render(<DistrictVisualization scenario={changed} selectedId="updated-id" onSelect={vi.fn()}/>);
  const buttons=within(diagram()).getAllByRole('button');
  expect(buttons).toHaveLength(5);
  const shapes=buttons.map(button=>button.querySelector('path')?.getAttribute('d'));
  expect(shapes.every(Boolean)).toBe(true);expect(new Set(shapes).size).toBe(5);
});

it('selects a district with click, Enter and Space, opening synchronized details',async()=>{
  const u=userEvent.setup(),onSelect=vi.fn();
  render(<Interactive onSelect={onSelect}/>);
  await u.click(district('Алматы'));
  expect(district('Алматы')).toHaveAttribute('aria-pressed','true');
  expect(district('Есиль')).toHaveAttribute('aria-pressed','false');
  expect(within(details()).getByRole('heading',{name:'Район Алматы'})).toBeVisible();
  district('Нура').focus();await u.keyboard('{Enter}');
  expect(district('Нура')).toHaveAttribute('aria-pressed','true');
  expect(within(details()).getByRole('heading',{name:'Район Нура'})).toBeVisible();
  district('Байконур').focus();await u.keyboard(' ');
  expect(district('Байконур')).toHaveAttribute('aria-pressed','true');
  expect(within(details()).getByRole('heading',{name:'Район Байконур'})).toBeVisible();
  expect(onSelect.mock.calls.map(([id])=>id)).toEqual(['almaty','nura','baikonur']);
});

it('moves focus and selection through districts in table order with arrows, Home and End',async()=>{
  const u=userEvent.setup();render(<Interactive/>);
  district('Есиль').focus();await u.keyboard('{ArrowRight}');
  expect(district('Алматы')).toHaveFocus();expect(district('Алматы')).toHaveAttribute('aria-pressed','true');
  await u.keyboard('{End}');
  expect(district('Нура')).toHaveFocus();expect(district('Нура')).toHaveAttribute('aria-pressed','true');
  await u.keyboard('{ArrowRight}');
  expect(district('Есиль')).toHaveFocus();
  await u.keyboard('{ArrowLeft}');expect(district('Нура')).toHaveFocus();
  await u.keyboard('{Home}');expect(district('Есиль')).toHaveFocus();
});

it('switches to individual server indicators without constructing category averages',async()=>{
  const u=userEvent.setup();render(<Interactive/>);
  for(const metric of ['T1','C1'] as const){
    await u.selectOptions(indicator(),metric);
    for(const d of s.districts)expect(district(d.name)).toHaveAccessibleName(new RegExp(format(d.indicators[metric])));
    expect(within(details()).getByText(s.indicator_names[metric],{exact:true})).toBeVisible();
  }
});

it('marks strictly below the API threshold, including its equality boundary and changed threshold',async()=>{
  const changed=structuredClone(s);
  changed.districts[0].indicators.T1=40;
  changed.districts[1].indicators.T1=39.99;
  const props={scenario:changed,selectedId:'esil',onSelect:vi.fn()};
  const {rerender}=render(<DistrictVisualization {...props}/>);
  await userEvent.setup().selectOptions(indicator(),'T1');
  expect(district('Есиль')).toHaveAccessibleName(/Вне критической зоны/);
  expect(district('Есиль')).not.toHaveTextContent('!');
  expect(district('Алматы')).toHaveAccessibleName(/Критическое значение/);
  expect(district('Алматы')).toHaveTextContent('!');
  rerender(<DistrictVisualization {...props} scenario={{...changed,scoring:{...changed.scoring,critical_threshold:45}}}/>);
  expect(district('Есиль')).toHaveAccessibleName(/Критическое значение/);
  expect(district('Есиль')).toHaveTextContent('!');
});

it('keeps citywide measures separate and only opens relevant measures on explicit action',async()=>{
  const u=userEvent.setup(),open=vi.fn(),plan=structuredClone(r.decisions);
  render(<Interactive decisions={plan} onOpenMeasures={open}/>);
  const city=screen.getByRole('region',{name:'Меры для всего города'});
  expect(within(city).getByText(s.measures.find(m=>m.id==='M12')!.name,{exact:false})).toBeVisible();
  await u.click(district('Нура'));await u.selectOptions(indicator(),'S1');
  for(const id of ['M7','M8','M10'])expect(within(details()).getByText(new RegExp(id))).toBeVisible();
  expect(within(details()).queryByText(/M5\b/)).not.toBeInTheDocument();
  expect(within(details()).queryByText(/M12\b/)).not.toBeInTheDocument();
  expect(open).not.toHaveBeenCalled();
  await u.click(screen.getByRole('button',{name:'Показать подходящие меры'}));
  expect(open).toHaveBeenCalledExactlyOnceWith('nura','S1');
  expect(plan).toEqual(r.decisions);
});

it('uses server before/after values on an identical absolute color scale',async()=>{
  const u=userEvent.setup(),changed=structuredClone(r),catalog=structuredClone(s);
  catalog.districts[0].indicators.T1=99;
  changed.district_before_after[0].before.T1=45;
  changed.district_before_after[0].after.T1=65;
  changed.district_before_after[1].before.T1=65;
  changed.district_before_after[1].after.T1=45;
  render(<DistrictVisualization scenario={catalog} selectedId="esil" onSelect={vi.fn()} result={changed}/>);
  await u.selectOptions(indicator(),'T1');
  await u.click(mode('До'));
  expect(district('Есиль')).toHaveAccessibleName(/45/);
  const fill=(name:string)=>district(name).querySelector('path')?.getAttribute('fill');
  const low=fill('Есиль'),high=fill('Алматы');
  expect(low).toBeTruthy();expect(high).toBeTruthy();expect(low).not.toBe(high);
  await u.click(mode('После'));
  expect(district('Есиль')).toHaveAccessibleName(/65/);
  expect(fill('Есиль')).toBe(high);expect(fill('Алматы')).toBe(low);
});

it('reads deltas directly from the response and distinguishes increase, zero and decline without color',async()=>{
  const u=userEvent.setup(),changed=structuredClone(r);
  // Deliberately inconsistent with after-before: this UI must display the returned delta.
  changed.district_before_after[0].before.T1=45;
  changed.district_before_after[0].after.T1=45;
  changed.indicator_deltas.esil.T1=7.25;
  changed.indicator_deltas.almaty.T1=-2.5;
  changed.indicator_deltas.saryarka.T1=0;
  render(<DistrictVisualization scenario={s} selectedId="esil" onSelect={vi.fn()} result={changed}/>);
  await u.selectOptions(indicator(),'T1');await u.click(mode('Изменение'));
  expect(district('Есиль')).toHaveAccessibleName(/\+7,25/);
  expect(district('Есиль')).toHaveAccessibleName(/рост|увеличение/i);
  expect(district('Алматы')).toHaveAccessibleName(/-2,5|−2,5/);
  expect(district('Алматы')).toHaveAccessibleName(/снижение/i);
  expect(district('Сарыарка')).toHaveAccessibleName(/без изменений/i);
  expect(district('Есиль')).not.toHaveAccessibleName(/критическ/i);
  expect(district('Алматы')).not.toHaveAccessibleName(/критическ/i);
});

it('marks critical final indicators in change mode regardless of the delta sign',async()=>{
  const u=userEvent.setup(),changed=structuredClone(r);
  changed.district_before_after[0].after.T1=39.99;
  changed.indicator_deltas.esil.T1=7;
  changed.district_before_after[1].after.T1=40;
  changed.indicator_deltas.almaty.T1=0;
  changed.district_before_after[2].after.T1=40;
  changed.indicator_deltas.saryarka.T1=-3;
  render(<DistrictVisualization scenario={s} selectedId="esil" onSelect={vi.fn()} result={changed}/>);
  await u.selectOptions(indicator(),'T1');await u.click(mode('Изменение'));
  expect(district('Есиль')).toHaveAccessibleName(/\+7.*Рост.*Критическое значение после решений/);
  expect(district('Есиль')).toHaveTextContent('!');
  expect(within(details()).getByRole('status')).toHaveTextContent('Критическое значение после решений: ниже 40');
  expect(district('Алматы')).toHaveAccessibleName(/Без изменений/);
  expect(district('Сарыарка')).toHaveAccessibleName(/Снижение/);
  for(const name of ['Алматы','Сарыарка']){
    expect(district(name)).not.toHaveAccessibleName(/Критическое значение/);
    expect(district(name)).not.toHaveTextContent('!');
  }
  expect(screen.getByText(/Показатель после решений строго ниже 40 — критическое/)).toBeVisible();
});

it('uses the calculated plan for result badges even when a different draft is passed',()=>{
  const edited:Decision[]=[{measure_id:'M4',district_id:'nura'},{measure_id:'M14'}];
  render(<DistrictVisualization scenario={s} selectedId="nura" onSelect={vi.fn()} result={r} decisions={edited}/>);
  for(const id of ['M7','M8','M10'])expect(within(details()).getByText(id,{exact:true})).toBeVisible();
  expect(within(details()).queryByText('M4',{exact:true})).not.toBeInTheDocument();
  const city=screen.getByRole('region',{name:'Меры для всего города'});
  expect(within(city).getByText('M12',{exact:true})).toBeVisible();
  expect(within(city).queryByText('M14',{exact:true})).not.toBeInTheDocument();
  expect(district('Нура')).toHaveAccessibleName(/Районных мер: 3/);
});
