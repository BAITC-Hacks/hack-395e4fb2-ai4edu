// Real HTTP verification; never used by the production frontend or mock mode.
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const base=process.env.API_URL || 'http://127.0.0.1:8080';
const snapshot=JSON.parse(await readFile(new URL('../src/mock-scenario.json',import.meta.url),'utf8'));
const scenario=await fetch(base+'/api/scenario').then(r=>r.json());
assert.deepEqual(scenario,snapshot,'Mock snapshot must match the live Go dataset');
const local=(measure_id,district_id='nura')=>({measure_id,district_id});
const golden=[local('M7'),local('M8'),local('M10'),{measure_id:'M12'},local('M5','saryarka')];
async function post(path,decisions){const response=await fetch(base+path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({decisions})});return {status:response.status,body:await response.json()};}
const result=await post('/api/simulate',golden);
assert.equal(result.status,200);assert.equal(result.body.total_cost,95);assert.equal(result.body.remaining_budget,5);assert.equal(result.body.final_score.toFixed(2),'56.54');assert.equal(result.body.critical_before,2);assert.equal(result.body.critical_after,0);
const explanation=await post('/api/explain',golden);assert.equal(explanation.status,200);assert.deepEqual(explanation.body.result,result.body);assert.ok(['fallback','llm'].includes(explanation.body.explanation.source));assert.ok(explanation.body.explanation.text.length>0);
for(const [label,plan,code] of [
  ['four decisions',golden.slice(1),'decision_count'],
  ['budget',[local('M3'),local('M13'),local('M7'),{measure_id:'M2'},local('M5','saryarka')],'budget_exceeded'],
  ['duplicate',[local('M7'),local('M7','esil'),local('M10'),{measure_id:'M12'},local('M5','saryarka')],'duplicate_measure'],
  ['third social',[local('M7'),local('M8'),local('M9'),local('M10'),{measure_id:'M12'}],'category_limit'],
  ['global conflict',[local('M1'),local('M3','esil'),local('M9'),local('M10'),{measure_id:'M12'}],'incompatible_measures'],
  ['M4/M7 same district',[local('M4'),local('M7'),local('M10'),{measure_id:'M12'},{measure_id:'M14'}],'incompatible_measures'],
  ['M5/M13 same district',[local('M5'),local('M13'),local('M9'),local('M10'),{measure_id:'M12'}],'incompatible_measures'],
]){const response=await post('/api/simulate',plan);assert.equal(response.status,422,label);assert.ok(response.body.validation_errors.some(e=>e.code===code),label);assert.ok(!('final_score' in response.body),label);console.log('PASS',label);}
for(const pair of [['M4','M7'],['M5','M13']]){const response=await post('/api/simulate',[local(pair[0]),local(pair[1],'esil'),local('M9','almaty'),local('M10'),{measure_id:'M12'}]);assert.equal(response.status,200,pair.join('/')+' in different districts');}
const parallel=await Promise.all([post('/api/simulate',golden),post('/api/simulate',[...golden].reverse())]);assert.deepEqual(parallel[0].body,parallel[1].body);
console.log(`PASS golden: cost=${result.body.total_cost}, final_score=${result.body.final_score}, display=${result.body.final_score.toFixed(2)}, explanation=${explanation.body.explanation.source}`);
console.log('PASS catalog parity, all incompatibilities, different districts, order independence and concurrent HTTP requests');
let best;
for(let attempt=0;attempt<60;attempt++){
  const reply=await fetch(base+'/api/recommend',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({mode:'best'})});
  const answer=await reply.json();
  if(reply.status===503){assert.equal(answer.validation_errors[0].code,'not_ready');if(attempt===0)console.log('PASS live best returns structured 503 not_ready; waiting');await new Promise(resolve=>setTimeout(resolve,2000));continue;}
  assert.equal(reply.status,200);best=answer.best;break;
}
assert.ok(best,'Optimizer did not finish within 120 seconds');
const verifiedBest=await post('/api/simulate',best.decisions);assert.equal(verifiedBest.status,200);
for(const field of ['final_score','total_cost','remaining_budget','critical_after'])assert.equal(best[field],verifiedBest.body[field]);
console.log(`PASS live best: score=${best.final_score}, cost=${best.total_cost}; independently verified by Go simulate`);
