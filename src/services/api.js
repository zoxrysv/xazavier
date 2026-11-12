import {request as mockRequest, setToken as setMockToken} from '../api/mockServer';

// In dev, Vite proxies /api/* → backend (see vite.config.js), so API_BASE stays ''.
// In production build, set VITE_API_URL to the backend origin if on a different host.
const IS_MOCK = !import.meta.env.VITE_API_URL;
const API_BASE = IS_MOCK ? '' : '';  // always use relative path; proxy handles it in dev
let liveToken = null;

const setToken = t => {
  liveToken = t;
  setMockToken(t);
};

const request = async (method, path, body) => {
  if (IS_MOCK) {
    return mockRequest(method, path, body);
  }
  const headers = { 'Content-Type': 'application/json' };
  if (liveToken) {
    headers['Authorization'] = `Bearer ${liveToken}`;
  }
  const res = await fetch(`${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    let errPayload = {};
    try {
      errPayload = await res.json();
    } catch (_) {
      errPayload = { title: res.statusText || 'Error', message: 'Request failed' };
    }
    const err = new Error(errPayload.message || 'Request failed');
    err.status = res.status;
    err.title = errPayload.title || 'Error';
    err.message = errPayload.message || 'Request failed';
    err.code = errPayload.code;
    throw err;
  }
  return res.json();
};

/* ===== services/ : API shape -> UI shape (ISO strings, DECIMAL strings, deleted_at) ===== */
const tm=x=>x==null?null:+new Date(x),nm=x=>x==null?null:Number(x);
const lgIn=l=>({...l,start_time:tm(l.start_time),end_time:tm(l.end_time),created_at:tm(l.created_at),updated_at:tm(l.updated_at),logged_value:nm(l.logged_value)});
const auIn=a=>({...a,changed_at:tm(a.changed_at),old_start_time:tm(a.old_start_time),old_end_time:tm(a.old_end_time),old_logged_value:nm(a.old_logged_value)});
const aIn=a=>({...a,unit:a.unit_name,archived:!!a.deleted_at}),cIn=c=>({...c,archived:!!c.deleted_at}),arch=b=>{const o={...b};if('unit'in o){o.unit_name=o.unit;delete o.unit}return o};
const RT='xazavier-refresh',sto={get:()=>{try{return localStorage.getItem(RT)}catch(e){return null}},set:v=>{try{v?localStorage.setItem(RT,v):localStorage.removeItem(RT)}catch(e){}}};
const begin=async r=>{setToken(r.access_token);sto.set(r.refresh_token);return r.user};
const call=async(m,p,b)=>{try{return await request(m,p,b)}catch(e){if(e.status==401){if(await S.restore())return request(m,p,b);window.dispatchEvent(new Event('xz-logout'))}throw e}};
const S={login:async b=>begin(await request('POST','/api/auth/login',b)),register:async b=>begin(await request('POST','/api/auth/register',b)),
restore:async()=>{const t=sto.get();if(!t)return null;try{const r=await request('POST','/api/auth/refresh',{refresh_token:t});setToken(r.access_token);return r.user}catch(e){sto.set(null);return null}},
logout:async()=>{const t=sto.get();sto.set(null);try{await request('POST','/api/auth/logout',{refresh_token:t})}catch(e){}setToken(null)},
clusters:async()=>(await call('GET','/api/clusters')).map(cIn),addCluster:b=>call('POST','/api/clusters',b),setCluster:(i,b)=>call('PUT','/api/clusters/'+i,b),
activities:async()=>(await call('GET','/api/activities')).map(aIn),addActivity:b=>call('POST','/api/activities',arch(b)),setActivity:(i,b)=>call('PUT','/api/activities/'+i,arch(b)),
logs:async()=>(await call('GET','/api/logs')).map(lgIn),log:async i=>lgIn(await call('GET','/api/logs/'+i)),addLog:async b=>lgIn(await call('POST','/api/logs',b)),setLog:async(i,b)=>lgIn(await call('PUT','/api/logs/'+i,b)),delLog:i=>call('DELETE','/api/logs/'+i),
audit:async i=>(await call('GET','/api/logs/'+i+'/audit')).map(auIn),auditAll:async()=>(await call('GET','/api/audit')).map(auIn),reset:()=>call('POST','/api/reset')};

export {S};
