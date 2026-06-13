'use strict';
'require view';
'require rpc';
'require ui';

var callPAC = rpc.declare({
	object: 'singbox.manager',
	method: 'pac',
	expect: { '': {} }
});

var callSetPAC = rpc.declare({
	object: 'singbox.manager',
	method: 'pac_set',
	params: [ 'pac' ],
	expect: { '': {} }
});

var callSetCustomPAC = rpc.declare({
	object: 'singbox.manager',
	method: 'pac_custom_set',
	params: [ 'pac' ],
	expect: { '': {} }
});

var callDeleteCustomPAC = rpc.declare({
	object: 'singbox.manager',
	method: 'pac_custom_delete',
	params: [ 'id' ],
	expect: { '': {} }
});

var callSaveRenderedPAC = rpc.declare({
	object: 'singbox.manager',
	method: 'pac_custom_save',
	params: [ 'pac' ],
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

function splitLines(value) {
	return (value || '').split(/\n+/).map(function(item) {
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

function renderList(values, emptyText) {
	values = values || [];
	return values.length ? E('ul', { 'class': 'singbox-manager-list' }, values.map(function(value) {
		return E('li', {}, value);
	})) : E('span', {}, emptyText || '-');
}

function readSettings(root) {
	var enabled = field(root, 'enabled');
	var localBypass = field(root, 'local_bypass');
	return {
		enabled: enabled ? enabled.checked : false,
		source: formValue(root, 'source') || 'generated',
		selected_custom: formValue(root, 'selected_custom'),
		local_bypass: localBypass ? localBypass.checked : true,
		whitelist: splitLines(formValue(root, 'whitelist')),
		blacklist: splitLines(formValue(root, 'blacklist')),
		custom_rules: splitLines(formValue(root, 'custom_rules'))
	};
}

function readCustomPAC(root, row) {
	var enabled = field(root, 'enabled');
	return {
		id: row.id || formValue(root, 'id'),
		enabled: enabled ? enabled.checked : true,
		name: formValue(root, 'name'),
		content: formValue(root, 'content')
	};
}

function showSettingsModal(view, data) {
	data = data || {};
	ui.showModal(_('PAC Settings'), [
		E('div', { 'class': 'singbox-manager-modal-form' }, [
			E('label', { 'class': 'singbox-manager-check' }, [
				E('input', { 'type': 'checkbox', 'name': 'enabled', 'checked': data.enabled ? 'checked' : null }),
				_('Enabled')
			]),
			E('label', {}, [
				_('Source'),
				E('select', { 'class': 'cbi-input-select', 'name': 'source' }, [
					E('option', { 'value': 'generated', 'selected': selected('generated', data.source || 'generated') }, _('Generated')),
					E('option', { 'value': 'custom', 'selected': selected('custom', data.source) }, _('Custom'))
				])
			]),
			E('label', {}, [
				_('Custom PAC'),
				E('select', { 'class': 'cbi-input-select', 'name': 'selected_custom' },
					[ E('option', { 'value': '' }, '-') ].concat((data.custom_pacs || []).map(function(row) {
						return E('option', {
							'value': row.id,
							'selected': selected(row.id, data.selected_custom)
						}, row.name || row.id);
					})))
			]),
			E('label', { 'class': 'singbox-manager-check' }, [
				E('input', { 'type': 'checkbox', 'name': 'local_bypass', 'checked': data.local_bypass !== false ? 'checked' : null }),
				_('Local Bypass')
			]),
			E('label', {}, [ _('Whitelist'), E('textarea', {
				'class': 'cbi-input-textarea',
				'name': 'whitelist',
				'rows': '4'
			}, (data.whitelist || []).join('\n')) ]),
			E('label', {}, [ _('Blacklist'), E('textarea', {
				'class': 'cbi-input-textarea',
				'name': 'blacklist',
				'rows': '4'
			}, (data.blacklist || []).join('\n')) ]),
			E('label', {}, [ _('Custom Rules'), E('textarea', {
				'class': 'cbi-input-textarea',
				'name': 'custom_rules',
				'rows': '4'
			}, (data.custom_rules || []).join('\n')) ])
		]),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(view, function() {
					var payload = readSettings(document.querySelector('.singbox-manager-modal-form'));
					return callSetPAC(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showCustomPACModal(view, row) {
	row = row || { enabled: true };
	ui.showModal(row.id ? _('Edit Custom PAC') : _('Add Custom PAC'), [
		E('div', { 'class': 'singbox-manager-modal-form' }, [
			E('label', { 'class': 'singbox-manager-check' }, [
				E('input', { 'type': 'checkbox', 'name': 'enabled', 'checked': row.enabled !== false ? 'checked' : null }),
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
			E('label', {}, [ _('Content'), E('textarea', {
				'class': 'cbi-input-textarea singbox-manager-code',
				'name': 'content',
				'rows': '14'
			}, row.content || '') ])
		]),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(view, function() {
					var payload = readCustomPAC(document.querySelector('.singbox-manager-modal-form'), row);
					return callSetCustomPAC(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showSaveRenderedModal(view) {
	ui.showModal(_('Save Generated PAC'), [
		E('div', { 'class': 'singbox-manager-modal-form' }, [
			E('label', { 'class': 'singbox-manager-check' }, [
				E('input', { 'type': 'checkbox', 'name': 'enabled', 'checked': 'checked' }),
				_('Enabled')
			]),
			E('label', {}, [ _('ID'), E('input', { 'class': 'cbi-input-text', 'name': 'id' }) ]),
			E('label', {}, [ _('Name'), E('input', { 'class': 'cbi-input-text', 'name': 'name' }) ])
		]),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'click': ui.createHandlerFn(view, function() {
					var payload = readCustomPAC(document.querySelector('.singbox-manager-modal-form'), {});
					return callSaveRenderedPAC(payload).then(function(result) {
						if (!showResult(result, _('Save failed')))
							return;
						ui.hideModal();
						return refreshView(view);
					});
				})
			}, _('Save'))
		])
	]);
}

function showDeleteModal(view, row) {
	ui.showModal(_('Delete Custom PAC'), [
		E('p', {}, _('Delete this custom PAC?')),
		E('div', { 'class': 'right' }, [
			E('button', { 'class': 'btn cbi-button', 'click': ui.hideModal }, _('Cancel')),
			' ',
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'click': ui.createHandlerFn(view, function() {
					return callDeleteCustomPAC(row.id).then(function(result) {
						if (!showResult(result, _('Delete failed')))
							return;
						ui.hideModal();
						return refreshView(view);
					});
				})
			}, _('Delete'))
		])
	]);
}

function renderCustomPACs(view, rows) {
	return E('table', { 'class': 'singbox-manager-table' }, [
		E('thead', {}, E('tr', {}, [
			E('th', {}, _('Name')),
			E('th', {}, _('Enabled')),
			E('th', {}, '')
		])),
		E('tbody', {}, rows.length ? rows.map(function(row) {
			return E('tr', {}, [
				E('td', {}, valueOrDash(row.name || row.id)),
				E('td', {}, row.enabled ? _('Yes') : _('No')),
				E('td', {}, E('div', { 'class': 'singbox-manager-actions' }, [
					E('button', {
						'class': 'btn cbi-button',
						'click': ui.createHandlerFn(view, function() { showCustomPACModal(view, row); })
					}, _('Edit')),
					E('button', {
						'class': 'btn cbi-button cbi-button-remove',
						'click': ui.createHandlerFn(view, function() { showDeleteModal(view, row); })
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 3 }, _('No custom PACs'))))
	]);
}

return view.extend({
	load: function() {
		return callPAC();
	},

	render: function(data) {
		data = data || {};
		var view = this;
		return E('div', { 'class': 'singbox-manager-page' }, [
			E('style', {}, [
				'.singbox-manager-page{display:grid;gap:16px}',
				'.singbox-manager-section{display:grid;gap:10px}',
				'.singbox-manager-section-header{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}',
				'.singbox-manager-section h3{margin:0;font-size:16px}',
				'.singbox-manager-grid{display:grid;grid-template-columns:minmax(120px,180px) minmax(0,1fr);gap:10px 14px;align-items:start}',
				'.singbox-manager-label{font-size:12px;color:var(--text-color-medium);font-weight:600}',
				'.singbox-manager-list{margin:0;padding-left:18px}',
				'.singbox-manager-actions{display:flex;gap:8px;flex-wrap:wrap}',
				'.singbox-manager-modal-form{display:grid;gap:12px;min-width:min(620px,90vw)}',
				'.singbox-manager-modal-form label{display:grid;gap:4px;font-size:12px;color:var(--text-color-medium)}',
				'.singbox-manager-check{display:flex!important;align-items:center;gap:8px}',
				'.singbox-manager-code{font:12px/1.5 monospace}',
				'.singbox-manager-table{width:100%;border-collapse:collapse}',
				'.singbox-manager-table th,.singbox-manager-table td{padding:10px;border-bottom:1px solid var(--border-color-medium);text-align:left;vertical-align:middle}',
				'.singbox-manager-table th{font-size:12px;color:var(--text-color-medium);font-weight:600}',
				'.singbox-manager-preview{box-sizing:border-box;max-width:100%;min-height:220px;overflow:auto;padding:12px;border:1px solid var(--border-color-medium);background:var(--background-color-high);font:12px/1.5 monospace;white-space:pre}',
				'@media(max-width:700px){.singbox-manager-grid{grid-template-columns:1fr}.singbox-manager-table{display:block;overflow-x:auto;white-space:nowrap}.singbox-manager-preview{white-space:pre-wrap;word-break:break-word}}'
			].join('')),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('div', { 'class': 'singbox-manager-section-header' }, [
					E('h3', {}, _('Proxy Auto-Config')),
					E('div', { 'class': 'singbox-manager-actions' }, [
						E('button', {
							'class': 'btn cbi-button',
							'click': ui.createHandlerFn(view, function() { showSettingsModal(view, data); })
						}, _('Settings')),
						E('button', {
							'class': 'btn cbi-button cbi-button-add',
							'click': ui.createHandlerFn(view, function() { showCustomPACModal(view); })
						}, _('Add Custom')),
						E('button', {
							'class': 'btn cbi-button cbi-button-apply',
							'click': ui.createHandlerFn(view, function() { showSaveRenderedModal(view); })
						}, _('Save Generated'))
					])
				]),
				E('div', { 'class': 'singbox-manager-grid' }, [
					E('div', { 'class': 'singbox-manager-label' }, _('Enabled')),
					E('div', {}, data.enabled ? _('Yes') : _('No')),
					E('div', { 'class': 'singbox-manager-label' }, _('URL')),
					E('div', {}, valueOrDash(data.url)),
					E('div', { 'class': 'singbox-manager-label' }, _('Source')),
					E('div', {}, valueOrDash(data.source)),
					E('div', { 'class': 'singbox-manager-label' }, _('Selected Custom')),
					E('div', {}, valueOrDash(data.selected_custom)),
					E('div', { 'class': 'singbox-manager-label' }, _('Local Bypass')),
					E('div', {}, data.local_bypass ? _('Yes') : _('No')),
					E('div', { 'class': 'singbox-manager-label' }, _('Whitelist')),
					E('div', {}, renderList(data.whitelist, _('No whitelist rules'))),
					E('div', { 'class': 'singbox-manager-label' }, _('Blacklist')),
					E('div', {}, renderList(data.blacklist, _('No blacklist rules'))),
					E('div', { 'class': 'singbox-manager-label' }, _('Custom Rules')),
					E('div', {}, renderList(data.custom_rules, _('No custom rules')))
				])
			]),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('h3', {}, _('Custom PACs')),
				renderCustomPACs(view, data.custom_pacs || [])
			]),
			E('div', { 'class': 'singbox-manager-section' }, [
				E('h3', {}, _('PAC Preview')),
				E('pre', { 'class': 'singbox-manager-preview' }, valueOrDash(data.preview))
			])
		]);
	}
});
