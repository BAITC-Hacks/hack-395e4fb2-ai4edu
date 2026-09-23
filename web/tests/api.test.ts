import {it,expect,vi} from 'vitest';
import {ApiError,assessPlan,explain,getScenario,recommendBest,simulate} from '../src/api';
import result from './fixtures/golden-result.json';
import explanation from './fixtures/golden-explain.json';
import scenario from '../src/mock-scenario.json';
const response=(body:unknown,status=200)=>({ok:status<400,status,json:async()=>body});
it('loads the typed catalog and validates actual Go fixtures',async()=>{const f=vi.fn().mockResolvedValueOnce(response(scenario)).mockResolvedValueOnce(response(result)).mockResolvedValueOnce(response(explanation));vi.stubGlobal('fetch',f);expect(await getScenario()).toEqual(scenario);expect(await simulate(result.decisions)).toEqual(result);expect((await explain(result.decisions)).explanation.source).toBe('fallback');});
it('omits district_id for city measures in the request',async()=>{const f=vi.fn().mockResolvedValue(response(result));vi.stubGlobal('fetch',f);await simulate(result.decisions);const body=JSON.parse(f.mock.calls[0][1].body);expect(body).toEqual({decisions:result.decisions});expect(body.decisions.find((d:any)=>d.measure_id==='M12')).not.toHaveProperty('district_id');});
it.each(['decision_count','duplicate_measure','budget_exceeded','category_limit','incompatible_measures','invalid_json','request_too_large'])('translates backend validation error %s',async code=>{vi.stubGlobal('fetch',vi.fn().mockResolvedValue(response({valid:false,validation_errors:[{code,message:'English backend detail',measure_ids:['M1','M3']}]},422)));await expect(simulate([])).rejects.toThrow(/[А-Яа-я]/);await expect(simulate([])).rejects.not.toThrow('English backend detail');});
it('reports unavailable backend',async()=>{vi.stubGlobal('fetch',vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));await expect(getScenario()).rejects.toThrow('Не удалось подключиться');});
it('rejects incomplete JSON instead of rendering invented numbers',async()=>{vi.stubGlobal('fetch',vi.fn().mockResolvedValue(response({valid:true})));await expect(simulate([])).rejects.toThrow('неполный результат');});
it('handles a non-JSON proxy 500 in Russian',async()=>{vi.stubGlobal('fetch',vi.fn().mockResolvedValue({ok:false,status:500,json:async()=>{throw new Error('HTML');}}));await expect(getScenario()).rejects.toThrow('Сервер расчёта временно недоступен');});
it('times out stalled requests',async()=>{vi.useFakeTimers();vi.stubGlobal('fetch',vi.fn((_url,options)=>new Promise((_resolve,reject)=>{options.signal.addEventListener('abort',()=>reject(new DOMException('aborted','AbortError')));})));const request=getScenario();const expectation=expect(request).rejects.toThrow('30 секунд');await vi.advanceTimersByTimeAsync(30000);await expectation;});
it('uses only relative paths, including best without a decisions field',async()=>{
  const best={decisions:result.decisions,final_score:result.final_score,total_cost:result.total_cost,remaining_budget:result.remaining_budget,critical_after:result.critical_after};
  const fetch=vi.fn().mockResolvedValue(response({best}));vi.stubGlobal('fetch',fetch);expect(await recommendBest()).toEqual(best);expect(fetch.mock.calls[0][0]).toBe('/api/recommend');expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({mode:'best'});
});
it('keeps 503 not_ready as a structured code',async()=>{
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue(response({valid:false,validation_errors:[{code:'not_ready',message:'Optimal scenario is still being computed'}]},503)));
  await expect(recommendBest()).rejects.toMatchObject({status:503,validation_errors:[{code:'not_ready',message:'Optimal scenario is still being computed'}]});
});
it('gets draft cost and validity from a 422 response, not a client sum',async()=>{
  const answer={valid:false,total_cost:73,remaining_budget:27,decisions:[{measure_id:'M12'}],validation_errors:[{code:'decision_count',message:'Exactly 5'}]};
  vi.stubGlobal('fetch',vi.fn().mockResolvedValue(response(answer,422)));expect(await assessPlan(answer.decisions)).toEqual(answer);
});
it('rejects assessment for a different plan',async()=>{vi.stubGlobal('fetch',vi.fn().mockResolvedValue(response(result)));await expect(assessPlan([{measure_id:'M14'}])).rejects.toBeInstanceOf(ApiError);});
