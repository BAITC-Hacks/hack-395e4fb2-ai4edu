import {categories, type Decision, type Explanation, type Scenario, type SimulationResult} from './types';
import {planKey, stableJSON, validatePlan} from './validation';
type Obj = Record<string, unknown>;
const obj = (v: unknown): v is Obj => !!v && typeof v === 'object' && !Array.isArray(v);
const num = (v: unknown): v is number => typeof v === 'number' && Number.isFinite(v);
const str = (v: unknown): v is string => typeof v === 'string' && v.length > 0;
const list = (v: unknown): v is unknown[] => Array.isArray(v);
const strings = (v: unknown): v is string[] => list(v) && v.every(str);
const numericMap = (v: unknown) => obj(v) && Object.values(v).every(num);
const metrics = ['T1','T2','E1','E2','S1','S2','B1','B2','C1','C2'];
const indicators = (v: unknown) => obj(v) && metrics.every(k => num(v[k]));
const pair = (v: unknown) => strings(v) && v.length === 2;
export function isDecision(v: unknown): v is Decision {
  return obj(v) && str(v.measure_id) && (!('district_id' in v) || typeof v.district_id === 'string') && Object.keys(v).every(k => ['measure_id','district_id'].includes(k));
}
export function isScenario(v: unknown): v is Scenario {
  if (!obj(v) || !['budget','required_decisions','max_measures_per_category','horizon_quarters'].every(k => num(v[k]) && (v[k] as number) > 0)) return false;
  if (!strings(v.indicator_order) || v.indicator_order.length !== metrics.length || !metrics.every(k => (v.indicator_order as string[]).includes(k))) return false;
  const names = v.indicator_names, cats = v.category_names;
  if (!obj(names) || !metrics.every(k => str(names[k])) || !obj(cats) || !categories.every(k => str(cats[k])) || !indicators(v.indicator_weights)) return false;
  const scoring = v.scoring;
  if (!obj(scoring) || !['average_weight','minimum_weight','critical_threshold','critical_penalty'].every(k => num(scoring[k]))) return false;
  if (!list(v.districts) || !v.districts.length || !v.districts.every(d => obj(d) && str(d.id) && str(d.name) && num(d.population_share) && indicators(d.indicators))) return false;
  if (!list(v.measures) || !v.measures.length || !v.measures.every(m => obj(m) && str(m.id) && str(m.name) && categories.includes(m.category as typeof categories[number]) && ['City','District'].includes(m.scope as string) && num(m.cost) && m.cost >= 0 && num(m.lag) && numericMap(m.effects))) return false;
  if (!list(v.synergies) || !v.synergies.every(s => obj(s) && pair(s.measure_ids) && str(s.district_measure_id) && numericMap(s.effects))) return false;
  return list(v.incompatibilities) && v.incompatibilities.every(r => obj(r) && pair(r.measure_ids) && typeof r.same_district_only === 'boolean');
}
export function isResult(v: unknown): v is SimulationResult {
  if (!obj(v) || v.valid !== true || !['total_cost','remaining_budget','base_score','final_score','score_delta','critical_before','critical_after'].every(k => num(v[k]))) return false;
  if (!list(v.decisions) || !v.decisions.every(isDecision)) return false;
  if (!list(v.district_before_after) || !v.district_before_after.length || !v.district_before_after.every(d => obj(d) && str(d.district_id) && str(d.name) && num(d.population_share) && num(d.score_before) && num(d.score_after) && indicators(d.before) && indicators(d.after))) return false;
  if (!obj(v.indicator_deltas) || !Object.values(v.indicator_deltas).every(indicators)) return false;
  if (!list(v.applied_effects) || !v.applied_effects.every(e => obj(e) && str(e.measure_id) && str(e.district_id) && num(e.lag_factor) && numericMap(e.effects))) return false;
  if (!list(v.applied_synergies) || !v.applied_synergies.every(e => obj(e) && pair(e.measure_ids) && str(e.district_id) && numericMap(e.effects))) return false;
  return [v.base_breakdown,v.final_breakdown].every(b => obj(b) && ['weighted_average','minimum_district','critical_penalty'].every(k => num(b[k])));
}
export function isExplanation(v: unknown): v is Explanation {
  return obj(v) && (v.source === 'llm' || v.source === 'fallback') && str(v.text);
}

// Check identity/completeness, never recalculate a score or an effect.
export function resultMatchesScenario(r: SimulationResult, s: Scenario, plan: Decision[] = r.decisions) {
  if (!validatePlan(s,r.decisions).valid || planKey(r.decisions)!==planKey(plan)) return false;
  const ids=r.district_before_after.map(d=>d.district_id);
  if(ids.length!==s.districts.length || new Set(ids).size!==ids.length) return false;
  for(const district of s.districts) {
    const actual=r.district_before_after.find(d=>d.district_id===district.id);
    if(!actual || !indicators(r.indicator_deltas[district.id]) || stableJSON(actual.before)!==stableJSON(district.indicators))return false;
  }
  for(const decision of r.decisions) {
    const measure=s.measures.find(m=>m.id===decision.measure_id)!;
    const targets=measure.scope==='City'?ids:[decision.district_id];
    if(!targets.every(id=>r.applied_effects.some(e=>e.measure_id===measure.id&&e.district_id===id)))return false;
  }
  return r.applied_effects.every(e=>ids.includes(e.district_id)&&r.decisions.some(d=>d.measure_id===e.measure_id)) && r.applied_synergies.every(e=>ids.includes(e.district_id)&&e.measure_ids.every(id=>r.decisions.some(d=>d.measure_id===id)));
}
