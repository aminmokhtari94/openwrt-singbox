'use strict';
'require view';
'require form';
'require rpc';

var callTUN = rpc.declare({
	object: 'singbox.manager',
	method: 'tun',
	expect: { '': {} }
});

function valueOrDash(value) {
	if (value === null || value === undefined || value === '')
		return '-';
	return value;
}

function renderStatus(data) {
	data = data || {};
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('h3', {}, _('TUN Status')),
		E('div', { 'class': 'singbox-manager-grid' }, [
			E('div', { 'class': 'singbox-manager-label' }, _('Enabled')),
			E('div', {}, data.enabled ? _('Yes') : _('No')),
			E('div', { 'class': 'singbox-manager-label' }, _('Interface')),
			E('div', {}, valueOrDash(data.interface)),
			E('div', { 'class': 'singbox-manager-label' }, _('Auto Route')),
			E('div', {}, data.auto_route ? _('Yes') : _('No')),
			E('div', { 'class': 'singbox-manager-label' }, _('Auto Redirect')),
			E('div', {}, data.auto_redirect ? _('Yes') : _('No')),
			E('div', { 'class': 'singbox-manager-label' }, _('IPv4 Address')),
			E('div', {}, valueOrDash(data.inet4_address)),
			E('div', { 'class': 'singbox-manager-label' }, _('IPv6 Address')),
			E('div', {}, valueOrDash(data.inet6_address))
		])
	]);
}

function renderForm() {
	var m = new form.Map('singbox-manager');
	var s = m.section(form.NamedSection, 'tun', 'tun', _('TUN'));
	s.anonymous = true;

	var o = s.option(form.Flag, 'enabled', _('Enable'));
	o.default = '0';
	o.rmempty = false;

	o = s.option(form.Flag, 'auto_route', _('Auto Route'));
	o.default = '1';
	o.rmempty = false;

	o = s.option(form.Flag, 'auto_redirect', _('Auto Redirect'));
	o.default = '1';
	o.rmempty = false;

	o = s.option(form.Value, 'inet4_address', _('IPv4 Address'));
	o.datatype = 'cidr4';
	o.placeholder = '172.19.0.1/30';
	o.rmempty = true;

	o = s.option(form.Value, 'inet6_address', _('IPv6 Address'));
	o.datatype = 'cidr6';
	o.placeholder = 'fdfe:dcba:9876::1/126';
	o.rmempty = true;

	return m.render();
}

return view.extend({
	load: function() {
		return Promise.all([
			callTUN(),
			renderForm()
		]);
	},

	render: function(results) {
		var data = results[0] || {};
		var formNode = results[1];
		return E('div', { 'class': 'singbox-manager-page' }, [
			E('style', {}, [
				'.singbox-manager-page{display:grid;gap:16px}',
				'.singbox-manager-section{display:grid;gap:10px}',
				'.singbox-manager-section h3{margin:0;font-size:16px}',
				'.singbox-manager-grid{display:grid;grid-template-columns:minmax(130px,190px) minmax(0,1fr);gap:10px 14px;align-items:start}',
				'.singbox-manager-label{font-size:12px;color:var(--text-color-medium);font-weight:600}',
				'@media(max-width:700px){.singbox-manager-grid{grid-template-columns:1fr}}'
			].join('')),
			renderStatus(data),
			formNode
		]);
	}
});
