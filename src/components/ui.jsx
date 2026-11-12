import {createElement as h} from 'react';
import {T,dur} from '../utils/format';

const fmtVal=(a,l)=>l.logged_value==null?'in progress':a.tracking_type=='duration'?dur(l.logged_value):l.logged_value+' '+(l.logged_value==1?a.unit.replace(/s$/,''):a.unit);
const Badges=({l})=>[l.status=='late'&&h('span',{className:'b late',key:1},'Late entry'),l.edited>0&&h('span',{className:'b ed',key:2},'Edited ×'+l.edited)];
const Flow=({on})=>h('div',{className:'flow'},['RECORD','VERIFY','OBSERVE','REFLECT'].map((s,i)=>h('span',{key:s},i?'  →  ':'',on==s?h('b',null,s):s)));


function Row({l,acts,open}){const a=acts[l.activity_id];return h('div',{className:'row',onClick:()=>open(l.id)},h('time',null,T(l.start_time)),h('span',{className:'nm'},a.name),h(Badges,{l}),h('span',{className:'val mono'},fmtVal(a,l)))}

export {fmtVal,Badges,Flow,Row};
