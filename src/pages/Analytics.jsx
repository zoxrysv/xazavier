import {createElement as h,useState} from 'react';
import {DAY,D,sod,dur} from '../utils/format';
import {Flow} from '../components/ui';

function Analytics({d}){const t0=sod(Date.now()),from=t0-6*DAY,pf=from-7*DAY,inR=(l,a,b)=>l.start_time>=a&&l.start_time<b,cur=d.logs.filter(l=>inR(l,from,t0+DAY)),prev=d.logs.filter(l=>inR(l,pf,from)),A=Object.fromEntries(d.activities.map(a=>[a.id,a]));
const[sel,setSel]=useState(d.activities.find(a=>a.name=='Smoking')?.id||d.activities[0]?.id);
if(!cur.length)return h('div',null,h('h1',null,'Analytics'),h('div',{className:'empty'},'Analytics require recorded activity data.'));
const tot={},cnt={},ptc={};cur.forEach(l=>{const a=A[l.activity_id];if(a.tracking_type=='duration')tot[a.name]=(tot[a.name]||0)+l.logged_value;cnt[a.name]=(cnt[a.name]||0)+(a.tracking_type=='count'?l.logged_value:1)});
prev.forEach(l=>{const a=A[l.activity_id];ptc[a.name]=(ptc[a.name]||0)+(a.tracking_type=='count'?l.logged_value:1)});
const sum=Object.values(tot).reduce((x,y)=>x+y,0),ent=Object.entries(tot).sort((a,b)=>b[1]-a[1]),mx=ent[0]?ent[0][1]:1,lates=cur.filter(l=>l.status=='late').length,eds=cur.filter(l=>l.edited).length;
const missDays=[...Array(7)].filter((_,i)=>!cur.some(l=>l.start_time>=from+i*DAY&&l.start_time<from+(i+1)*DAY)).length;
const sa=A[sel],days=[...Array(7)].map((_,i)=>{const ls=cur.filter(l=>l.activity_id==sel&&l.start_time>=from+i*DAY&&l.start_time<from+(i+1)*DAY);return{k:new Date(from+i*DAY).toLocaleDateString('en-GB',{weekday:'short'}),v:ls.reduce((x,l)=>x+l.logged_value,0)}}),dm=Math.max(1,...days.map(x=>x.v));
const obs=[];if(ent[0])obs.push(ent[0][0]+' accounted for '+Math.round(100*ent[0][1]/sum)+'% of recorded activity time.');obs.push(lates+' record'+(lates==1?' was':'s were')+' entered late in this period.');
['Smoking'].forEach(n=>{if(cnt[n]!=null&&ptc[n]!=null)obs.push('Smoking frequency '+(cnt[n]>ptc[n]?'increased':cnt[n]<ptc[n]?'decreased':'did not change')+' compared with the previous 7-day period ('+ptc[n]+' → '+cnt[n]+').')});
const sl=cur.filter(l=>A[l.activity_id].name=='Sleep');if(sl.length)obs.push('Average recorded sleep: '+dur(Math.round(sl.reduce((x,l)=>x+l.logged_value,0)/sl.length))+' across '+sl.length+' records.');
return h('div',null,h(Flow,{on:'REFLECT'}),h('h1',null,'Analytics'),h('p',{className:'sub'},'Observations generated only from recorded data.'),
h('div',{className:'meta'},h('span',null,'Data period ',h('b',null,D(from)+' – '+D(t0))),h('span',null,'Records ',h('b',null,cur.length)),h('span',null,'Late entries ',h('b',null,lates)),h('span',null,'Edited records ',h('b',null,eds)),h('span',null,'Missing days ',h('b',null,missDays))),
h('h2',null,'Observations'),obs.map((o,i)=>h('p',{key:i,style:{margin:'6px 0'}},o)),
h('h2',null,'Time per activity'),ent.map(([k,v])=>h('div',{className:'bar',key:k},h('span',null,k),h('i',{style:{width:Math.max(2,v/mx*55)+'%'}}),h('span',{className:'mono val'},dur(v)+' · '+Math.round(100*v/sum)+'%'))),
h('h2',null,'Activity frequency'),h('div',{className:'card'},Object.entries(cnt).map(([k,v])=>h('div',{className:'row',key:k},h('span',{className:'nm'},k),h('span',{className:'val mono'},v)))),
h('h2',null,'Daily values'),h('select',{value:sel,onChange:x=>setSel(x.target.value),style:{marginBottom:14}},d.activities.map(a=>h('option',{key:a.id,value:a.id},a.name))),
h('div',{className:'cols'},days.map((x,i)=>h('div',{key:i},x.v?x.v+'':'',h('i',{style:{height:x.v/dm*80+'%'}})))),h('div',{className:'cols',style:{height:20,border:0}},days.map((x,i)=>h('div',{key:i,style:{justifyContent:'flex-start'}},x.k))),
h('p',{className:'note'},sa&&sa.tracking_type=='duration'?'Values in minutes.':'Values in '+(sa?sa.unit:'')+'.'))}

export default Analytics;
