'use strict';
'require view';
'require rpc';
'require poll';
'require ui';

var callLogs = rpc.declare({
	object: 'singbox.manager',
	method: 'logs',
	params: [ 'lines' ],
	expect: { '': {} }
});

function valueOrDash(value) {
	if (value === null || value === undefined || value === '')
		return '-';
	return value;
}

function downloadText(text) {
	var blob = new Blob([ text || '' ], { type: 'text/plain' });
	var url = URL.createObjectURL(blob);
	var link = E('a', { 'href': url, 'download': 'singbox-manager.log' });
	document.body.appendChild(link);
	link.click();
	link.remove();
	window.setTimeout(function() {
		URL.revokeObjectURL(url);
	}, 1000);
}

function renderLogs(view, data) {
	data = data || {};
	var text = data.text || '';
	return E('div', { 'class': 'singbox-manager-page' }, [
		E('style', {}, [
			'.singbox-manager-page{display:grid;gap:16px}',
			'.singbox-manager-section-header{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}',
			'.singbox-manager-actions{display:flex;gap:8px;flex-wrap:wrap}',
			'.singbox-manager-log{box-sizing:border-box;max-width:100%;min-height:420px;overflow:auto;padding:12px;border:1px solid var(--border-color-medium);background:var(--background-color-high);font:12px/1.5 monospace;white-space:pre-wrap;word-break:break-word}'
		].join('')),
		E('div', { 'class': 'singbox-manager-section-header' }, [
			E('h3', {}, _('Logs')),
			E('div', { 'class': 'singbox-manager-actions' }, [
				E('button', {
					'class': 'btn cbi-button',
					'click': ui.createHandlerFn(view, function() {
						return view.load().then(function(result) {
							var replacement = renderLogs(view, result);
							var root = document.querySelector('.singbox-manager-page');
							if (root)
								root.parentNode.replaceChild(replacement, root);
						});
					})
				}, _('Refresh')),
				E('button', {
					'class': 'btn cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(view, function() {
						downloadText(text);
					})
				}, _('Download'))
			])
		]),
		E('pre', { 'class': 'singbox-manager-log' }, valueOrDash(text))
	]);
}

return view.extend({
	load: function() {
		return callLogs(300);
	},

	render: function(data) {
		var view = this;
		var root = renderLogs(view, data);

		poll.add(function() {
			return view.load().then(function(result) {
				var replacement = renderLogs(view, result);
				root.parentNode.replaceChild(replacement, root);
				root = replacement;
			});
		}, 5);

		return root;
	}
});
