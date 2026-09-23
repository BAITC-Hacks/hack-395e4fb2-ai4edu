import {fireEvent,render,screen,within} from '@testing-library/react';
import {it,expect,vi} from 'vitest';
import {AutopilotDetails} from '../src/components/AutopilotDetails';
import {Result} from '../src/pages/Result';
import {isAutopilotRun} from '../src/contract';
import {readHistory,saveHistory} from '../src/storage';
import {datasetKey} from '../src/validation';
import type {AutopilotRun,OptimalityProof,SavedRun,Scenario,SimulationResult} from '../src/types';
import scenario from '../src/mock-scenario.json';
import result from './fixtures/golden-result.json';

const now='2026-09-23T12:00:00Z';
const proof:OptimalityProof={proven:true,objective:'score',best_value:57.236735,selected_value:57.236735,gap:0,score_ceiling:57.236735};
const completed=(patch:Partial<AutopilotRun>={}):AutopilotRun=>({id:'proof-run',status:'completed',goal:'Улучшить Score',explanation:'Проверенный результат.',result:result as SimulationResult,review:{approved:true,feedback:'Расчёты проверены.',tradeoffs:[]},events:[{agent:'planner',stage:'search',message:'Проверены допустимые планы.',iteration:1,at:now}],usage:[],estimated_cost_usd:0.02,budget_usd:0.25,iterations:1,evaluated:120,feasible:20,exhaustive:true,message:'План готов.',created_at:now,updated_at:now,...patch});
const saved=(autopilot:AutopilotRun):SavedRun=>({id:'saved-proof',name:'Проверенный план',createdAt:now,scenario:scenario as Scenario,datasetKey:datasetKey(scenario as Scenario),result:result as SimulationResult,explanation:{source:'llm',text:autopilot.explanation},autopilot});

it('states goal-constrained optimality and score ceiling only from a proven exhaustive certificate',()=>{
  render(<AutopilotDetails run={completed({optimality:proof})}/>);
  const section=screen.getByRole('region',{name:'Проверка оптимальности плана'});
  expect(within(section).getByRole('heading',{name:'Оптимальность подтверждена',exact:true})).toBeVisible();
  expect(within(section).getByText(/по формализованной цели координатора и её ограничениям/)).toBeVisible();
  expect(within(section).getByText(/Максимальный Score при заданных ограничениях/)).toBeVisible();
  expect(within(section).getByText(/баллах модели, а не в процентах/)).toBeVisible();
  expect(section.textContent).not.toContain('100%');
});
it('preserves older saved runs without inventing a proof or score ceiling',()=>{
  const old=completed();expect(isAutopilotRun(old)).toBe(true);saveHistory([saved(old)]);expect(readHistory().runs).toHaveLength(1);
  render(<AutopilotDetails run={old}/>);
  expect(screen.queryByRole('region',{name:'Проверка оптимальности плана'})).not.toBeInTheDocument();
  expect(screen.queryByText(/Максимальный Score/)).not.toBeInTheDocument();
});
it('shows an unproven gap without calling the selected plan optimal',()=>{
  render(<AutopilotDetails run={completed({exhaustive:false,optimality:{proven:false,objective:'focus_district',best_value:58,selected_value:55,gap:3}})}/>);
  expect(screen.getByRole('heading',{name:'Оптимальность не подтверждена'})).toBeVisible();
  expect(screen.queryByRole('heading',{name:'Оптимальность подтверждена',exact:true})).not.toBeInTheDocument();
  expect(screen.getByText('Score приоритетного района')).toBeVisible();expect(screen.getByText('Отставание от лучшего, баллы')).toBeVisible();
  expect(screen.queryByText(/Максимальный Score/)).not.toBeInTheDocument();
});
it('shows critical-first values as positive counts with a lower-is-better gap',()=>{
  render(<AutopilotDetails run={completed({optimality:{proven:false,objective:'critical_first',best_value:0,selected_value:2,gap:2}})}/>);
  const section=screen.getByRole('region',{name:'Проверка оптимальности плана'});
  expect(within(section).getByText('Количество критических показателей')).toBeVisible();expect(within(section).getByText(/Меньше — лучше/)).toBeVisible();expect(within(section).getByText('Лишних критических показателей')).toBeVisible();
  expect(section.querySelectorAll('dd')[0].textContent).toBe('2');expect(section.querySelectorAll('dd')[1].textContent).toBe('0');expect(section.querySelectorAll('dd')[2].textContent).toBe('2');
  expect(screen.queryByText(/Максимальный Score/)).not.toBeInTheDocument();
});
it('does not turn a zero primary gap into a proof when the server rejected the score tie-break',()=>{
  render(<AutopilotDetails run={completed({optimality:{proven:false,objective:'critical_first',best_value:0,selected_value:0,gap:0}})}/>);
  expect(screen.getByRole('heading',{name:'Оптимальность не подтверждена'})).toBeVisible();
  expect(screen.getByText(/Нулевая разница сама по себе не подтверждает оптимальность/)).toBeVisible();
});
it('collapses a completed trace and keeps a running trace open',()=>{
  const view=render(<AutopilotDetails run={completed()}/>);
  const summary=screen.getByText('Ход работы агентов · событий: 1');expect(summary.closest('details')).not.toHaveAttribute('open');
  fireEvent.click(summary);expect(summary.closest('details')).toHaveAttribute('open');
  view.rerender(<AutopilotDetails run={completed({status:'running'})}/>);expect(screen.getByText('Проверены допустимые планы.')).toBeVisible();expect(screen.getByText('Ход работы агентов · событий: 1').closest('details')).toHaveAttribute('open');
});
it('presents numeric outcome and point units before agent details',()=>{
  render(<Result run={saved(completed())} onEdit={vi.fn()} explaining={false} explainError="" onExplain={vi.fn()}/>);
  const summary=screen.getByRole('region',{name:'Итог сценария'}), agents=screen.getByRole('region',{name:'Ход работы AI-автопилота'});
  expect(summary.compareDocumentPosition(agents)&Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(screen.getByText('Score — баллы нашей модели города. Это не процент качества жизни.')).toBeVisible();
});
it.each(['master','planner'] as const)('shows reviewer routing to %s without implying completion',target=>{
  render(<AutopilotDetails run={completed({status:'running',review:{approved:false,feedback:'Нужна доработка.',tradeoffs:[],revision_target:target}})}/>);
  expect(screen.getByText(target==='master'?'Уточнение цели поручено координатору.':'Доработка направлена планировщику.')).toBeVisible();
  expect(screen.queryByText('Проверяющий одобрил вариант')).not.toBeInTheDocument();
});
it('validates optional proof and revision fields while retaining old contracts',()=>{
  expect(isAutopilotRun(completed({optimality:proof}))).toBe(true);
  expect(isAutopilotRun(completed({review:{approved:true,feedback:'OK',tradeoffs:[],revision_target:'none'}}))).toBe(true);
  for(const invalid of [{...proof,gap:-1},{...proof,gap:NaN},{...proof,objective:'anything'},{...proof,score_ceiling:Infinity},{...proof,objective:'critical_first'},{...proof,gap:1}])expect(isAutopilotRun(completed({optimality:invalid as OptimalityProof}))).toBe(false);
  expect(isAutopilotRun(completed({exhaustive:false,optimality:proof}))).toBe(false);
  expect(isAutopilotRun({...completed(),review:{approved:true,feedback:'OK',tradeoffs:[],revision_target:'unknown'}})).toBe(false);
});
