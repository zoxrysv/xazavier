import {useState,useEffect,useCallback} from 'react';
import {S} from '../services/api';

function useData(user){const[d,setD]=useState(null);const load=useCallback(async()=>{const[c,a,l]=await Promise.all([S.clusters(),S.activities(),S.logs()]);setD({clusters:[...c],activities:[...a],logs:[...l]})},[]);useEffect(()=>{if(user)load();else setD(null)},[user&&user.id]);return[d,load]}

export {useData};
