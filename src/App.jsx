import {createElement as h,useState,useEffect} from 'react';
import {S} from './services/api';

import Dashboard from './pages/Dashboard';
import Log from './pages/ActivityLog';
import Add from './pages/AddActivity';
import Analytics from './pages/Analytics';
import Master from './pages/MasterActivities';
import Settings from './pages/Settings';
import Auth from './pages/Auth';
import Detail from './components/ActivityDetail';
import Navigation from './layouts/Navigation';
import {useData} from './hooks/useData';
function App(){const[user,setUser]=useState(undefined),[d,reload]=useData(user),[pg,setPg]=useState('dash'),[open,setOpen]=useState(null),[tt,setTt]=useState(null),[theme,setThemeS]=useState('auto');
useEffect(()=>{S.restore().then(setUser);const f=()=>setUser(null);window.addEventListener('xz-logout',f);return()=>window.removeEventListener('xz-logout',f)},[]);
const toast=m=>{setTt(m);setTimeout(()=>setTt(null),2200)},logout=async()=>{await S.logout();setUser(null);setPg('dash')};
const setTheme=t=>{setThemeS(t);if(t=='auto')document.documentElement.removeAttribute('data-theme');else document.documentElement.setAttribute('data-theme',t)};
if(user===undefined)return h('div',{className:'empty',style:{padding:40}},'Checking session…');
if(!user)return h(Auth,{onAuth:setUser});
let body=h('div',{className:'empty'},'Loading records…');
if(d){const P={d,reload,toast,open:setOpen,go:setPg};body={dash:h(Dashboard,P),log:h(Log,P),add:h(Add,P),ana:h(Analytics,P),master:h(Master,P),set:h(Settings,{reload,theme,setTheme,user,logout})}[pg]}
return h('div',{className:'app'},h(Navigation,{pg,setPg}),
h('main',null,body),open&&d&&h(Detail,{id:open,d,reload,toast,close:()=>setOpen(null)}),tt&&h('div',{className:'toast'},tt))}

export default App;
