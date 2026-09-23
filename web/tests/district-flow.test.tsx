import {render,screen,within,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach,expect,it,vi} from 'vitest';
import App from '../src/App';
import type {Decision,Scenario,SimulationResult} from '../src/types';
import {DRAFT_KEY,saveDraft,saveHistory} from '../src/storage';
import {datasetKey,validatePlan} from '../src/validation';
import scenario from '../src/mock-scenario.json';
import result from './fixtures/golden-result.json';
import explanation from './fixtures/golden-explain.json';

const s=scenario as Scenario;
vi.mock('../src/api',async(importOriginal)=>{
  const actual=await importOriginal<typeof import('../src/api')>();
  return {...actual,assessPlan:vi.fn(async(decisions:Decision[])=>{
    const checked=validatePlan(s,decisions);
    return {valid:checked.valid,total_cost:checked.cost,remaining_budget:checked.remaining,decisions,validation_errors:checked.errors};
  })};
});
beforeEach(()=>{
  vi.stubGlobal('fetch',vi.fn(async(path:string)=>({ok:true,status:200,json:async()=>structuredClone(path.endsWith('/scenario')?scenario:path.endsWith('/simulate')?result:explanation)})));
});
const diagram=()=>screen.getByRole('group',{name:'Схема пяти районов'});
const district=(name:string)=>within(diagram()).getByRole('button',{name:new RegExp(`^${name}`)});
const selectTableDistrict=(name:string)=>screen.getByRole('button',{name:`Выбрать район ${name} в таблице`});
const draft=()=>JSON.parse(localStorage.getItem(DRAFT_KEY)!).decisions;

it('synchronizes district selection in both directions between overview SVG and existing table',async()=>{
  const u=userEvent.setup();render(<App/>);
  await screen.findByRole('heading',{name:'Город в ваших руках'});
  await u.click(district('Алматы'));
  expect(selectTableDistrict('Алматы')).toHaveAttribute('aria-pressed','true');
  expect(within(screen.getByRole('region',{name:'Выбранный район'})).getByRole('heading',{name:'Район Алматы'})).toBeVisible();
  await u.click(selectTableDistrict('Байконур'));
  expect(district('Байконур')).toHaveAttribute('aria-pressed','true');
  expect(district('Алматы')).toHaveAttribute('aria-pressed','false');
  expect(selectTableDistrict('Алматы')).toHaveAttribute('aria-pressed','false');
  expect(within(screen.getByRole('region',{name:'Выбранный район'})).getByRole('heading',{name:'Район Байконур'})).toBeVisible();
});

it('opens only relevant API measures with the chosen district while preserving decisions until explicit selection',async()=>{
  saveDraft({name:'План до перехода',decisions:result.decisions});
  const u=userEvent.setup();render(<App/>);
  await screen.findByRole('heading',{name:'Город в ваших руках'});
  await u.click(district('Алматы'));
  await u.selectOptions(screen.getByRole('combobox',{name:'Показатель на схеме'}),'T1');
  const original=structuredClone(draft());
  await u.click(screen.getByRole('button',{name:'Показать подходящие меры'}));
  await screen.findByRole('heading',{name:'Ваши решения',exact:true});
  expect(screen.getByRole('region',{name:'Меры по выбранному показателю'})).toHaveTextContent('Алматы');
  const relevant=s.measures.filter(m=>m.effects.T1!==undefined&&m.effects.T1!==0);
  const cards=screen.getAllByRole('article');
  expect(cards).toHaveLength(relevant.length);
  for(const measure of relevant){
    const card=document.getElementById(`title-${measure.id}`)!.closest('article')!;
    expect(card).toBeVisible();
    if(measure.scope==='District')expect(within(card).getByRole('combobox',{name:'Район мероприятия'})).toHaveValue('almaty');
    else expect(within(card).queryByRole('combobox')).not.toBeInTheDocument();
  }
  expect(draft()).toEqual(original);
  await u.click(screen.getByRole('button',{name:'Показать все мероприятия'}));
  expect(screen.getAllByRole('article')).toHaveLength(s.measures.length);
  expect(draft()).toEqual(original);
});

it('preserves the current draft when opening relevant measures from an archived result',async()=>{
  const current=[{measure_id:'M4',district_id:'esil'}];
  saveDraft({name:'Текущий черновик',decisions:current});
  saveHistory([{id:'old-run',name:'Ранее рассчитанный план',createdAt:'2026-09-23T12:00:00Z',scenario:s,datasetKey:datasetKey(s),result:result as SimulationResult}]);
  const u=userEvent.setup();render(<App/>);
  await screen.findByRole('heading',{name:'Город в ваших руках'});
  await u.click(screen.getByRole('button',{name:'История',exact:true}));
  await u.click(screen.getByRole('button',{name:'Открыть результат'}));
  await u.click(district('Нура'));
  await u.selectOptions(screen.getByRole('combobox',{name:'Показатель на схеме'}),'S1');
  // Result badges still belong to the archive, not the unrelated draft.
  expect(within(screen.getByRole('region',{name:'Выбранный район'})).getByText('M7')).toBeVisible();
  await u.click(screen.getByRole('button',{name:'Показать подходящие меры'}));
  expect(screen.getByRole('region',{name:'Меры по выбранному показателю'})).toHaveTextContent('Нура');
  expect(draft()).toEqual(current);
  expect(JSON.parse(localStorage.getItem(DRAFT_KEY)!).name).toBe('Текущий черновик');
  expect(screen.getByRole('button',{name:'Удалить M4',exact:true})).toBeVisible();
  expect(screen.queryByRole('button',{name:'Удалить M7',exact:true})).not.toBeInTheDocument();
});

it('synchronizes result details with the SVG and removes the calculated map after a plan edit',async()=>{
  saveDraft({name:'План для карты',decisions:result.decisions});
  const u=userEvent.setup();render(<App/>);
  await screen.findByRole('heading',{name:'Город в ваших руках'});
  await u.click(screen.getByRole('button',{name:'Ваши решения',exact:true}));
  const run=screen.getByRole('button',{name:'Рассчитать результат'});
  await waitFor(()=>expect(run).toHaveAttribute('aria-disabled','false'));
  await u.click(run);await screen.findByRole('region',{name:'Итог сценария'});
  await u.click(district('Есиль'));
  expect(screen.getByLabelText('Открыть показатели района Есиль').closest('details')).toHaveAttribute('open');
  await u.click(screen.getByLabelText('Открыть показатели района Сарыарка'));
  expect(district('Сарыарка')).toHaveAttribute('aria-pressed','true');
  expect(screen.getByLabelText('Открыть показатели района Сарыарка').closest('details')).toHaveAttribute('open');
  expect(screen.getByLabelText('Открыть показатели района Есиль').closest('details')).not.toHaveAttribute('open');
  await u.click(screen.getByRole('button',{name:'Изменить решения'}));
  expect(draft()).toEqual(result.decisions);
  await u.click(screen.getByRole('button',{name:'Удалить M7',exact:true}));
  await u.click(screen.getByRole('button',{name:'Результат',exact:true}));
  expect(screen.getByRole('heading',{name:'Рассчитайте текущий план'})).toBeVisible();
  expect(screen.queryByRole('group',{name:'Схема пяти районов'})).not.toBeInTheDocument();
  expect(screen.queryByRole('group',{name:'Режим схемы'})).not.toBeInTheDocument();
  expect(screen.queryByRole('region',{name:'Итог сценария'})).not.toBeInTheDocument();
});
