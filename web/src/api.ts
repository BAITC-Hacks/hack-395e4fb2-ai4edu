import type {Candidate, Decision, ExplainResponse, PlanAssessment, Scenario, SimulationResult, ValidationError} from './types';
import {isDecision, isExplanation, isResult, isScenario} from './contract';
import {planKey} from './validation';

const messages: Record<string,string> = {
  decision_count:'Нужно ровно пять решений.', duplicate_measure:'Мероприятия не должны повторяться.',
  unknown_measure:'Каталог изменился: выбранная мера не найдена. Обновите каталог.',
  district_required:'Для районной меры выберите район.', unknown_district:'Выбранный район не найден. Обновите каталог.',
  city_district_forbidden:'У городской меры не должно быть выбранного района.', budget_exceeded:'Стоимость плана превышает бюджет.',
  category_limit:'Можно выбрать не больше двух мер одного направления.', incompatible_measures:'В плане есть несовместимые мероприятия.',
  invalid_json:'Сервер не смог прочитать план. Обновите страницу и повторите отправку.', request_too_large:'План слишком большой для отправки.',
  not_ready:'Считается… Сервер ещё ищет лучший план.'
};
export function validationMessage(e:ValidationError) {
  return `${messages[e.code] || 'Сервер отклонил план. Проверьте решения.'}${e.measure_ids?.length ? ` (${e.measure_ids.join(', ')})` : ''}`;
}
function validationErrors(value:unknown): ValidationError[] {
  return Array.isArray(value) ? value.filter((e):e is ValidationError => !!e && typeof e.code==='string' && typeof e.message==='string' && (e.measure_ids===undefined || Array.isArray(e.measure_ids)&&e.measure_ids.every((id:unknown)=>typeof id==='string'))) : [];
}
export class ApiError extends Error {
  constructor(message: string, public status = 0, public validation_errors:ValidationError[] = []) { super(message); this.name = 'ApiError'; }
}
async function request(path: string, payload?:object, options:{signal?:AbortSignal;acceptValidation?:boolean}={}): Promise<unknown> {
  const controller = new AbortController();
  const abort=()=>controller.abort();
  if(options.signal?.aborted)throw new DOMException('Request cancelled','AbortError');
  options.signal?.addEventListener('abort',abort,{once:true});
  const timeout = setTimeout(() => controller.abort(), 30000);
  try {
    const response = await fetch(path, {signal:controller.signal, ...(payload ? {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)} : {})});
    const body = await response.json().catch(() => null);
    if (!response.ok && !(options.acceptValidation&&response.status===422)) {
      const errors = validationErrors(body?.validation_errors);
      const message = errors.length ? errors.map(validationMessage).join(' ') : response.status >= 500 ? 'Сервер расчёта временно недоступен. Попробуйте ещё раз.' : `Не удалось выполнить запрос (HTTP ${response.status}).`;
      throw new ApiError(message, response.status, errors);
    }
    return body;
  } catch (e) {
    if(options.signal?.aborted)throw new DOMException('Request cancelled','AbortError');
    if (e instanceof ApiError) throw e;
    throw new ApiError(controller.signal.aborted ? 'Сервер не ответил за 30 секунд. План сохранён; попробуйте ещё раз.' : 'Не удалось подключиться к Go API. Проверьте, что сервер запущен, и повторите попытку.');
  } finally { clearTimeout(timeout);options.signal?.removeEventListener('abort',abort); }
}
export async function getScenario(): Promise<Scenario> {
  const body = await request('/api/scenario');
  if (!isScenario(body)) throw new ApiError('Сервер вернул неполный каталог. Проверьте версию Go API и повторите загрузку.');
  return body;
}
export async function simulate(decisions: Decision[]): Promise<SimulationResult> {
  const body = await request('/api/simulate', {decisions});
  if (!isResult(body)) throw new ApiError('Сервер вернул неполный результат. План сохранён; повторите расчёт.');
  return body;
}
export async function explain(decisions: Decision[]): Promise<ExplainResponse> {
  const body = await request('/api/explain', {decisions}) as Partial<ExplainResponse> | null;
  if (!body || !isResult(body.result) || !isExplanation(body.explanation)) throw new ApiError('Ответ с объяснением неполный. Результат расчёта сохранён.');
  return body as ExplainResponse;
}

// /simulate also validates incomplete drafts: 422 still contains authoritative cost and errors.
export async function assessPlan(decisions:Decision[],signal?:AbortSignal):Promise<PlanAssessment> {
  const body=await request('/api/simulate',{decisions},{signal,acceptValidation:true}) as Partial<PlanAssessment>|null;
  if(!body || typeof body.valid!=='boolean' || !Number.isFinite(body.total_cost) || !Number.isFinite(body.remaining_budget) || !Array.isArray(body.decisions) || !body.decisions.every(isDecision) || planKey(body.decisions)!==planKey(decisions))throw new ApiError('Не удалось подтвердить бюджет и валидность плана. Повторите проверку.');
  const errors=validationErrors(body.validation_errors);
  if(!body.valid&&!errors.length)throw new ApiError('Сервер не указал причины отклонения плана. Повторите проверку.');
  return {valid:body.valid,total_cost:body.total_cost!,remaining_budget:body.remaining_budget!,decisions:body.decisions,validation_errors:errors};
}
export async function recommendBest(signal?:AbortSignal):Promise<Candidate> {
  const body=await request('/api/recommend',{mode:'best'},{signal}) as {best?:Candidate}|null;
  const best=body?.best;
  if(!best || !Array.isArray(best.decisions) || !best.decisions.every(isDecision) || ![best.final_score,best.total_cost,best.remaining_budget,best.critical_after].every(Number.isFinite))throw new ApiError('Сервер вернул неполную рекомендацию. Повторите запрос.');
  return best;
}
