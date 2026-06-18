'use strict';
'require view';
'require rpc';
'require poll';
'require ui';
'require view.singbox-manager.theme as theme';

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

var callSetMode = rpc.declare({
	object: 'singbox.manager',
	method: 'manager_set_mode',
	params: [ 'mode' ],
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

var callValidate = rpc.declare({
	object: 'singbox.manager',
	method: 'validate',
	expect: { '': {} }
});

var callLogs = rpc.declare({
	object: 'singbox.manager',
	method: 'logs',
	params: [ 'lines' ],
	expect: { '': {} }
});

var MODE_OPTIONS = [ 'direct', 'rule', 'global' ];
var POLL_INTERVAL = 5;

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
	return '%.2f GiB'.format(value / 1024 / 1024 / 1024);
}

function formatRate(bytesPerSec) {
	return formatBytes(bytesPerSec) + '/s';
}

function healthClass(value) {
	if (value === 'ok')
		return 'ok';
	if (value && value !== 'unknown')
		return 'error';
	return '';
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

function pushHistory(view, data) {
	view.hist = view.hist || [];
	view.hist.push({
		rx: Number(data.rx_bytes || 0),
		tx: Number(data.tx_bytes || 0),
		conn: Number(data.connections || 0)
	});
	if (view.hist.length > 48)
		view.hist.shift();
	return view.hist;
}

function deltaSeries(hist, key) {
	var out = [];
	for (var i = 1; i < hist.length; i++)
		out.push(Math.max(0, hist[i][key] - hist[i - 1][key]));
	return out;
}

// sparkline builds an SVG line+area chart via innerHTML so it renders in the
// SVG namespace (LuCI's E() only creates HTML elements). Inputs are numeric
// byte deltas, so string interpolation here is safe.
function sparkline(values, color) {
	var box = E('div', { 'class': 'singbox-manager-spark' });
	if (!values || values.length < 2) {
		box.appendChild(E('span', { 'class': 'singbox-manager-spark-empty' }, _('Collecting data…')));
		return box;
	}
	var w = 600, h = 72, pad = 5;
	var max = values.reduce(function(m, v) { return Math.max(m, v); }, 1);
	var step = w / (values.length - 1);
	var pts = values.map(function(v, i) {
		return [ i * step, h - pad - (v / max) * (h - 2 * pad) ];
	});
	var line = pts.map(function(p, i) { return (i ? 'L' : 'M') + p[0].toFixed(1) + ',' + p[1].toFixed(1); }).join(' ');
	var area = 'M0,' + h + ' ' + pts.map(function(p) { return 'L' + p[0].toFixed(1) + ',' + p[1].toFixed(1); }).join(' ') + ' L' + w + ',' + h + ' Z';
	box.innerHTML = '<svg viewBox="0 0 ' + w + ' ' + h + '" preserveAspectRatio="none" style="width:100%;height:100%;display:block">'
		+ '<path d="' + area + '" fill="' + color + '" fill-opacity="0.14"/>'
		+ '<path d="' + line + '" fill="none" stroke="' + color + '" stroke-width="2" vector-effect="non-scaling-stroke"/>'
		+ '</svg>';
	return box;
}

function lastValue(series) {
	return series.length ? series[series.length - 1] : 0;
}

function metric(label, value, accent) {
	return E('div', { 'class': 'singbox-manager-metric' }, [
		E('div', { 'class': 'singbox-manager-metric-label' }, label),
		E('div', { 'class': 'singbox-manager-metric-value' + (accent ? ' ' + accent : '') }, valueOrDash(value))
	]);
}

function renderHero(view, data) {
	var running = !!data.running;
	var primary = running
		? E('button', {
			'class': 'btn cbi-button cbi-button-remove',
			'click': ui.createHandlerFn(view, function() {
				return callStop().then(function(result) {
					showResult(result, _('sing-box stopped'), _('Stop failed'));
				}).then(L.bind(view.load, view));
			})
		}, _('Stop'))
		: E('button', {
			'class': 'btn cbi-button cbi-button-apply',
			'click': ui.createHandlerFn(view, function() {
				return startRuntime(data).then(function(result) {
					showResult(result, _('sing-box started'), _('Start failed'));
				}).then(L.bind(view.load, view));
			})
		}, _('Start'));

	var subtitle = running
		? _('PID %s').format(valueOrDash(data.sing_box_pid))
		: (data.manager_enabled ? _('Manager enabled') : _('Manager disabled'));

	var modeSelect = E('select', {
		'class': 'cbi-input-select',
		'change': ui.createHandlerFn(view, function(ev) {
			var mode = ev.target.value;
			return callSetMode(mode).then(function(result) {
				showResult(result, _('Mode set to %s').format(mode), _('Set mode failed'));
			}).then(L.bind(view.load, view));
		})
	}, MODE_OPTIONS.map(function(opt) {
		return E('option', { 'value': opt, 'selected': opt === data.runtime_mode ? 'selected' : null }, opt);
	}));

	return E('div', { 'class': 'singbox-manager-hero' }, [
		E('div', { 'class': 'singbox-manager-hero-status' }, [
			E('span', { 'class': 'singbox-manager-dot' + (running ? ' on' : (data.daemon ? ' idle' : ' off')) }),
			E('div', {}, [
				E('div', { 'class': 'singbox-manager-hero-state' }, running ? _('Running') : _('Stopped')),
				E('div', { 'class': 'singbox-manager-hero-sub' }, subtitle)
			])
		]),
		E('div', { 'class': 'singbox-manager-hero-facts' }, [
			E('div', {}, [ E('span', {}, _('Group')), E('strong', {}, valueOrDash(data.active_group)) ]),
			E('div', {}, [ E('span', {}, _('Mode')), modeSelect ]),
			E('div', {}, [ E('span', {}, _('Outbound')), E('strong', {}, valueOrDash(data.selected_outbound)) ]),
			E('div', {}, [ E('span', {}, _('Health')), E('strong', { 'class': healthClass(data.health) }, valueOrDash(data.health) + (data.latency_ms ? ' · ' + data.latency_ms + ' ms' : '')) ])
		]),
		E('div', { 'class': 'singbox-manager-hero-action' }, primary)
	]);
}

function renderThroughput(view, data, hist) {
	var rxSeries = deltaSeries(hist, 'rx');
	var txSeries = deltaSeries(hist, 'tx');
	return E('div', { 'class': 'singbox-manager-charts' }, [
		E('div', { 'class': 'singbox-manager-chart' }, [
			E('div', { 'class': 'singbox-manager-chart-head' }, [
				E('span', { 'class': 'singbox-manager-chart-title' }, _('Download')),
				E('span', { 'class': 'singbox-manager-chart-rate' }, formatRate(lastValue(rxSeries) / POLL_INTERVAL))
			]),
			sparkline(rxSeries, '#2271b1'),
			E('div', { 'class': 'singbox-manager-chart-foot' }, _('Total %s').format(formatBytes(data.rx_bytes)))
		]),
		E('div', { 'class': 'singbox-manager-chart' }, [
			E('div', { 'class': 'singbox-manager-chart-head' }, [
				E('span', { 'class': 'singbox-manager-chart-title' }, _('Upload')),
				E('span', { 'class': 'singbox-manager-chart-rate' }, formatRate(lastValue(txSeries) / POLL_INTERVAL))
			]),
			sparkline(txSeries, '#0f7a39'),
			E('div', { 'class': 'singbox-manager-chart-foot' }, _('Total %s').format(formatBytes(data.tx_bytes)))
		])
	]);
}

function renderToolbar(view, data) {
	return E('div', { 'class': 'singbox-manager-toolbar' }, [
		E('button', {
			'class': 'btn cbi-button',
			'disabled': data.running ? null : 'disabled',
			'click': ui.createHandlerFn(view, function() {
				return callRestart().then(function(result) {
					showResult(result, _('sing-box restarted'), _('Restart failed'));
				}).then(L.bind(view.load, view));
			})
		}, _('Restart')),
		E('button', {
			'class': 'btn cbi-button',
			'click': ui.createHandlerFn(view, function() {
				return callReload().then(function(result) {
					showResult(result, _('sing-box reloaded'), _('Reload failed'));
				}).then(L.bind(view.load, view));
			})
		}, _('Reload')),
		E('button', {
			'class': 'btn cbi-button',
			'click': ui.createHandlerFn(view, function() {
				return callValidate().then(function(result) {
					ui.addNotification(null, E('p', result.ok ? _('Configuration is valid') : (result.errors || [ _('Configuration has errors') ]).join('; ')));
				});
			})
		}, _('Validate')),
		E('button', {
			'class': 'btn cbi-button',
			'click': ui.createHandlerFn(view, function() {
				return callHealthCheck().then(function(result) {
					ui.addNotification(null, E('p', result.ok ? _('Health check complete') : (result.errors || [ _('Health check failed') ]).join('; ')));
				}).then(L.bind(view.load, view));
			})
		}, _('Check Health'))
	]);
}

function renderLive(view, data) {
	data = data || {};
	var hist = pushHistory(view, data);
	return E('div', { 'class': 'singbox-manager-live' }, [
		renderHero(view, data),
		renderThroughput(view, data, hist),
		E('div', { 'class': 'singbox-manager-metrics' }, [
			metric(_('Daemon'), data.daemon ? _('Online') : _('Offline')),
			metric(_('Connections'), data.connections || 0),
			metric(_('Memory'), formatBytes((data.memory_kb || 0) * 1024)),
			metric(_('Strategy'), data.strategy)
		]),
		renderToolbar(view, data)
	]);
}

function downloadText(text) {
	var blob = new Blob([ text || '' ], { type: 'text/plain' });
	var url = URL.createObjectURL(blob);
	var link = E('a', { 'href': url, 'download': 'singbox-manager.log' });
	document.body.appendChild(link);
	link.click();
	link.remove();
	window.setTimeout(function() { URL.revokeObjectURL(url); }, 1000);
}

function renderLogs(view, data) {
	data = data || {};
	var text = data.text || '';
	return E('div', { 'class': 'singbox-manager-logs' }, [
		E('div', { 'class': 'singbox-manager-section-header' }, [
			E('h3', {}, _('Logs')),
			E('div', { 'class': 'singbox-manager-toolbar' }, [
				E('button', {
					'class': 'btn cbi-button',
					'click': ui.createHandlerFn(view, function() {
						return callLogs(300).then(function(result) {
							view.logsData = result || {};
							var replacement = renderLogs(view, view.logsData);
							var current = document.querySelector('.singbox-manager-logs');
							if (current)
								current.parentNode.replaceChild(replacement, current);
						});
					})
				}, _('Refresh')),
				E('button', {
					'class': 'btn cbi-button',
					'click': function() { downloadText(text); }
				}, _('Download'))
			])
		]),
		E('pre', { 'class': 'singbox-manager-log' }, valueOrDash(text))
	]);
}

return view.extend({
	load: function() {
		return Promise.all([ callStatus(), callLogs(300) ]).then(function(results) {
			return { status: results[0], logs: results[1] };
		});
	},

	render: function(data) {
		var view = this;
		data = data || {};
		view.logsData = data.logs || {};
		theme.inject();

		var live = renderLive(view, data.status || {});
		var logs = renderLogs(view, view.logsData);

		var root = E('div', { 'class': 'singbox-manager-dashboard' }, [
			live,
			logs
		]);

		poll.add(L.bind(function() {
			return callStatus().then(L.bind(function(status) {
				var replacement = renderLive(this, status || {});
				var current = root.querySelector('.singbox-manager-live');
				if (current)
					current.parentNode.replaceChild(replacement, current);
			}, this));
		}, this), POLL_INTERVAL);

		poll.add(L.bind(function() {
			return callLogs(300).then(L.bind(function(result) {
				this.logsData = result || {};
				var replacement = renderLogs(this, this.logsData);
				var current = root.querySelector('.singbox-manager-logs');
				if (current)
					current.parentNode.replaceChild(replacement, current);
			}, this));
		}, this), POLL_INTERVAL);

		return root;
	}
});
