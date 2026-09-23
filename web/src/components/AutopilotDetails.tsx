import type {AutopilotRun,AutopilotStatus} from '../types';

export const statusNames:Record<AutopilotStatus,string>={queued:'В очереди',running:'Агенты готовят план',completed:'План проверен и готов',needs_clarification:'Нужно уточнить цель',infeasible:'Нет плана с такими ограничениями',needs_review:'Проверка не завершена',budget_exceeded:'Лимит расходов AI достигнут',unavailable:'AI временно недоступен',cancelled:'Запуск остановлен',timeout:'Время запуска истекло',failed:'Запуск завершился ошибкой'};
const roles:Record<string,string>={master:'Координатор',search:'Поиск вариантов',planner:'Планировщик',reviewer:'Проверяющий',system:'Система'};
export function AutopilotDetails({run}:{run:AutopilotRun}) {
  const uncertain=run.usage.some(u=>u.uncertain);
  return <section className="panel autopilot-details" aria-label="Ход работы AI-автопилота">
    <div className="section-heading"><div><p className="eyebrow">AI-АВТОПИЛОТ</p><h2>{statusNames[run.status]}</h2><p>{run.goal||'Максимальное качество жизни в пределах бюджета'}</p></div><span className={`tag ${run.status==='completed'?'':'fallback'}`}>{run.status==='completed'?'Одобрено проверяющим':'Результат ещё не одобрен'}</span></div>
    <div className="autopilot-body">
      {run.brief?.summary&&<p>{run.brief.summary}</p>}
      {run.message&&<p role="status">{run.message}</p>}
      {run.status==='needs_clarification'&&run.brief?.clarification&&<div className="notice warning">{run.brief.clarification}</div>}
      {run.status!=='completed'&&run.result&&<div className="notice warning">Найден промежуточный вариант. Он не прошёл весь цикл проверки и не заменяет ваш план.</div>}
      <ol className="agent-timeline" aria-label="Действия агентов" aria-live="polite" aria-relevant="additions text">
        {run.events.map((e,i)=><li key={`${i}-${e.at}`}><div><strong>{roles[e.agent]||e.agent}</strong><span>Шаг {e.iteration||1} · {new Date(e.at).toLocaleTimeString('ru-RU')}</span></div><p>{e.message}</p></li>)}
      </ol>
      {run.review&&<div className="autopilot-review"><h3>{run.review.approved&&run.status==='completed'?'Проверяющий одобрил вариант':'Обратная связь проверяющего'}</h3><p>{run.review.feedback}</p>{run.review.tradeoffs.length>0&&<><h4>Компромиссы</h4><ul>{run.review.tradeoffs.map((t,i)=><li key={i}>{t}</li>)}</ul></>}</div>}
      <dl className="autopilot-facts"><div><dt>Итерации проверки</dt><dd>{run.iterations}</dd></div><div><dt>Проверено вариантов</dt><dd>{run.evaluated.toLocaleString('ru-RU')}</dd></div><div><dt>Допустимых вариантов</dt><dd>{run.feasible.toLocaleString('ru-RU')}</dd></div><div><dt>Расходы AI, оценка</dt><dd>${run.estimated_cost_usd.toFixed(4)} / ${run.budget_usd.toFixed(2)}</dd></div></dl>
      <p className="help">{run.exhaustive?'Поиск завершил полный перебор для заданных ограничений.':'Поиск пока не подтвердил полный перебор.'} {uncertain?'Часть расходов оценена с запасом: провайдер не подтвердил точное использование.':'Стоимость приблизительная; фактическое списание определяет AI-провайдер.'}</p>
      {run.usage.length>0&&<details><summary>Использование моделей</summary><ul className="usage-list">{run.usage.map((u,i)=><li key={i}>{u.model}: {u.input_tokens} входных / {u.output_tokens} выходных токенов · ${u.estimated_cost_usd.toFixed(4)}{u.uncertain?' (оценка с запасом)':''}</li>)}</ul></details>}
    </div>
  </section>;
}
