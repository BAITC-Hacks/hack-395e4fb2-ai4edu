import {act,fireEvent,render,screen,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {expect,it,vi} from 'vitest';
import {ImprovePlan} from '../src/components/ImprovePlan';
import type {Scenario} from '../src/types';
import scenario from '../src/mock-scenario.json';
import result from './fixtures/golden-result.json';
import improved from './fixtures/golden-improve.json';
const response=(body:unknown,status=200)=>({ok:status<400,status,json:async()=>body});
const props=()=>({scenario:scenario as Scenario,plan:result.decisions,assessment:result,ready:true,busy:false,mock:false,onApply:vi.fn()});
const search=()=>screen.getByRole('button',{name:'Улучшить мой план'});

it('requests the submitted plan once and never applies an improvement without a click',async()=>{
  let resolve!:(value:unknown)=>void;
  const fetch=vi.fn(()=>new Promise(r=>{resolve=r;}));vi.stubGlobal('fetch',fetch);
  const p=props();render(<ImprovePlan {...p}/>);
  fireEvent.click(search());fireEvent.click(search());expect(fetch).toHaveBeenCalledTimes(1);
  expect(search()).toBeDisabled();expect(screen.getByText('Ищем улучшения вашего плана…')).toBeVisible();
  expect(fetch.mock.calls[0]).toEqual(['/api/recommend',expect.objectContaining({method:'POST',body:JSON.stringify({mode:'improve',decisions:result.decisions})})]);
  await act(async()=>resolve(response(improved)));
  const first=screen.getByRole('article',{name:'Вариант 1'});
  expect(within(first).getByText(/M5 · Перевод/)).toBeVisible();
  expect(within(first).getByText(/M3 · Линия/)).toBeVisible();
  expect(within(first).getByText('57,20556')).toBeVisible();
  expect(within(first).getByText('+0,66249')).toBeVisible();
  expect(p.onApply).not.toHaveBeenCalled();
  fireEvent.click(within(first).getByRole('button',{name:'Применить вариант 1'}));
  expect(p.onApply).toHaveBeenCalledWith(improved.improvements[0].decisions);
});
it.each([{ready:false},{mock:true},{busy:true}])('does not request recommendations when unavailable: %j',overrides=>{
  const fetch=vi.fn();vi.stubGlobal('fetch',fetch);render(<ImprovePlan {...props()} {...overrides}/>);
  expect(search()).toBeDisabled();fireEvent.click(search());expect(fetch).not.toHaveBeenCalled();
});
it('explains empty results without claiming a global optimum',async()=>{
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue(response({current_score:result.final_score,improvements:[]})));
  const p=props();render(<ImprovePlan {...p}/>);fireEvent.click(search());
  expect(await screen.findByText(/Улучшений одной заменой не найдено/)).toHaveTextContent('не означает, что план лучший');
  expect(p.onApply).not.toHaveBeenCalled();expect(screen.queryByRole('article')).not.toBeInTheDocument();
});
it('handles validation errors in Russian and lets the user retry without modifying the plan',async()=>{
  const fetch=vi.fn().mockResolvedValueOnce(response({valid:false,validation_errors:[{code:'budget_exceeded',message:'Budget exceeded'}]},422)).mockResolvedValueOnce(response(improved));vi.stubGlobal('fetch',fetch);
  const p=props();render(<ImprovePlan {...p}/>);fireEvent.click(search());
  expect(await screen.findByRole('alert')).toHaveTextContent('Стоимость плана превышает бюджет');
  expect(p.onApply).not.toHaveBeenCalled();fireEvent.click(screen.getByRole('button',{name:'Повторить попытку'}));
  expect(await screen.findByRole('article',{name:'Вариант 1'})).toBeVisible();
});
it('does not poll improve on 503 and reports a server error',async()=>{
  const fetch=vi.fn().mockResolvedValue(response({},503));vi.stubGlobal('fetch',fetch);
  render(<ImprovePlan {...props()}/>);fireEvent.click(search());
  expect(await screen.findByRole('alert')).toHaveTextContent('Сервер расчёта временно недоступен');expect(fetch).toHaveBeenCalledTimes(1);
});
it('discards displayed suggestions immediately when the district changes',async()=>{
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue(response(improved)));
  const p=props();const view=render(<ImprovePlan {...p}/>);fireEvent.click(search());await screen.findByRole('article',{name:'Вариант 1'});
  view.rerender(<ImprovePlan {...p} plan={p.plan.map(d=>d.measure_id==='M5'?{...d,district_id:'almaty'}:d)}/>);
  expect(screen.queryByRole('article')).not.toBeInTheDocument();expect(p.onApply).not.toHaveBeenCalled();
});
it('ignores a late response across rapid A → B → A edits, even if the transport ignores abort',async()=>{
  let finishOld!:(value:unknown)=>void;
  const fetch=vi.fn().mockImplementationOnce(()=>new Promise(r=>{finishOld=r;})).mockResolvedValue(response({current_score:result.final_score,improvements:[]}));vi.stubGlobal('fetch',fetch);
  const p=props();const view=render(<ImprovePlan {...p}/>);fireEvent.click(search());
  const signal=fetch.mock.calls[0][1].signal as AbortSignal;
  view.rerender(<ImprovePlan {...p} plan={p.plan.slice(1)} ready={false}/>);
  expect(signal.aborted).toBe(true);
  view.rerender(<ImprovePlan {...p}/>);fireEvent.click(search());await screen.findByText(/Улучшений одной заменой не найдено/);
  await act(async()=>finishOld(response(improved)));
  expect(screen.queryByRole('article')).not.toBeInTheDocument();expect(screen.getByText(/Улучшений одной заменой не найдено/)).toBeVisible();expect(p.onApply).not.toHaveBeenCalled();
});
it('aborts on navigation away and never displays the old response after returning',async()=>{
  let finish!:(value:unknown)=>void;
  const fetch=vi.fn(()=>new Promise(r=>{finish=r;}));vi.stubGlobal('fetch',fetch);
  const p=props();const view=render(<ImprovePlan {...p}/>);fireEvent.click(search());view.unmount();
  expect((fetch.mock.calls[0] as unknown as [string,RequestInit])[1].signal?.aborted).toBe(true);
  render(<ImprovePlan {...p}/>);await act(async()=>finish(response(improved)));
  expect(screen.queryByRole('article')).not.toBeInTheDocument();expect(search()).not.toBeDisabled();expect(p.onApply).not.toHaveBeenCalled();
});
it('cancels a pending search and ignores its late completion',async()=>{
  let finish!:(value:unknown)=>void;vi.stubGlobal('fetch',vi.fn(()=>new Promise(r=>{finish=r;})));
  const p=props();render(<ImprovePlan {...p}/>);fireEvent.click(search());fireEvent.click(screen.getByRole('button',{name:'Отменить поиск'}));
  await act(async()=>finish(response(improved)));expect(screen.queryByRole('article')).not.toBeInTheDocument();expect(search()).not.toBeDisabled();
});
it('rejects a candidate which changes more than one decision',async()=>{
  const invalid=structuredClone(improved);invalid.improvements[0].decisions.find(d=>d.measure_id==='M7')!.district_id='esil';
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue(response(invalid)));render(<ImprovePlan {...props()}/>);fireEvent.click(search());
  expect(await screen.findByRole('alert')).toHaveTextContent('не соответствует текущему плану');expect(screen.queryByRole('article')).not.toBeInTheDocument();
});
it('allows keyboard request and explicit application of a district-only change',async()=>{
  const candidate={...improved.improvements[0],decisions:result.decisions.map(d=>d.measure_id==='M5'?{...d,district_id:'almaty'}:d)};
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue(response({current_score:result.final_score,improvements:[candidate]})));
  const p=props();render(<ImprovePlan {...p}/>);const u=userEvent.setup();
  await u.tab();expect(search()).toHaveFocus();await u.keyboard('{Enter}');
  const card=await screen.findByRole('article',{name:'Вариант 1'});
  expect(within(card).getByRole('heading')).toHaveTextContent('Изменить район');
  expect(within(card).getByText(/M5.*Сарыарка/)).toBeVisible();expect(within(card).getByText(/M5.*Алматы/)).toBeVisible();
  await u.tab();expect(screen.getByRole('button',{name:'Применить вариант 1'})).toHaveFocus();
  await u.keyboard(' ');expect(p.onApply).toHaveBeenCalledWith(candidate.decisions);
});
