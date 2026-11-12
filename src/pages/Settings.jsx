import {createElement as h} from 'react';
import {S} from '../services/api';

function Settings({reload,theme,setTheme,user,logout}){return h('div',null,h('h1',null,'Settings'),h('p',{className:'sub'},'Record reality, not the version of yourself you wish to be.'),h('h2',null,'Account'),h('div',{className:'card'},[['Name',user.full_name],['Email',user.email],['Timezone',user.timezone]].map(([k,v])=>h('div',{className:'row',key:k,style:{cursor:'default'}},h('span',{className:'nm'},k),h('span',{className:'val'},v)))),h('div',{style:{marginTop:12}},h('button',{className:'btn ghost sm',onClick:logout},'Sign out')),
h('h2',null,'Appearance'),h('select',{value:theme,onChange:x=>setTheme(x.target.value)},['auto','light','dark'].map(t=>h('option',{key:t},t))),
h('h2',null,'Integrity rules (enforced by the server)'),h('div',{className:'card'},[['Normal entry','Within 3 hours of start, same local day'],['Late entry','After 3 hours, or on a different day'],['Beyond 24 hours','Rejected — cannot be backdated'],['Editing','Locked 24 hours after the start time'],['Recording time','Set by the server, never by the client'],['Audit trail','Previous start, end and value; notes are not audited']].map(([k,v])=>h('div',{className:'row',key:k,style:{cursor:'default'}},h('span',{className:'nm'},k),h('span',{className:'val'},v)))),
h('h2',null,'Demo data'),h('p',{className:'note'},'This build runs on a local mock of the REST API.'),h('button',{className:'btn ghost sm',onClick:async()=>{if(confirm('Replace all demo data?')){await S.reset();location.reload()}}},'Reset demo data'))}

export default Settings;
