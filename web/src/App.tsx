import {useEffect,useRef,useState} from 'react';
import {getScenario,simulate,explain} from './api';
import {isScenario,resultMatchesScenario} from './contract';
import mockSnapshot from './mock-scenario.json';
import {datasetKey,stableJSON,validatePlan} from './validation';
import {readDraft,readHistory,saveDraft,saveHistory,samePlan} from './storage';
import type {Decision,MeasureFocus,Metric,Page,SavedRun,Scenario} from './types';
import {Layout} from './components/Layout';
import {Empty,ErrorBox,Loading} from './components/common';
import {Overview} from './pages/Overview';
import {Decisions} from './pages/Decisions';
import {Result} from './pages/Result';
import {History} from './pages/History';

export default function App() {
  const [initial] = useState(readDraft);
  const [initialHistory] = useState(readHistory);
  const [page,setPage] = useState<Page>('overview');
  const [measureFocus,setMeasureFocus] = useState<MeasureFocus|null>(null);
  const [mock,setMock] = useState(import.meta.env.VITE_MOCK_MODE==='true');
  const [scenario,setScenario] = useState<Scenario>();
  const [loading,setLoading] = useState(true);
  const [loadError,setLoadError] = useState('');
  const [reload,setReload] = useState(0);
  const [plan,setPlan] = useState<Decision[]>(initial.draft.decisions);
  const [previousPlan,setPreviousPlan] = useState<Decision[]|null>(null);
  const [name,setName] = useState(initial.draft.name);
  const [runs,setRuns] = useState(initialHistory.runs);
  const [current,setCurrent] = useState<SavedRun|null>(null);
  const [archived,setArchived] = useState(false);
  const [deleted,setDeleted] = useState<SavedRun|null>(null);
  const [storageNotice,setStorageNotice] = useState(initial.notice || initialHistory.notice);
  const [runError,setRunError] = useState('');
  const [busy,setBusy] = useState(false);
  const sending = useRef(false);
  const explainingIds = useRef(new Set<string>());
  const [explainState,setExplainState] = useState<Record<string,{loading:boolean;error:string}>>({});
  const lastDraft = useRef(stableJSON(initial.draft)), lastHistory = useRef(stableJSON(initialHistory.runs));

  useEffect(()=>{
    let alive=true;
    setLoading(true);setLoadError('');setScenario(undefined);
    const load = mock ? (isScenario(mockSnapshot)?Promise.resolve(mockSnapshot):Promise.reject(new Error('Mock-каталог повреждён.'))) : getScenario();
    load.then(s=>{if(alive)setScenario(s);}).catch(e=>{if(alive)setLoadError(e.message);}).finally(()=>{if(alive)setLoading(false);});
    return ()=>{alive=false;};
  },[mock,reload]);
  useEffect(()=>{
    // Do not overwrite unreadable storage during initial render.
    const key=stableJSON({name,decisions:plan});
    if(lastDraft.current===key)return;
    lastDraft.current=key;
    const notice=saveDraft({name,decisions:plan});if(notice)setStorageNotice(notice);
  },[name,plan]);
  useEffect(()=>{
    const key=stableJSON(runs);
    if(lastHistory.current===key)return;
    lastHistory.current=key;
    const notice=saveHistory(runs);if(notice)setStorageNotice(notice);
  },[runs]);
  function navigate(p:Page) {
    setMeasureFocus(null);setPage(p);
    requestAnimationFrame(()=>{document.getElementById('main-content')?.focus();window.scrollTo({top:0,behavior:'instant'});});
  }
  function changePlan(next:Decision[]) {
    if(sending.current)return;
    setPlan(next);setPreviousPlan(null);setCurrent(null);setArchived(false);setRunError('');
  }
  function applyRecommendation(next:Decision[]) {
    if(sending.current)return;
    const previous=plan.map(d=>({...d}));
    changePlan(next);setPreviousPlan(previous);
    requestAnimationFrame(()=>document.getElementById('undo-recommendation')?.focus());
  }
  function undoRecommendation() {
    if(!previousPlan||sending.current)return;
    changePlan(previousPlan);
    requestAnimationFrame(()=>document.getElementById('plan')?.focus());
  }
  async function loadExplanation(run:SavedRun) {
    if(explainingIds.current.has(run.id)||mock)return;
    explainingIds.current.add(run.id);
    setExplainState(p=>({...p,[run.id]:{loading:true,error:''}}));
    try {
      const answer=await explain(run.result.decisions);
      // /explain calculates again. Never attach text to a different numerical result.
      if(stableJSON(answer.result)!==stableJSON(run.result))throw new Error('Расчёт в ответе объяснения отличается. Возможно, данные сервера изменились. Выполните новый расчёт.');
      const updated={...run,explanation:answer.explanation};
      setCurrent(p=>p?.id===run.id?updated:p);
      setRuns(p=>p.map(r=>r.id===run.id?updated:r));
      setExplainState(p=>({...p,[run.id]:{loading:false,error:''}}));
    } catch(e) {setExplainState(p=>({...p,[run.id]:{loading:false,error:e instanceof Error?e.message:'Не удалось получить объяснение.'}}));}
    finally{explainingIds.current.delete(run.id);}
  }
  async function run() {
    if(sending.current||mock||!scenario||!validatePlan(scenario,plan).valid)return;
    sending.current=true;setBusy(true);setRunError('');
    const submitted=plan.map(d=>({...d}));
    try {
      const latest=await getScenario();
      if(datasetKey(latest)!==datasetKey(scenario)) {setScenario(latest);throw new Error('Каталог или правила изменились. План сохранён и проверен заново. Просмотрите его перед повторным запуском.');}
      const result=await simulate(submitted);
      if(!resultMatchesScenario(result,latest,submitted))throw new Error('Ответ сервера неполный или рассчитан по другому плану либо каталогу. План сохранён. Повторите расчёт.');
      const saved:SavedRun={id:crypto.randomUUID(),name:name.trim()||'Без названия',createdAt:new Date().toISOString(),scenario:latest,datasetKey:datasetKey(latest),result};
      setRuns(p=>[saved,...p]);setCurrent(saved);setArchived(false);navigate('result');
      void loadExplanation(saved);
    } catch(e){setRunError(e instanceof Error?e.message:'Не удалось рассчитать сценарий. План сохранён.');}
    finally{sending.current=false;setBusy(false);}
  }
  const visible=current&&(archived||scenario&&samePlan(current,plan,datasetKey(scenario)))?current:null;
  function editResult() {
    if(!visible)return;
    if(archived){changePlan(visible.result.decisions.map(d=>({...d})));setName(visible.name);}
    navigate('decisions');
  }
  function openMeasures(districtId:string,metric:Metric) {
    navigate('decisions');
    setMeasureFocus({districtId,metric});
  }
  function openResultMeasures(districtId:string,metric:Metric) {
    if(!visible||sending.current)return;
    // Looking at measures must never replace the draft with an archived plan.
    // Restoring an archive remains the explicit "Изменить решения" action.
    openMeasures(districtId,metric);
  }
  return <Layout page={page} navigate={navigate} scenario={scenario} mock={mock} onMode={()=>{if(sending.current)return;setMock(p=>!p);setCurrent(null);setArchived(false);setRunError('');}}>
    {storageNotice && <div className="notice storage-notice" role="status"><span>{storageNotice}</span><button className="text-button" onClick={()=>setStorageNotice('')} aria-label="Скрыть уведомление о хранении">Закрыть</button></div>}
    {page==='history'?<History runs={runs} onStart={()=>navigate('decisions')} onOpen={r=>{setCurrent(r);setArchived(true);navigate('result');}} onDelete={id=>{setDeleted(runs.find(r=>r.id===id)||null);setRuns(p=>p.filter(r=>r.id!==id));}} canUndo={!!deleted} onUndo={()=>{if(deleted){setRuns(p=>[deleted,...p].sort((a,b)=>b.createdAt.localeCompare(a.createdAt)));setDeleted(null);}}}/>
      :page==='result'&&visible?<>{archived&&<div className="notice">Сохранённый расчёт от {new Date(visible.createdAt).toLocaleString('ru-RU')}. Используется каталог на момент расчёта.</div>}<Result key={visible.id} run={visible} onEdit={editResult} onOpenMeasures={openResultMeasures} explaining={explainState[visible.id]?.loading||false} explainError={mock&&!visible.explanation?'В учебном режиме (mock) объяснение недоступно.':explainState[visible.id]?.error||''} onExplain={()=>loadExplanation(visible)}/></>
      :loading?<Loading>Загружаем каталог и правила сценария…</Loading>
      :loadError?<section className="empty panel"><h1>Каталог пока недоступен</h1><ErrorBox message={loadError} retry={()=>setReload(p=>p+1)}/><p>Выбранный план сохранён. Для знакомства с мерами без сервера можно открыть учебный режим.</p><button className="secondary" onClick={()=>setMock(true)}>Открыть учебный режим (mock)</button></section>
      :scenario?page==='overview'?<Overview scenario={scenario} decisions={plan} onStart={()=>navigate('decisions')} onOpenMeasures={openMeasures}/>:page==='decisions'?<>{previousPlan&&<div className="notice" role="status">Рекомендация применена. Предыдущий план можно вернуть до следующего изменения решений.<button id="undo-recommendation" className="text-button" disabled={busy} onClick={undoRecommendation}>Отменить применение рекомендации</button></div>}<Decisions scenario={scenario} focus={measureFocus} onClearFocus={()=>setMeasureFocus(null)} plan={plan} name={name} onName={setName} onChange={changePlan} onRecommend={applyRecommendation} onRun={run} busy={busy} mock={mock} error={runError}/></>:<Empty title="Рассчитайте текущий план" action={()=>navigate('decisions')}>После изменения решений нужен новый расчёт. Предыдущие успешные результаты доступны в истории.</Empty>:null}
  </Layout>;
}
