import {act,fireEvent,render,screen,waitFor,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach,it,expect,vi} from 'vitest';
import {StrictMode} from 'react';
import App from '../src/App';
import {Decisions} from '../src/pages/Decisions';
import {History} from '../src/pages/History';
import scenario from '../src/mock-scenario.json';
import result from './fixtures/golden-result.json';
import explanation from './fixtures/golden-explain.json';
import improved from './fixtures/golden-improve.json';
import {assessPlan} from '../src/api';
import {DRAFT_KEY,HISTORY_KEY,saveDraft} from '../src/storage';
import {datasetKey,validatePlan} from '../src/validation';
import type {Decision,SavedRun,Scenario,SimulationResult} from '../src/types';

const s=scenario as Scenario;
// Keep page-flow tests focused on explicit runs. The assessment hook/API have separate tests.
vi.mock('../src/api',async(importOriginal)=>{
  const actual=await importOriginal<typeof import('../src/api')>();
  return {...actual,assessPlan:vi.fn(async(decisions:Decision[])=>{
    const checked=validatePlan(s,decisions);
    return {valid:checked.valid,total_cost:checked.cost,remaining_budget:checked.remaining,decisions,validation_errors:checked.errors};
  })};
});
const reply=(body:unknown,status=200)=>({ok:status<400,status,json:async()=>structuredClone(body)});
const user=()=>userEvent.setup();
let fetchMock:ReturnType<typeof vi.fn>;
beforeEach(()=>{
  fetchMock=vi.fn(async(path:string)=>reply(path.endsWith('/scenario')?scenario:path.endsWith('/simulate')?result:explanation));
  vi.stubGlobal('fetch',fetchMock);
});
async function openDecisions(){render(<App/>);await screen.findByRole('heading',{name:'Город в ваших руках'});await user().click(screen.getByRole('button',{name:'Ваши решения',exact:true}));await screen.findByRole('heading',{name:'Ваши решения',exact:true});const draft=JSON.parse(localStorage.getItem(DRAFT_KEY)||'null');if(draft&&validatePlan(s,draft.decisions).valid)await waitFor(()=>expect(runButton()).toHaveAttribute('aria-disabled','false'));}
const card=(id:string)=>document.getElementById(`title-${id}`)!.closest('article')!;
async function add(id:string,district?:string){const u=user();if(district)await u.selectOptions(within(card(id)).getByRole('combobox'),district);await u.click(within(card(id)).getByRole('button',{name:'Добавить решение'}));}
async function goldenPlan(){for(const [id,d] of [['M7','nura'],['M8','nura'],['M10','nura'],['M12',undefined],['M5','saryarka']] as const)await add(id,d);if(!screen.queryByText('Учебный режим (mock) — без расчёта.'))await waitFor(()=>expect(runButton()).toHaveAttribute('aria-disabled','false'));}
const runButton=()=>screen.getByRole('button',{name:'Рассчитать результат'});
it('runs opening → five selections → real fixture result → fallback → editing; invalidates old result',async()=>{
  await openDecisions();expect(screen.getAllByRole('article')).toHaveLength(14);expect(runButton()).toHaveAttribute('aria-disabled','true');
  await goldenPlan();expect(runButton()).toHaveAttribute('aria-disabled','false');await user().click(runButton());
  expect(await screen.findByText('56,54')).toBeVisible();expect(await screen.findByText('Резервное объяснение · без ИИ')).toBeVisible();
  expect(screen.getByText('Сработавшие синергии')).toBeVisible();
  expect(JSON.parse(localStorage.getItem(HISTORY_KEY)!).runs).toHaveLength(1);
  expect(fetchMock.mock.calls.filter(c=>c[0].endsWith('/simulate'))).toHaveLength(1);
  await user().click(screen.getByRole('button',{name:'Изменить решения'}));
  expect(screen.getByRole('combobox',{name:'Район выбранной меры M7'})).toHaveValue('nura');
  await user().click(screen.getByRole('button',{name:'Удалить M7',exact:true}));
  await user().click(screen.getByRole('button',{name:'Результат',exact:true}));
  expect(await screen.findByRole('heading',{name:'Рассчитайте текущий план'})).toBeVisible();expect(screen.queryByText('56,54')).not.toBeInTheDocument();
});
it('persists across navigation and remounts, preserves city payload shape',async()=>{
  await openDecisions();await add('M12');await add('M4','esil');await user().click(screen.getByRole('button',{name:'Обзор города',exact:true}));await user().click(screen.getByRole('button',{name:'Ваши решения',exact:true}));expect(screen.getByRole('combobox',{name:'Район выбранной меры M4'})).toHaveValue('esil');
  expect(JSON.parse(localStorage.getItem(DRAFT_KEY)!).decisions).toEqual([{measure_id:'M12'},{measure_id:'M4',district_id:'esil'}]);
});
it('recovers a draft after remount and blocks stale catalog entries',async()=>{
  saveDraft({name:'Черновик',decisions:[{measure_id:'M99'},{measure_id:'M4',district_id:'deleted'}]});await openDecisions();
  expect(screen.getByText('Недоступная мера M99')).toBeVisible();expect(within(screen.getByRole('complementary',{name:'Выбранный план'})).getByText(/район больше не доступен/)).toBeVisible();expect(runButton()).toHaveAttribute('aria-disabled','true');
});
it('does not overwrite damaged localStorage in StrictMode and remains usable',async()=>{
  localStorage.setItem(DRAFT_KEY,'broken');render(<StrictMode><App/></StrictMode>);await screen.findByRole('heading',{name:'Город в ваших руках'});expect(screen.getByText(/Черновик повреждён/)).toBeVisible();expect(localStorage.getItem(DRAFT_KEY)).toBe('broken');
});
it('blocks same-district edits and keeps original district, then permits a valid edit',async()=>{
  await openDecisions();await add('M4','esil');await add('M7','nura');
  const select=screen.getByRole('combobox',{name:'Район выбранной меры M7'});
  await user().selectOptions(select,'esil');expect(screen.getByRole('alert')).toHaveTextContent('M4 и M7 несовместимы');expect(select).toHaveValue('nura');
  await user().selectOptions(select,'almaty');expect(select).toHaveValue('almaty');
});
it('replaces a selected measure without needing a sixth slot',async()=>{
  await openDecisions();await goldenPlan();await user().click(screen.getByRole('button',{name:'Заменить M5',exact:true}));await user().click(within(card('M14')).getByRole('button',{name:'Заменить этим решением'}));expect(screen.queryByRole('button',{name:'Удалить M5',exact:true})).not.toBeInTheDocument();expect(screen.getByRole('button',{name:'Удалить M14',exact:true})).toBeVisible();await waitFor(()=>expect(runButton()).toHaveAttribute('aria-disabled','false'));
});
it('filters all five directions and shows duplicate reasons next to the action',async()=>{
  await openDecisions();await add('M12');expect(within(card('M12')).getByText(/уже в плане. Можно изменить/)).toBeVisible();
  for(const c of ['Транспорт','Экология','Соцсфера','Безопасность','Сервисы']){await user().click(screen.getByRole('button',{name:c,exact:true}));expect(screen.getAllByRole('article').length).toBe(s.measures.filter(m=>s.category_names[m.category]===c).length);}
});
it('retains selection when simulate fails and allows retry',async()=>{
  saveDraft({name:'План',decisions:result.decisions});await openDecisions();fetchMock.mockImplementation(async(path:string)=>path.endsWith('/simulate')?reply({validation_errors:[{code:'budget_exceeded',message:'Budget exceeded'}]},422):reply(scenario));
  await user().click(runButton());expect(await screen.findByRole('alert')).toHaveTextContent('Стоимость плана превышает бюджет');expect(screen.getByRole('button',{name:'Удалить M7',exact:true})).toBeVisible();expect(runButton()).toHaveAttribute('aria-disabled','false');
});
it('keeps result when explanation fails, and retries explanation separately',async()=>{
  saveDraft({name:'План',decisions:result.decisions});await openDecisions();fetchMock.mockImplementation(async(path:string)=>{if(path.endsWith('/explain'))throw new TypeError('offline');return reply(path.endsWith('/scenario')?scenario:result);});
  await user().click(runButton());expect(await screen.findByText('56,54')).toBeVisible();expect(await screen.findByRole('alert')).toHaveTextContent('Рассчитанные показатели сохранены');
  fetchMock.mockImplementation(async()=>reply(explanation));await user().click(screen.getByRole('button',{name:'Повторить попытку'}));expect(await screen.findByText('Резервное объяснение · без ИИ')).toBeVisible();
});
it('does not attach an explanation for a different numerical result',async()=>{
  saveDraft({name:'План',decisions:result.decisions});await openDecisions();fetchMock.mockImplementation(async(path:string)=>reply(path.endsWith('/scenario')?scenario:path.endsWith('/simulate')?result:{...explanation,result:{...result,final_score:1}}));await user().click(runButton());expect(await screen.findByRole('alert')).toHaveTextContent('отличается');expect(screen.getByText('56,54')).toBeVisible();expect(screen.queryByText('Резервное объяснение · без ИИ')).not.toBeInTheDocument();
});
it('ignores repeated submission while request is in flight',async()=>{
  saveDraft({name:'План',decisions:result.decisions});await openDecisions();let resolve!:(x:unknown)=>void;const pending=new Promise(r=>{resolve=r;});fetchMock.mockImplementation((path:string)=>path.endsWith('/simulate')?pending:Promise.resolve(reply(path.endsWith('/scenario')?scenario:explanation)));
  const button=runButton();fireEvent.click(button);fireEvent.click(button);await waitFor(()=>expect(fetchMock.mock.calls.filter(c=>c[0].endsWith('/simulate'))).toHaveLength(1));expect(screen.getByRole('button',{name:'Считаем результат…'})).toHaveAttribute('aria-disabled','true');await act(async()=>resolve(reply(result)));expect(await screen.findByText('56,54')).toBeVisible();
});
it('marks successful AI text as AI and renders it as text, not HTML',async()=>{
  saveDraft({name:'План',decisions:result.decisions});await openDecisions();fetchMock.mockImplementation(async(path:string)=>reply(path.endsWith('/scenario')?scenario:path.endsWith('/simulate')?result:{result,explanation:{source:'llm',text:'Рекомендации. <img src=x onerror=alert(1)> Учитывайте лаг.'}}));await user().click(runButton());expect(await screen.findByText('Объяснение ИИ')).toBeVisible();expect(screen.getByText(/<img src=x/)).toBeVisible();expect(document.querySelector('.explanation img')).toBeNull();
});
it('requires review if the catalog changes immediately before simulation',async()=>{
  saveDraft({name:'План',decisions:result.decisions});await openDecisions();fetchMock.mockResolvedValue(reply({...scenario,budget:90}));await user().click(runButton());expect(await screen.findByRole('alert')).toHaveTextContent('Каталог или правила изменились');expect(runButton()).toHaveAttribute('aria-disabled','true');expect(fetchMock.mock.calls.filter(c=>c[0].endsWith('/simulate'))).toHaveLength(0);
});
it('offers explicit mock after backend failure and never fakes arbitrary results',async()=>{
  fetchMock.mockRejectedValue(new TypeError('offline'));render(<App/>);expect(await screen.findByRole('heading',{name:'Каталог пока недоступен'})).toBeVisible();await user().click(screen.getByRole('button',{name:'Повторить попытку'}));expect(await screen.findByRole('alert')).toHaveTextContent('Не удалось подключиться');await user().click(screen.getByRole('button',{name:'Открыть учебный режим (mock)'}));await screen.findByRole('heading',{name:'Город в ваших руках'});expect(screen.getByText('Учебный режим (mock) — без расчёта.')).toBeVisible();await user().click(screen.getByRole('button',{name:'Ваши решения',exact:true}));await goldenPlan();await user().click(runButton());expect(fetchMock.mock.calls.filter(c=>c[0].endsWith('/simulate'))).toHaveLength(0);expect(runButton()).toHaveAttribute('aria-disabled','true');
});
it('supports keyboard navigation, labeled selects and keyboard activation',async()=>{
  render(<App/>);await screen.findByRole('heading',{name:'Город в ваших руках'});const u=user();await u.tab();expect(screen.getByRole('link',{name:'К содержимому'})).toHaveFocus();
  screen.getByRole('button',{name:'Ваши решения',exact:true}).focus();await u.keyboard('{Enter}');await screen.findByRole('heading',{name:'Ваши решения',exact:true});
  const select=within(card('M4')).getByRole('combobox',{name:'Район мероприятия'});select.focus();expect(select).toHaveFocus();await u.selectOptions(select,'esil');await u.tab();expect(within(card('M4')).getByRole('button',{name:'Добавить решение'})).toHaveFocus();await u.keyboard('{Enter}');expect(screen.getByRole('combobox',{name:'Район выбранной меры M4'})).toHaveValue('esil');
});
it('preserves read-only history, compares and warns about dataset differences',async()=>{
  const a:SavedRun={id:'a',name:'План A',createdAt:'2026-09-23T00:00:00Z',scenario:s,datasetKey:datasetKey(s),result:result as SimulationResult};const b={...a,id:'b',name:'План B',scenario:{...s,budget:90},datasetKey:datasetKey({...s,budget:90})};
  render(<History runs={[a,b]} onOpen={vi.fn()} onDelete={vi.fn()} onUndo={vi.fn()} canUndo={false} onStart={vi.fn()}/>);for(const c of screen.getAllByRole('checkbox'))await user().click(c);expect(screen.getByText(/нельзя считать полностью сопоставимыми/)).toBeVisible();expect(screen.getByRole('region',{name:'Сравнение критических показателей'})).toBeVisible();
});
it('deletes saved history and supports undo',async()=>{
  saveDraft({name:'План',decisions:result.decisions});await openDecisions();await user().click(runButton());await screen.findByText('56,54');await user().click(screen.getByRole('button',{name:'История',exact:true}));await user().click(screen.getByRole('button',{name:'Удалить сценарий План'}));expect(screen.getByRole('heading',{name:'Здесь будут ваши расчёты'})).toBeVisible();await user().click(screen.getByRole('button',{name:'Отменить удаление'}));expect(screen.getByRole('heading',{name:'План',exact:true})).toBeVisible();
});
it('explicitly applies an improvement, invalidates the old result and revalidates both apply and undo',async()=>{
  saveDraft({name:'Исходный план',decisions:result.decisions});await openDecisions();
  await user().click(runButton());await screen.findByText('56,54');
  await user().click(screen.getByRole('button',{name:'Изменить решения'}));
  await waitFor(()=>expect(runButton()).toHaveAttribute('aria-disabled','false'));
  fetchMock.mockImplementation(async(path:string)=>reply(path.endsWith('/recommend')?improved:path.endsWith('/scenario')?scenario:path.endsWith('/simulate')?result:explanation));
  await user().click(screen.getByRole('button',{name:'Улучшить мой план'}));await screen.findByRole('article',{name:'Вариант 1'});
  expect(JSON.parse(localStorage.getItem(DRAFT_KEY)!).decisions).toEqual(result.decisions);
  expect(screen.getByRole('button',{name:'Удалить M5',exact:true})).toBeVisible();
  await user().click(screen.getByRole('button',{name:'Применить вариант 1'}));
  expect(JSON.parse(localStorage.getItem(DRAFT_KEY)!).decisions).toEqual(improved.improvements[0].decisions);
  expect(screen.queryByRole('button',{name:'Удалить M5',exact:true})).not.toBeInTheDocument();
  expect(screen.getByRole('combobox',{name:'Район выбранной меры M3'})).toHaveValue('nura');
  await waitFor(()=>expect(assessPlan).toHaveBeenLastCalledWith(improved.improvements[0].decisions,expect.any(AbortSignal)));
  await user().click(screen.getByRole('button',{name:'Результат',exact:true}));
  expect(screen.getByRole('heading',{name:'Рассчитайте текущий план'})).toBeVisible();
  expect(JSON.parse(localStorage.getItem(HISTORY_KEY)!).runs).toHaveLength(1);
  await user().click(screen.getByRole('button',{name:'Ваши решения',exact:true}));
  await user().click(screen.getByRole('button',{name:'Отменить применение рекомендации'}));
  expect(JSON.parse(localStorage.getItem(DRAFT_KEY)!).decisions).toEqual(result.decisions);
  expect(screen.getByRole('combobox',{name:'Район выбранной меры M5'})).toHaveValue('saryarka');
  await waitFor(()=>expect(assessPlan).toHaveBeenLastCalledWith(result.decisions,expect.any(AbortSignal)));
  await waitFor(()=>expect(runButton()).toHaveAttribute('aria-disabled','false'));
  expect(screen.queryByRole('button',{name:'Отменить применение рекомендации'})).not.toBeInTheDocument();
});
it('clears undo after a later manual edit so it cannot silently discard new work',async()=>{
  saveDraft({name:'План',decisions:result.decisions});await openDecisions();
  fetchMock.mockResolvedValue(reply(improved));await user().click(screen.getByRole('button',{name:'Улучшить мой план'}));
  await screen.findByRole('article',{name:'Вариант 2'});await user().click(screen.getByRole('button',{name:'Применить вариант 2'}));
  expect(screen.getByRole('button',{name:'Отменить применение рекомендации'})).toBeVisible();
  await user().click(screen.getByRole('button',{name:'Удалить M14',exact:true}));
  expect(screen.queryByRole('button',{name:'Отменить применение рекомендации'})).not.toBeInTheDocument();
  expect(JSON.parse(localStorage.getItem(DRAFT_KEY)!).decisions).toHaveLength(4);
});
