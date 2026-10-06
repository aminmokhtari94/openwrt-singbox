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

var callCleanup = rpc.declare({
	object: 'singbox.manager',
	method: 'cleanup',
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
var LOG_LINES = 300;
var HISTORY = 48;

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

function reportResult(result, successText, failureText) {
	if (result && result.ok)
		theme.notify(result.message || successText);
	else
		theme.error(result, failureText);
}

function startRuntime(data) {
	var enabled = data.manager_enabled ? Promise.resolve({ ok: true }) : callSetManagerEnabled(true);
	return enabled.then(function(result) {
		if (!result.ok)
			return result;
		return callStart();
	});
}

// runAction calls an RPC, reports its outcome and re-renders the live panel
// right away instead of leaving stale state up until the next poll tick.
function runAction(view, call, successText, failureText) {
	return call().then(function(result) {
		reportResult(result, successText, failureText);
	}).then(function() {
		return refreshLive(view, true);
	});
}

function pushHistory(view, data) {
	view.hist = view.hist || [];
	view.hist.push({
		t: Date.now(),
		rx: Number(data.rx_bytes || 0),
		tx: Number(data.tx_bytes || 0)
	});
	if (view.hist.length > HISTORY)
		view.hist.shift();
	return view.hist;
}

// rateSeries turns cumulative byte counters into bytes/second using the real
// time between samples (polls drift, and action-triggered refreshes add extra
// samples). A counter that went backwards means sing-box restarted: count 0.
function rateSeries(hist, key) {
	var out = [];
	for (var i = 1; i < hist.length; i++) {
		var dt = (hist[i].t - hist[i - 1].t) / 1000;
		var delta = hist[i][key] - hist[i - 1][key];
		out.push(dt > 0 && delta > 0 ? delta / dt : 0);
	}
	return out;
}

// sparkline builds an SVG line+area chart via innerHTML so it renders in the
// SVG namespace (LuCI's E() only creates HTML elements). Inputs are numeric
// rates, so string interpolation here is safe.
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
		E('div', { 'class': 'singbox-manager-metric-value' + (accent ? ' ' + accent : '') }, theme.text(value))
	]);
}

function renderHero(view, data) {
	var running = !!data.running;
	var paused = !running && !!data.manager_paused;
	var primary = running
		? E('button', {
			'class': 'btn cbi-button cbi-button-remove',
			'title': _('Stop the proxy and hold it down (survives reboot) until started again'),
			'click': ui.createHandlerFn(view, function() {
				return runAction(view, callStop, _('sing-box stopped'), _('Stop failed'));
			})
		}, _('Stop'))
		: E('button', {
			'class': 'btn cbi-button cbi-button-apply',
			'disabled': data.daemon ? null : 'disabled',
			'click': ui.createHandlerFn(view, function() {
				return runAction(view, function() { return startRuntime(data); }, _('sing-box started'), _('Start failed'));
			})
		}, _('Start'));

	var subtitle = running
		? _('PID %s').format(valueOrDash(data.sing_box_pid))
		: (!data.daemon ? _('Manager daemon is not running')
			: (paused ? _('Management mode — proxy held down, LAN untouched')
				: (data.manager_enabled ? _('Manager enabled') : _('Manager disabled'))));

	var modeSelect = E('select', {
		'class': 'cbi-input-select singbox-manager-mode',
		'change': ui.createHandlerFn(view, function(ev) {
			var mode = ev.target.value;
			return runAction(view, function() { return callSetMode(mode); },
				_('Mode set to %s').format(mode), _('Set mode failed'));
		})
	}, MODE_OPTIONS.map(function(opt) {
		return E('option', { 'value': opt, 'selected': opt === data.runtime_mode ? 'selected' : null }, opt);
	}));

	var health = valueOrDash(data.health) + (data.latency_ms ? ' · ' + data.latency_ms + ' ms' : '');

	return E('div', { 'class': 'singbox-manager-hero' }, [
		E('div', { 'class': 'singbox-manager-hero-status' }, [
			E('span', { 'class': 'singbox-manager-dot' + (running ? ' on' : (data.daemon ? ' idle' : ' off')) }),
			E('div', {}, [
				E('div', { 'class': 'singbox-manager-hero-state' }, running ? _('Running') : (paused ? _('Paused') : _('Stopped'))),
				E('div', { 'class': 'singbox-manager-hero-sub' }, [ subtitle ])
			])
		]),
		E('div', { 'class': 'singbox-manager-hero-facts' }, [
			E('div', {}, [ E('span', {}, _('Group')), E('strong', {}, theme.text(data.active_group)) ]),
			E('div', {}, [ E('span', {}, _('Mode')), modeSelect ]),
			E('div', {}, [ E('span', {}, _('Outbound')), E('strong', {}, theme.text(data.selected_outbound)) ]),
			E('div', {}, [ E('span', {}, _('Health')), E('strong', { 'class': healthClass(data.health) }, [ health ]) ])
		]),
		E('div', { 'class': 'singbox-manager-hero-action' }, primary)
	]);
}

function renderChart(title, series, color, total) {
	return E('div', { 'class': 'singbox-manager-chart' }, [
		E('div', { 'class': 'singbox-manager-chart-head' }, [
			E('span', { 'class': 'singbox-manager-chart-title' }, title),
			E('span', { 'class': 'singbox-manager-chart-rate' }, [ formatRate(lastValue(series)) ])
		]),
		sparkline(series, color),
		E('div', { 'class': 'singbox-manager-chart-foot' }, [ _('Total %s').format(formatBytes(total)) ])
	]);
}

function renderThroughput(data, hist) {
	return E('div', { 'class': 'singbox-manager-charts' }, [
		renderChart(_('Download'), rateSeries(hist, 'rx'), '#2271b1', data.rx_bytes),
		renderChart(_('Upload'), rateSeries(hist, 'tx'), '#0f7a39', data.tx_bytes)
	]);
}

function renderToolbar(view, data) {
	return E('div', { 'class': 'singbox-manager-toolbar' }, [
		E('button', {
			'class': 'btn cbi-button',
			'disabled': data.running ? null : 'disabled',
			'click': ui.createHandlerFn(view, function() {
				return runAction(view, callRestart, _('sing-box restarted'), _('Restart failed'));
			})
		}, _('Restart')),
		E('button', {
			'class': 'btn cbi-button',
			'disabled': data.running ? null : 'disabled',
			'title': _('Re-render the config and hot-reload sing-box'),
			'click': ui.createHandlerFn(view, function() {
				return runAction(view, callReload, _('sing-box reloaded'), _('Reload failed'));
			})
		}, _('Reload')),
		E('button', {
			'class': 'btn cbi-button',
			'click': ui.createHandlerFn(view, function() {
				return callValidate().then(function(result) {
					reportResult(result, _('Configuration is valid'), _('Configuration has errors'));
				});
			})
		}, _('Validate')),
		E('button', {
			'class': 'btn cbi-button',
			'click': ui.createHandlerFn(view, function() {
				return runAction(view, callHealthCheck, _('Health check complete'), _('Health check failed'));
			})
		}, _('Check Health'))
	]);
}

// renderStaleAlert warns when sing-box is down but its firewall rules and policy
// routing are still installed: proxied devices are then steered to a port
// nothing listens on and lose internet until the rules are removed.
function renderStaleAlert(view, data) {
	if (!data.stale_datapath)
		return '';
	return E('div', { 'class': 'singbox-manager-alert' }, [
		E('div', {}, [
			E('strong', {}, [ _('Proxied devices have no internet') ]),
			E('div', {}, [ _('sing-box is not running, but its transparent-proxy firewall rules and routing are still active. Start sing-box, or remove the rules to restore direct internet.') ])
		]),
		E('button', {
			'class': 'btn cbi-button cbi-button-negative',
			'click': ui.createHandlerFn(view, function() {
				return runAction(view, callCleanup, _('Proxy rules removed'), _('Removing proxy rules failed'));
			})
		}, [ _('Remove proxy rules') ])
	]);
}

function renderLive(view, data) {
	data = data || {};
	var hist = pushHistory(view, data);
	return E('div', { 'class': 'singbox-manager-live' }, [
		renderStaleAlert(view, data),
		renderHero(view, data),
		renderThroughput(data, hist),
		E('div', { 'class': 'singbox-manager-metrics' }, [
			metric(_('Daemon'), data.daemon ? _('Online') : _('Offline'), data.daemon ? 'ok' : 'error'),
			metric(_('Connections'), String(data.connections || 0)),
			metric(_('Memory'), formatBytes((data.memory_kb || 0) * 1024)),
			metric(_('CPU'), data.cpu_percent ? data.cpu_percent + ' %' : '-'),
			metric(_('Strategy'), data.strategy)
		]),
		renderToolbar(view, data)
	]);
}

// refreshLive fetches status and swaps the live panel. A poll tick is skipped
// while the user is interacting with a control inside the panel (e.g. the mode
// dropdown is open), otherwise the rebuild would close it mid-selection.
function refreshLive(view, force) {
	return callStatus().then(function(status) {
		var current = view.root && view.root.querySelector('.singbox-manager-live');
		if (!current)
			return;
		var active = document.activeElement;
		if (!force && active && active.tagName === 'SELECT' && current.contains(active))
			return;
		current.parentNode.replaceChild(renderLive(view, status || {}), current);
	});
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

// updateLogs replaces the log text in place. When the reader is parked at the
// bottom it follows new lines; when they have scrolled up to read something,
// their position is kept instead of jumping on every poll.
function updateLogs(view, data) {
	var pre = view.logEl;
	if (!pre)
		return;
	var text = theme.stripAnsi((data && data.text) || '');
	view.logText = text;
	var atBottom = pre.scrollHeight - pre.scrollTop - pre.clientHeight < 24;
	var top = pre.scrollTop;
	pre.textContent = text || _('No log output yet');
	pre.scrollTop = atBottom ? pre.scrollHeight : top;
}

function refreshLogs(view) {
	return callLogs(LOG_LINES).then(function(result) {
		updateLogs(view, result || {});
	});
}

function renderLogs(view, data) {
	view.logEl = E('pre', { 'class': 'singbox-manager-log' });
	var box = E('div', { 'class': 'singbox-manager-logs' }, [
		E('div', { 'class': 'singbox-manager-section-header' }, [
			E('h3', {}, _('Logs')),
			E('div', { 'class': 'singbox-manager-toolbar' }, [
				E('button', {
					'class': 'btn cbi-button',
					'click': ui.createHandlerFn(view, function() { return refreshLogs(view); })
				}, _('Refresh')),
				E('button', {
					'class': 'btn cbi-button',
					'click': function() { downloadText(view.logText); }
				}, _('Download'))
			])
		]),
		view.logEl
	]);
	updateLogs(view, data);
	// Start at the newest line once the element is laid out.
	window.requestAnimationFrame(function() {
		view.logEl.scrollTop = view.logEl.scrollHeight;
	});
	return box;
}

return view.extend({
	handleSaveApply: null,
	handleSave: null,
	handleReset: null,

	load: function() {
		return Promise.all([ callStatus(), callLogs(LOG_LINES) ]).then(function(results) {
			return { status: results[0], logs: results[1] };
		});
	},

	render: function(data) {
		var view = this;
		data = data || {};
		theme.inject();

		view.root = E('div', { 'class': 'singbox-manager-dashboard' }, [
			renderLive(view, data.status || {}),
			renderLogs(view, data.logs || {})
		]);

		poll.add(function() { return refreshLive(view, false); }, POLL_INTERVAL);
		poll.add(function() { return refreshLogs(view); }, POLL_INTERVAL);

		return view.root;
	}
});
