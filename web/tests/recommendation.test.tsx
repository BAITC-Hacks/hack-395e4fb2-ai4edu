import {act,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {it,expect,vi} from 'vitest';
import {Recommendation} from '../src/components/Recommendation';
import scenario from '../src/mock-scenario.json';
import result from './fixtures/golden-result.json';
import type {Scenario} from '../src/types';
const best={decisions:result.decisions,final_score:result.final_score,total_cost:result.total_cost,remaining_budget:result.remaining_budget,critical_after:result.critical_after};
const response=(body:unknown,status=200)=>({ok:status<400,status,json:async()=>body});
const notReady=response({valid:false,validation_errors:[{code:'not_ready',message:'Still computing'}]},503);
it('shows Считается… for not_ready, retries and applies only the returned candidate',async()=>{
  vi.useFakeTimers();const fetch=vi.fn().mockResolvedValueOnce(notReady).mockResolvedValueOnce(response({best}));vi.stubGlobal('fetch',fetch);const apply=vi.fn();render(<Recommendation scenario={scenario as Scenario} mock={false} busy={false} onApply={apply}/>);
  await act(async()=>{fireEvent.click(screen.getByRole('button',{name:'Показать лучший план'}));});expect(screen.getByText('Считается…')).toBeVisible();expect(fetch).toHaveBeenCalledTimes(1);
  await act(async()=>{await vi.advanceTimersByTimeAsync(2000);});expect(fetch).toHaveBeenCalledTimes(2);expect(screen.getByText('56,54 Score')).toBeVisible();expect(apply).not.toHaveBeenCalled();fireEvent.click(screen.getByRole('button',{name:'Заменить текущий план рекомендацией'}));expect(apply).toHaveBeenCalledWith(best.decisions);
});
it('cancels retries on request and on unmount',async()=>{
  vi.useFakeTimers();const fetch=vi.fn().mockResolvedValue(notReady);vi.stubGlobal('fetch',fetch);const view=render(<Recommendation scenario={scenario as Scenario} mock={false} busy={false} onApply={vi.fn()}/>);
  await act(async()=>{fireEvent.click(screen.getByRole('button',{name:'Показать лучший план'}));});fireEvent.click(screen.getByRole('button',{name:'Отменить ожидание'}));await act(async()=>{await vi.advanceTimersByTimeAsync(20000);});expect(fetch).toHaveBeenCalledTimes(1);
  await act(async()=>{fireEvent.click(screen.getByRole('button',{name:'Показать лучший план'}));});view.unmount();await act(async()=>{await vi.advanceTimersByTimeAsync(20000);});expect(fetch).toHaveBeenCalledTimes(2);
});
it('does not retry an unrelated 503 automatically',async()=>{
  const fetch=vi.fn().mockResolvedValue(response({validation_errors:[{code:'maintenance',message:'Down'}]},503));vi.stubGlobal('fetch',fetch);render(<Recommendation scenario={scenario as Scenario} mock={false} busy={false} onApply={vi.fn()}/>);fireEvent.click(screen.getByRole('button',{name:'Показать лучший план'}));await screen.findByRole('alert');expect(fetch).toHaveBeenCalledTimes(1);expect(screen.queryByText('Считается…')).not.toBeInTheDocument();
});
it('does not contact the optimizer in mock mode',()=>{const fetch=vi.fn();vi.stubGlobal('fetch',fetch);render(<Recommendation scenario={scenario as Scenario} mock busy={false} onApply={vi.fn()}/>);expect(screen.getByRole('button',{name:'Показать лучший план'})).toBeDisabled();expect(fetch).not.toHaveBeenCalled();});
