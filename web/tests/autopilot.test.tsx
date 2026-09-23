import {fireEvent,render,screen,waitFor} from '@testing-library/react';
import {it,expect,vi} from 'vitest';
import App from '../src/App';
import {isAutopilotRun} from '../src/contract';
import {ACTIVE_AUTOPILOT_KEY} from '../src/useAutopilot';
import {DRAFT_KEY,HISTORY_KEY} from '../src/storage';
import {stableJSON} from '../src/validation';
import type {AutopilotRun,SimulationResult} from '../src/types';
import scenario from '../src/mock-scenario.json';
import result from './fixtures/golden-result.json';

const now='2026-09-23T12:00:00Z';
function run(status:AutopilotRun['status']='running',patch:Partial<AutopilotRun>={}):AutopilotRun {
  return {id:'run-1',status,goal:'Помочь Нуре',explanation:'',events:[{agent:'master',stage:'brief',message:'Цель переведена в ограничения.',iteration:0,at:now}],usage:[],estimated_cost_usd:0.02,budget_usd:0.25,iterations:0,evaluated:10,feasible:4,exhaustive:false,message:'',created_at:now,updated_at:now,...patch};
}
const finished=()=>run('completed',{result:result as SimulationResult,explanation:'План улучшает доступность школ и безопасность Нуры.',review:{approved:true,feedback:'Ограничения соблюдены; расчёты подтверждены.',tradeoffs:['Транспорт не изменился.']},iterations:2,exhaustive:true});
const reply=(body:unknown,status=200)=>({ok:status<400,status,json:async()=>structuredClone(body)});
function backend(next:()=>AutopilotRun) {
  const fetch=vi.fn(async(path:string,options?:RequestInit)=>{
    if(path.endsWith('/scenario'))return reply(scenario);
    if(path==='/api/autopilot')return reply(run('queued'),202);
    if(path.endsWith('/cancel'))return reply(run('cancelled'));
    if(path.startsWith('/api/autopilot/'))return reply(next());
    if(path.endsWith('/simulate')){const decisions=JSON.parse(String(options?.body)).decisions;return reply({valid:false,total_cost:0,remaining_budget:100,decisions,validation_errors:[{code:'decision_count',message:'Five decisions required'}]},422);}
    throw new Error(`Unexpected endpoint ${path}`);
  });vi.stubGlobal('fetch',fetch);return fetch;
}
async function open(){render(<App/>);await screen.findByRole('heading',{name:'Город в ваших руках'});fireEvent.click(screen.getByRole('button',{name:'Поручить план AI'}));}
async function start(){fireEvent.change(screen.getByRole('textbox',{name:'Что нужно улучшить?'}),{target:{value:'Помочь Нуре'}});fireEvent.click(screen.getByRole('button',{name:'Создать план с AI'}));}

it('creates and saves a reviewed result from one goal without selecting decisions or requesting explain',async()=>{
  const fetch=backend(finished);await open();await start();
  expect(await screen.findByRole('heading',{name:'План проверен и готов'})).toBeVisible();
  expect(screen.getByText('56,54')).toBeVisible();expect(screen.getByText('План улучшает доступность школ и безопасность Нуры.')).toBeVisible();
  const history=JSON.parse(localStorage.getItem(HISTORY_KEY)!);expect(history.runs).toHaveLength(1);expect(history.runs[0].autopilot.review.approved).toBe(true);
  expect(JSON.parse(localStorage.getItem(DRAFT_KEY)!).decisions).toEqual(result.decisions);
  expect(fetch.mock.calls.some(([p])=>p.endsWith('/explain')||p.endsWith('/simulate'))).toBe(false);
  expect(JSON.parse(String(fetch.mock.calls.find(([p])=>p==='/api/autopilot')![1]?.body))).toEqual({goal:'Помочь Нуре'});
  expect(localStorage.getItem(ACTIVE_AUTOPILOT_KEY)).toBeNull();
});
it('shows reviewer feedback and a second planner iteration while continuing across navigation',async()=>{
  let current=run('running',{review:{approved:false,feedback:'Проверьте районные компромиссы.',tradeoffs:[]},iterations:1,events:[...run().events,{agent:'reviewer',stage:'retry',message:'Нужна доработка объяснения.',iteration:1,at:now},{agent:'planner',stage:'retry',message:'Учитываю замечания проверяющего.',iteration:2,at:now}]});
  backend(()=>current);await open();await start();expect(await screen.findByText('Нужна доработка объяснения.')).toBeVisible();expect(screen.getByText('Учитываю замечания проверяющего.')).toBeVisible();
  fireEvent.click(screen.getByRole('button',{name:'Обзор города',exact:true}));expect(await screen.findByRole('button',{name:'Посмотреть работу агентов'})).toBeVisible();current=finished();
  expect(await screen.findByText('56,54',{},{timeout:2500})).toBeVisible();
});
it.each(['budget_exceeded','needs_review','infeasible','timeout'] as const)('keeps %s partial results unapproved and out of plan/history',async status=>{
  backend(()=>run(status,{result:result as SimulationResult,message:'Автоматическая проверка не завершена.'}));await open();await start();
  expect(await screen.findByText('Автоматическая проверка не завершена.')).toBeVisible();
  expect(screen.getByText(/промежуточный вариант/)).toBeVisible();expect(localStorage.getItem(HISTORY_KEY)).toBeNull();expect(screen.getByRole('heading',{name:'AI-автопилот'})).toBeVisible();
  await waitFor(()=>expect(screen.getByRole('button',{name:'Создать план с AI'})).toBeEnabled());
});
it('cancels the server run and never auto-applies a completion that races the stop',async()=>{
  let current=run();const fetch=backend(()=>current);await open();await start();await screen.findByText('Цель переведена в ограничения.');
  current=finished();fireEvent.click(screen.getByRole('button',{name:'Остановить запуск'}));
  await waitFor(()=>expect(fetch.mock.calls.some(([p])=>p.endsWith('/cancel'))).toBe(true));
  await waitFor(()=>expect(JSON.parse(localStorage.getItem(HISTORY_KEY)!).runs).toHaveLength(1));
  expect(screen.getByRole('heading',{name:'AI-автопилот'})).toBeVisible();expect(localStorage.getItem(DRAFT_KEY)).toBeNull();
});
it('does not overwrite a draft edited while agents were running',async()=>{
  let current=run();backend(()=>current);await open();await start();await screen.findByText('Цель переведена в ограничения.');
  fireEvent.click(screen.getByRole('button',{name:'Ваши решения',exact:true}));fireEvent.change(await screen.findByRole('textbox',{name:'Название сценария'}),{target:{value:'Мой новый ручной план'}});current=finished();
  await waitFor(()=>expect(JSON.parse(localStorage.getItem(HISTORY_KEY)!).runs).toHaveLength(1),{timeout:2500});
  expect(screen.getByRole('textbox',{name:'Название сценария'})).toHaveValue('Мой новый ручной план');expect(JSON.parse(localStorage.getItem(DRAFT_KEY)!).decisions).toEqual([]);
});
it('resumes an active ID after reload without submitting a second paid run',async()=>{
  localStorage.setItem(ACTIVE_AUTOPILOT_KEY,JSON.stringify({id:'run-1',scenario,draftKey:stableJSON({name:'Мой план развития',decisions:[]}),suppressApply:false}));
  const fetch=backend(finished);render(<App/>);expect(await screen.findByText('56,54')).toBeVisible();expect(fetch.mock.calls.some(([p])=>p==='/api/autopilot')).toBe(false);
});
it('rejects result application when the scenario changes while agents work',async()=>{
  let scenarios=0;const fetch=backend(finished);const implementation=fetch.getMockImplementation()!;fetch.mockImplementation(async(path,opts)=>path.endsWith('/scenario')&&++scenarios===3?reply({...scenario,budget:99}):implementation(path,opts));
  await open();await start();expect(await screen.findByText(/Каталог или правила изменились во время запуска/)).toBeVisible();expect(localStorage.getItem(HISTORY_KEY)).toBeNull();
});
it('does not start a paid run in mock mode',async()=>{
  const fetch=backend(run);await open();fireEvent.click(screen.getByRole('button',{name:/Серверный расчёт/}));expect(await screen.findByText(/В mock-режиме автопилот отключён/)).toBeVisible();expect(screen.getByRole('button',{name:'Создать план с AI'})).toBeDisabled();expect(fetch.mock.calls.some(([p])=>p==='/api/autopilot')).toBe(false);
});
it('validates complete reviews and simulation data rather than trusting status alone',()=>{
  expect(isAutopilotRun(finished())).toBe(true);expect(isAutopilotRun({...finished(),result:undefined})).toBe(false);expect(isAutopilotRun({...finished(),review:{approved:false,feedback:'No',tradeoffs:[]}})).toBe(false);expect(isAutopilotRun({...finished(),result:{valid:true}})).toBe(false);
  expect(isAutopilotRun(run('running',{brief:{goal:{objective:'max_score',max_critical:null},summary:'Цель',clarification:''}}))).toBe(true);
});
it('shows a clarification instead of manufacturing a successful plan',async()=>{
  backend(()=>run('needs_clarification',{brief:{goal:{objective:'',protect_districts:null},summary:'',clarification:'Какой район должен быть в приоритете?'}}));await open();await start();
  expect(await screen.findByText('Какой район должен быть в приоритете?')).toBeVisible();expect(localStorage.getItem(HISTORY_KEY)).toBeNull();await waitFor(()=>expect(screen.getByRole('button',{name:'Создать план с AI'})).toBeEnabled());
});
it('shows catalog failure and allows retry when autopilot is opened directly',async()=>{
  const fetch=vi.fn().mockRejectedValue(new TypeError('offline'));vi.stubGlobal('fetch',fetch);render(<App/>);fireEvent.click(screen.getByRole('button',{name:'AI-автопилот',exact:true}));
  expect(await screen.findByRole('alert')).toHaveTextContent('Каталог недоступен');expect(screen.getByRole('button',{name:'Создать план с AI'})).toBeDisabled();
  fetch.mockResolvedValue(reply(scenario));fireEvent.click(screen.getByRole('button',{name:'Повторить попытку'}));await waitFor(()=>expect(screen.getByRole('button',{name:'Создать план с AI'})).toBeEnabled());
});
