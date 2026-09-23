import {useState} from 'react';
import {act,fireEvent,render,screen,within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach,expect,it,vi} from 'vitest';
import {DistrictVisualization} from '../src/components/DistrictVisualization';
import type {Decision,Metric,Scenario,SimulationResult} from '../src/types';
import scenario from '../src/mock-scenario.json';
import fixture from './fixtures/golden-result.json';

const s=scenario as Scenario,r=fixture as SimulationResult;
let reduced=false;
const preferenceListeners=new Set<(event:MediaQueryListEvent)=>void>();
beforeEach(()=>{
  reduced=false;
  preferenceListeners.clear();
  vi.stubGlobal('matchMedia',vi.fn((query:string)=>({
    media:query,
    get matches(){return query==='(prefers-reduced-motion: reduce)'&&reduced;},
    onchange:null,
    addEventListener:(_type:string,listener:(event:MediaQueryListEvent)=>void)=>preferenceListeners.add(listener),
    removeEventListener:(_type:string,listener:(event:MediaQueryListEvent)=>void)=>preferenceListeners.delete(listener),
    addListener:(listener:(event:MediaQueryListEvent)=>void)=>preferenceListeners.add(listener),
    removeListener:(listener:(event:MediaQueryListEvent)=>void)=>preferenceListeners.delete(listener),
    dispatchEvent:()=>true,
  })));
});
function setReducedMotion(value:boolean){
  act(()=>{
    reduced=value;
    for(const listener of preferenceListeners)listener({matches:value,media:'(prefers-reduced-motion: reduce)'} as MediaQueryListEvent);
  });
}
function Interactive({result,decisions=[],onOpenMeasures}:{result?:SimulationResult;decisions?:Decision[];onOpenMeasures?:(id:string,metric:Metric)=>void}){
  const [selected,setSelected]=useState('esil');
  return <DistrictVisualization scenario={s} selectedId={selected} onSelect={setSelected} result={result} decisions={decisions} onOpenMeasures={onOpenMeasures}/>;
}
const scene=()=>{
  const node=document.querySelector('.city-construction');
  expect(node).not.toBeNull();
  return node!;
};
const indicator=()=>screen.getByRole('combobox',{name:'Показатель на схеме'});
const district=(name:string)=>within(screen.getByRole('group',{name:'Схема пяти районов'})).getByRole('button',{name:new RegExp(`^${name}`)});
const mode=(name:string)=>within(screen.getByRole('group',{name:'Режим схемы'})).getByRole('button',{name,exact:true});
const pause=()=>screen.getByRole('button',{name:'Пауза анимации'});
const replay=()=>screen.getByRole('button',{name:'Повторить анимацию'});
function endAnimation(node:Element=screen.getByTestId('city-animation-end')){
  // jsdom exposes WebkitAnimation styles but no AnimationEvent constructor.
  // React therefore subscribes to the prefixed event in this environment.
  const type='AnimationEvent' in window?'animationend':'webkitAnimationEnd';
  fireEvent(node,new Event(type,{bubbles:true}));
}
function mockViewport(){
  const instances:ViewportObserver[]=[];
  class ViewportObserver {
    observe=vi.fn();
    disconnect=vi.fn();
    constructor(readonly callback:IntersectionObserverCallback){instances.push(this);}
    enter(ratio:number){
      const target=this.observe.mock.calls[0][0] as Element;
      act(()=>this.callback([{target,isIntersecting:ratio>0,intersectionRatio:ratio} as IntersectionObserverEntry],this as unknown as IntersectionObserver));
    }
  }
  vi.stubGlobal('IntersectionObserver',ViewportObserver);
  return instances;
}

it('starts one decorative run and finishes on the CSS timeline completion event',()=>{
  render(<Interactive/>);
  const initial=scene(),run=initial.getAttribute('data-run');
  expect(initial).toHaveAttribute('data-motion','running');
  expect(pause()).toBeEnabled();
  // jsdom does not run CSS animations: dispatch their actual completion event,
  // rather than pretending a timer proves visual duration or construction progress.
  endAnimation();
  expect(scene()).toBe(initial);
  expect(scene()).toHaveAttribute('data-motion','complete');
  expect(scene()).toHaveAttribute('data-run',run);
  expect(replay()).toBeEnabled();
});

it('does not finish the whole scene when an intermediate illustration animation ends',()=>{
  render(<Interactive/>);
  const part=scene().querySelector('path');
  expect(part).not.toBeNull();
  endAnimation(part!);
  expect(scene()).toHaveAttribute('data-motion','running');
});

it('pauses and resumes the same run while district and indicator actions stay usable',async()=>{
  const u=userEvent.setup(),open=vi.fn();
  render(<Interactive onOpenMeasures={open}/>);
  const initial=scene(),run=initial.getAttribute('data-run');
  await u.click(pause());
  expect(scene()).toHaveAttribute('data-motion','paused');
  await u.click(district('Нура'));
  await u.selectOptions(indicator(),'T1');
  expect(district('Нура')).toHaveAttribute('aria-pressed','true');
  expect(screen.getByRole('heading',{name:'Район Нура'})).toBeVisible();
  expect(open).not.toHaveBeenCalled();
  await u.click(screen.getByRole('button',{name:'Показать подходящие меры'}));
  expect(open).toHaveBeenCalledExactlyOnceWith('nura','T1');
  expect(scene()).toBe(initial);
  expect(scene()).toHaveAttribute('data-run',run);
  expect(scene()).toHaveAttribute('data-motion','paused');
  await u.click(screen.getByRole('button',{name:'Продолжить анимацию'}));
  expect(scene()).toBe(initial);
  expect(scene()).toHaveAttribute('data-motion','running');
});

it('never restarts a running or finished scene after district, indicator, mode or unrelated plan changes',async()=>{
  const u=userEvent.setup();
  const {rerender}=render(<Interactive result={r}/>);
  const initial=scene(),run=initial.getAttribute('data-run');
  await u.click(district('Алматы'));
  await u.selectOptions(indicator(),'E2');
  for(const label of ['До','После','Изменение']){
    await u.click(mode(label));
    expect(scene()).toBe(initial);
    expect(scene()).toHaveAttribute('data-motion','running');
  }
  rerender(<Interactive result={structuredClone(r)} decisions={[{measure_id:'M14'}]}/>);
  expect(scene()).toBe(initial);
  expect(scene()).toHaveAttribute('data-run',run);
  expect(scene()).toHaveAttribute('data-motion','running');
  endAnimation();
  await u.click(district('Нура'));
  await u.selectOptions(indicator(),'S1');
  await u.click(mode('После'));
  rerender(<Interactive result={structuredClone(r)} decisions={[]}/>);
  expect(scene()).toBe(initial);
  expect(scene()).toHaveAttribute('data-run',run);
  expect(scene()).toHaveAttribute('data-motion','complete');
});

it('restarts construction only after an explicit replay and preserves data selection',async()=>{
  const u=userEvent.setup();render(<Interactive result={r}/>);
  await u.click(district('Нура'));await u.selectOptions(indicator(),'S2');await u.click(mode('Изменение'));
  endAnimation();
  const previous=scene(),run=previous.getAttribute('data-run');
  await u.click(replay());
  expect(scene()).not.toBe(previous);
  expect(scene().getAttribute('data-run')).not.toBe(run);
  expect(scene()).toHaveAttribute('data-motion','running');
  expect(district('Нура')).toHaveAttribute('aria-pressed','true');
  expect(indicator()).toHaveValue('S2');
  expect(mode('Изменение')).toHaveAttribute('aria-pressed','true');
});

it('shows the completed static illustration with disabled motion controls for reduced motion',async()=>{
  reduced=true;
  const u=userEvent.setup();render(<Interactive/>);
  const initial=scene();
  expect(initial).toHaveAttribute('data-motion','reduced');
  expect(pause()).toBeDisabled();expect(replay()).toBeDisabled();
  await u.click(district('Байконур'));await u.selectOptions(indicator(),'C1');
  expect(district('Байконур')).toHaveAttribute('aria-pressed','true');
  expect(indicator()).toHaveValue('C1');
  expect(scene()).toBe(initial);
  expect(scene()).toHaveAttribute('data-motion','reduced');
});

it('respects a live reduced-motion change and requires explicit replay if motion is restored',async()=>{
  const u=userEvent.setup();render(<Interactive/>);
  await u.click(pause());
  const run=scene().getAttribute('data-run');
  setReducedMotion(true);
  expect(scene()).toHaveAttribute('data-motion','reduced');
  expect(replay()).toBeDisabled();
  setReducedMotion(false);
  expect(scene()).toHaveAttribute('data-motion','complete');
  expect(scene()).toHaveAttribute('data-run',run);
  await u.click(replay());
  expect(scene()).toHaveAttribute('data-motion','running');
  expect(scene().getAttribute('data-run')).not.toBe(run);
});

it('keeps the five mobile HTML selection buttons synchronized with the SVG and details',async()=>{
  const u=userEvent.setup();render(<Interactive/>);
  const mobile=screen.getByRole('group',{name:'Выбор района',hidden:true});
  const buttons=within(mobile).getAllByRole('button',{hidden:true});
  expect(buttons).toHaveLength(s.districts.length);
  const nura=within(mobile).getByRole('button',{name:/^Нура/,hidden:true});
  await u.click(nura);
  expect(nura).toHaveAttribute('aria-pressed','true');
  expect(district('Нура')).toHaveAttribute('aria-pressed','true');
  expect(screen.getByRole('heading',{name:'Район Нура'})).toBeVisible();
  await u.click(district('Есиль'));
  expect(nura).toHaveAttribute('aria-pressed','false');
  expect(within(mobile).getByRole('button',{name:/Есиль/,hidden:true})).toHaveAttribute('aria-pressed','true');
  nura.focus();await u.keyboard('{Enter}');
  expect(district('Нура')).toHaveAttribute('aria-pressed','true');
});

it('limits symbolic renovation accents to the calculated plan, with citywide measures affecting all five districts',()=>{
  const onlyLocal={...structuredClone(r),decisions:[{measure_id:'M7',district_id:'nura'}]};
  const edited:Decision[]=[{measure_id:'M4',district_id:'esil'}];
  const {rerender}=render(<Interactive result={onlyLocal} decisions={edited}/>);
  const affected=()=>new Set(Array.from(scene().querySelectorAll('.city-renovation-accent')).map(node=>node.closest('[data-city-slot]')?.getAttribute('data-city-slot')));
  expect(affected()).toEqual(new Set(['nura']));
  // Every district receives a real citywide measure. Accents are not inferred
  // from indicator deltas or from a different current draft.
  rerender(<Interactive result={{...onlyLocal,decisions:[{measure_id:'M12'}]}} decisions={edited}/>);
  expect(affected()).toEqual(new Set(s.districts.map(d=>d.id)));
  rerender(<Interactive decisions={edited}/>);
  expect(affected().size).toBe(0);
});

it('waits for the actual city SVG to enter the viewport once and never resumes a user pause on reentry',async()=>{
  const observers=mockViewport(),u=userEvent.setup();render(<Interactive/>);
  const initial=scene(),run=initial.getAttribute('data-run'),observer=observers[0];
  expect(initial).toHaveAttribute('data-motion','paused');
  expect(screen.getByText('Сцена готова к появлению')).toBeVisible();
  expect(observer.observe).toHaveBeenCalledExactlyOnceWith(screen.getByRole('group',{name:'Схема пяти районов'}));
  observer.enter(0);observer.enter(.05);
  expect(scene()).toHaveAttribute('data-motion','paused');
  observer.enter(.15);
  expect(scene()).toHaveAttribute('data-motion','running');
  expect(scene()).toBe(initial);expect(scene()).toHaveAttribute('data-run',run);
  expect(observer.disconnect).toHaveBeenCalledOnce();
  expect(screen.queryByText('Сцена готова к появлению')).not.toBeInTheDocument();
  await u.click(pause());
  observer.enter(0);observer.enter(.8);
  expect(scene()).toHaveAttribute('data-motion','paused');
  expect(scene()).toBe(initial);expect(scene()).toHaveAttribute('data-run',run);
});

it.each(['Повторить анимацию','Продолжить анимацию'])('honors explicit %s before visibility and ignores queued intersection events afterward',async(label)=>{
  const observers=mockViewport(),u=userEvent.setup();render(<Interactive/>);
  const observer=observers[0];
  await u.click(screen.getByRole('button',{name:label}));
  const active=scene(),run=active.getAttribute('data-run');
  expect(active).toHaveAttribute('data-motion','running');
  expect(observer.disconnect).toHaveBeenCalledOnce();
  expect(screen.queryByText('Сцена готова к появлению')).not.toBeInTheDocument();
  await u.click(pause());observer.enter(1);
  expect(scene()).toBe(active);expect(scene()).toHaveAttribute('data-run',run);
  expect(scene()).toHaveAttribute('data-motion','paused');
});

it('cancels pending viewport entry when reduced motion is enabled and does not auto-start when restored',()=>{
  const observers=mockViewport();render(<Interactive/>);
  const observer=observers[0],initial=scene();
  setReducedMotion(true);
  expect(scene()).toHaveAttribute('data-motion','reduced');
  expect(observer.disconnect).toHaveBeenCalled();
  expect(replay()).toBeDisabled();
  observer.enter(1);
  setReducedMotion(false);observer.enter(1);
  expect(scene()).toBe(initial);
  expect(scene()).toHaveAttribute('data-motion','complete');
  expect(screen.queryByText('Сцена готова к появлению')).not.toBeInTheDocument();
  expect(observers).toHaveLength(1);
});

it('does not observe viewport entry for an initially reduced-motion scene',()=>{
  const observers=mockViewport();reduced=true;render(<Interactive/>);
  expect(scene()).toHaveAttribute('data-motion','reduced');
  expect(observers).toHaveLength(0);
  setReducedMotion(false);
  expect(scene()).toHaveAttribute('data-motion','complete');
  expect(observers).toHaveLength(0);
});
