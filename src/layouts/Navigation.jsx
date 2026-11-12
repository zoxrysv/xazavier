import {createElement as h} from 'react';

const NAV=[['dash','Dashboard'],['log','Activity Log'],['add','Add Activity'],['ana','Analytics'],['master','Master Activities'],['set','Settings']];

export default function Navigation({pg,setPg}){return h('nav',{className:'side'},h('div',{className:'brand'},'Xazavier'),h('div',{className:'tag'},'Record → Verify → Observe → Reflect'),NAV.map(([k,v])=>h('button',{key:k,className:pg==k?'on':'',onClick:()=>setPg(k)},k=='master'?'Master':v)))}
