import {createElement as h,useState,useEffect} from 'react';
import {T,D,DT} from '../utils/format';
import {Row} from '../components/ui';
import {S} from '../services/api';

function Log({d,open}){const[del,setDel]=useState([]);useEffect(()=>{S.auditAll().then(r=>setDel(r.filter(a=>a.action_type=='DELETE')))},[d]);const acts=Object.fromEntries(d.activities.map(a=>[a.id,a])),g={};[...d.logs].sort((a,b)=>b.start_time-a.start_time).forEach(l=>(g[l.log_date]=g[l.log_date]||[]).push(l));
return h('div',null,h('h1',null,'Activity Log'),h('p',{className:'sub'},'Every record, in activity-time order. Open one to see when it was created and how it changed.'),
Object.keys(g).length?Object.entries(g).map(([k,ls])=>h('div',{key:k},h('h2',null,D(ls[0].start_time)),ls.sort((a,b)=>a.start_time-b.start_time).map(l=>h(Row,{key:l.id,l,acts,open})))):h('div',{className:'empty'},'No records exist for this period.'),del.length>0&&h('div',null,h('h2',null,'Deleted records'),h('p',{className:'note'},'Deleted records remain visible here as audit entries.'),del.map(x=>h('div',{className:'row',key:x.id,style:{cursor:'default'}},h('time',null,T(x.old_start_time)),h('span',{className:'nm'},D(x.old_start_time)+' — removed '+DT(x.changed_at)),h('span',{className:'val mono'},x.old_logged_value==null?'—':String(x.old_logged_value))))))}

export default Log;
