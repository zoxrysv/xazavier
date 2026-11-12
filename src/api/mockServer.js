import {MIN,HR,DAY,LATE_WINDOW,dkey,sod,ago,classify} from '../utils/format';

/* ===== api/ : mock REST server mirroring the PostgreSQL schema (users, sessions, clusters, activities, daily_logs, daily_logs_audit) ===== */
const DB_KEY='xazavier-demo-v2',DEMO='demo@xazavier.app',uuid=()=>crypto.randomUUID(),iso=t=>new Date(t).toISOString(),ACC={};let TOKEN=null;
const sha=async x=>[...new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(x)))].map(b=>b.toString(16).padStart(2,'0')).join('');
function seed(){const t0=sod(Date.now()),n=Date.now(),U='demo';
const clusters=[[1,'Foundation'],[2,'Execution'],[3,'Non-essential']].map(([id,name])=>({id,user_id:U,name,icon:null,color:null,deleted_at:null,created_at:t0-30*DAY}));
const A=(id,name,c,type,unit)=>({id,user_id:U,cluster_id:c,name,tracking_type:type,unit_name:unit,deleted_at:null,created_at:t0-30*DAY});
const activities=[A(1,'Sleep',1,'duration','minutes'),A(2,'Meal',1,'count','meals'),A(3,'Prayer',1,'count','prayers'),A(4,'Coding',2,'duration','minutes'),A(5,'Studying',2,'duration','minutes'),A(6,'Smoking',3,'count','cigarettes'),A(7,'Gaming',3,'duration','minutes')];
const logs=[],audit=[];let id=1;
const add=(a,day,sh,eh,v,lag)=>{const st=t0-day*DAY+sh*HR;if(st>n)return;const en=eh!=null?Math.min(t0-day*DAY+eh*HR,n):null,val=v!=null?v:Math.round((en-st)/MIN),cr=Math.min(n,(en||st)+lag*MIN);
logs.push({id:id++,user_id:U,activity_id:a,log_date:dkey(st),start_time:st,end_time:en,logged_value:val,entry_status:classify(st,cr)=='late'?'LATE':'NORMAL',notes:'',created_at:cr,updated_at:cr})};
for(let i=0;i<7;i++){const k=(i*3)%5;
add(1,i,0.2,6.4+k/10,null,5);add(2,i,8.9,null,1,3);add(4,i,9.5+k/20,10.3+k/10,null,i==2?500:4);add(2,i,13.2,null,1,6);add(3,i,12.1,null,1,8);
add(5,i,14.5,15.5+k/5,null,7);add(6,i,16.3,null,1+k%3,2);add(7,i,19.7,19.7+1+k*.5,null,6);add(2,i,20.4,null,1,9)}
const e=logs.find(l=>l.activity_id==4&&l.log_date==dkey(t0));if(e){const at=Math.min(n,e.created_at+90*MIN);audit.push({id:uuid(),user_id:U,log_id:e.id,action_type:'UPDATE',old_start_time:e.start_time,old_end_time:e.end_time-5*MIN,old_logged_value:e.logged_value-5,changed_at:at});e.updated_at=at}
return{users:[],sessions:[],clusters,activities,logs,audit}}
let db;try{db=JSON.parse(localStorage.getItem(DB_KEY))}catch(e){}if(!db)db=seed();
const save=()=>{try{localStorage.setItem(DB_KEY,JSON.stringify(db))}catch(e){}};
const rej=(title,message)=>{throw{title,message}},unauth=()=>{throw{status:401,title:'Session expired',message:'Please sign in again.'}};
const view=l=>({...l,status:l.entry_status.toLowerCase(),edited:db.audit.filter(a=>a.log_id==l.id&&a.action_type=='UPDATE').length});
const pub=u=>({id:u.id,email:u.email,full_name:u.full_name,timezone:u.timezone});
async function mkSession(u){const rt=uuid()+uuid(),at=uuid(),now=Date.now(),s={id:uuid(),user_id:u.id,refresh_token_hash:await sha(rt),user_agent:navigator.userAgent.slice(0,200),is_revoked:false,expires_at:now+30*DAY,created_at:now};
db.sessions.push(s);ACC[at]={sid:s.id,exp:now+15*MIN};save();return{access_token:at,refresh_token:rt,user:pub(u)}}
const request=async(method,path,body)=>{await new Promise(r=>setTimeout(r,60));const now=Date.now(),[res,id,sub]=path.split('/').filter(Boolean).slice(1);
if(res=='auth'){const em=((body&&body.email)||'').trim().toLowerCase();
if(id=='register'){if(db.users.some(u=>u.email==em))rej('Registration rejected','This email address is already registered.');if((body.password||'').length<8||!em||!body.full_name)rej('Registration rejected','Name, email and a password of at least 8 characters are required.');
const u={id:uuid(),email:em,password_hash:await sha(body.password),full_name:body.full_name,timezone:body.timezone||'Asia/Jakarta',created_at:iso(now)};db.users.push(u);return mkSession(u)}
if(id=='login'){if(em==DEMO&&!db.users.some(u=>u.email==em))db.users.push({id:'demo',email:em,password_hash:await sha('demo1234'),full_name:'Demo User',timezone:'Asia/Jakarta',created_at:iso(now)});
const u=db.users.find(x=>x.email==em);if(!u||u.password_hash!=await sha(body.password||''))rej('Sign-in failed','The email or password is incorrect.');return mkSession(u)}
const hh=await sha(body.refresh_token||''),s=db.sessions.find(x=>x.refresh_token_hash==hh);
if(id=='refresh'){if(!s||s.is_revoked||s.expires_at<now)unauth();const at=uuid();ACC[at]={sid:s.id,exp:now+15*MIN};return{access_token:at,user:pub(db.users.find(u=>u.id==s.user_id))}}
if(s){s.is_revoked=true;save()}return{ok:true}}
const ss=ACC[TOKEN]&&db.sessions.find(x=>x.id==ACC[TOKEN].sid);if(!ss||ss.is_revoked||ACC[TOKEN].exp<now)unauth();
const U=ss.user_id,own=a=>a.filter(x=>x.user_id==U);
if(res=='me')return pub(db.users.find(u=>u.id==U));
if(res=='reset'){db=seed();save();return{ok:true}}
if(res=='clusters'){if(method=='GET')return own(db.clusters);
if(method=='POST'){if(own(db.clusters).some(c=>!c.deleted_at&&c.name.toLowerCase()==body.name.toLowerCase()))rej('Cluster not created','An active cluster with this name already exists.');const c={id:uuid(),user_id:U,name:body.name,icon:null,color:null,deleted_at:null,created_at:now};db.clusters.push(c);save();return c}
const c=own(db.clusters).find(x=>x.id==id);if(body.name)c.name=body.name;if('archived'in body)c.deleted_at=body.archived?now:null;save();return c}
if(res=='activities'){if(method=='GET')return own(db.activities);
if(method=='POST'){if(!own(db.clusters).some(c=>c.id==body.cluster_id&&!c.deleted_at))rej('Activity not created','The selected cluster does not exist or is archived.');
if(own(db.activities).some(a=>!a.deleted_at&&a.cluster_id==body.cluster_id&&a.name.toLowerCase()==body.name.toLowerCase()))rej('Activity not created','An active activity with this name already exists in this cluster.');
const a={id:uuid(),user_id:U,cluster_id:body.cluster_id,name:body.name,tracking_type:body.tracking_type,unit_name:body.unit_name,deleted_at:null,created_at:now};db.activities.push(a);save();return a}
const a=own(db.activities).find(x=>x.id==id);if(body.name)a.name=body.name;if('archived'in body)a.deleted_at=body.archived?now:null;save();return a}
if(res=='audit')return own(db.audit);
if(res=='logs'){
if(sub=='audit')return own(db.audit).filter(a=>a.log_id==id).sort((x,y)=>x.changed_at-y.changed_at);
if(method=='GET'&&!id)return own(db.logs).map(view);
if(method=='POST'){const b=body,a=own(db.activities).find(x=>x.id==b.activity_id),cl=a&&own(db.clusters).find(c=>c.id==a.cluster_id);
if(!a||a.deleted_at||cl.deleted_at)rej('Recording rejected','This activity or its cluster is archived or does not exist.');
const isD=a.tracking_type=='duration';
if(isD&&(!b.end_time||b.end_time<b.start_time))rej('Recording rejected','A duration activity needs an end time that is not earlier than its start time.');
if(!isD&&!(b.logged_value>=0&&b.logged_value!==''))rej('Recording rejected','A count activity needs a non-negative quantity.');
if(b.start_time>now+5*MIN||(b.end_time||0)>now+5*MIN)rej('Recording rejected','This activity is set in the future. Only activity that has already started can be recorded.');
const lag=now-b.start_time;
if(lag>LATE_WINDOW)rej('Recording rejected','This activity started '+ago(lag)+' ago, beyond the 24-hour limit, and can no longer be added.\n\nThis restriction protects the integrity of historical data.');
const l={id:uuid(),user_id:U,activity_id:a.id,log_date:dkey(b.start_time),start_time:b.start_time,end_time:isD?b.end_time:null,logged_value:isD?Math.round((b.end_time-b.start_time)/MIN):+b.logged_value,entry_status:classify(b.start_time,now)=='late'?'LATE':'NORMAL',notes:b.notes||'',created_at:now,updated_at:now};/* created_at and entry_status are ALWAYS server-side */
db.logs.push(l);save();return view(l)}
const l=own(db.logs).find(x=>x.id==id);if(!l)rej('Not found','This record does not exist.');
if(method=='GET')return view(l);
const locked=now-l.start_time>LATE_WINDOW;
if(method=='PUT'){if(locked)rej('Edit rejected','This record started more than 24 hours ago and is permanently locked.\n\nThis restriction protects the integrity of historical data.');
const a=db.activities.find(x=>x.id==l.activity_id),o={t:l.end_time,v:l.logged_value};
if(a.tracking_type=='duration'&&body.end_time!=null){if(body.end_time<l.start_time)rej('Edit rejected','The end time cannot be earlier than the start time.');if(body.end_time>now+5*MIN)rej('Edit rejected','The end time is in the future.');l.end_time=body.end_time;l.logged_value=Math.round((l.end_time-l.start_time)/MIN)}
if(a.tracking_type=='count'&&body.logged_value!=null){if(!(body.logged_value>=0))rej('Edit rejected','The quantity cannot be negative.');l.logged_value=+body.logged_value}
if(body.notes!==undefined)l.notes=body.notes;
if(o.t!==l.end_time||o.v!==l.logged_value)db.audit.push({id:uuid(),user_id:U,log_id:l.id,action_type:'UPDATE',old_start_time:l.start_time,old_end_time:o.t,old_logged_value:o.v,changed_at:now});
l.updated_at=now;save();return view(l)}
if(locked)rej('Delete rejected','This record started more than 24 hours ago and is locked.');
db.audit.push({id:uuid(),user_id:U,log_id:l.id,action_type:'DELETE',old_start_time:l.start_time,old_end_time:l.end_time,old_logged_value:l.logged_value,changed_at:now});db.logs=db.logs.filter(x=>x!==l);save();return{ok:true}}};

export const setToken=t=>{TOKEN=t};
export {request};
