import type {Decision, ExplainResponse, Scenario, SimulationResult} from './types';
import {isExplanation, isResult, isScenario} from './contract';

const messages: Record<string,string> = {
  decision_count:'Нужно ровно пять решений.', duplicate_measure:'Мероприятия не должны повторяться.',
  unknown_measure:'Каталог изменился: выбранная мера не найдена. Обновите каталог.',
  district_required:'Для районной меры выберите район.', unknown_district:'Выбранный район не найден. Обновите каталог.',
  city_district_forbidden:'У городской меры не должно быть выбранного района.', budget_exceeded:'Стоимость плана превышает бюджет.',
  category_limit:'Можно выбрать не больше двух мер одного направления.', incompatible_measures:'В плане есть несовместимые мероприятия.',
  invalid_json:'Сервер не смог прочитать план. Обновите страницу и повторите отправку.', request_too_large:'План слишком большой для отправки.'
};
export class ApiError extends Error { constructor(message: string, public status = 0) { super(message); this.name = 'ApiError'; } }
async function request(path: string, decisions?: Decision[]): Promise<unknown> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 30000);
  try {
    const response = await fetch(path, {signal:controller.signal, ...(decisions ? {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({decisions})} : {})});
    const body = await response.json().catch(() => null);
    if (!response.ok) {
      const errors = body?.validation_errors;
      const message = Array.isArray(errors) ? errors.map(e => `${messages[e.code] || 'Сервер отклонил план. Проверьте решения.'}${Array.isArray(e.measure_ids) && e.measure_ids.length ? ` (${e.measure_ids.join(', ')})` : ''}`).join(' ') : response.status >= 500 ? 'Сервер расчёта временно недоступен. Попробуйте ещё раз.' : `Не удалось выполнить запрос (HTTP ${response.status}).`;
      throw new ApiError(message, response.status);
    }
    return body;
  } catch (e) {
    if (e instanceof ApiError) throw e;
    throw new ApiError(controller.signal.aborted ? 'Сервер не ответил за 30 секунд. План сохранён; попробуйте ещё раз.' : 'Не удалось подключиться к Go API. Проверьте, что сервер запущен, и повторите попытку.');
  } finally { clearTimeout(timeout); }
}
export async function getScenario(): Promise<Scenario> {
  const body = await request('/api/scenario');
  if (!isScenario(body)) throw new ApiError('Сервер вернул неполный каталог. Проверьте версию Go API и повторите загрузку.');
  return body;
}
export async function simulate(decisions: Decision[]): Promise<SimulationResult> {
  const body = await request('/api/simulate', decisions);
  if (!isResult(body)) throw new ApiError('Сервер вернул неполный результат. План сохранён; повторите расчёт.');
  return body;
}
export async function explain(decisions: Decision[]): Promise<ExplainResponse> {
  const body = await request('/api/explain', decisions) as Partial<ExplainResponse> | null;
  if (!body || !isResult(body.result) || !isExplanation(body.explanation)) throw new ApiError('Ответ с объяснением неполный. Результат расчёта сохранён.');
  return body as ExplainResponse;
}
