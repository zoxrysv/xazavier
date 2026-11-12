import { createElement as h, useState, useEffect } from 'react';
import { D } from '../utils/format';
import { S } from '../services/api';

function Master({ d, reload }) {
  const [cn, setCn] = useState('');
  const activeClusters = d.clusters.filter(c => !c.archived);
  const [f, setF] = useState({ name: '', cluster_id: activeClusters[0]?.id || '', tracking_type: 'duration', unit: 'minutes' });

  // sync cluster_id default when clusters first load
  useEffect(() => { if (!f.cluster_id && activeClusters[0]?.id) setF(p => ({ ...p, cluster_id: activeClusters[0].id })) }, [activeClusters.length]);

  return h('div', null, h('h1', null, 'Master Activities'), h('p', { className: 'sub' }, 'Define what gets recorded. Clusters and activities are loaded from the API, not hard-coded.'),
    h('div', { className: 'card', style: { padding: '12px 16px', marginBottom: 6 } }, h('b', null, 'Duration'), ' — recorded with a start and end time (Coding, Sleep, Gaming). ', h('br'), h('b', null, 'Count'), ' — recorded as a quantity at a moment (Smoking, Meals, Prayers).'),
    d.clusters.map(c => h('div', { key: c.id }, h('h2', { style: { display: 'flex', gap: 10, alignItems: 'center' } }, h('span', { style: { flex: 1, opacity: c.archived ? .5 : 1 } }, c.name + (c.archived ? ' (archived)' : '')),
      h('button', { className: 'btn ghost sm', onClick: async () => { const n = prompt('Rename cluster', c.name); if (n) { await S.setCluster(c.id, { name: n }); reload() } } }, 'Rename'),
      h('button', { className: 'btn ghost sm', onClick: async () => { await S.setCluster(c.id, { archived: !c.archived }); reload() } }, c.archived ? 'Restore' : 'Archive')),

      d.activities.filter(a => a.cluster_id == c.id).map(a => h('div', { className: 'row', key: a.id, style: { cursor: 'default', opacity: a.archived ? .5 : 1 } },
        h('span', { className: 'nm' }, a.name),
        h('span', { className: 'b' }, a.tracking_type),
        h('span', { className: 'val' }, a.unit + ' · since ' + D(a.created_at)),
        h('button', {
          className: 'btn ghost sm', onClick: async () => {
            const n = prompt('Rename activity', a.name);
            if (n && n.trim() && n.trim() !== a.name) { await S.setActivity(a.id, { name: n.trim() }); reload() }
          }
        }, 'Rename'),
        h('button', { className: 'btn ghost sm', onClick: async () => { await S.setActivity(a.id, { archived: !a.archived }); reload() } }, a.archived ? 'Restore' : 'Archive')
      )))),
    h('h2', null, 'New cluster'), h('div', { className: 'g2' }, h('input', { placeholder: 'Cluster name', value: cn, onChange: x => setCn(x.target.value) }), h('button', { className: 'btn', onClick: async () => { if (cn.trim()) { await S.addCluster({ name: cn.trim() }); setCn(''); reload() } } }, 'Add cluster')),
    h('h2', null, 'New activity'),
    activeClusters.length === 0
      ? h('p', { className: 'note' }, 'Create a cluster first before adding activities.')
      : h('div', null,
        h('input', { placeholder: 'Activity name', value: f.name, onChange: x => setF({ ...f, name: x.target.value }) }),
        h('div', { className: 'g2' },
          h('select', { value: f.cluster_id, onChange: x => setF({ ...f, cluster_id: x.target.value }) }, activeClusters.map(c => h('option', { key: c.id, value: c.id }, c.name))),
          h('select', { value: f.tracking_type, onChange: x => setF({ ...f, tracking_type: x.target.value, unit: x.target.value == 'duration' ? 'minutes' : f.unit === 'minutes' ? 'times' : f.unit }) }, h('option', { value: 'duration' }, 'Duration'), h('option', { value: 'count' }, 'Count'))),
        f.tracking_type == 'count' && h('input', { style: { marginTop: 12 }, placeholder: 'Unit (e.g. cigarettes)', value: f.unit === 'minutes' ? '' : f.unit, onChange: x => setF({ ...f, unit: x.target.value }) }),
        h('div', { style: { marginTop: 12 } }, h('button', { className: 'btn', disabled: !f.name.trim() || !f.cluster_id, onClick: async () => { if (f.name.trim() && f.cluster_id) { await S.addActivity({ ...f, name: f.name.trim() }); setF({ ...f, name: '' }); reload() } } }, 'Add activity'))))
}

export default Master;
