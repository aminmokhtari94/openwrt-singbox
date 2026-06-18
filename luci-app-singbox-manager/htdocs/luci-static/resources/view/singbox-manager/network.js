'use strict';
'require view';
'require rpc';
'require ui';
'require view.singbox-manager.theme as theme';

var callTProxy = rpc.declare({
	object: 'singbox.manager',
	method: 'tproxy',
	expect: { '': {} }
});

var callTUN = rpc.declare({
	object: 'singbox.manager',
	method: 'tun',
	expect: { '': {} }
});

var callSetTProxy = rpc.declare({
	object: 'singbox.manager',
	method: 'tproxy_set',
	params: [ 'tproxy' ],
	expect: { '': {} }
});

var callSetTUN = rpc.declare({
	object: 'singbox.manager',
	method: 'tun_set',
	params: [ 'tun' ],
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

function checkValue(root, name) {
	var input = field(root, name);
	return input ? input.checked : false;
}

function splitList(value) {
	return (value || '').split(/[\s,]+/).map(function(item) {
		return item.trim();
	}).filter(function(item) {
		return item !== '';
	});
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
	if (result && result.ok)
		return true;
	ui.addNotification(null, E('p', (result && result.errors || [ fallback ]).join('; ')));
	return false;
}

// Every control applies the whole section on change (no staged Save & Apply),
// then the daemon reloads sing-box so the change takes effect immediately.
function readTProxyForm(root) {
	return {
		enabled: checkValue(root, 'tp_enabled'),
		lan_ifnames: splitList(formValue(root, 'tp_lan')),
		include_subnet: splitList(formValue(root, 'tp_include')),
		exclude_subnet: splitList(formValue(root, 'tp_exclude')),
		include_mac: splitList(formValue(root, 'tp_mac')),
		dns_hijack: checkValue(root, 'tp_dns'),
		kill_switch: checkValue(root, 'tp_kill')
	};
}

function readTUNForm(root) {
	return {
		enabled: checkValue(root, 'tun_enabled'),
		auto_route: checkValue(root, 'tun_route'),
		auto_redirect: checkValue(root, 'tun_redirect'),
		inet4_address: formValue(root, 'tun_inet4'),
		inet6_address: formValue(root, 'tun_inet6')
	};
}

function applyTProxy(view) {
	var root = document.querySelector('.singbox-manager-page');
	return callSetTProxy(readTProxyForm(root)).then(function(result) {
		if (showResult(result, _('Save failed')))
			ui.addNotification(null, E('p', _('Transparent proxy settings applied')));
		return refreshView(view);
	});
}

function applyTUN(view) {
	var root = document.querySelector('.singbox-manager-page');
	return callSetTUN(readTUNForm(root)).then(function(result) {
		if (showResult(result, _('Save failed')))
			ui.addNotification(null, E('p', _('TUN settings applied')));
		return refreshView(view);
	});
}

function toggle(view, name, checked, apply) {
	return E('label', { 'class': 'singbox-manager-toggle' }, [
		E('input', {
			'type': 'checkbox',
			'name': name,
			'checked': checked ? 'checked' : null,
			'change': ui.createHandlerFn(view, apply)
		}),
		E('span', {})
	]);
}

function listField(view, name, value, placeholder, apply) {
	return E('input', {
		'class': 'cbi-input-text',
		'name': name,
		'value': (value || []).join(', '),
		'placeholder': placeholder,
		'change': ui.createHandlerFn(view, apply)
	});
}

function textField(view, name, value, placeholder, apply) {
	return E('input', {
		'class': 'cbi-input-text',
		'name': name,
		'value': value || '',
		'placeholder': placeholder,
		'change': ui.createHandlerFn(view, apply)
	});
}

function row(label, hint, control) {
	return E('div', { 'class': 'singbox-manager-row' }, [
		E('div', { 'class': 'singbox-manager-row-text' }, [
			E('div', { 'class': 'singbox-manager-row-label' }, label),
			hint ? E('div', { 'class': 'singbox-manager-row-hint' }, hint) : ''
		]),
		E('div', { 'class': 'singbox-manager-row-control' }, control)
	]);
}

function renderTProxy(view, data) {
	data = data || {};
	var apply = function() { return applyTProxy(view); };
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('div', { 'class': 'singbox-manager-section-header' }, [
			E('h3', {}, _('Transparent Proxy')),
			E('span', { 'class': 'singbox-manager-badge' + (data.enabled ? ' on' : '') }, data.enabled ? _('On') : _('Off'))
		]),
		E('p', { 'class': 'singbox-manager-hint' }, _('Routes LAN traffic through sing-box without configuring a proxy on each client.')),
		E('div', { 'class': 'singbox-manager-rows' }, [
			row(_('Enable'), _('Intercept and route forwarded LAN traffic.'), toggle(view, 'tp_enabled', data.enabled, apply)),
			row(_('LAN interfaces'), _('Interfaces whose traffic is intercepted.'), listField(view, 'tp_lan', data.lan_ifnames, 'br-lan', apply)),
			row(_('Include subnets'), _('Only these source subnets are routed (empty = all).'), listField(view, 'tp_include', data.include_subnet, '192.168.1.0/24', apply)),
			row(_('Exclude subnets'), _('Source subnets that bypass the proxy.'), listField(view, 'tp_exclude', data.exclude_subnet, '192.168.0.0/16', apply)),
			row(_('Device MAC filters'), _('Only these MACs are routed (empty = all devices).'), listField(view, 'tp_mac', data.include_mac, '00:11:22:33:44:55', apply)),
			row(_('DNS capture'), _('Redirect LAN DNS queries into sing-box.'), toggle(view, 'tp_dns', data.dns_hijack, apply)),
			row(_('Kill switch'), _('Block forwarded LAN traffic when routing is down.'), toggle(view, 'tp_kill', data.kill_switch, apply))
		]),
		E('div', { 'class': 'singbox-manager-meta' }, [
			E('span', {}, _('TProxy port: %s').format(valueOrDash(data.tproxy_port))),
			E('span', {}, _('DNS port: %s').format(valueOrDash(data.dns_port))),
			E('span', {}, _('nftables include: %s').format(data.nftables_present ? _('present') : _('absent')))
		])
	]);
}

function renderTUN(view, data) {
	data = data || {};
	var apply = function() { return applyTUN(view); };
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('div', { 'class': 'singbox-manager-section-header' }, [
			E('h3', {}, _('TUN')),
			E('span', { 'class': 'singbox-manager-badge' + (data.enabled ? ' on' : '') }, data.enabled ? _('On') : _('Off'))
		]),
		E('p', { 'class': 'singbox-manager-hint' }, _('System-wide routing via a virtual interface. Cannot be enabled together with transparent proxy.')),
		E('div', { 'class': 'singbox-manager-rows' }, [
			row(_('Enable'), _('Create the %s interface and route through it.').format(valueOrDash(data.interface || 'singbox0')), toggle(view, 'tun_enabled', data.enabled, apply)),
			row(_('Auto route'), _('Install routes for traffic handled by TUN.'), toggle(view, 'tun_route', data.auto_route, apply)),
			row(_('Auto redirect'), _('Redirect traffic into TUN automatically where supported.'), toggle(view, 'tun_redirect', data.auto_redirect, apply)),
			row(_('IPv4 address'), '', textField(view, 'tun_inet4', data.inet4_address, '172.19.0.1/30', apply)),
			row(_('IPv6 address'), '', textField(view, 'tun_inet6', data.inet6_address, 'fdfe:dcba:9876::1/126', apply))
		])
	]);
}

function renderPreview(data) {
	data = data || {};
	if (!data.enabled)
		return '';
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('h3', {}, _('nftables Preview')),
		E('pre', { 'class': 'singbox-manager-preview' }, valueOrDash(data.nftables_preview))
	]);
}

return view.extend({
	load: function() {
		return Promise.all([ callTProxy(), callTUN() ]);
	},

	render: function(results) {
		var view = this;
		var tproxyData = (results && results[0]) || {};
		var tunData = (results && results[1]) || {};
		theme.inject();
		return E('div', { 'class': 'singbox-manager-page narrow' }, [
			renderTProxy(view, tproxyData),
			renderTUN(view, tunData),
			renderPreview(tproxyData)
		]);
	}
});
