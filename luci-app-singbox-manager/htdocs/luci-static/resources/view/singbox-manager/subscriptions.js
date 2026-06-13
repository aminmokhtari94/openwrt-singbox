'use strict';
'require view';
'require rpc';
'require ui';

var callSubscriptions = rpc.declare({
	object: 'singbox.manager',
	method: 'subscriptions',
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

function readImport(root) {
	return {
		input: formValue(root, 'input'),
		name: formValue(root, 'name'),
		id: formValue(root, 'id'),
		format: formValue(root, 'format')
	};
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

function showEditModal(view, row) {
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

function showDeleteModal(view, row) {
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
					'placeholder': 'https://...'
				})
			]),
			E('label', {}, [ _('Name'), E('input', { 'class': 'cbi-input-text', 'name': 'name' }) ]),
			E('label', {}, [ _('ID'), E('input', { 'class': 'cbi-input-text', 'name': 'id' }) ]),
			E('label', {}, [
				_('Format'),
				E('select', { 'class': 'cbi-input-select', 'name': 'format' }, [
					E('option', { 'value': 'auto' }, 'auto'),
					E('option', { 'value': 'plain' }, 'plain'),
					E('option', { 'value': 'base64' }, 'base64')
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
					var request = readImport(document.querySelector('.singbox-manager-import-grid'));
					return callImport(request).then(function(result) {
						if (result.ok) {
							ui.addNotification(null, E('p', _('Imported %d nodes').format(result.imported || 0)));
							ui.hideModal();
							return refreshView(view);
						}
						else if (result.saved)
							ui.addNotification(null, E('p', _('Subscription saved; import failed: %s').format((result.errors || [ _('Import failed') ]).join('; '))));
						else
							ui.addNotification(null, E('p', (result.errors || [ _('Import failed') ]).join('; ')));
					});
				})
			}, _('Import'))
		])
	]);
}

function renderTable(view, rows) {
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
							showEditModal(view, row);
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
							showDeleteModal(view, row);
						})
					}, _('Delete'))
				]))
			]);
		}) : E('tr', {}, E('td', { 'colspan': 9 }, _('No subscriptions'))))
	]);
}

return view.extend({
	load: function() {
		return callSubscriptions();
	},

	render: function(data) {
		var view = this;
		return E('div', { 'class': 'singbox-manager-page' }, [
			E('style', {}, [
				'.singbox-manager-page{display:grid;gap:16px}',
				'.singbox-manager-section-header{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}',
				'.singbox-manager-import-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px}',
				'.singbox-manager-import-grid label{display:grid;gap:4px;font-size:12px;color:var(--text-color-medium)}',
				'.singbox-manager-import-link{grid-column:1/-1}',
				'.singbox-manager-import-link textarea{min-height:96px;resize:vertical}',
				'.singbox-manager-modal-form{display:grid;gap:12px;min-width:min(520px,90vw)}',
				'.singbox-manager-modal-form label{display:grid;gap:4px;font-size:12px;color:var(--text-color-medium)}',
				'.singbox-manager-modal-form label:first-child{display:flex;align-items:center;gap:8px}',
				'.singbox-manager-table{width:100%;border-collapse:collapse}',
				'.singbox-manager-table th,.singbox-manager-table td{padding:10px;border-bottom:1px solid var(--border-color-medium);text-align:left;vertical-align:middle}',
				'.singbox-manager-table th{font-size:12px;color:var(--text-color-medium);font-weight:600}',
				'.singbox-manager-actions{display:flex;gap:8px;flex-wrap:wrap}',
				'.singbox-manager-status-ok{color:#0f7a39;font-weight:600}',
				'.singbox-manager-status-error{color:#b42318;font-weight:600}',
				'@media(max-width:700px){.singbox-manager-table{display:block;overflow-x:auto;white-space:nowrap}}'
			].join('')),
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
			renderTable(view, (data && data.subscriptions) || [])
		]);
	}
});
