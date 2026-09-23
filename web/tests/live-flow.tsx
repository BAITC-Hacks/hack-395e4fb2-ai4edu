import {render,screen,waitFor,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {it,expect,vi} from 'vitest';
import App from '../src/App';
import {HISTORY_KEY} from '../src/storage';
it('runs the actual application against the running Go API and restores its draft',async()=>{
  // Only resolve relative URLs. Every response comes from the real server.
  const realFetch=globalThis.fetch;
  const origin=process.env.API_URL||'http://127.0.0.1:8080';
  vi.stubGlobal('fetch',(input:string,options:RequestInit)=>realFetch(origin+input,options));
  const u=userEvent.setup();
  const view=render(<App/>);
  await screen.findByRole('heading',{name:'Город в ваших руках'});
  await u.click(screen.getByRole('button',{name:'Составить план'}));
  for(const [id,district] of [['M7','nura'],['M8','nura'],['M10','nura'],['M12',undefined],['M5','saryarka']] as const){
    const card=document.getElementById(`title-${id}`)!.closest('article')!;
    if(district)await u.selectOptions(within(card).getByRole('combobox'),district);
    await u.click(within(card).getByRole('button',{name:'Добавить решение'}));
  }
  await waitFor(()=>expect(screen.getByRole('button',{name:'Рассчитать результат'})).toHaveAttribute('aria-disabled','false'));
  await u.click(screen.getByRole('button',{name:'Рассчитать результат'}));
  expect(await screen.findByText('56,54')).toBeVisible();
  expect(await screen.findByText('Резервное объяснение · без AI',{}, {timeout:15000})).toBeVisible();
  const history=JSON.parse(localStorage.getItem(HISTORY_KEY)!).runs;
  expect(history[0].result.total_cost).toBe(95);expect(history[0].result.final_score).toBeCloseTo(56.54307,8);expect(history[0].explanation.source).toBe('fallback');
  view.unmount();render(<App/>);await screen.findByRole('heading',{name:'Город в ваших руках'});await u.click(screen.getByRole('button',{name:'Ваши решения',exact:true}));expect(screen.getByRole('combobox',{name:'Район выбранной меры M7'})).toHaveValue('nura');await waitFor(()=>expect(screen.getByRole('button',{name:'Рассчитать результат'})).toHaveAttribute('aria-disabled','false'));
});
