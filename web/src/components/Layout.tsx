import {Building2, LayoutDashboard, SlidersHorizontal, ChartNoAxesCombined, History, MapPin} from 'lucide-react';
import type {Page,Scenario} from '../types';
const nav = [{id:'overview',label:'Обзор города',icon:LayoutDashboard},{id:'decisions',label:'Ваши решения',icon:SlidersHorizontal},{id:'result',label:'Результат',icon:ChartNoAxesCombined},{id:'history',label:'История',icon:History}] as const;
export function Layout({page,navigate,scenario,mock,onMode,children}:{page:Page;navigate:(p:Page)=>void;scenario?:Scenario;mock:boolean;onMode:()=>void;children:React.ReactNode}) {
  return <div className="shell"><a href="#main-content" className="skip-link">К содержимому</a><aside className="sidebar">
    <button className="brand" onClick={()=>navigate('overview')} aria-label="Аким — обзор города"><span className="brand-mark"><Building2 size={25}/></span>аким<span className="brand-dot">.</span></button>
    <div className="project-label">ГОРОД НАЧИНАЕТСЯ С РЕШЕНИЙ</div>
    <div className="workspace"><span className="city-icon"><MapPin size={18}/></span><div><strong>Астана</strong><small>Городская симуляция</small></div></div>
    <p className="nav-label">ВАШ СЦЕНАРИЙ</p><nav aria-label="Основная навигация">{nav.map(({id,label,icon:Icon},i)=><button key={id} onClick={()=>navigate(id)} className={page===id?'nav active':'nav'} aria-current={page===id?'page':undefined} title={label} aria-label={label}><Icon size={20} aria-hidden="true"/><b>{label}</b><span>0{i+1}</span></button>)}</nav>
    <div className="mission"><span>ВАША ЗАДАЧА</span><h3>Пять решений.<br/>Один город.</h3><p>Распределите ресурсы так, чтобы жизнь в каждом районе стала лучше.</p>{scenario && <small>{scenario.budget} единиц бюджета · {scenario.horizon_quarters} кварталов</small>}</div>
    <div className="sidebar-footer">HACKALEM <span>2026 ↗</span></div>
  </aside><main id="main-content" tabIndex={-1}><header><div className="breadcrumb">Рабочее пространство <span>/</span><strong>{nav.find(n=>n.id===page)?.label}</strong></div><button className={`mode ${mock?'mock':''}`} onClick={onMode}>{mock?'Учебный режим (mock)':'Серверный расчёт'}<span>{mock?'Подключить сервер':'Перейти в учебный режим'}</span></button></header>
    <div className="content">{mock && <div className="notice mock-banner"><strong>Учебный режим (mock) — без расчёта.</strong> Используется сохранённый каталог. Можно собрать план; расчёт и объяснение ИИ недоступны.</div>}{children}<footer><span>Аким на 5 часов · HackAlem</span><span>Синтетические данные · не прогноз для реального города</span></footer></div>
  </main></div>;
}
