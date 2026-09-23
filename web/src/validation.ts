import type { Decision, Scenario, ValidationError } from './types';

// Selection rules and budget only. The Go engine owns all score/effect calculations.
export function validatePlan(s: Scenario, plan: Decision[], complete = true) {
  const errors: ValidationError[] = [];
  const add = (code: string, message: string, ...measure_ids: string[]) => errors.push({code, message, measure_ids});
  if ((complete && plan.length !== s.required_decisions) || plan.length > s.required_decisions)
    add('decision_count', `Нужно ровно ${s.required_decisions} решений. Сейчас: ${plan.length}.`);
  const seen = new Map<string, Decision>();
  const counts: Record<string, number> = {};
  let cost = 0;
  for (const d of plan) {
    if (seen.has(d.measure_id)) add('duplicate_measure', `${d.measure_id} уже есть в плане. Повторы запрещены.`, d.measure_id);
    seen.set(d.measure_id, d);
    const m = s.measures.find(m => m.id === d.measure_id);
    if (!m) { add('unknown_measure', `Мера ${d.measure_id} больше не доступна в каталоге. Удалите или замените её.`, d.measure_id); continue; }
    cost += m.cost;
    counts[m.category] = (counts[m.category] || 0) + 1;
    if (m.scope === 'City' && 'district_id' in d) add('city_district_forbidden', `${m.id}: городская мера применяется ко всем районам; район нужно убрать.`, m.id);
    if (m.scope === 'District') {
      if (!d.district_id) add('district_required', `${m.id}: сначала выберите район.`, m.id);
      else if (!s.districts.some(x => x.id === d.district_id)) add('unknown_district', `${m.id}: район больше не доступен. Выберите другой.`, m.id);
    }
  }
  if (cost > s.budget) add('budget_exceeded', `Не хватает ${cost - s.budget} ед. бюджета: план стоит ${cost}, доступно ${s.budget}.`);
  for (const [category, count] of Object.entries(counts)) {
    if (count > s.max_measures_per_category) add('category_limit', `В направлении «${s.category_names[category as keyof typeof s.category_names]}» максимум ${s.max_measures_per_category} меры. Сейчас: ${count}.`);
  }
  for (const rule of s.incompatibilities) {
    const [a, b] = rule.measure_ids.map(id => seen.get(id));
    if (!a || !b) continue;
    if (rule.same_district_only && (!a.district_id || a.district_id !== b.district_id)) continue;
    add('incompatible_measures', `${a.measure_id} и ${b.measure_id} несовместимы ${rule.same_district_only ? 'в одном районе. Выберите другой район или замените меру.' : 'во всём городе. Выберите одну из этих мер.'}`, ...rule.measure_ids);
  }
  return {errors, cost, remaining: s.budget - cost, valid: errors.length === 0};
}

export function planKey(plan: Decision[]) {
  return JSON.stringify(plan.map(d => ({measure_id:d.measure_id, ...('district_id' in d ? {district_id:d.district_id} : {})})).sort((a,b) => a.measure_id.localeCompare(b.measure_id) || (a.district_id || '').localeCompare(b.district_id || '')));
}

// Canonical snapshots, not a made-up backend version. Object key order has no significance.
export function stableJSON(value: unknown): string {
  if (Array.isArray(value)) return '[' + value.map(stableJSON).join(',') + ']';
  if (value && typeof value === 'object') return '{' + Object.entries(value).sort(([a],[b]) => a.localeCompare(b)).map(([k,v]) => JSON.stringify(k)+':'+stableJSON(v)).join(',') + '}';
  return JSON.stringify(value);
}
export const datasetKey = (scenario: Scenario) => stableJSON(scenario);
