'use strict';
'require view';
'require rpc';
'require ui';
'require view.singbox-manager.theme as theme';

var callRouting = rpc.declare({
	object: 'singbox.manager',
	method: 'routing',
	expect: { '': {} }
});

var callSetRouteRule = rpc.declare({
	object: 'singbox.manager',
	method: 'route_rule_set',
	params: [ 'rule' ],
	expect: { '': {} }
});

var callDeleteRouteRule = rpc.declare({
	object: 'singbox.manager',
	method: 'route_rule_delete',
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

var callSetGroup = rpc.declare({
	object: 'singbox.manager',
	method: 'group_set',
	params: [ 'group' ],
	expect: { '': {} }
});

var callSetMode = rpc.declare({
	object: 'singbox.manager',
	method: 'manager_set_mode',
	params: [ 'mode' ],
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

// devicePicker renders the LAN devices as click-to-add chips next to a
// sources/IP input, so an address can be picked instead of typed.
function devicePicker(devices, fieldName) {
	devices = (devices || []).filter(function(device) { return device.ip; });
	if (!devices.length)
		return '';
	return E('div', { 'class': 'singbox-manager-devicepicker' }, [
		E('div', { 'class': 'singbox-manager-devicepicker-label' }, _('LAN devices — click to add')),
		E('div', { 'class': 'singbox-manager-chips' }, devices.map(function(device) {
			var label = device.name ? (device.name + ' · ' + device.ip) : device.ip;
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

function readRouteRuleForm(root, row, activeGroup) {
	var enabled = field(root, 'enabled');
	return {
		id: row.id || formValue(root, 'id'),
		enabled: enabled ? enabled.checked : true,
		name: formValue(root, 'name'),
		group: activeGroup,
		sources: splitList(formValue(root, 'sources')),
		rulesets: splitList(formValue(root, 'rulesets')),
		outbound: formValue(root, 'outbound')
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

function showRouteRuleModal(view, data, row) {
	row = row || { enabled: true, sources: [], rulesets: [], outbound: 'direct' };
	ui.showModal(row.id ? _('Edit Route Rule') : _('Add Route Rule'), [
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
				'disabled': row.id ? 'disabled' : null,
				'placeholder': 'r10_iran_direct'
			}) ]),
			E('label', {}, [ _('Name'), E('input', {
				'class': 'cbi-input-text',
				'name': 'name',
				'value': row.name || row.id || ''
			}) ]),
			E('label', {}, [ _('Source IPs / CIDRs (optional)'), E('input', {
				'class': 'cbi-input-text',
				'name': 'sources',
				'value': (row.sources || []).join(', '),
				'placeholder': '192.168.200.125'
			}) ]),
			devicePicker(data.devices, 'sources'),
			E('label', {}, [ _('Rule sets (optional)'), E('input', {
				'class': 'cbi-input-text',
				'name': 'rulesets',
				'value': (row.rulesets || []).join(', '),
				'placeholder': 'geoip-ir'
			}) ]),
			E('label', {}, [
				_('Outbound'),
				E('select', { 'class': 'cbi-input-select', 'name': 'outbound' }, [
					E('option', { 'value': 'direct', 'selected': selected('direct', row.outbound || 'direct') }, 'direct'),
					E('option', { 'value': 'proxy', 'selected': selected('proxy', row.outbound) }, 'proxy'),
					E('option', { 'value': 'block', 'selected': selected('block', row.outbound) }, 'block')
				])
			]),
			E('p', { 'class': 'singbox-manager-inline-help' }, _('A rule matches traffic by source AND rule set (when both are set). Provide at least one. Rules apply in ID order; unmatched traffic uses the final outbound.'))
		]),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(view, function() {
					var payload = readRouteRuleForm(document.querySelector('.singbox-manager-modal-form'), row, data.active_group);
					return callSetRouteRule(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Route rule saved')));
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showDeleteRouteRuleModal(view, row) {
	ui.showModal(_('Delete Route Rule'), [
		E('p', {}, _('Delete this route rule?')),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'click': ui.createHandlerFn(view, function() {
					return callDeleteRouteRule(row.id).then(function(result) {
						if (!showResult(result, _('Delete failed')))
							return;
						ui.hideModal();
						ui.addNotification(null, E('p', _('Route rule deleted')));
						return refreshView(view);
					});
				})
			}, _('Delete'))
		])
	]);
}

function showRuleSetModal(view, row) {
	row = row || { enabled: true, type: 'remote', format: 'binary', update_interval: '168h' };
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
				'disabled': row.id ? 'disabled' : null,
				'placeholder': 'geoip-ir'
			}) ]),
			E('label', {}, [ _('Name'), E('input', {
				'class': 'cbi-input-text',
				'name': 'name',
				'value': row.name || row.id || ''
			}) ]),
			E('label', {}, [
				_('Type'),
				E('select', { 'class': 'cbi-input-select', 'name': 'type' }, [
					E('option', { 'value': 'remote', 'selected': selected('remote', row.type || 'remote') }, 'remote'),
					E('option', { 'value': 'local', 'selected': selected('local', row.type) }, 'local')
				])
			]),
			E('label', {}, [
				_('Format'),
				E('select', { 'class': 'cbi-input-select', 'name': 'format' }, [
					E('option', { 'value': 'binary', 'selected': selected('binary', row.format || 'binary') }, 'binary (.srs)'),
					E('option', { 'value': 'source', 'selected': selected('source', row.format) }, 'source (.json)')
				])
			]),
			E('label', {}, [ _('URL (remote)'), E('input', {
				'class': 'cbi-input-text',
				'name': 'url',
				'value': row.url || '',
				'placeholder': 'https://example.com/geoip-ir.srs'
			}) ]),
			E('label', {}, [ _('Path (local / cache)'), E('input', {
				'class': 'cbi-input-text',
				'name': 'path',
				'value': row.path || ''
			}) ]),
			E('label', {}, [ _('Update interval'), E('input', {
				'class': 'cbi-input-text',
				'name': 'update_interval',
				'value': row.update_interval || '168h'
			}) ])
		]),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
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
		E('p', {}, _('Delete this rule set? Rules that only matched it are removed too.')),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
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

function renderMode(view, data) {
	var group = (data && data.group) || {};
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('h3', {}, _('Routing Mode')),
		(data.errors && data.errors.length) ? E('div', { 'class': 'singbox-manager-warning' }, data.errors.join('; ')) : '',
		E('div', { 'class': 'singbox-manager-controls' }, [
			E('label', { 'class': 'singbox-manager-inline-control', 'title': _('direct = no proxy, global = everything proxied, rule = apply the route rules below') }, [
				_('Mode'),
				E('select', {
					'class': 'cbi-input-select',
					'change': ui.createHandlerFn(view, function(ev) {
						return callSetMode(ev.target.value).then(function(result) {
							showResult(result, _('Save failed'));
							ui.addNotification(null, E('p', _('Runtime mode updated')));
							return refreshView(view);
						});
					})
				}, [
					E('option', { 'value': 'direct', 'selected': selected('direct', data.runtime_mode) }, 'direct'),
					E('option', { 'value': 'rule', 'selected': selected('rule', data.runtime_mode || 'rule') }, 'rule'),
					E('option', { 'value': 'global', 'selected': selected('global', data.runtime_mode) }, 'global')
				])
			]),
			E('label', { 'class': 'singbox-manager-inline-control', 'title': _('Outbound for traffic that matches no rule (rule mode only).') }, [
				_('Final'),
				E('select', {
					'class': 'cbi-input-select',
					'change': ui.createHandlerFn(view, function(ev) {
						var next = Object.assign({}, group, { route_final: ev.target.value });
						return callSetGroup(next).then(function(result) {
							showResult(result, _('Save failed'));
							ui.addNotification(null, E('p', _('Final outbound updated')));
							return refreshView(view);
						});
					})
				}, [
					E('option', { 'value': 'proxy', 'selected': selected('proxy', data.route_final || 'proxy') }, 'proxy'),
					E('option', { 'value': 'direct', 'selected': selected('direct', data.route_final) }, 'direct'),
					E('option', { 'value': 'block', 'selected': selected('block', data.route_final) }, 'block')
				])
			])
		])
	]);
}

function renderRouteRules(view, data) {
	var rules = ((data && data.route_rules) || []).filter(function(rule) {
		return !rule.group || rule.group === data.active_group;
	});
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Rule')),
			E('th', {}, _('Sources')),
			E('th', {}, _('Rule sets')),
			E('th', {}, _('Outbound')),
			E('th', {}, _('Enabled')),
			E('th', {}, '')
		])),
		E('tbody', {}, rules.length ? rules.map(function(rule) {
			return E('tr', {}, [
				E('td', {}, valueOrDash(rule.name || rule.id)),
				E('td', {}, (rule.sources || []).join(', ') || '-'),
				E('td', {}, (rule.rulesets || []).join(', ') || '-'),
				E('td', {}, valueOrDash(rule.outbound)),
				E('td', {}, rule.enabled ? _('Yes') : _('No')),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() { showRouteRuleModal(view, data, rule); })
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() { showDeleteRouteRuleModal(view, rule); })
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 6 }, _('No route rules — all traffic uses the final outbound'))))
	]);
}

function renderRuleSets(view, rulesets) {
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Rule set')),
			E('th', {}, _('Type')),
			E('th', {}, _('Updated')),
			E('th', {}, _('Enabled')),
			E('th', {}, '')
		])),
		E('tbody', {}, rulesets.length ? rulesets.map(function(ruleset) {
			return E('tr', {}, [
				E('td', {}, valueOrDash(ruleset.name || ruleset.id)),
				E('td', {}, valueOrDash(ruleset.type)),
				E('td', {}, valueOrDash(ruleset.last_update)),
				E('td', {}, ruleset.enabled ? _('Yes') : _('No')),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button',
						'disabled': ruleset.type === 'remote' ? null : 'disabled',
						'click': ui.createHandlerFn(view, function() {
							return callRefreshRuleSet(ruleset.id).then(function(result) {
								if (result.ok)
									ui.addNotification(null, E('p', _('Rule set updated (%d bytes)').format(result.bytes || 0)));
								else
									ui.addNotification(null, E('p', (result.errors || [ _('Update failed') ]).join('; ')));
								return refreshView(view);
							});
						})
					}, _('Update')),
					E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() { showRuleSetModal(view, ruleset); })
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() { showDeleteRuleSetModal(view, ruleset); })
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 5 }, _('No rule sets'))))
	]);
}

return view.extend({
	load: function() {
		return callRouting();
	},

	render: function(data) {
		var view = this;
		data = data || {};
		var rulesets = data.rulesets || [];
		theme.inject();
		return E('div', { 'class': 'singbox-manager-page' }, [
			renderMode(view, data),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('div', { 'class': 'singbox-manager-section-header' }, [
					E('h3', {}, _('Route Rules')),
					E('button', {
						'class': 'btn cbi-button cbi-button-add',
						'click': ui.createHandlerFn(view, function() { showRouteRuleModal(view, data); })
					}, _('Add'))
				]),
				renderRouteRules(view, data)
			]),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('div', { 'class': 'singbox-manager-section-header' }, [
					E('h3', {}, _('Rule Sets')),
					E('button', {
						'class': 'btn cbi-button cbi-button-add',
						'click': ui.createHandlerFn(view, function() { showRuleSetModal(view); })
					}, _('Add'))
				]),
				renderRuleSets(view, rulesets)
			])
		]);
	}
});
