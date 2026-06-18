'use strict';
'require view';
'require rpc';
'require ui';
'require view.singbox-manager.theme as theme';

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

var callSetGroup = rpc.declare({
	object: 'singbox.manager',
	method: 'group_set',
	params: [ 'group' ],
	expect: { '': {} }
});

var callSetSubscription = rpc.declare({
	object: 'singbox.manager',
	method: 'subscription_set',
	params: [ 'subscription' ],
	expect: { '': {} }
});

var callDeleteSubscription = rpc.declare({
	object: 'singbox.manager',
	method: 'subscription_delete',
	params: [ 'id' ],
	expect: { '': {} }
});

var callImport = rpc.declare({
	object: 'singbox.manager',
	method: 'subscription_import',
	params: [ 'request' ],
	expect: { '': {} }
});

var callRefresh = rpc.declare({
	object: 'singbox.manager',
	method: 'refresh_subscription',
	params: [ 'id' ],
	expect: { '': {} }
});

var callRefreshAll = rpc.declare({
	object: 'singbox.manager',
	method: 'refresh_subscriptions',
	expect: { '': {} }
});

function valueOrDash(value) {
	if (value === null || value === undefined || value === '')
		return '-';
	return value;
}

function statusClass(value) {
	if (value === 'ok')
		return 'singbox-manager-status-ok';
	if (value && value !== 'unknown')
		return 'singbox-manager-status-error';
	return '';
}

function field(root, name) {
	return root.querySelector('[name="%s"]'.format(name));
}

function formValue(root, name) {
	var input = field(root, name);
	return input ? input.value.trim() : '';
}

function splitList(value) {
	return (value || '').split(/[\s,]+/).map(function(item) {
		return item.trim();
	}).filter(function(item) {
		return item !== '';
	});
}

function selected(value, current) {
	return value === current ? 'selected' : null;
}

function refreshView(view) {
	return view.load().then(function(data) {
		var replacement = view.render(data);
		var root = document.querySelector('.singbox-manager-page');
		if (root)
			root.parentNode.replaceChild(replacement, root);
	});
}

function showResult(result, fallback) {
	if (result.ok)
		return true;

	ui.addNotification(null, E('p', (result.errors || [ fallback ]).join('; ')));
	return false;
}

/* ---- Proxy section ---- */

function strategyLabel(value) {
	switch (value) {
		case 'manual': return _('Manual');
		case 'selector': return _('Selector');
		case 'urltest': return _('URL test');
		case 'load-balance': return _('Load balance');
		default: return value || '-';
	}
}

function changeStrategy(view, group, value) {
	var payload = Object.assign({}, group || {});
	payload.strategy = value;
	return callSetGroup(payload).then(function(result) {
		if (!showResult(result, _('Save failed')))
			return refreshView(view);
		ui.addNotification(null, E('p', _('Strategy saved')));
		return refreshView(view);
	});
}

function renderProxy(view, data) {
	var group = (data && data.group) || {};
	var strategy = (data && data.strategy) || group.strategy || 'manual';
	var selectedNode = (data && data.selected_node) || group.selected_node || '';
	var activeGroup = (data && data.active_group) || group.name || group.id || '';
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('div', { 'class': 'singbox-manager-section-header' }, [
			E('h3', {}, _('Proxy'))
		]),
		E('div', { 'class': 'singbox-manager-grid' }, [
			E('div', { 'class': 'singbox-manager-label' }, _('Active group')),
			E('div', {}, valueOrDash(activeGroup)),
			E('div', { 'class': 'singbox-manager-label' }, _('Strategy')),
			E('div', {}, E('select', {
				'class': 'cbi-input-select',
				'name': 'strategy',
				'change': ui.createHandlerFn(view, function(ev) {
					return changeStrategy(view, group, ev.target.value);
				})
			}, [
				E('option', { 'value': 'manual', 'selected': selected('manual', strategy) }, strategyLabel('manual')),
				E('option', { 'value': 'selector', 'selected': selected('selector', strategy) }, strategyLabel('selector')),
				E('option', { 'value': 'urltest', 'selected': selected('urltest', strategy) }, strategyLabel('urltest')),
				E('option', { 'value': 'load-balance', 'selected': selected('load-balance', strategy) }, strategyLabel('load-balance'))
			])),
			E('div', { 'class': 'singbox-manager-label' }, _('Selected node')),
			E('div', {}, valueOrDash(selectedNode))
		])
	]);
}

/* ---- Subscriptions section ---- */

function readImport(root) {
	return {
		input: formValue(root, 'input'),
		name: formValue(root, 'name'),
		format: formValue(root, 'format'),
		update_interval: formValue(root, 'update_interval')
	};
}

function readSubscriptionForm(root, row) {
	var enabled = field(root, 'enabled');
	return {
		id: row.id,
		enabled: enabled ? enabled.checked : !!row.enabled,
		name: formValue(root, 'name'),
		url: formValue(root, 'url'),
		format: formValue(root, 'format'),
		update_interval: formValue(root, 'update_interval')
	};
}

function showEditSubscriptionModal(view, row) {
	ui.showModal(_('Edit Subscription'), [
		E('div', { 'class': 'singbox-manager-modal-form' }, [
			E('label', {}, [ _('Enabled'), E('input', {
				'type': 'checkbox',
				'name': 'enabled',
				'checked': row.enabled ? 'checked' : null
			}) ]),
			E('label', {}, [ _('Name'), E('input', {
				'class': 'cbi-input-text',
				'name': 'name',
				'value': row.name || row.id || ''
			}) ]),
			E('label', {}, [ _('URL'), E('input', {
				'class': 'cbi-input-text',
				'name': 'url',
				'value': row.url || ''
			}) ]),
			E('label', {}, [
				_('Format'),
				E('select', { 'class': 'cbi-input-select', 'name': 'format' }, [
					E('option', { 'value': 'auto', 'selected': row.format === 'auto' ? 'selected' : null }, 'auto'),
					E('option', { 'value': 'plain', 'selected': row.format === 'plain' ? 'selected' : null }, 'plain'),
					E('option', { 'value': 'base64', 'selected': row.format === 'base64' ? 'selected' : null }, 'base64')
				])
			]),
			E('label', {}, [ _('Update Interval'), E('input', {
				'class': 'cbi-input-text',
				'name': 'update_interval',
				'value': row.update_interval || '24h'
			}) ])
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
					var payload = readSubscriptionForm(document.querySelector('.singbox-manager-modal-form'), row);
					return callSetSubscription(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Subscription saved')));
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showDeleteSubscriptionModal(view, row) {
	ui.showModal(_('Delete Subscription'), [
		E('p', {}, _('Delete this subscription and all nodes imported from it?')),
		E('div', { 'class': 'right' }, [
			E('button', {
				'class': 'btn cbi-button',
				'click': ui.hideModal
			}, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'click': ui.createHandlerFn(view, function() {
					return callDeleteSubscription(row.id).then(function(result) {
						if (!showResult(result, _('Delete failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Subscription deleted')));
						return refreshView(view);
					});
				})
			}, _('Delete'))
		])
	]);
}

function showImportModal(view) {
	ui.showModal(_('Import Subscription'), [
		E('div', { 'class': 'singbox-manager-import-grid' }, [
			E('label', { 'class': 'singbox-manager-import-link' }, [
				_('Subscription or config link'),
				E('textarea', {
					'class': 'cbi-input-textarea',
					'name': 'input',
					'rows': '4',
					'placeholder': 'https://example.com/sub\n— or paste vmess:// / vless:// / ss:// / trojan:// links'
				})
			]),
			E('label', {}, [ _('Name (optional)'), E('input', {
				'class': 'cbi-input-text',
				'name': 'name',
				'placeholder': _('Derived from the link')
			}) ]),
			E('label', {}, [
				_('Format'),
				E('select', { 'class': 'cbi-input-select', 'name': 'format' }, [
					E('option', { 'value': 'auto' }, _('auto-detect')),
					E('option', { 'value': 'base64' }, 'base64'),
					E('option', { 'value': 'plain' }, 'plain')
				])
			]),
			E('label', {}, [ _('Update every'), E('input', {
				'class': 'cbi-input-text',
				'name': 'update_interval',
				'value': '24h',
				'placeholder': '24h'
			}) ]),
			E('p', { 'class': 'singbox-manager-import-help' }, _('A URL is saved as a refreshable subscription. Pasted node links are imported once.'))
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
					var request = readImport(document.querySelector('.singbox-manager-import-grid'));
					if (!request.input) {
						ui.addNotification(null, E('p', _('A subscription or config link is required')));
						return;
					}
					return callImport(request).then(function(result) {
						if (result.ok) {
							ui.addNotification(null, E('p', result.remote ? _('Subscription saved and refreshed') : _('Imported %d nodes').format(result.imported || 0)));
							ui.hideModal();
							return refreshView(view);
						}
						else if (result.saved) {
							ui.addNotification(null, E('p', _('Subscription saved; import failed: %s').format((result.errors || [ _('Import failed') ]).join('; '))));
							return refreshView(view);
						}
						else
							ui.addNotification(null, E('p', (result.errors || [ _('Import failed') ]).join('; ')));
					});
				})
			}, _('Import'))
		])
	]);
}

function renderSubscriptionsTable(view, rows) {
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Name')),
			E('th', {}, _('URL')),
			E('th', {}, _('Format')),
			E('th', {}, _('Health')),
			E('th', {}, _('Latency')),
			E('th', {}, _('Last update')),
			E('th', {}, _('Last error')),
			E('th', {}, _('Last check')),
			E('th', {}, '')
		])),
		E('tbody', {}, rows.length ? rows.map(function(row) {
			return E('tr', {}, [
				E('td', {}, valueOrDash(row.name || row.id)),
				E('td', {}, valueOrDash(row.url)),
				E('td', {}, valueOrDash(row.format)),
				E('td', { 'class': statusClass(row.health) }, valueOrDash(row.health)),
				E('td', {}, row.latency_ms ? '%d ms'.format(row.latency_ms) : '-'),
				E('td', {}, valueOrDash(row.last_update)),
				E('td', { 'class': row.last_error ? 'singbox-manager-status-error' : '' }, valueOrDash(row.last_error)),
				E('td', {}, valueOrDash(row.last_check)),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() {
							showEditSubscriptionModal(view, row);
						})
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-apply',
						'disabled': row.enabled ? null : 'disabled',
						'click': ui.createHandlerFn(view, function() {
							return callRefresh(row.id).then(function(result) {
								if (result.ok) {
									ui.addNotification(null, E('p', _('Imported %d nodes').format(result.imported || 0)));
									return refreshView(view);
								}
								else
									ui.addNotification(null, E('p', (result.errors || [ _('Refresh failed') ]).join('; ')));
							});
						})
					}, _('Refresh')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() {
							showDeleteSubscriptionModal(view, row);
						})
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 9 }, _('No subscriptions'))))
	]);
}

function renderSubscriptions(view, subscriptions) {
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('div', { 'class': 'singbox-manager-section-header' }, [
			E('h3', {}, _('Subscriptions')),
			E('div', { 'class': 'singbox-manager-actions' }, [
				E('button', {
					'class': 'btn cbi-button cbi-button-add',
					'click': ui.createHandlerFn(view, function() {
						showImportModal(view);
					})
				}, _('Import')),
				E('button', {
					'class': 'btn cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(view, function() {
						return callRefreshAll().then(function(result) {
							if (result.ok)
								ui.addNotification(null, E('p', _('Refreshed %d subscriptions, imported %d nodes').format(result.refreshed || 0, result.imported || 0)));
							else
								ui.addNotification(null, E('p', _('Refresh-all finished with %d failures').format((result.failures || []).length)));
							return refreshView(view);
						});
					})
				}, _('Refresh All'))
			])
		]),
		renderSubscriptionsTable(view, subscriptions)
	]);
}

/* ---- Nodes section ---- */

var NODE_TYPES = [ 'shadowsocks', 'trojan', 'vmess', 'vless', 'hysteria2', 'tuic', 'direct' ];
var WITH_SERVER = [ 'shadowsocks', 'trojan', 'vmess', 'vless', 'hysteria2', 'tuic' ];
var TLS_TYPES = [ 'trojan', 'vmess', 'vless', 'hysteria2', 'tuic' ];
var TRANSPORT_TYPES = [ 'trojan', 'vmess', 'vless' ];

// Each field declares which node types it belongs to; the modal shows only the
// fields the selected type actually uses instead of every field at once.
var NODE_FIELDS = [
	{ name: 'server', label: _('Server'), kind: 'text', placeholder: 'example.com', types: WITH_SERVER },
	{ name: 'port', label: _('Port'), kind: 'number', types: WITH_SERVER },
	{ name: 'method', label: _('Method'), kind: 'text', placeholder: 'aes-256-gcm', types: [ 'shadowsocks' ] },
	{ name: 'uuid', label: _('UUID'), kind: 'text', types: [ 'vmess', 'vless', 'tuic' ] },
	{ name: 'password', label: _('Password'), kind: 'password', types: [ 'shadowsocks', 'trojan', 'hysteria2', 'tuic' ] },
	{ name: 'security', label: _('Encryption'), kind: 'select', options: [ 'auto', 'none', 'zero', 'aes-128-gcm', 'chacha20-poly1305' ], types: [ 'vmess' ] },
	{ name: 'flow', label: _('Flow'), kind: 'select', options: [ '', 'xtls-rprx-vision' ], types: [ 'vless' ] },
	{ name: 'congestion', label: _('Congestion control'), kind: 'select', options: [ '', 'bbr', 'cubic', 'new_reno' ], types: [ 'tuic' ] },
	{ name: 'udp_relay_mode', label: _('UDP relay mode'), kind: 'select', options: [ '', 'native', 'quic' ], types: [ 'tuic' ] },
	{ name: 'transport', label: _('Transport'), kind: 'select', options: [ 'tcp', 'ws', 'grpc', 'httpupgrade' ], types: TRANSPORT_TYPES },
	{ name: 'host', label: _('Transport host'), kind: 'text', types: TRANSPORT_TYPES },
	{ name: 'path', label: _('Transport path'), kind: 'text', placeholder: '/path', types: TRANSPORT_TYPES },
	{ name: 'sni', label: _('TLS SNI'), kind: 'text', types: TLS_TYPES },
	{ name: 'alpn', label: _('TLS ALPN'), kind: 'text', placeholder: 'h2, http/1.1', types: TLS_TYPES }
];

var NODE_FLAGS = [
	{ name: 'tls', label: _('TLS'), types: TLS_TYPES },
	{ name: 'insecure', label: _('Allow insecure TLS'), types: TLS_TYPES }
];

function appliesTo(types, type) {
	return types.indexOf(type) !== -1;
}

function buildNodeField(def, row) {
	var control;
	if (def.kind === 'select') {
		var current = row[def.name] != null && row[def.name] !== '' ? row[def.name] : def.options[0];
		control = E('select', { 'class': 'cbi-input-select', 'name': def.name }, def.options.map(function(opt) {
			return E('option', { 'value': opt, 'selected': selected(opt, current) }, opt || _('(none)'));
		}));
	}
	else {
		control = E('input', {
			'class': def.kind === 'password' ? 'cbi-input-password' : 'cbi-input-text',
			'type': def.kind === 'password' ? 'password' : (def.kind === 'number' ? 'number' : 'text'),
			'name': def.name,
			'value': row[def.name] != null ? row[def.name] : '',
			'placeholder': def.placeholder || ''
		});
	}
	return E('label', { 'data-types': def.types.join(' ') }, [ def.label, control ]);
}

function applyNodeType(root, type) {
	root.querySelectorAll('[data-types]').forEach(function(node) {
		var types = (node.getAttribute('data-types') || '').split(' ');
		node.style.display = appliesTo(types, type) ? '' : 'none';
	});
}

function readNode(root, row) {
	row = row || {};
	var type = formValue(root, 'type');
	var node = {
		id: row.id || formValue(root, 'id'),
		enabled: field(root, 'enabled').checked,
		name: formValue(root, 'name'),
		type: type,
		tag: formValue(root, 'tag')
	};
	NODE_FIELDS.forEach(function(def) {
		if (!appliesTo(def.types, type))
			return;
		if (def.name === 'port')
			node.port = Number(formValue(root, 'port') || 0);
		else
			node[def.name] = formValue(root, def.name);
	});
	NODE_FLAGS.forEach(function(def) {
		if (!appliesTo(def.types, type))
			return;
		var input = field(root, def.name);
		node[def.name] = input ? input.checked : false;
	});
	return node;
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
	var type = row.type || 'shadowsocks';

	var typeSelect = E('select', {
		'class': 'cbi-input-select',
		'name': 'type',
		'change': function(ev) {
			applyNodeType(document.querySelector('.singbox-manager-modal-form'), ev.target.value);
		}
	}, NODE_TYPES.map(function(t) {
		return E('option', { 'value': t, 'selected': selected(t, type) }, t);
	}));

	var head = [
		E('label', {}, [ _('ID'), E('input', {
			'class': 'cbi-input-text',
			'name': 'id',
			'value': row.id || '',
			'disabled': row.id ? 'disabled' : null,
			'placeholder': 'my_node'
		}) ]),
		E('label', {}, [ _('Name'), E('input', { 'class': 'cbi-input-text', 'name': 'name', 'value': row.name || '' }) ]),
		E('label', {}, [ _('Type'), typeSelect ])
	];
	var fields = NODE_FIELDS.map(function(def) { return buildNodeField(def, row); });
	var enabled = E('label', { 'class': 'singbox-manager-check' }, [
		E('input', { 'name': 'enabled', 'type': 'checkbox', 'checked': row.enabled !== false ? 'checked' : null }),
		_('Enabled')
	]);
	var flags = NODE_FLAGS.map(function(def) {
		return E('label', { 'class': 'singbox-manager-check', 'data-types': def.types.join(' ') }, [
			E('input', { 'name': def.name, 'type': 'checkbox', 'checked': row[def.name] ? 'checked' : null }),
			def.label
		]);
	});
	var tag = E('label', {}, [ _('Tag (optional)'), E('input', { 'class': 'cbi-input-text', 'name': 'tag', 'value': row.tag || '', 'placeholder': row.id || '' }) ]);

	ui.showModal(row.id ? _('Edit Node') : _('Add Node'), [
		E('div', { 'class': 'singbox-manager-modal-form wide' }, head.concat(fields).concat([ enabled ]).concat(flags).concat([ tag ])),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(view, function() {
					var root = document.querySelector('.singbox-manager-modal-form');
					var node = readNode(root, row);
					if (!node.id) {
						ui.addNotification(null, E('p', _('Node ID is required')));
						return;
					}
					return callSetNode(node).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Node saved')));
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
	applyNodeType(document.querySelector('.singbox-manager-modal-form'), type);
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

function selectNode(view, node) {
	return callSelectNode(node.id).then(function(result) {
		if (!result.ok)
			ui.addNotification(null, E('p', (result.errors || [ _('Select failed') ]).join('; ')));
		else if (result.reload_error)
			ui.addNotification(null, E('p', _('Node selected; reload failed: %s').format(result.reload_error)));
		else
			ui.addNotification(null, E('p', result.reloaded ? _('Node selected and applied') : _('Node selected')));
		return refreshView(view);
	});
}

function renderNodesTable(view, nodes, subscriptions, selectedNode) {
	var active = view.activeNodeFilter || 'all';
	var groups = groupedNodes(nodes, subscriptions, active);
	var rows = [];
	groups.forEach(function(group) {
		rows.push(E('tr', { 'class': 'singbox-manager-group-row' }, [
			E('td', { 'colspan': 10 }, group.label)
		]));
		group.nodes.forEach(function(node) {
			var manual = !node.subscription;
			var nodeSelected = node.id === selectedNode;
			rows.push(E('tr', {}, [
				E('td', {}, valueOrDash(node.name || node.id)),
				E('td', {}, valueOrDash(node.type)),
				E('td', {}, valueOrDash(node.server || node.address)),
				E('td', {}, valueOrDash(sourceLabel(node.subscription, subscriptions))),
				E('td', {}, valueOrDash(node.health)),
				E('td', {}, node.latency_ms ? '%d ms'.format(node.latency_ms) : '-'),
				E('td', {}, valueOrDash(node.last_check)),
				E('td', {}, node.enabled ? _('Yes') : _('No')),
				E('td', {}, nodeSelected ? _('Yes') : '-'),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button' + (nodeSelected ? ' cbi-button-apply' : ''),
						'disabled': node.enabled && !nodeSelected ? null : 'disabled',
						'click': ui.createHandlerFn(view, function() {
							return selectNode(view, node);
						})
					}, nodeSelected ? _('Selected') : _('Select')),
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

function renderNodes(view, nodes, subscriptions, selectedNode) {
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('div', { 'class': 'singbox-manager-section-header' }, [
			E('h3', {}, _('Nodes')),
			E('button', {
				'class': 'btn cbi-button cbi-button-add',
				'click': ui.createHandlerFn(view, function() {
					showNodeModal(view);
				})
			}, _('Add'))
		]),
		renderFilters(view, nodes, subscriptions),
		renderNodesTable(view, nodes, subscriptions, selectedNode)
	]);
}

return view.extend({
	load: function() {
		return callNodes();
	},

	render: function(data) {
		var view = this;
		var nodes = (data && data.nodes) || [];
		var subscriptionList = (data && data.subscriptions) || [];
		var subscriptions = subscriptionMap(subscriptionList);
		var selectedNode = data && data.selected_node;
		theme.inject();
		return E('div', { 'class': 'singbox-manager-page' }, [
			renderProxy(view, data || {}),
			renderSubscriptions(view, subscriptionList),
			renderNodes(view, nodes, subscriptions, selectedNode)
		]);
	}
});
