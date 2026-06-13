'use strict';
'require view';
'require rpc';
'require ui';

var callNodes = rpc.declare({
	object: 'singbox.manager',
	method: 'nodes',
	expect: { '': {} }
});

var callSetNode = rpc.declare({
	object: 'singbox.manager',
	method: 'node_set',
	params: [ 'node' ],
	expect: { '': {} }
});

var callDeleteNode = rpc.declare({
	object: 'singbox.manager',
	method: 'node_delete',
	params: [ 'id' ],
	expect: { '': {} }
});

var callSelectNode = rpc.declare({
	object: 'singbox.manager',
	method: 'node_select',
	params: [ 'id' ],
	expect: { '': {} }
});

var callStatus = rpc.declare({
	object: 'singbox.manager',
	method: 'status',
	expect: { '': {} }
});

var callReload = rpc.declare({
	object: 'singbox.manager',
	method: 'reload',
	expect: { '': {} }
});

var callNodePingTest = rpc.declare({
	object: 'singbox.manager',
	method: 'node_ping_test',
	params: [ 'id' ],
	expect: { '': {} }
});

var callNodeLatencyTest = rpc.declare({
	object: 'singbox.manager',
	method: 'node_latency_test',
	params: [ 'id', 'url' ],
	expect: { '': {} }
});

function valueOrDash(value) {
	if (value === null || value === undefined || value === '')
		return '-';
	return value;
}

function field(root, name) {
	return root.querySelector('[name="%s"]'.format(name));
}

function formValue(root, name) {
	var input = field(root, name);
	return input ? input.value.trim() : '';
}

function readNode(root, row) {
	row = row || {};
	var port = Number(formValue(root, 'port') || 0);
	return {
		id: row.id || formValue(root, 'id'),
		enabled: field(root, 'enabled').checked,
		name: formValue(root, 'name'),
		type: formValue(root, 'type'),
		server: formValue(root, 'server'),
		port: port,
		uuid: formValue(root, 'uuid'),
		password: formValue(root, 'password'),
		method: formValue(root, 'method'),
		security: formValue(root, 'security'),
		tls: field(root, 'tls').checked,
		transport: formValue(root, 'transport'),
		sni: formValue(root, 'sni'),
		tag: formValue(root, 'tag')
	};
}

function fillForm(root, node) {
	[ 'id', 'name', 'type', 'server', 'port', 'uuid', 'password', 'method', 'security', 'transport', 'sni', 'tag' ].forEach(function(name) {
		var input = field(root, name);
		if (input)
			input.value = node[name] || '';
	});
	field(root, 'enabled').checked = node.enabled !== false;
	field(root, 'tls').checked = !!node.tls;
}

function refreshView(view) {
	return view.load().then(function(data) {
		var replacement = view.render(data);
		var root = document.querySelector('.singbox-manager-page');
		if (root)
			root.parentNode.replaceChild(replacement, root);
	});
}

function subscriptionMap(subscriptions) {
	var byID = {};
	(subscriptions || []).forEach(function(row) {
		byID[row.id] = row;
	});
	return byID;
}

function sourceLabel(id, subscriptions) {
	if (!id)
		return _('Manual');
	return subscriptions[id] ? (subscriptions[id].name || id) : id;
}

function filterID(node) {
	return node.subscription ? 'sub:' + node.subscription : 'manual';
}

function availableFilters(nodes, subscriptions) {
	var filters = [
		{ id: 'all', label: _('All') },
		{ id: 'manual', label: _('Manual') }
	];
	var seen = {};
	nodes.forEach(function(node) {
		if (!node.subscription || seen[node.subscription])
			return;
		seen[node.subscription] = true;
		filters.push({
			id: 'sub:' + node.subscription,
			label: sourceLabel(node.subscription, subscriptions)
		});
	});
	return filters;
}

function filteredNodes(nodes, active) {
	if (!active || active === 'all')
		return nodes;
	return nodes.filter(function(node) {
		return filterID(node) === active;
	});
}

function groupedNodes(nodes, subscriptions, active) {
	var groups = [];
	var bySource = {};
	filteredNodes(nodes, active).forEach(function(node) {
		var id = filterID(node);
		if (!bySource[id]) {
			bySource[id] = {
				label: sourceLabel(node.subscription, subscriptions),
				nodes: []
			};
			groups.push(bySource[id]);
		}
		bySource[id].nodes.push(node);
	});
	return groups;
}

function renderFilters(view, nodes, subscriptions) {
	var active = view.activeNodeFilter || 'all';
	return E('div', { 'class': 'singbox-manager-filters' }, availableFilters(nodes, subscriptions).map(function(filter) {
		return E('button', {
			'class': 'btn cbi-button' + (active === filter.id ? ' cbi-button-apply' : ''),
			'click': ui.createHandlerFn(view, function() {
				view.activeNodeFilter = filter.id;
				return refreshView(view);
			})
		}, filter.label);
	}));
}

function showNodeModal(view, row) {
	row = row || { enabled: true, type: 'shadowsocks' };
	ui.showModal(row.id ? _('Edit Node') : _('Add Node'), [
		E('div', { 'class': 'singbox-manager-modal-form' }, [
			E('label', {}, [ _('ID'), E('input', { 'class': 'cbi-input-text', 'name': 'id' }) ]),
			E('label', {}, [ _('Name'), E('input', { 'class': 'cbi-input-text', 'name': 'name' }) ]),
			E('label', {}, [
				_('Type'),
				E('select', { 'class': 'cbi-input-select', 'name': 'type' }, [
					E('option', { 'value': 'shadowsocks' }, 'shadowsocks'),
					E('option', { 'value': 'trojan' }, 'trojan'),
					E('option', { 'value': 'vmess' }, 'vmess'),
					E('option', { 'value': 'vless' }, 'vless'),
					E('option', { 'value': 'hysteria2' }, 'hysteria2'),
					E('option', { 'value': 'tuic' }, 'tuic'),
					E('option', { 'value': 'direct' }, 'direct')
				])
			]),
			E('label', {}, [ _('Server'), E('input', { 'class': 'cbi-input-text', 'name': 'server' }) ]),
			E('label', {}, [ _('Port'), E('input', { 'class': 'cbi-input-text', 'name': 'port', 'type': 'number', 'min': '1', 'max': '65535' }) ]),
			E('label', {}, [ _('UUID'), E('input', { 'class': 'cbi-input-text', 'name': 'uuid' }) ]),
			E('label', {}, [ _('Password'), E('input', { 'class': 'cbi-input-password', 'name': 'password', 'type': 'password' }) ]),
			E('label', {}, [ _('Method'), E('input', { 'class': 'cbi-input-text', 'name': 'method' }) ]),
			E('label', {}, [ _('Security'), E('input', { 'class': 'cbi-input-text', 'name': 'security' }) ]),
			E('label', {}, [ _('Transport'), E('input', { 'class': 'cbi-input-text', 'name': 'transport' }) ]),
			E('label', {}, [ _('SNI'), E('input', { 'class': 'cbi-input-text', 'name': 'sni' }) ]),
			E('label', {}, [ _('Tag'), E('input', { 'class': 'cbi-input-text', 'name': 'tag' }) ]),
			E('label', { 'class': 'singbox-manager-check' }, [
				E('input', { 'name': 'enabled', 'type': 'checkbox', 'checked': 'checked' }),
				_('Enabled')
			]),
			E('label', { 'class': 'singbox-manager-check' }, [
				E('input', { 'name': 'tls', 'type': 'checkbox' }),
				_('TLS')
			])
		]),
		E('div', { 'class': 'right' }, [
			E('button', {
				'class': 'btn cbi-button',
				'click': ui.hideModal
			}, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(view, function() {
					var root = document.querySelector('.singbox-manager-modal-form');
					var node = readNode(root, row);
					return callSetNode(node).then(function(result) {
						if (result.ok)
							ui.addNotification(null, E('p', _('Node saved')));
						else
							ui.addNotification(null, E('p', (result.errors || [ _('Save failed') ]).join('; ')));
						ui.hideModal();
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
	fillForm(document.querySelector('.singbox-manager-modal-form'), row);
}

function showDeleteNodeModal(view, node) {
	ui.showModal(_('Delete Node'), [
		E('p', {}, _('Delete this manual node?')),
		E('div', { 'class': 'right' }, [
			E('button', {
				'class': 'btn cbi-button',
				'click': ui.hideModal
			}, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'click': ui.createHandlerFn(view, function() {
					return callDeleteNode(node.id).then(function(result) {
						if (!result.ok)
							ui.addNotification(null, E('p', (result.errors || [ _('Delete failed') ]).join('; ')));
						ui.hideModal();
						return refreshView(view);
					});
				})
			}, _('Delete'))
		])
	]);
}

function renderEditor(view) {
	return E('div', { 'class': 'singbox-manager-section-header' }, [
		E('h3', {}, _('Nodes')),
		E('button', {
			'class': 'btn cbi-button cbi-button-add',
			'click': ui.createHandlerFn(view, function() {
				showNodeModal(view);
			})
		}, _('Add'))
	]);
}

function selectNode(view, node) {
	return callSelectNode(node.id).then(function(result) {
		if (!result.ok) {
			ui.addNotification(null, E('p', (result.errors || [ _('Select failed') ]).join('; ')));
			return result;
		}
		return callStatus().then(function(status) {
			if (!status.running) {
				ui.addNotification(null, E('p', _('Node selected')));
				return result;
			}
			return callReload().then(function(reload) {
				if (reload.ok)
					ui.addNotification(null, E('p', _('Node selected and runtime reloaded')));
				else
					ui.addNotification(null, E('p', (reload.errors || [ _('Node selected, reload failed') ]).join('; ')));
				return reload;
			});
		});
	}).then(function() {
		return refreshView(view);
	});
}

function renderTable(view, nodes, subscriptions, selectedNode) {
	var active = view.activeNodeFilter || 'all';
	var groups = groupedNodes(nodes, subscriptions, active);
	var rows = [];
	groups.forEach(function(group) {
		rows.push(E('tr', { 'class': 'singbox-manager-group-row' }, [
			E('td', { 'colspan': 10 }, group.label)
		]));
		group.nodes.forEach(function(node) {
			var manual = !node.subscription;
			var selected = node.id === selectedNode;
			rows.push(E('tr', {}, [
				E('td', {}, valueOrDash(node.name || node.id)),
				E('td', {}, valueOrDash(node.type)),
				E('td', {}, valueOrDash(node.server || node.address)),
				E('td', {}, valueOrDash(sourceLabel(node.subscription, subscriptions))),
				E('td', {}, valueOrDash(node.health)),
				E('td', {}, node.latency_ms ? '%d ms'.format(node.latency_ms) : '-'),
				E('td', {}, valueOrDash(node.last_check)),
				E('td', {}, node.enabled ? _('Yes') : _('No')),
				E('td', {}, selected ? _('Yes') : '-'),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button' + (selected ? ' cbi-button-apply' : ''),
						'disabled': node.enabled && !selected ? null : 'disabled',
						'click': ui.createHandlerFn(view, function() {
							return selectNode(view, node);
						})
					}, selected ? _('Selected') : _('Select')),
					E('button', {
						'class': 'btn cbi-button',
						'disabled': node.enabled ? null : 'disabled',
						'click': ui.createHandlerFn(view, function() {
							return callNodePingTest(node.id).then(function(result) {
								if (result.ok)
									ui.addNotification(null, E('p', _('Node ping passed in %d ms').format(result.latency_ms || 0)));
								else
									ui.addNotification(null, E('p', (result.errors || [ _('Node ping failed') ]).join('; ')));
								return refreshView(view);
							});
						})
					}, _('Ping')),
					E('button', {
						'class': 'btn cbi-button',
						'disabled': node.enabled ? null : 'disabled',
						'click': ui.createHandlerFn(view, function() {
							return callNodeLatencyTest(node.id, '').then(function(result) {
								if (result.ok)
									ui.addNotification(null, E('p', _('URL test passed in %d ms').format(result.latency_ms || 0)));
								else
									ui.addNotification(null, E('p', (result.errors || [ _('URL test failed') ]).join('; ')));
								return refreshView(view);
							});
						})
					}, _('URL')),
					manual ? E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() {
							showNodeModal(view, node);
						})
					}, _('Edit')) : '',
					manual ? E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() {
							showDeleteNodeModal(view, node);
						})
					}, _('Delete')) : ''
				]))
			]));
		});
	});
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Name')),
			E('th', {}, _('Type')),
			E('th', {}, _('Server')),
			E('th', {}, _('Source')),
			E('th', {}, _('Health')),
			E('th', {}, _('Latency')),
			E('th', {}, _('Last check')),
			E('th', {}, _('Enabled')),
			E('th', {}, _('Selected')),
			E('th', {}, '')
		])),
		E('tbody', {}, rows.length ? rows : E('tr', {}, E('td', { 'colspan': 10 }, _('No nodes'))))
	]);
}

return view.extend({
	load: function() {
		return callNodes();
	},

	render: function(data) {
		var nodes = (data && data.nodes) || [];
		var subscriptions = subscriptionMap((data && data.subscriptions) || []);
		var selectedNode = data && data.selected_node;
		return E('div', { 'class': 'singbox-manager-page' }, [
			E('style', {}, [
				'.singbox-manager-page{display:grid;gap:16px}',
				'.singbox-manager-section-header{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}',
				'.singbox-manager-modal-form{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px;min-width:min(760px,90vw)}',
				'.singbox-manager-modal-form label{display:grid;gap:4px;font-size:12px;color:var(--text-color-medium)}',
				'.singbox-manager-check{display:flex!important;align-items:center;gap:8px;margin-top:20px}',
				'.singbox-manager-table{width:100%;border-collapse:collapse}',
				'.singbox-manager-table th,.singbox-manager-table td{padding:10px;border-bottom:1px solid var(--border-color-medium);text-align:left;vertical-align:middle}',
				'.singbox-manager-table th{font-size:12px;color:var(--text-color-medium);font-weight:600}',
				'.singbox-manager-actions{display:flex;gap:8px;flex-wrap:wrap}',
				'.singbox-manager-filters{display:flex;gap:8px;flex-wrap:wrap}',
				'.singbox-manager-group-row td{background:var(--background-color-high);font-size:12px;font-weight:600;color:var(--text-color-medium)}',
				'@media(max-width:700px){.singbox-manager-table{display:block;overflow-x:auto;white-space:nowrap}}'
			].join('')),
			renderEditor(this),
			renderFilters(this, nodes, subscriptions),
			renderTable(this, nodes, subscriptions, selectedNode)
		]);
	}
});
