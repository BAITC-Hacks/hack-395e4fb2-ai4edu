import {render,screen,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {expect,it,vi} from 'vitest';
import {BeforeAfterBars,ResultCharts} from '../src/components/ResultCharts';
import {Result} from '../src/pages/Result';
import {format} from '../src/components/common';
import {datasetKey} from '../src/validation';
import type {Scenario,SimulationResult} from '../src/types';
import scenario from '../src/mock-scenario.json';
import fixture from './fixtures/golden-result.json';
const s=scenario as Scenario,r=fixture as SimulationResult;

it('compares every district using the server values and switches the displayed indicator',async()=>{
  render(<ResultCharts result={r} scenario={s}/>);
  const scores=screen.getByRole('region',{name:'Оценки районов'});
  expect(within(scores).getAllByRole('listitem')).toHaveLength(5);
  for(const d of r.district_before_after){
    const row=within(scores).getByRole('listitem',{name:d.name});
    expect(within(row).getByText(format(d.score_before,6))).toBeVisible();
    expect(within(row).getByText(format(d.score_after,6))).toBeVisible();
  }
  await userEvent.setup().selectOptions(screen.getByRole('combobox',{name:'Показатель для сравнения'}),'C2');
  const indicators=screen.getByRole('region',{name:'Где меняется жизнь'});
  for(const d of r.district_before_after){
    const row=within(indicators).getByRole('listitem',{name:`${d.name} · ${s.indicator_names.C2}`});
    expect(within(row).getByText(format(d.after.C2,6))).toBeVisible();
    expect(within(row).getByText('↑ Рост')).toBeVisible();
  }
});
it('marks values strictly below the supplied threshold and labels decline without color',()=>{
  render(<ul><BeforeAfterBars label="Проверка порога" before={40} after={39.99} threshold={40}/></ul>);
  expect(screen.getByText('↓ Снижение')).toBeVisible();
  expect(screen.queryByText('! До: критическое значение')).not.toBeInTheDocument();
  expect(screen.getByText('! После: критическое значение')).toBeVisible();
});
it('uses a changed scenario threshold and distinguishes unchanged values',()=>{
  render(<ul><BeforeAfterBars label="Другой порог" before={43} after={43} threshold={45}/></ul>);
  expect(screen.getByText('= Без изменений')).toBeVisible();
  expect(screen.getByText('! До: критическое значение')).toBeVisible();
  expect(screen.getByText('! После: критическое значение')).toBeVisible();
});
it('does not constrain the city Score to 0–100 and retains detailed indicator tables',()=>{
  render(<Result run={{id:'negative',name:'Тест',createdAt:'2026-09-23',scenario:s,datasetKey:datasetKey(s),result:{...r,final_score:-2.5}}} onEdit={vi.fn()} explaining={false} explainError="" onExplain={vi.fn()}/>);
  expect(within(screen.getByRole('region',{name:'Итог сценария'})).getByText('-2,5')).toBeVisible();
  expect(screen.getByRole('table',{name:'Нура: показатели до и после'})).toBeVisible();
});
