import {isAutopilotRun, isDecision, isExplanation, isResult, isScenario, resultMatchesScenario} from './contract';
import {datasetKey, planKey, stableJSON} from './validation';
import type {Decision, SavedRun} from './types';
export const DRAFT_KEY = 'akim.draft.v1';
export const HISTORY_KEY = 'akim.history.v1';
export interface Draft {name: string; decisions: Decision[]}
export function readDraft(): {draft: Draft; notice: string} {
  const empty = {name:'Мой план развития', decisions:[]};
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (!raw) return {draft:empty, notice:''};
    const value = JSON.parse(raw);
    if (value?.version !== 1 || typeof value.name !== 'string' || value.name.length > 120 || !Array.isArray(value.decisions) || value.decisions.length > 100 || !value.decisions.every(isDecision)) throw new Error();
    return {draft:{name:value.name,decisions:value.decisions}, notice:'Черновик восстановлен. Решения повторно проверены по загруженному каталогу.'};
  } catch { return {draft:empty,notice:'Черновик повреждён или хранилище недоступно. Можно собрать новый план.'}; }
}
export function readHistory(): {runs: SavedRun[]; notice: string} {
  try {
    const raw = localStorage.getItem(HISTORY_KEY);
    if (!raw) return {runs:[],notice:''};
    const value = JSON.parse(raw);
    if (value?.version !== 1 || !Array.isArray(value.runs)) throw new Error();
    const runs = value.runs.filter((r: SavedRun) => r && typeof r.id === 'string' && typeof r.name === 'string' && r.name.length <= 120 && typeof r.createdAt === 'string' && Number.isFinite(Date.parse(r.createdAt)) && isScenario(r.scenario) && isResult(r.result) && r.datasetKey === datasetKey(r.scenario) && (!r.explanation || isExplanation(r.explanation)));
    const unique = runs.filter((r:SavedRun,i:number) => runs.findIndex((other:SavedRun) => other.id === r.id) === i && resultMatchesScenario(r.result,r.scenario) && (!r.autopilot || isAutopilotRun(r.autopilot)&&r.autopilot.status==='completed'&&stableJSON(r.autopilot.result)===stableJSON(r.result)));
    return {runs:unique,notice:unique.length === value.runs.length ? '' : 'Некоторые записи истории повреждены и пропущены.'};
  } catch { return {runs:[],notice:'Не удалось прочитать историю. Повреждённые данные не используются.'}; }
}
export function saveDraft(draft: Draft): string {
  return write(DRAFT_KEY, {version:1,...draft});
}
export function saveHistory(runs: SavedRun[]): string {
  return write(HISTORY_KEY, {version:1,runs});
}
function write(key: string, value: unknown) {
  try { localStorage.setItem(key, JSON.stringify(value)); return ''; }
  catch { return 'Не удалось сохранить данные в браузере: хранилище недоступно или заполнено. Текущий план и результат доступны до перезагрузки.'; }
}
export function samePlan(run: SavedRun, decisions: Decision[], key: string) {
  return run.datasetKey === key && planKey(run.result.decisions) === planKey(decisions);
}
