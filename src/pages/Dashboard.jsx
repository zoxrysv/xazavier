import {createElement as h} from 'react';
import {DAY,sod} from '../utils/format';
import {Flow,Row} from '../components/ui';

function Dashboard({d,open}){const acts=Object.fromEntries(d.activities.map(a=>[a.id,a])),t0=sod(Date.now());
const today=d.logs.filter(l=>l.start_time>=t0).sort((a,b)=>a.start_time-b.start_time),late=today.filter(l=>l.status=='late').length,ed=today.filter(l=>l.edited).length;
const past=d.logs.filter(l=>l.start_time<t0&&l.start_time>=t0-7*DAY),exp=d.activities.filter(a=>!a.archived&&new Set(past.filter(l=>l.activity_id==a.id).map(l=>l.log_date)).size>=4),missing=exp.filter(a=>!today.some(l=>l.activity_id==a.id)),comp=exp.length?Math.round(100*(exp.length-missing.length)/exp.length):100;
return h('div',null,h(Flow,{on:'OBSERVE'}),h('h1',null,'What actually happened'),h('p',{className:'sub'},new Date().toLocaleDateString('en-GB',{weekday:'long',day:'numeric',month:'long',year:'numeric'})),
h('div',{className:'stats'},[[today.length,'Recorded today'],[late,'Late entries'],[ed,'Edited records'],[comp+'%','Data completeness']].map(([v,k])=>h('div',{key:k},h('b',{className:'mono'},v),h('span',null,k)))),
h('p',{className:'note'},missing.length?'No record yet today for activities recorded on most recent days: '+missing.map(a=>a.name).join(', ')+'. Completeness measures recorded data, not performance.':'All activities recorded on most recent days have a record today. Completeness measures recorded data, not performance.'),
h('h2',null,'Today'),today.length?today.map(l=>h(Row,{key:l.id,l,acts,open})):h('div',{className:'empty'},'No activity has been recorded today.'))}

export default Dashboard;
