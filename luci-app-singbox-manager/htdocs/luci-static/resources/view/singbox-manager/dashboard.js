'use strict';
'require view';
'require rpc';
'require poll';
'require ui';

var callStatus = rpc.declare({
	object: 'singbox.manager',
	method: 'status',
	expect: { '': {} }
});

var callHealthCheck = rpc.declare({
	object: 'singbox.manager',
	method: 'health_check',
	expect: { '': {} }
});

var callStart = rpc.declare({
	object: 'singbox.manager',
	method: 'start',
	expect: { '': {} }
});

var callSetManagerEnabled = rpc.declare({
	object: 'singbox.manager',
	method: 'manager_set_enabled',
	params: [ 'enabled' ],
	expect: { '': {} }
});

var callStop = rpc.declare({
	object: 'singbox.manager',
	method: 'stop',
	expect: { '': {} }
});

var callRestart = rpc.declare({
	object: 'singbox.manager',
	method: 'restart',
	expect: { '': {} }
});

var callReload = rpc.declare({
	object: 'singbox.manager',
	method: 'reload',
	expect: { '': {} }
});

function valueOrDash(value) {
	if (value === null || value === undefined || value === '')
		return '-';
	return value;
}

function formatBytes(value) {
	value = Number(value || 0);
	if (value < 1024)
		return '%d B'.format(value);
	if (value < 1024 * 1024)
		return '%.1f KiB'.format(value / 1024);
	if (value < 1024 * 1024 * 1024)
		return '%.1f MiB'.format(value / 1024 / 1024);
	return '%.1f GiB'.format(value / 1024 / 1024 / 1024);
}

function renderMetric(label, value) {
	return E('div', { 'class': 'singbox-manager-metric' }, [
		E('div', { 'class': 'singbox-manager-metric-label' }, label),
		E('div', { 'class': 'singbox-manager-metric-value' }, valueOrDash(value))
	]);
}

function showResult(result, successText, failureText) {
	var message = result && result.ok ? (result.message || successText) : ((result && result.errors || [ failureText ]).join('; '));
	ui.addNotification(null, E('p', message));
}

function startRuntime(data) {
	var enabled = data.manager_enabled ? Promise.resolve({ ok: true }) : callSetManagerEnabled(true);
	return enabled.then(function(result) {
		if (!result.ok)
			return result;
		return callStart();
	});
}

function updateHistory(view, data) {
	view.statsHistory = view.statsHistory || [];
	view.statsHistory.push({
		connections: Number(data.connections || 0),
		rx: Number(data.rx_bytes || 0),
		tx: Number(data.tx_bytes || 0)
	});
	if (view.statsHistory.length > 30)
		view.statsHistory.shift();
	return view.statsHistory;
}

function renderBars(values, className) {
	var max = values.reduce(function(best, value) {
		return Math.max(best, value);
	}, 1);
	return E('div', { 'class': 'singbox-manager-bars ' + className }, values.map(function(value) {
		return E('span', {
			'style': 'height:%d%%'.format(Math.max(8, Math.round(value / max * 100)))
		});
	}));
}

function renderUsageGraph(history) {
	var connections = history.map(function(item) { return item.connections; });
	var traffic = history.map(function(item, index) {
		if (index === 0)
			return 0;
		var previous = history[index - 1];
		return Math.max(0, item.rx - previous.rx) + Math.max(0, item.tx - previous.tx);
	});
	return E('div', { 'class': 'singbox-manager-graph' }, [
		E('div', {}, [
			E('div', { 'class': 'singbox-manager-graph-label' }, _('Connections')),
			renderBars(connections, 'singbox-manager-bars-connections')
		]),
		E('div', {}, [
			E('div', { 'class': 'singbox-manager-graph-label' }, _('Traffic')),
			renderBars(traffic, 'singbox-manager-bars-traffic')
		])
	]);
}

function renderStatus(data) {
	var running = data.running ? _('Running') : _('Stopped');
	var daemon = data.daemon ? _('Online') : _('Offline');
	var history = updateHistory(this, data || {});

	return E('div', { 'class': 'singbox-manager-dashboard' }, [
		E('style', {}, [
			'.singbox-manager-dashboard{display:grid;gap:16px}',
			'.singbox-manager-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:12px}',
			'.singbox-manager-metric{border:1px solid var(--border-color-medium);border-radius:8px;padding:12px;background:var(--background-color-high)}',
			'.singbox-manager-metric-label{font-size:12px;color:var(--text-color-medium);margin-bottom:6px}',
			'.singbox-manager-metric-value{font-size:18px;font-weight:600;overflow-wrap:anywhere}',
			'.singbox-manager-actions{display:flex;gap:8px;flex-wrap:wrap}',
			'.singbox-manager-graph{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:12px}',
			'.singbox-manager-graph>div{border:1px solid var(--border-color-medium);border-radius:8px;padding:12px;background:var(--background-color-high)}',
			'.singbox-manager-graph-label{font-size:12px;color:var(--text-color-medium);font-weight:600;margin-bottom:8px}',
			'.singbox-manager-bars{height:96px;display:flex;align-items:end;gap:3px}',
			'.singbox-manager-bars span{flex:1;min-width:4px;background:#2271b1;border-radius:2px 2px 0 0}',
			'.singbox-manager-bars-traffic span{background:#0f7a39}'
		].join('')),
		E('div', { 'class': 'singbox-manager-grid' }, [
			renderMetric(_('Daemon'), daemon),
			renderMetric(_('Manager'), data.manager_enabled ? _('Enabled') : _('Disabled')),
			renderMetric(_('Runtime'), running),
			renderMetric(_('Group'), data.active_group),
			renderMetric(_('Mode'), data.runtime_mode),
			renderMetric(_('Strategy'), data.strategy),
			renderMetric(_('Health'), data.health),
			renderMetric(_('Outbound'), data.selected_outbound),
			renderMetric(_('Latency'), data.latency_ms ? '%d ms'.format(data.latency_ms) : '-'),
			renderMetric(_('Memory'), formatBytes((data.memory_kb || 0) * 1024)),
			renderMetric(_('CPU'), '%s%%'.format(valueOrDash(data.cpu_percent))),
			renderMetric(_('Connections'), data.connections || 0),
			renderMetric(_('Download'), formatBytes(data.rx_bytes)),
			renderMetric(_('Upload'), formatBytes(data.tx_bytes))
		]),
		renderUsageGraph(history),
		E('div', { 'class': 'singbox-manager-actions' }, [
			E('button', {
				'class': 'btn cbi-button cbi-button-apply',
				'disabled': data.running ? 'disabled' : null,
				'click': ui.createHandlerFn(this, function() {
					return startRuntime(data).then(function(result) {
						showResult(result, _('sing-box started'), _('Start failed'));
					}).then(L.bind(this.load, this));
				})
			}, _('Start')),
			E('button', {
				'class': 'btn cbi-button cbi-button-remove',
				'disabled': data.running ? null : 'disabled',
				'click': ui.createHandlerFn(this, function() {
					return callStop().then(function(result) {
						showResult(result, _('sing-box stopped'), _('Stop failed'));
					}).then(L.bind(this.load, this));
				})
			}, _('Stop')),
			E('button', {
				'class': 'btn cbi-button',
				'disabled': data.running ? null : 'disabled',
				'click': ui.createHandlerFn(this, function() {
					return callRestart().then(function(result) {
						showResult(result, _('sing-box restarted'), _('Restart failed'));
					}).then(L.bind(this.load, this));
				})
			}, _('Restart')),
			E('button', {
				'class': 'btn cbi-button',
				'click': ui.createHandlerFn(this, function() {
					return callReload().then(function(result) {
						showResult(result, _('sing-box reloaded'), _('Reload failed'));
					}).then(L.bind(this.load, this));
				})
			}, _('Reload')),
			E('button', {
				'class': 'btn cbi-button',
				'click': ui.createHandlerFn(this, function() {
					return rpc.declare({ object: 'singbox.manager', method: 'validate' })().then(function(result) {
						ui.addNotification(null, E('p', result.ok ? _('Configuration is valid') : _('Configuration has errors')));
					});
				})
			}, _('Validate')),
			E('button', {
				'class': 'btn cbi-button',
				'click': ui.createHandlerFn(this, function() {
					return callHealthCheck().then(function(result) {
						ui.addNotification(null, E('p', result.ok ? _('Health check complete') : (result.errors || [ _('Health check failed') ]).join('; ')));
						window.location.reload();
					});
				})
			}, _('Check Health'))
		])
	]);
}

return view.extend({
	load: function() {
		return callStatus();
	},

	render: function(data) {
		var root = renderStatus.call(this, data || {});

		poll.add(L.bind(function() {
			return callStatus().then(function(status) {
				var replacement = renderStatus.call(this, status || {});
				root.parentNode.replaceChild(replacement, root);
				root = replacement;
			}.bind(this));
		}, this), 5);

		return root;
	}
});
