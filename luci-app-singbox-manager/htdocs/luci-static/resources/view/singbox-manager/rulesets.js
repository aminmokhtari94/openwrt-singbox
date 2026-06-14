'use strict';
'require view';
'require rpc';
'require ui';

var callRuleSets = rpc.declare({
	object: 'singbox.manager',
	method: 'rulesets',
	expect: { '': {} }
});

var callSetRoutingProfile = rpc.declare({
	object: 'singbox.manager',
	method: 'routing_profile_set',
	params: [ 'profile' ],
	expect: { '': {} }
});

var callDeleteRoutingProfile = rpc.declare({
	object: 'singbox.manager',
	method: 'routing_profile_delete',
	params: [ 'id' ],
	expect: { '': {} }
});

var callSetRuleSet = rpc.declare({
	object: 'singbox.manager',
	method: 'ruleset_set',
	params: [ 'ruleset' ],
	expect: { '': {} }
});

var callDeleteRuleSet = rpc.declare({
	object: 'singbox.manager',
	method: 'ruleset_delete',
	params: [ 'id' ],
	expect: { '': {} }
});

var callRefreshRuleSet = rpc.declare({
	object: 'singbox.manager',
	method: 'refresh_ruleset',
	params: [ 'id' ],
	expect: { '': {} }
});

var callSetSourceRule = rpc.declare({
	object: 'singbox.manager',
	method: 'source_rule_set',
	params: [ 'rule' ],
	expect: { '': {} }
});

var callDeleteSourceRule = rpc.declare({
	object: 'singbox.manager',
	method: 'source_rule_delete',
	params: [ 'id' ],
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

function readRoutingProfileForm(root, row) {
	var enabled = field(root, 'enabled');
	return {
		id: row.id || formValue(root, 'id'),
		enabled: enabled ? enabled.checked : true,
		name: formValue(root, 'name'),
		mode: formValue(root, 'mode'),
		rulesets: splitList(formValue(root, 'rulesets')),
		final: formValue(root, 'final')
	};
}

function readRuleSetForm(root, row) {
	var enabled = field(root, 'enabled');
	return {
		id: row.id || formValue(root, 'id'),
		enabled: enabled ? enabled.checked : true,
		name: formValue(root, 'name'),
		type: formValue(root, 'type'),
		format: formValue(root, 'format'),
		url: formValue(root, 'url'),
		path: formValue(root, 'path'),
		update_interval: formValue(root, 'update_interval')
	};
}

function readSourceRuleForm(root, row) {
	var enabled = field(root, 'enabled');
	return {
		id: row.id || formValue(root, 'id'),
		enabled: enabled ? enabled.checked : true,
		name: formValue(root, 'name'),
		profile: formValue(root, 'profile'),
		sources: splitList(formValue(root, 'sources')),
		outbound: formValue(root, 'outbound')
	};
}

function showRoutingProfileModal(view, row) {
	row = row || { enabled: true, mode: 'rule', rulesets: [], final: 'proxy' };
	ui.showModal(row.id ? _('Edit Routing Profile') : _('Add Routing Profile'), [
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
					E('option', { 'value': 'rule', 'selected': selected('rule', row.mode || 'rule') }, 'rule'),
					E('option', { 'value': 'global', 'selected': selected('global', row.mode) }, 'global')
				])
			]),
			E('label', {}, [ _('Rule Sets'), E('input', {
				'class': 'cbi-input-text',
				'name': 'rulesets',
				'value': (row.rulesets || []).join(', ')
			}) ]),
			E('label', {}, [
				_('Final'),
				E('select', { 'class': 'cbi-input-select', 'name': 'final' }, [
					E('option', { 'value': 'direct', 'selected': selected('direct', row.final) }, 'direct'),
					E('option', { 'value': 'proxy', 'selected': selected('proxy', row.final || 'proxy') }, 'proxy'),
					E('option', { 'value': 'block', 'selected': selected('block', row.final) }, 'block')
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
					var payload = readRoutingProfileForm(document.querySelector('.singbox-manager-modal-form'), row);
					return callSetRoutingProfile(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Routing profile saved')));
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showDeleteRoutingProfileModal(view, row) {
	ui.showModal(_('Delete Routing Profile'), [
		E('p', {}, _('Delete this routing profile?')),
		E('div', { 'class': 'right' }, [
			E('button', {
				'class': 'btn cbi-button',
				'click': ui.hideModal
			}, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'click': ui.createHandlerFn(view, function() {
					return callDeleteRoutingProfile(row.id).then(function(result) {
						if (!showResult(result, _('Delete failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Routing profile deleted')));
						return refreshView(view);
					});
				})
			}, _('Delete'))
		])
	]);
}

function showRuleSetModal(view, row) {
	row = row || { enabled: true, type: 'remote', format: 'srs', update_interval: '168h' };
	ui.showModal(row.id ? _('Edit Rule Set') : _('Add Rule Set'), [
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
					E('option', { 'value': 'local', 'selected': selected('local', row.type) }, 'local'),
					E('option', { 'value': 'remote', 'selected': selected('remote', row.type || 'remote') }, 'remote')
				])
			]),
			E('label', {}, [
				_('Format'),
				E('select', { 'class': 'cbi-input-select', 'name': 'format' }, [
					E('option', { 'value': 'srs', 'selected': selected('srs', row.format || 'srs') }, 'srs'),
					E('option', { 'value': 'binary', 'selected': selected('binary', row.format) }, 'binary'),
					E('option', { 'value': 'source', 'selected': selected('source', row.format) }, 'source')
				])
			]),
			E('label', {}, [ _('URL'), E('input', {
				'class': 'cbi-input-text',
				'name': 'url',
				'value': row.url || ''
			}) ]),
			E('label', {}, [ _('Path'), E('input', {
				'class': 'cbi-input-text',
				'name': 'path',
				'value': row.path || ''
			}) ]),
			E('label', {}, [ _('Update Interval'), E('input', {
				'class': 'cbi-input-text',
				'name': 'update_interval',
				'value': row.update_interval || '168h'
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
					var payload = readRuleSetForm(document.querySelector('.singbox-manager-modal-form'), row);
					return callSetRuleSet(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Rule set saved')));
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showDeleteRuleSetModal(view, row) {
	ui.showModal(_('Delete Rule Set'), [
		E('p', {}, _('Delete this rule set and remove it from routing profiles?')),
		E('div', { 'class': 'right' }, [
			E('button', {
				'class': 'btn cbi-button',
				'click': ui.hideModal
			}, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'click': ui.createHandlerFn(view, function() {
					return callDeleteRuleSet(row.id).then(function(result) {
						if (!showResult(result, _('Delete failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Rule set deleted')));
						return refreshView(view);
					});
				})
			}, _('Delete'))
		])
	]);
}

function showSourceRuleModal(view, row, profiles, device) {
	row = row || {
		id: device ? 'device_' + device.ip.replace(/[^A-Za-z0-9]/g, '_') : '',
		enabled: true,
		profile: profiles.length ? profiles[0].id : '',
		sources: device ? [ device.ip ] : [],
		outbound: 'proxy',
		name: device ? (device.name || device.ip) : ''
	};
	ui.showModal(row.id ? _('Edit Source Rule') : _('Add Source Rule'), [
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
				_('Profile'),
				E('select', { 'class': 'cbi-input-select', 'name': 'profile' }, profiles.map(function(profile) {
					return E('option', {
						'value': profile.id,
						'selected': selected(profile.id, row.profile)
					}, profile.name || profile.id);
				}))
			]),
			E('label', {}, [ _('Source IPs'), E('input', {
				'class': 'cbi-input-text',
				'name': 'sources',
				'value': (row.sources || []).join(', ')
			}) ]),
			E('label', {}, [
				_('Outbound'),
				E('select', { 'class': 'cbi-input-select', 'name': 'outbound' }, [
					E('option', { 'value': 'proxy', 'selected': selected('proxy', row.outbound || 'proxy') }, 'proxy'),
					E('option', { 'value': 'direct', 'selected': selected('direct', row.outbound) }, 'direct'),
					E('option', { 'value': 'dns', 'selected': selected('dns', row.outbound) }, 'dns-only'),
					E('option', { 'value': 'block', 'selected': selected('block', row.outbound) }, 'block')
				])
			])
		]),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(view, function() {
					var payload = readSourceRuleForm(document.querySelector('.singbox-manager-modal-form'), row);
					return callSetSourceRule(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Source rule saved')));
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showDeleteSourceRuleModal(view, row) {
	ui.showModal(_('Delete Source Rule'), [
		E('p', {}, _('Delete this source routing rule?')),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'click': ui.createHandlerFn(view, function() {
					return callDeleteSourceRule(row.id).then(function(result) {
						if (!showResult(result, _('Delete failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Source rule deleted')));
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
			E('th', {}, _('Rule Sets')),
			E('th', {}, _('Final')),
			E('th', {}, _('Enabled')),
			E('th', {}, '')
		])),
		E('tbody', {}, profiles.length ? profiles.map(function(profile) {
			return E('tr', {}, [
				E('td', {}, valueOrDash(profile.name || profile.id)),
				E('td', {}, valueOrDash(profile.mode)),
				E('td', {}, (profile.rulesets || []).join(', ') || '-'),
				E('td', {}, valueOrDash(profile.final)),
				E('td', {}, profile.enabled ? _('Yes') : _('No')),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() {
							showRoutingProfileModal(view, profile);
						})
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() {
							showDeleteRoutingProfileModal(view, profile);
						})
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 6 }, _('No routing profiles'))))
	]);
}

function renderRuleSets(view, rulesets) {
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Rule Set')),
			E('th', {}, _('Type')),
			E('th', {}, _('Format')),
			E('th', {}, _('Path')),
			E('th', {}, _('Last Update')),
			E('th', {}, _('Last Error')),
			E('th', {}, _('Enabled')),
			E('th', {}, '')
		])),
		E('tbody', {}, rulesets.length ? rulesets.map(function(row) {
			var remote = row.enabled && row.type === 'remote' && row.url;
			return E('tr', {}, [
				E('td', {}, valueOrDash(row.name || row.id)),
				E('td', {}, valueOrDash(row.type)),
				E('td', {}, valueOrDash(row.format)),
				E('td', {}, valueOrDash(row.path)),
				E('td', {}, valueOrDash(row.last_update)),
				E('td', { 'class': row.last_error ? 'singbox-manager-status-error' : '' }, valueOrDash(row.last_error)),
				E('td', {}, row.enabled ? _('Yes') : _('No')),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button',
						'disabled': remote ? null : 'disabled',
						'click': ui.createHandlerFn(view, function() {
							return callRefreshRuleSet(row.id).then(function(result) {
								if (result.ok)
									ui.addNotification(null, E('p', _('Rule set refreshed: %s').format(result.path || row.id)));
								else
									ui.addNotification(null, E('p', (result.errors || [ _('Rule set refresh failed') ]).join('; ')));
							}).then(function() {
								return refreshView(view);
							});
						})
					}, _('Refresh')),
					E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() {
							showRuleSetModal(view, row);
						})
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() {
							showDeleteRuleSetModal(view, row);
						})
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 8 }, _('No rule sets'))))
	]);
}

function renderSourceRules(view, rows, profiles) {
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Rule')),
			E('th', {}, _('Profile')),
			E('th', {}, _('Source IPs')),
			E('th', {}, _('Outbound')),
			E('th', {}, _('Enabled')),
			E('th', {}, '')
		])),
		E('tbody', {}, rows.length ? rows.map(function(row) {
			return E('tr', {}, [
				E('td', {}, valueOrDash(row.name || row.id)),
				E('td', {}, valueOrDash(row.profile)),
				E('td', {}, (row.sources || []).join(', ') || '-'),
				E('td', {}, valueOrDash(row.outbound)),
				E('td', {}, row.enabled ? _('Yes') : _('No')),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() {
							showSourceRuleModal(view, row, profiles);
						})
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() {
							showDeleteSourceRuleModal(view, row);
						})
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 6 }, _('No source rules'))))
	]);
}

function renderDevices(view, devices, profiles) {
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Device')),
			E('th', {}, _('IP')),
			E('th', {}, _('MAC')),
			E('th', {}, _('Source')),
			E('th', {}, '')
		])),
		E('tbody', {}, devices.length ? devices.map(function(device) {
			return E('tr', {}, [
				E('td', {}, valueOrDash(device.name)),
				E('td', {}, valueOrDash(device.ip)),
				E('td', {}, valueOrDash(device.mac)),
				E('td', {}, valueOrDash(device.source)),
				E('td', {}, E('button', {
					'class': 'btn cbi-button cbi-button-add',
					'disabled': profiles.length ? null : 'disabled',
					'click': ui.createHandlerFn(view, function() {
						showSourceRuleModal(view, null, profiles, device);
					})
				}, _('Route')))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 5 }, _('No connected devices'))))
	]);
}

return view.extend({
	load: function() {
		return callRuleSets();
	},

	render: function(data) {
		var view = this;
		var profiles = (data && data.profiles) || [];
		var rulesets = (data && data.rulesets) || [];
		var sourceRules = (data && data.source_rules) || [];
		var devices = (data && data.devices) || [];
		return E('div', { 'class': 'singbox-manager-page' }, [
			E('style', {}, [
				'.singbox-manager-page{display:grid;gap:16px}',
				'.singbox-manager-section{display:grid;gap:10px}',
				'.singbox-manager-section-header{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}',
				'.singbox-manager-section h3{margin:0;font-size:16px}',
				'.singbox-manager-modal-form{display:grid;gap:12px;min-width:min(560px,90vw)}',
				'.singbox-manager-modal-form label{display:grid;gap:4px;font-size:12px;color:var(--text-color-medium)}',
				'.singbox-manager-check{display:flex!important;align-items:center;gap:8px}',
				'.singbox-manager-table{width:100%;border-collapse:collapse}',
				'.singbox-manager-table th,.singbox-manager-table td{padding:10px;border-bottom:1px solid var(--border-color-medium);text-align:left;vertical-align:middle}',
				'.singbox-manager-table th{font-size:12px;color:var(--text-color-medium);font-weight:600}',
				'.singbox-manager-actions{display:flex;gap:8px;flex-wrap:wrap}',
				'.singbox-manager-status-error{color:#b42318;font-weight:600}',
				'@media(max-width:700px){.singbox-manager-table{display:block;overflow-x:auto;white-space:nowrap}}'
			].join('')),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('div', { 'class': 'singbox-manager-section-header' }, [
					E('h3', {}, _('Routing Profiles')),
					E('button', {
						'class': 'btn cbi-button cbi-button-add',
						'click': ui.createHandlerFn(view, function() {
							showRoutingProfileModal(view);
						})
					}, _('Add'))
				]),
				renderProfiles(view, profiles)
			]),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('div', { 'class': 'singbox-manager-section-header' }, [
					E('h3', {}, _('Rule Sets')),
					E('button', {
						'class': 'btn cbi-button cbi-button-add',
						'click': ui.createHandlerFn(view, function() {
							showRuleSetModal(view);
						})
					}, _('Add'))
				]),
				renderRuleSets(view, rulesets)
			]),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('div', { 'class': 'singbox-manager-section-header' }, [
					E('h3', {}, _('Source Routing')),
					E('button', {
						'class': 'btn cbi-button cbi-button-add',
						'disabled': profiles.length ? null : 'disabled',
						'click': ui.createHandlerFn(view, function() {
							showSourceRuleModal(view, null, profiles);
						})
					}, _('Add'))
				]),
				renderSourceRules(view, sourceRules, profiles)
			]),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('h3', {}, _('Connected Devices')),
				renderDevices(view, devices, profiles)
			])
		]);
	}
});
