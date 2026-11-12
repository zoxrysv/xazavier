import {createElement as h,useState,useEffect} from 'react';
import {LATE_WINDOW,T,D,DT,dkey,dur,ago} from '../utils/format';
import {fmtVal} from './ui';
import {S} from '../services/api';

function Detail({id,d,reload,toast,close}){const[l,setL]=useState(null),[au,setAu]=useState([]),[ed,setEd]=useState(false),[v,setV]=useState(''),[et,setEt]=useState(''),[n,setN]=useState(''),[err,setErr]=useState(null);
const load=async()=>{const x=await S.log(id);setL(x);setV(x.logged_value);setEt(x.end_time?T(x.end_time):'');setN(x.notes||'');setAu((await S.audit(id)).filter(a=>a.action_type=='UPDATE'))};useEffect(()=>{load()},[id]);if(!l)return null;
const a=d.activities.find(x=>x.id==l.activity_id),cl=d.clusters.find(c=>c.id==a.cluster_id),isD=a.tracking_type=='duration',locked=Date.now()-l.start_time>LATE_WINDOW,fv=x=>x==null?'—':isD?dur(Math.round(x)):x,ft=x=>x?T(x):'—';
const ch=au.map((x,i)=>{const nx=au[i+1],nE=nx?nx.old_end_time:l.end_time,nV=nx?nx.old_logged_value:l.logged_value,r=[];if(x.old_end_time!=nE)r.push(['End time',ft(x.old_end_time),ft(nE)]);if(x.old_logged_value!=nV)r.push([isD?'Duration':'Quantity',fv(x.old_logged_value),fv(nV)]);return{at:x.changed_at,r}});
const save=async()=>{setErr(null);try{await S.setLog(id,isD?{end_time:new Date(dkey(l.end_time||l.start_time)+'T'+et).getTime(),notes:n}:{logged_value:v,notes:n});await load();await reload();setEd(false);toast('↻ History updated')}catch(x){setErr(x);toast('✕ Edit rejected')}};
const st={color:'var(--mu)',padding:0,fontSize:12};
return h('div',{className:'modal',onClick:close},h('div',{className:'sheet',onClick:x=>x.stopPropagation()},
h('div',{style:{display:'flex',justifyContent:'space-between'}},h('span',{className:'sub'},cl?cl.name:''),h('button',{className:'btn ghost sm',onClick:close},'Close')),
h('h1',null,a.name),h('p',{className:'sub'},fmtVal(a,l)),
h('dl',{className:'kv'},h('dt',null,'Activity time'),h('dd',{className:'mono'},D(l.start_time),h('br'),T(l.start_time)+(l.end_time?' — '+T(l.end_time):'')),
h('dt',null,'Recorded'),h('dd',{className:'mono'},DT(l.created_at)),h('dt',null,'Updated'),h('dd',{className:'mono'},DT(l.updated_at)),
h('dt',null,'Entry type'),h('dd',null,l.status=='late'?'⚠ Late entry — recorded '+ago(l.created_at-l.start_time)+' after the activity started':'✓ Normal entry'),
h('dt',null,'Notes'),h('dd',null,l.notes||'—')),
err&&h('div',{className:'status rej'},h('b',null,'✕ '+err.title),h('small',{style:{whiteSpace:'pre-line'}},err.message)),
ed?h('div',null,isD?h('div',null,h('label',null,'End time'),h('input',{type:'time',value:et,onChange:x=>setEt(x.target.value)})):h('div',null,h('label',null,'Quantity'),h('input',{type:'number',min:0,value:v,onChange:x=>setV(x.target.value)})),
h('label',null,'Notes'),h('textarea',{rows:2,value:n,onChange:x=>setN(x.target.value)}),h('p',{className:'note'},'Changed times and values keep their previous version in the audit trail. Notes are not audited.'),h('button',{className:'btn sm',onClick:save},'Save change')):
locked?h('p',{className:'note'},'🔒 This record started more than 24 hours ago and is permanently locked.'):
h('div',{style:{display:'flex',gap:8,marginTop:18}},h('button',{className:'btn ghost sm',onClick:()=>setEd(true)},'Edit'),h('button',{className:'btn ghost sm',onClick:async()=>{if(confirm('Delete this record? The record is removed; its deletion stays in the audit trail.')){try{await S.delLog(id);await reload();close();toast('↻ History updated')}catch(x){setErr(x)}}}},'Delete')),
h('h2',null,'History'),h('div',{className:'aud'},h('div',null,h('b',null,'Record created'),h('div',{className:'mono',style:st},DT(l.created_at))),
ch.map((x,i)=>h('div',{key:i},h('b',null,'Record changed'),h('div',{className:'mono',style:st},DT(x.at)),x.r.map(([k,p,q])=>h('div',{key:k,style:{padding:0}},k+': '+p+' → '+q)))))))}

export default Detail;
