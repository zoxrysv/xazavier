import {createElement as h,useState} from 'react';
import {MIN,T,dkey,ago,classify} from '../utils/format';
import {Flow} from '../components/ui';
import {S} from '../services/api';

function Add({d,reload,toast,go}){const act=d.activities.filter(a=>!a.archived),[aid,setA]=useState(''),[date,setDate]=useState(dkey(Date.now())),[s,setS]=useState(T(Date.now()-30*MIN)),[e,setE]=useState(T(Date.now())),[val,setVal]=useState(1),[notes,setN]=useState(''),[err,setErr]=useState(null),[busy,setB]=useState(false);
const a=act.find(x=>x.id==aid),isD=a&&a.tracking_type=='duration',ts=x=>new Date(date+'T'+x).getTime();
const st=a?classify(ts(s),Date.now()):null,lag=a?Date.now()-ts(s):0;
const submit=async()=>{setB(true);setErr(null);try{const l=await S.addLog({activity_id:aid,start_time:ts(s),end_time:isD?ts(e):null,logged_value:val,notes});await reload();toast(l.status=='late'?'⚠ Late entry recorded':'✓ Record saved');go('log')}catch(x){setErr(x);toast('✕ Record rejected')}setB(false)};
return h('div',null,h(Flow,{on:'RECORD'}),h('h1',null,'Add Activity'),h('p',{className:'sub'},'Record what happened, when it happened. The recording time is set by the server.'),
h('label',null,'Activity'),h('select',{value:aid,onChange:x=>{setA(x.target.value);setErr(null)}},h('option',{value:''},'Select…'),act.map(x=>h('option',{key:x.id,value:x.id},x.name+' — '+x.tracking_type))),
a&&h('div',null,h('label',null,'Date'),h('input',{type:'date',value:date,max:dkey(Date.now()),onChange:x=>setDate(x.target.value)}),
isD?h('div',{className:'g2'},h('div',null,h('label',null,'Start time'),h('input',{type:'time',value:s,onChange:x=>setS(x.target.value)})),h('div',null,h('label',null,'End time'),h('input',{type:'time',value:e,onChange:x=>setE(x.target.value)})))
:h('div',{className:'g2'},h('div',null,h('label',null,'Quantity ('+a.unit+')'),h('input',{type:'number',min:0,value:val,onChange:x=>setVal(x.target.value)})),h('div',null,h('label',null,'Time'),h('input',{type:'time',value:s,onChange:x=>setS(x.target.value)}))),
h('label',null,'Notes (optional)'),h('textarea',{rows:2,value:notes,onChange:x=>setN(x.target.value)}),
err?h('div',{className:'status rej'},h('b',null,'✕ '+err.title),h('small',{style:{whiteSpace:'pre-line'}},err.message)):
st=='normal'?h('div',{className:'status'},h('b',null,'✓ Recorded on time'),h('small',null,'Within 3 hours of the start time, on the same day.')):
st=='late'?h('div',{className:'status late'},h('b',null,'⚠ Late entry'),h('small',null,'This activity started '+ago(lag)+' ago (over 3 hours, or on a different day). It will be permanently marked LATE.')):
h('div',{className:'status rej'},h('b',null,'✕ Recording will be rejected'),h('small',null,'This activity started '+ago(lag)+' ago. Entries older than 24 hours can no longer be backdated, which protects the integrity of historical data.')),
h('div',{style:{marginTop:16}},h('button',{className:'btn',disabled:busy||st=='rejected',onClick:submit},'Save record'))))}

export default Add;
