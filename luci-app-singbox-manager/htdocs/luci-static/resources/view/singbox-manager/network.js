'use strict';
'require view';
'require rpc';
'require ui';
'require view.singbox-manager.theme as theme';

var callTransparent = rpc.declare({
	object: 'singbox.manager',
	method: 'transparent',
	expect: { '': {} }
});

var callTUN = rpc.declare({
	object: 'singbox.manager',
	method: 'tun',
	expect: { '': {} }
});

var callSetTransparent = rpc.declare({
	object: 'singbox.manager',
	method: 'transparent_set',
	params: [ 'transparent' ],
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
function readTransparentForm(root) {
	return {
		default_mode: formValue(root, 'tr_mode') || 'off',
		lan_ifnames: splitList(formValue(root, 'tr_lan')),
		bypass_subnet: splitList(formValue(root, 'tr_bypass')),
		dns_hijack: checkValue(root, 'tr_dns'),
		kill_switch: checkValue(root, 'tr_kill'),
		devices: readDevices(root)
	};
}

function rowFieldValue(rowEl, cls) {
	var el = rowEl.querySelector('.' + cls);
	return el ? el.value.trim() : '';
}

function readDevices(root) {
	var devices = [];
	var rows = root.querySelectorAll('.singbox-manager-device-row');
	for (var i = 0; i < rows.length; i++) {
		var rowEl = rows[i];
		var name = rowFieldValue(rowEl, 'dev-name');
		var mac = rowFieldValue(rowEl, 'dev-mac');
		var ipv4 = rowFieldValue(rowEl, 'dev-ipv4');
		var ipv6 = rowFieldValue(rowEl, 'dev-ipv6');
		// Skip fully-empty placeholder rows so they are not persisted.
		if (!name && !mac && !ipv4 && !ipv6)
			continue;
		var enabledEl = rowEl.querySelector('.dev-enabled');
		var udpEl = rowEl.querySelector('.dev-udp');
		devices.push({
			id: rowEl.getAttribute('data-id') || '',
			enabled: enabledEl ? enabledEl.checked : false,
			name: name,
			mac: mac,
			ipv4: ipv4,
			ipv6: ipv6,
			mode: rowFieldValue(rowEl, 'dev-mode') || 'default',
			bypass_udp: udpEl ? udpEl.checked : false,
			group: rowFieldValue(rowEl, 'dev-group')
		});
	}
	return devices;
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

function applyTransparent(view) {
	var root = document.querySelector('.singbox-manager-page');
	return callSetTransparent(readTransparentForm(root)).then(function(result) {
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

function selectField(view, name, value, options, apply) {
	return E('select', {
		'class': 'cbi-input-select',
		'name': name,
		'change': ui.createHandlerFn(view, apply)
	}, options.map(function(opt) {
		return E('option', {
			'value': opt.value,
			'selected': opt.value === value ? 'selected' : null
		}, opt.label);
	}));
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

var MODE_OPTIONS = [
	{ value: 'off', label: _('Off — only listed devices') },
	{ value: 'tproxy', label: _('tproxy — TCP + UDP') }
];

var DEVICE_MODE_OPTIONS = [
	{ value: 'default', label: _('Default') },
	{ value: 'tproxy', label: _('tproxy') },
	{ value: 'bypass', label: _('Bypass') }
];

function deviceRow(view, device) {
	device = device || {};
	var apply = function() { return applyTransparent(view); };
	var modeSelect = E('select', { 'class': 'cbi-input-select dev-mode', 'change': ui.createHandlerFn(view, apply) },
		DEVICE_MODE_OPTIONS.map(function(opt) {
			return E('option', {
				'value': opt.value,
				'selected': (device.mode || 'default') === opt.value ? 'selected' : null
			}, opt.label);
		}));
	var removeBtn = E('button', {
		'class': 'cbi-button cbi-button-remove',
		'click': ui.createHandlerFn(view, function(ev) {
			var rowEl = ev.target.closest('.singbox-manager-device-row');
			if (rowEl)
				rowEl.parentNode.removeChild(rowEl);
			return applyTransparent(view);
		})
	}, _('Remove'));
	return E('div', { 'class': 'singbox-manager-device-row', 'data-id': device.id || '' }, [
		E('input', { 'class': 'cbi-input-text dev-name', 'value': device.name || '', 'placeholder': _('Name'), 'change': ui.createHandlerFn(view, apply) }),
		E('input', { 'class': 'cbi-input-text dev-mac', 'value': device.mac || '', 'placeholder': 'aa:bb:cc:dd:ee:ff', 'change': ui.createHandlerFn(view, apply) }),
		E('input', { 'class': 'cbi-input-text dev-ipv4', 'value': device.ipv4 || '', 'placeholder': '192.168.1.50', 'change': ui.createHandlerFn(view, apply) }),
		E('input', { 'class': 'cbi-input-text dev-ipv6', 'value': device.ipv6 || '', 'placeholder': _('IPv6 (optional)'), 'change': ui.createHandlerFn(view, apply) }),
		modeSelect,
		E('input', { 'class': 'cbi-input-text dev-group', 'value': device.group || '', 'placeholder': _('Group (optional)'), 'change': ui.createHandlerFn(view, apply) }),
		E('label', { 'class': 'singbox-manager-toggle', 'title': _('Send this device\'s UDP directly (TCP still proxied)') }, [
			E('input', { 'type': 'checkbox', 'class': 'dev-udp', 'checked': device.bypass_udp ? 'checked' : null, 'change': ui.createHandlerFn(view, apply) }),
			E('span', {})
		]),
		E('label', { 'class': 'singbox-manager-toggle' }, [
			E('input', { 'type': 'checkbox', 'class': 'dev-enabled', 'checked': device.enabled ? 'checked' : null, 'change': ui.createHandlerFn(view, apply) }),
			E('span', {})
		]),
		removeBtn
	]);
}

// deviceLabel formats a discovered LAN device as "hostname · ip · mac",
// dropping whichever parts are unknown.
function deviceLabel(device) {
	var parts = [];
	if (device.name)
		parts.push(device.name);
	if (device.ip)
		parts.push(device.ip);
	if (device.mac)
		parts.push(device.mac);
	return parts.join(' · ') || device.ip || device.mac || _('Unknown device');
}

// deviceFromLan maps a discovered device (single ip + mac + name) onto the
// per-device override fields, routing the address to ipv4 or ipv6.
function deviceFromLan(d) {
	var isV6 = (d.ip || '').indexOf(':') !== -1;
	return {
		name: d.name || '',
		mac: d.mac || '',
		ipv4: isV6 ? '' : (d.ip || ''),
		ipv6: isV6 ? (d.ip || '') : '',
		mode: 'default',
		enabled: true
	};
}

// deviceAlreadyListed reports whether an override row already matches the
// device by MAC or address, so a chip click cannot create duplicates.
function deviceAlreadyListed(container, dev) {
	var rows = container.querySelectorAll('.singbox-manager-device-row');
	for (var i = 0; i < rows.length; i++) {
		var mac = rowFieldValue(rows[i], 'dev-mac').toLowerCase();
		if (dev.mac && mac && mac === dev.mac.toLowerCase())
			return true;
		if (dev.ipv4 && rowFieldValue(rows[i], 'dev-ipv4') === dev.ipv4)
			return true;
		if (dev.ipv6 && rowFieldValue(rows[i], 'dev-ipv6') === dev.ipv6)
			return true;
	}
	return false;
}

// lanDevicePicker renders discovered LAN devices as click-to-add chips; a
// click appends a pre-filled override row (matched by MAC, robust to DHCP).
function lanDevicePicker(view, lanDevices) {
	lanDevices = (lanDevices || []).filter(function(d) { return d.ip || d.mac; });
	if (!lanDevices.length)
		return '';
	return E('div', { 'class': 'singbox-manager-devicepicker' }, [
		E('div', { 'class': 'singbox-manager-devicepicker-label' }, _('LAN devices — click to add an override')),
		E('div', { 'class': 'singbox-manager-chips' }, lanDevices.map(function(d) {
			return E('button', {
				'type': 'button',
				'class': 'btn cbi-button singbox-manager-chip',
				'click': function(ev) {
					ev.preventDefault();
					var container = document.querySelector('.singbox-manager-devices');
					if (!container)
						return;
					var dev = deviceFromLan(d);
					if (deviceAlreadyListed(container, dev))
						return;
					container.appendChild(deviceRow(view, dev));
				}
			}, deviceLabel(d));
		}))
	]);
}

function renderDeviceEditor(view, devices, lanDevices) {
	var header = E('div', { 'class': 'singbox-manager-device-row singbox-manager-device-head' }, [
		E('span', {}, _('Name')),
		E('span', {}, _('MAC')),
		E('span', {}, _('IPv4')),
		E('span', {}, _('IPv6')),
		E('span', {}, _('Mode')),
		E('span', {}, _('Group')),
		E('span', { 'title': _('Send this device\'s UDP directly (TCP still proxied)') }, _('UDP')),
		E('span', {}, _('On')),
		E('span', {}, '')
	]);
	var rows = (devices || []).map(function(d) { return deviceRow(view, d); });
	var addBtn = E('button', {
		'class': 'cbi-button cbi-button-add',
		'click': ui.createHandlerFn(view, function() {
			var container = document.querySelector('.singbox-manager-devices');
			if (container)
				container.appendChild(deviceRow(view, { mode: 'default' }));
		})
	}, _('Add device'));
	return E('div', { 'class': 'singbox-manager-device-editor' }, [
		E('h4', {}, _('Per-device overrides')),
		E('p', { 'class': 'singbox-manager-hint' }, _('Each device follows the default mode unless overridden. Match by MAC (robust) and/or IP. Enable UDP to send a device\'s UDP directly while its TCP stays proxied — useful for gaming consoles, where proxied UDP breaks NAT type and adds latency.')),
		E('div', { 'class': 'singbox-manager-devices' }, [ header ].concat(rows)),
		lanDevicePicker(view, lanDevices),
		addBtn
	]);
}

function renderTransparent(view, data) {
	data = data || {};
	var apply = function() { return applyTransparent(view); };
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('div', { 'class': 'singbox-manager-section-header' }, [
			E('h3', {}, _('Transparent Proxy')),
			E('span', { 'class': 'singbox-manager-badge' + (data.active ? ' on' : '') }, data.active ? _('On') : _('Off'))
		]),
		E('p', { 'class': 'singbox-manager-hint' }, _('Routes LAN traffic through sing-box without configuring a proxy on each client. The default mode applies to every LAN device; pin individual devices below.')),
		E('div', { 'class': 'singbox-manager-rows' }, [
			row(_('Default mode'), _('off = only listed devices are proxied; tproxy = TCP + UDP for everyone in scope.'), selectField(view, 'tr_mode', data.default_mode || 'off', MODE_OPTIONS, apply)),
			row(_('LAN interfaces'), _('Interfaces whose traffic is intercepted.'), listField(view, 'tr_lan', data.lan_ifnames, 'br-lan', apply)),
			row(_('Bypass subnets'), _('Destination subnets that always egress directly.'), listField(view, 'tr_bypass', data.bypass_subnet, '192.168.0.0/16', apply)),
			row(_('DNS capture'), _('Hijack LAN DNS queries into sing-box.'), toggle(view, 'tr_dns', data.dns_hijack, apply)),
			row(_('Kill switch'), _('Block forwarded LAN traffic when routing is down.'), toggle(view, 'tr_kill', data.kill_switch, apply))
		]),
		renderDeviceEditor(view, data.devices, data.lan_devices),
		E('div', { 'class': 'singbox-manager-meta' }, [
			E('span', {}, _('tproxy port: %s').format(valueOrDash(data.tproxy_port))),
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
	if (!data.active)
		return '';
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('h3', {}, _('nftables Preview')),
		E('pre', { 'class': 'singbox-manager-preview' }, valueOrDash(data.nftables_preview))
	]);
}

return view.extend({
	load: function() {
		return Promise.all([ callTransparent(), callTUN() ]);
	},

	render: function(results) {
		var view = this;
		var transparentData = (results && results[0]) || {};
		var tunData = (results && results[1]) || {};
		theme.inject();
		return E('div', { 'class': 'singbox-manager-page' }, [
			renderTransparent(view, transparentData),
			renderTUN(view, tunData),
			renderPreview(transparentData)
		]);
	}
});
