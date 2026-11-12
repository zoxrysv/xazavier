import {createElement as h} from 'react';

const MIN=60000,HR=3600000,DAY=86400000,LATE_WINDOW=24*HR,LATE_AFTER=3*HR;
const pad=n=>String(n).padStart(2,'0'),T=t=>{const d=new Date(t);return pad(d.getHours())+':'+pad(d.getMinutes())},
D=t=>new Date(t).toLocaleDateString('en-GB',{day:'2-digit',month:'short',year:'numeric'}),
DT=t=>D(t)+' — '+T(t),dkey=t=>{const d=new Date(t);return d.getFullYear()+'-'+pad(d.getMonth()+1)+'-'+pad(d.getDate())},
sod=t=>{const d=new Date(t);d.setHours(0,0,0,0);return d.getTime()},
dur=m=>m<60?m+'m':Math.floor(m/60)+'h '+pad(m%60)+'m',
ago=ms=>{const m=Math.round(ms/MIN);return dur(Math.max(m,0))};
/* Rule communication only. The server is authoritative. */
function classify(start,created){const lag=created-start;return lag>LATE_WINDOW?'rejected':(lag>LATE_AFTER||dkey(created)!=dkey(start))?'late':'normal'}

export {MIN,HR,DAY,LATE_WINDOW,LATE_AFTER,pad,T,D,DT,dkey,sod,dur,ago,classify};
