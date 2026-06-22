'use strict';
'require view';
'require rpc';
'require ui';
'require view.singbox-manager.theme as theme';

var callDNS = rpc.declare({
	object: 'singbox.manager',
	method: 'dns',
	expect: { '': {} }
});

var callSetDNSServer = rpc.declare({
	object: 'singbox.manager',
	method: 'dns_server_set',
	params: [ 'server' ],
	expect: { '': {} }
});

var callDeleteDNSServer = rpc.declare({
	object: 'singbox.manager',
	method: 'dns_server_delete',
	params: [ 'id' ],
	expect: { '': {} }
});

var callSetDNSRule = rpc.declare({
	object: 'singbox.manager',
	method: 'dns_rule_set',
	params: [ 'rule' ],
	expect: { '': {} }
});

var callDeleteDNSRule = rpc.declare({
	object: 'singbox.manager',
	method: 'dns_rule_delete',
	params: [ 'id' ],
	expect: { '': {} }
});

var callDNSTest = rpc.declare({
	object: 'singbox.manager',
	method: 'dns_test',
	params: [ 'server', 'domain' ],
	expect: { '': {} }
});

var callSetGroup = rpc.declare({
	object: 'singbox.manager',
	method: 'group_set',
	params: [ 'group' ],
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

function addToField(root, name, value) {
	var input = field(root, name);
	if (!input || !value)
		return;
	var current = splitList(input.value);
	if (current.indexOf(value) === -1)
		current.push(value);
	input.value = current.join(', ');
}

// devicePicker renders the LAN devices as click-to-add chips next to the
// sources input, so an address can be picked instead of typed.
function devicePicker(devices, fieldName) {
	devices = (devices || []).filter(function(device) { return device.ip; });
	if (!devices.length)
		return '';
	return E('div', { 'class': 'singbox-manager-devicepicker' }, [
		E('div', { 'class': 'singbox-manager-devicepicker-label' }, _('LAN devices — click to add')),
		E('div', { 'class': 'singbox-manager-chips' }, devices.map(function(device) {
			var parts = [];
			if (device.name)
				parts.push(device.name);
			parts.push(device.ip);
			if (device.mac)
				parts.push(device.mac);
			var label = parts.join(' · ');
			return E('button', {
				'type': 'button',
				'class': 'btn cbi-button singbox-manager-chip',
				'title': device.mac || '',
				'click': function(ev) {
					ev.preventDefault();
					addToField(document.querySelector('.singbox-manager-modal-form'), fieldName, device.ip);
				}
			}, label);
		}))
	]);
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

function serverOptions(servers, current) {
	return (servers || []).map(function(server) {
		return E('option', {
			'value': server.id,
			'selected': selected(server.id, current)
		}, server.name || server.id);
	});
}

function readServerForm(root, row) {
	var enabled = field(root, 'enabled');
	return {
		id: row.id || formValue(root, 'id'),
		enabled: enabled ? enabled.checked : true,
		name: formValue(root, 'name'),
		type: formValue(root, 'type'),
		address: formValue(root, 'address'),
		detour: formValue(root, 'detour')
	};
}

function readRuleForm(root, row, activeGroup) {
	var enabled = field(root, 'enabled');
	return {
		id: row.id || formValue(root, 'id'),
		enabled: enabled ? enabled.checked : true,
		name: formValue(root, 'name'),
		group: activeGroup,
		sources: splitList(formValue(root, 'sources')),
		rulesets: splitList(formValue(root, 'rulesets')),
		server: formValue(root, 'server')
	};
}

function showServerModal(view, row) {
	row = row || { enabled: true, type: 'doh', detour: 'proxy' };
	ui.showModal(row.id ? _('Edit DNS Server') : _('Add DNS Server'), [
		E('div', { 'class': 'singbox-manager-modal-form' }, [
			E('label', { 'class': 'singbox-manager-check' }, [
				E('input', {
					'type': 'checkbox',
					'name': 'enabled',
					'checked': row.enabled !== false ? 'checked' : null
				}),
				_('Enabled')
			]),
			E('label', {}, [ _('ID'), E('input', {
				'class': 'cbi-input-text',
				'name': 'id',
				'value': row.id || '',
				'disabled': row.id ? 'disabled' : null
			}) ]),
			E('label', {}, [ _('Name'), E('input', {
				'class': 'cbi-input-text',
				'name': 'name',
				'value': row.name || row.id || ''
			}) ]),
			E('label', {}, [
				_('Type'),
				E('select', { 'class': 'cbi-input-select', 'name': 'type' }, [
					E('option', { 'value': 'udp', 'selected': selected('udp', row.type || 'udp') }, 'udp'),
					E('option', { 'value': 'tcp', 'selected': selected('tcp', row.type) }, 'tcp'),
					E('option', { 'value': 'tls', 'selected': selected('tls', row.type) }, 'tls (DoT)'),
					E('option', { 'value': 'https', 'selected': selected('https', row.type) }, 'https (DoH)'),
					E('option', { 'value': 'quic', 'selected': selected('quic', row.type) }, 'quic (DoQ)')
				])
			]),
			E('label', {}, [ _('Address'), E('input', {
				'class': 'cbi-input-text',
				'name': 'address',
				'value': row.address || '',
				'placeholder': 'https://1.1.1.1/dns-query'
			}) ]),
			E('label', { 'title': _('Detour chooses how the resolver\'s own queries exit: direct, through the proxy, or unset (follow routing).') }, [
				_('Detour'),
				E('select', { 'class': 'cbi-input-select', 'name': 'detour' }, [
					E('option', { 'value': '', 'selected': selected('', row.detour || '') }, _('(unset)')),
					E('option', { 'value': 'direct', 'selected': selected('direct', row.detour) }, 'direct'),
					E('option', { 'value': 'proxy', 'selected': selected('proxy', row.detour) }, 'proxy')
				])
			])
		]),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(view, function() {
					var payload = readServerForm(document.querySelector('.singbox-manager-modal-form'), row);
					return callSetDNSServer(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('DNS server saved')));
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showDeleteServerModal(view, row) {
	ui.showModal(_('Delete DNS Server'), [
		E('p', {}, _('Delete this DNS server? Rules that reference it are removed too.')),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'click': ui.createHandlerFn(view, function() {
					return callDeleteDNSServer(row.id).then(function(result) {
						if (!showResult(result, _('Delete failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('DNS server deleted')));
						return refreshView(view);
					});
				})
			}, _('Delete'))
		])
	]);
}

function showRuleModal(view, data, row) {
	row = row || { enabled: true, sources: [], rulesets: [] };
	var servers = (data && data.servers) || [];
	ui.showModal(row.id ? _('Edit DNS Rule') : _('Add DNS Rule'), [
		E('div', { 'class': 'singbox-manager-modal-form' }, [
			E('label', { 'class': 'singbox-manager-check' }, [
				E('input', {
					'type': 'checkbox',
					'name': 'enabled',
					'checked': row.enabled !== false ? 'checked' : null
				}),
				_('Enabled')
			]),
			E('label', {}, [ _('ID'), E('input', {
				'class': 'cbi-input-text',
				'name': 'id',
				'value': row.id || '',
				'disabled': row.id ? 'disabled' : null
			}) ]),
			E('label', {}, [ _('Name'), E('input', {
				'class': 'cbi-input-text',
				'name': 'name',
				'value': row.name || row.id || ''
			}) ]),
			E('label', {}, [ _('Source IPs / CIDRs'), E('input', {
				'class': 'cbi-input-text',
				'name': 'sources',
				'value': (row.sources || []).join(', '),
				'placeholder': '192.168.200.124, 192.168.200.0/24'
			}) ]),
			devicePicker(data.devices, 'sources'),
			E('label', {}, [ _('Rule sets (optional)'), E('input', {
				'class': 'cbi-input-text',
				'name': 'rulesets',
				'value': (row.rulesets || []).join(', '),
				'placeholder': 'geosite-ir'
			}) ]),
			E('label', {}, [
				_('Resolver'),
				E('select', { 'class': 'cbi-input-select', 'name': 'server' }, serverOptions(servers, row.server))
			]),
			E('p', { 'class': 'singbox-manager-inline-help' }, _('Queries from the matched sources (and/or rule sets) use the selected resolver. Provide at least one source or rule set.'))
		]),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(view, function() {
					var payload = readRuleForm(document.querySelector('.singbox-manager-modal-form'), row, data.active_group);
					return callSetDNSRule(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('DNS rule saved')));
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showDeleteRuleModal(view, row) {
	ui.showModal(_('Delete DNS Rule'), [
		E('p', {}, _('Delete this DNS rule?')),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'click': ui.createHandlerFn(view, function() {
					return callDeleteDNSRule(row.id).then(function(result) {
						if (!showResult(result, _('Delete failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('DNS rule deleted')));
						return refreshView(view);
					});
				})
			}, _('Delete'))
		])
	]);
}

function renderResolution(view, data) {
	var servers = (data && data.servers) || [];
	var group = (data && data.group) || {};
	var capture = data && data.capture_enabled;
	var options = [ E('option', { 'value': '', 'selected': selected('', data.dns_final || '') }, _('(first enabled server)')) ]
		.concat(serverOptions(servers, data.dns_final));
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('div', { 'class': 'singbox-manager-section-header' }, [
			E('h3', {}, _('DNS Resolution')),
			E('span', { 'class': 'singbox-manager-badge' + (capture ? ' on' : ' off') },
				capture ? _('Capture: on (tproxy)') : _('Capture: off'))
		]),
		renderWarnings(data && data.warnings),
		E('label', { 'class': 'singbox-manager-inline-control' }, [
			_('Default resolver'),
			E('select', {
				'class': 'cbi-input-select',
				'change': ui.createHandlerFn(view, function(ev) {
					var next = Object.assign({}, group, { dns_final: ev.target.value });
					return callSetGroup(next).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return refreshView(view);
						ui.addNotification(null, E('p', _('Default resolver updated')));
						return refreshView(view);
					});
				})
			}, options)
		])
	]);
}

function renderServers(view, servers) {
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Server')),
			E('th', {}, _('Type')),
			E('th', {}, _('Address')),
			E('th', {}, _('Detour')),
			E('th', {}, _('Enabled')),
			E('th', {}, '')
		])),
		E('tbody', {}, servers.length ? servers.map(function(server) {
			return E('tr', {}, [
				E('td', {}, valueOrDash(server.name || server.id)),
				E('td', {}, valueOrDash(server.type)),
				E('td', {}, valueOrDash(server.address)),
				E('td', {}, valueOrDash(server.detour)),
				E('td', {}, server.enabled ? _('Yes') : _('No')),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button',
						'disabled': server.enabled ? null : 'disabled',
						'click': ui.createHandlerFn(view, function() {
							return callDNSTest(server.id, 'example.com').then(function(result) {
								if (result.ok)
									ui.addNotification(null, E('p', _('DNS test passed in %d ms').format(result.latency_ms || 0)));
								else
									ui.addNotification(null, E('p', (result.errors || [ _('DNS test failed') ]).join('; ')));
							});
						})
					}, _('Test')),
					E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() { showServerModal(view, server); })
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() { showDeleteServerModal(view, server); })
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 6 }, _('No DNS servers'))))
	]);
}

function renderRules(view, data) {
	var rules = ((data && data.rules) || []).filter(function(rule) {
		return !rule.group || rule.group === data.active_group;
	});
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Rule')),
			E('th', {}, _('Sources')),
			E('th', {}, _('Rule sets')),
			E('th', {}, _('Resolver')),
			E('th', {}, _('Enabled')),
			E('th', {}, '')
		])),
		E('tbody', {}, rules.length ? rules.map(function(rule) {
			return E('tr', {}, [
				E('td', {}, valueOrDash(rule.name || rule.id)),
				E('td', {}, (rule.sources || []).join(', ') || '-'),
				E('td', {}, (rule.rulesets || []).join(', ') || '-'),
				E('td', {}, valueOrDash(rule.server)),
				E('td', {}, rule.enabled ? _('Yes') : _('No')),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() { showRuleModal(view, data, rule); })
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() { showDeleteRuleModal(view, rule); })
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 6 }, _('No DNS rules — all queries use the default resolver'))))
	]);
}

function renderWarnings(warnings) {
	warnings = warnings || [];
	if (!warnings.length)
		return '';
	return E('div', { 'class': 'singbox-manager-warning' }, warnings.join('; '));
}

function renderDebug(data) {
	return E('details', { 'class': 'singbox-manager-details' }, [
		E('summary', {}, _('Rendered DNS')),
		E('div', { 'class': 'singbox-manager-grid' }, [
			E('div', { 'class': 'singbox-manager-label' }, _('Active DNS inbound')),
			E('pre', { 'class': 'singbox-manager-preview' }, JSON.stringify(data.active_dns_inbound || null, null, 2)),
			E('div', { 'class': 'singbox-manager-label' }, _('Servers')),
			E('pre', { 'class': 'singbox-manager-preview' }, JSON.stringify(data.rendered_servers || [], null, 2)),
			E('div', { 'class': 'singbox-manager-label' }, _('Rules')),
			E('pre', { 'class': 'singbox-manager-preview' }, JSON.stringify(data.rendered_rules || [], null, 2))
		])
	]);
}

return view.extend({
	load: function() {
		return callDNS();
	},

	render: function(data) {
		var view = this;
		data = data || {};
		var servers = data.servers || [];
		theme.inject();
		return E('div', { 'class': 'singbox-manager-page' }, [
			renderResolution(view, data),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('div', { 'class': 'singbox-manager-section-header' }, [
					E('h3', {}, _('DNS Servers')),
					E('button', {
						'class': 'btn cbi-button cbi-button-add',
						'click': ui.createHandlerFn(view, function() { showServerModal(view); })
					}, _('Add'))
				]),
				renderServers(view, servers)
			]),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('div', { 'class': 'singbox-manager-section-header' }, [
					E('h3', {}, _('DNS Rules (per source)')),
					E('button', {
						'class': 'btn cbi-button cbi-button-add',
						'click': ui.createHandlerFn(view, function() { showRuleModal(view, data); })
					}, _('Add'))
				]),
				renderRules(view, data)
			]),
			renderDebug(data)
		]);
	}
});
