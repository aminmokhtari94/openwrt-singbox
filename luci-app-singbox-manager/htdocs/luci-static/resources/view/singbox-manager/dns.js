'use strict';
'require view';
'require rpc';
'require ui';

var callDNS = rpc.declare({
	object: 'singbox.manager',
	method: 'dns',
	expect: { '': {} }
});

var callSetDNSProfile = rpc.declare({
	object: 'singbox.manager',
	method: 'dns_profile_set',
	params: [ 'profile' ],
	expect: { '': {} }
});

var callDeleteDNSProfile = rpc.declare({
	object: 'singbox.manager',
	method: 'dns_profile_delete',
	params: [ 'id' ],
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

var callDNSTest = rpc.declare({
	object: 'singbox.manager',
	method: 'dns_test',
	params: [ 'server', 'domain' ],
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

function readProfileForm(root, row) {
	var enabled = field(root, 'enabled');
	var hijack = field(root, 'hijack');
	return {
		id: row.id || formValue(root, 'id'),
		enabled: enabled ? enabled.checked : true,
		name: formValue(root, 'name'),
		mode: formValue(root, 'mode'),
		servers: splitList(formValue(root, 'servers')),
		hijack: hijack ? hijack.checked : false
	};
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

function showProfileModal(view, row) {
	row = row || { enabled: true, mode: 'split', servers: [] };
	ui.showModal(row.id ? _('Edit DNS Profile') : _('Add DNS Profile'), [
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
				_('Mode'),
				E('select', { 'class': 'cbi-input-select', 'name': 'mode' }, [
					E('option', { 'value': 'direct', 'selected': selected('direct', row.mode) }, 'direct'),
					E('option', { 'value': 'proxy', 'selected': selected('proxy', row.mode) }, 'proxy'),
					E('option', { 'value': 'split', 'selected': selected('split', row.mode || 'split') }, 'split')
				])
			]),
			E('label', {}, [ _('Servers'), E('input', {
				'class': 'cbi-input-text',
				'name': 'servers',
				'value': (row.servers || []).join(', ')
			}) ]),
			E('label', { 'class': 'singbox-manager-check' }, [
				E('input', {
					'type': 'checkbox',
					'name': 'hijack',
					'checked': row.hijack ? 'checked' : null
				}),
				_('Hijack')
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
					var payload = readProfileForm(document.querySelector('.singbox-manager-modal-form'), row);
					return callSetDNSProfile(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('DNS profile saved')));
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showDeleteProfileModal(view, row) {
	ui.showModal(_('Delete DNS Profile'), [
		E('p', {}, _('Delete this DNS profile?')),
		E('div', { 'class': 'right' }, [
			E('button', {
				'class': 'btn cbi-button',
				'click': ui.hideModal
			}, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'click': ui.createHandlerFn(view, function() {
					return callDeleteDNSProfile(row.id).then(function(result) {
						if (!showResult(result, _('Delete failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('DNS profile deleted')));
						return refreshView(view);
					});
				})
			}, _('Delete'))
		])
	]);
}

function showServerModal(view, row) {
	row = row || { enabled: true, type: 'udp', detour: 'direct' };
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
					E('option', { 'value': 'tls', 'selected': selected('tls', row.type) }, 'tls'),
					E('option', { 'value': 'dot', 'selected': selected('dot', row.type) }, 'dot'),
					E('option', { 'value': 'doh', 'selected': selected('doh', row.type) }, 'doh'),
					E('option', { 'value': 'doq', 'selected': selected('doq', row.type) }, 'doq'),
					E('option', { 'value': 'https', 'selected': selected('https', row.type) }, 'https'),
					E('option', { 'value': 'quic', 'selected': selected('quic', row.type) }, 'quic')
				])
			]),
			E('label', {}, [ _('Address'), E('input', {
				'class': 'cbi-input-text',
				'name': 'address',
				'value': row.address || ''
			}) ]),
			E('label', {}, [
				_('Detour'),
				E('select', { 'class': 'cbi-input-select', 'name': 'detour' }, [
					E('option', { 'value': '', 'selected': selected('', row.detour || '') }, '-'),
					E('option', { 'value': 'direct', 'selected': selected('direct', row.detour) }, 'direct'),
					E('option', { 'value': 'proxy', 'selected': selected('proxy', row.detour) }, 'proxy')
				])
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
		E('p', {}, _('Delete this DNS server and remove it from profiles?')),
		E('div', { 'class': 'right' }, [
			E('button', {
				'class': 'btn cbi-button',
				'click': ui.hideModal
			}, _('Cancel')),
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

function renderProfiles(view, profiles) {
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Profile')),
			E('th', {}, _('Mode')),
			E('th', {}, _('Servers')),
			E('th', {}, _('Hijack')),
			E('th', {}, _('Enabled')),
			E('th', {}, '')
		])),
		E('tbody', {}, profiles.length ? profiles.map(function(profile) {
			return E('tr', {}, [
				E('td', {}, valueOrDash(profile.name || profile.id)),
				E('td', {}, valueOrDash(profile.mode)),
				E('td', {}, (profile.servers || []).join(', ') || '-'),
				E('td', {}, profile.hijack ? _('Yes') : _('No')),
				E('td', {}, profile.enabled ? _('Yes') : _('No')),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() {
							showProfileModal(view, profile);
						})
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() {
							showDeleteProfileModal(view, profile);
						})
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 6 }, _('No DNS profiles'))))
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
						'click': ui.createHandlerFn(view, function() {
							showServerModal(view, server);
						})
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() {
							showDeleteServerModal(view, server);
						})
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 6 }, _('No DNS servers'))))
	]);
}

return view.extend({
	load: function() {
		return callDNS();
	},

	render: function(data) {
		var view = this;
		var profiles = (data && data.profiles) || [];
		var servers = (data && data.servers) || [];
		return E('div', { 'class': 'singbox-manager-page' }, [
			E('style', {}, [
				'.singbox-manager-page{display:grid;gap:16px}',
				'.singbox-manager-section{display:grid;gap:10px}',
				'.singbox-manager-section-header{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}',
				'.singbox-manager-section h3{margin:0;font-size:16px}',
				'.singbox-manager-modal-form{display:grid;gap:12px;min-width:min(520px,90vw)}',
				'.singbox-manager-modal-form label{display:grid;gap:4px;font-size:12px;color:var(--text-color-medium)}',
				'.singbox-manager-check{display:flex!important;align-items:center;gap:8px}',
				'.singbox-manager-table{width:100%;border-collapse:collapse}',
				'.singbox-manager-table th,.singbox-manager-table td{padding:10px;border-bottom:1px solid var(--border-color-medium);text-align:left;vertical-align:middle}',
				'.singbox-manager-table th{font-size:12px;color:var(--text-color-medium);font-weight:600}',
				'.singbox-manager-actions{display:flex;gap:8px;flex-wrap:wrap}',
				'@media(max-width:700px){.singbox-manager-table{display:block;overflow-x:auto;white-space:nowrap}}'
			].join('')),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('div', { 'class': 'singbox-manager-section-header' }, [
					E('h3', {}, _('DNS Profiles')),
					E('button', {
						'class': 'btn cbi-button cbi-button-add',
						'click': ui.createHandlerFn(view, function() {
							showProfileModal(view);
						})
					}, _('Add'))
				]),
				renderProfiles(view, profiles)
			]),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('div', { 'class': 'singbox-manager-section-header' }, [
					E('h3', {}, _('DNS Servers')),
					E('button', {
						'class': 'btn cbi-button cbi-button-add',
						'click': ui.createHandlerFn(view, function() {
							showServerModal(view);
						})
					}, _('Add'))
				]),
				renderServers(view, servers)
			])
		]);
	}
});
