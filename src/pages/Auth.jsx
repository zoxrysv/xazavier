import {createElement as h,useState} from 'react';
import {S} from '../services/api';

function Auth({onAuth}){const[mode,setM]=useState('login'),[f,setF]=useState({email:'',password:'',full_name:''}),[err,setErr]=useState(null),[busy,setB]=useState(false),set=k=>x=>setF({...f,[k]:x.target.value}),L=mode=='login';
const go=async()=>{setB(true);setErr(null);try{onAuth(L?await S.login(f):await S.register({...f,timezone:Intl.DateTimeFormat().resolvedOptions().timeZone||'Asia/Jakarta'}))}catch(x){setErr(x)}setB(false)};
return h('div',{style:{maxWidth:380,margin:'0 auto',padding:'12vh 22px'}},h('div',{className:'brand'},'Xazavier'),h('h1',{style:{marginTop:18}},L?'Sign in':'Create account'),h('p',{className:'sub'},'Record reality, not the version of yourself you wish to be.'),
!L&&h('div',null,h('label',null,'Full name'),h('input',{value:f.full_name,onChange:set('full_name')})),
h('label',null,'Email'),h('input',{type:'email',value:f.email,onChange:set('email'),autoComplete:'email'}),h('label',null,'Password'),h('input',{type:'password',value:f.password,onChange:set('password'),onKeyDown:x=>x.key=='Enter'&&go()}),
err&&h('div',{className:'status rej'},h('b',null,'✕ '+err.title),h('small',null,err.message)),
h('div',{style:{marginTop:18,display:'flex',gap:10}},h('button',{className:'btn',disabled:busy,onClick:go},L?'Sign in':'Create account'),h('button',{className:'btn ghost',onClick:()=>{setM(L?'register':'login');setErr(null)}},L?'Register':'Back to sign in')),
L&&h('p',{className:'note'},'Demo account: demo@xazavier.app / demo1234'))}

export default Auth;
