import {ArrowRight, Bus, Leaf, HeartPulse, ShieldCheck, Wrench, LoaderCircle} from 'lucide-react';
import type {Category, Effects, Scenario} from '../types';
export const format = (value: number, digits = 2) => new Intl.NumberFormat('ru-RU', {maximumFractionDigits:digits}).format(value);
export const signed = (value: number) => `${value > 0 ? '+' : ''}${format(value)}`;
export const icons = {Transport:Bus, Ecology:Leaf, Social:HeartPulse, Safety:ShieldCheck, Services:Wrench};
export function CategoryIcon({category}:{category:Category}) { const Icon = icons[category]; return <Icon size={19} aria-hidden="true"/>; }
export function EffectsList({effects,scenario}:{effects:Effects;scenario:Scenario}) {
  return <ul className="effects">{scenario.indicator_order.filter(k => effects[k] !== undefined).map(k => <li key={k}><span>{scenario.indicator_names[k]}</span><strong className={effects[k]! < 0 ? 'danger' : ''}>{signed(effects[k]!)}</strong></li>)}</ul>;
}
export function Loading({children}:{children:React.ReactNode}) { return <div className="loading" role="status"><LoaderCircle className="spin" size={22} aria-hidden="true"/>{children}</div>; }
export function ErrorBox({message,retry}:{message:string;retry?:()=>void}) { return <div className="notice error" role="alert"><p>{message}</p>{retry && <button className="secondary" onClick={retry}>Повторить попытку</button>}</div>; }
export function Empty({title,children,action,label='К решениям'}:{title:string;children:React.ReactNode;action?:()=>void;label?:string}) {
  return <section className="empty panel"><h2>{title}</h2><p>{children}</p>{action && <button className="cta" onClick={action}>{label}<ArrowRight size={18}/></button>}</section>;
}
