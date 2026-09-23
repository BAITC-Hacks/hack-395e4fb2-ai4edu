import {describe,it,expect} from 'vitest';
import snapshot from '../src/mock-scenario.json';
import {isScenario} from '../src/contract';
import {datasetKey,planKey,validatePlan} from '../src/validation';
import type {Decision,Scenario} from '../src/types';
const s=snapshot as Scenario;
const local=(measure_id:string,district_id='nura'):Decision=>({measure_id,district_id});
export const golden:Decision[]=[local('M7'),local('M8'),local('M10'),{measure_id:'M12'},local('M5','saryarka')];
const codes=(p:Decision[],complete=false)=>validatePlan(s,p,complete).errors.map(e=>e.code);
describe('The Go scenario selection contract',()=>{
  it('contains all 14 measures and the full API schema',()=>{expect(isScenario(snapshot)).toBe(true);expect(s.measures.map(m=>m.id)).toEqual(Array.from({length:14},(_,i)=>`M${i+1}`));});
  it('accepts the documented 95-unit plan without calculating scores',()=>{expect(validatePlan(s,golden)).toEqual({valid:true,errors:[],cost:95,remaining:5});});
  it('requires exactly five, accepts partial drafts, rejects six',()=>{expect(codes(golden.slice(1),true)).toContain('decision_count');expect(codes(golden.slice(1))).not.toContain('decision_count');expect(codes([...golden,local('M11')])).toContain('decision_count');});
  it('rejects cost above 100 and permits exactly 100',()=>{const p=[local('M3'),local('M13'),local('M7'),{measure_id:'M2'},local('M5','saryarka')];expect(codes(p)).toContain('budget_exceeded');expect(validatePlan(s,[local('M3'),local('M7'),local('M8'),local('M10'),{measure_id:'M12'}])).toEqual({valid:true,errors:[],cost:100,remaining:0});expect(validatePlan({...s,budget:94},golden).valid).toBe(false);});
  it('forbids repeating a measure even across districts and counts repeat cost',()=>{const p=[local('M4'),local('M4','esil')];expect(codes(p)).toContain('duplicate_measure');expect(validatePlan(s,p,false).cost).toBe(30);});
  it('rejects a third measure of a direction',()=>{expect(codes([local('M7'),local('M8'),local('M9')])).toContain('category_limit');});
  it.each([['nura','nura'],['nura','esil']])('M1/M3 conflict globally (%s/%s)',(a,b)=>{expect(codes([local('M1',a),local('M3',b)])).toContain('incompatible_measures');});
  it.each([['M4','M7'],['M5','M13']])('%s/%s conflict only in the same district and after a district edit',(a,b)=>{
    const plan=[local(a,'nura'),local(b,'esil')];expect(codes(plan)).not.toContain('incompatible_measures');
    expect(codes([plan[0],{...plan[1],district_id:'nura'}])).toContain('incompatible_measures');
    expect(codes([plan[1],plan[0]])).not.toContain('incompatible_measures');
  });
  it('requires an existing district, forbids any district field for city measures',()=>{
    expect(codes([{measure_id:'M4'}])).toContain('district_required');expect(codes([local('M4','deleted')])).toContain('unknown_district');
    expect(codes([local('M12')])).toContain('city_district_forbidden');expect(codes([{measure_id:'M12',district_id:undefined}])).toContain('city_district_forbidden');
    expect(codes([{measure_id:'M12',district_id:null} as unknown as Decision])).toContain('city_district_forbidden');
  });
  it('revalidates removed catalog measures and new category limits',()=>{
    expect(validatePlan({...s,measures:s.measures.filter(m=>m.id!=='M7')},golden).errors.some(e=>e.code==='unknown_measure')).toBe(true);
    expect(validatePlan({...s,max_measures_per_category:1},golden).errors.some(e=>e.code==='category_limit')).toBe(true);
  });
  it('plan identity ignores order but notices district changes',()=>{expect(planKey(golden)).toBe(planKey([...golden].reverse()));expect(planKey(golden)).not.toBe(planKey(golden.map(d=>d.measure_id==='M7'?local('M7','esil'):d)));});
  it('dataset identity notices changed rules, names and indicators',()=>{expect(datasetKey(s)).toBe(datasetKey({...s}));expect(datasetKey(s)).not.toBe(datasetKey({...s,budget:99}));const changed=structuredClone(s);changed.districts[0].indicators.T1++;expect(datasetKey(changed)).not.toBe(datasetKey(s));});
});
