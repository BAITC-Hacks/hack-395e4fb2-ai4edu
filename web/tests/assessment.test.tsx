import {act,renderHook,waitFor} from '@testing-library/react';
import {it,expect,vi} from 'vitest';
import {usePlanAssessment} from '../src/usePlanAssessment';
import scenario from '../src/mock-scenario.json';
import type {Decision,Scenario} from '../src/types';
const response=(decisions:Decision[],cost:number)=>({ok:false,status:422,json:async()=>({valid:false,decisions,total_cost:cost,remaining_budget:100-cost,validation_errors:[{code:'decision_count',message:'Exactly 5'}]})});
it('suppresses stale budget immediately and ignores late replies after a district change',async()=>{
  const pending:((r:unknown)=>void)[]=[];vi.stubGlobal('fetch',vi.fn(()=>new Promise(resolve=>pending.push(resolve))));
  const view=renderHook(({plan})=>usePlanAssessment(plan,scenario as Scenario,false),{initialProps:{plan:[{measure_id:'M7',district_id:'nura'}]}});
  await waitFor(()=>expect(pending).toHaveLength(1));view.rerender({plan:[{measure_id:'M7',district_id:'esil'}]});expect(view.result.current.answer).toBeUndefined();await waitFor(()=>expect(pending).toHaveLength(2));
  await act(async()=>pending[1](response([{measure_id:'M7',district_id:'esil'}],70)));expect(view.result.current.answer?.total_cost).toBe(70);
  await act(async()=>pending[0](response([{measure_id:'M7',district_id:'nura'}],24)));expect(view.result.current.answer?.total_cost).toBe(70);
});
it('keeps budget unavailable on network failure and supports retry',async()=>{
  const fetch=vi.fn().mockRejectedValueOnce(new TypeError('offline')).mockResolvedValueOnce(response([],0));vi.stubGlobal('fetch',fetch);const view=renderHook(()=>usePlanAssessment([],scenario as Scenario,false));await waitFor(()=>expect(view.result.current.error).toContain('Не удалось подключиться'));expect(view.result.current.answer).toBeUndefined();act(()=>view.result.current.retry());await waitFor(()=>expect(view.result.current.answer?.total_cost).toBe(0));
});
