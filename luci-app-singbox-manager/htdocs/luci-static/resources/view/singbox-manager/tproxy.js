'use strict';
'require view';
'require form';
'require rpc';

var callTProxy = rpc.declare({
	object: 'singbox.manager',
	method: 'tproxy',
	expect: { '': {} }
});

function valueOrDash(value) {
	if (value === null || value === undefined || value === '')
		return '-';
	return value;
}

function renderList(values, emptyText) {
	values = values || [];
	return values.length ? E('ul', { 'class': 'singbox-manager-list' }, values.map(function(value) {
		return E('li', {}, value);
	})) : E('span', {}, emptyText || '-');
}

function renderStatus(data) {
	data = data || {};
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('h3', {}, _('Transparent Proxy Status')),
		E('div', { 'class': 'singbox-manager-grid' }, [
			E('div', { 'class': 'singbox-manager-label' }, _('Enabled')),
			E('div', {}, data.enabled ? _('Yes') : _('No')),
			E('div', { 'class': 'singbox-manager-label' }, _('LAN Interfaces')),
			E('div', {}, renderList(data.lan_ifnames, _('No LAN interfaces'))),
			E('div', { 'class': 'singbox-manager-label' }, _('TProxy Port')),
			E('div', {}, valueOrDash(data.tproxy_port)),
			E('div', { 'class': 'singbox-manager-label' }, _('DNS Hijacking')),
			E('div', {}, data.dns_hijack ? _('Yes') : _('No')),
			E('div', { 'class': 'singbox-manager-label' }, _('DNS Port')),
			E('div', {}, valueOrDash(data.dns_port)),
			E('div', { 'class': 'singbox-manager-label' }, _('Include Subnets')),
			E('div', {}, renderList(data.include_subnet, _('All routed destinations'))),
			E('div', { 'class': 'singbox-manager-label' }, _('Exclude Subnets')),
			E('div', {}, renderList(data.exclude_subnet, _('No excluded subnets'))),
			E('div', { 'class': 'singbox-manager-label' }, _('MAC Filters')),
			E('div', {}, renderList(data.include_mac, _('All LAN devices'))),
			E('div', { 'class': 'singbox-manager-label' }, _('nftables Include')),
			E('div', {}, valueOrDash(data.nftables_include)),
			E('div', { 'class': 'singbox-manager-label' }, _('Include Present')),
			E('div', {}, data.nftables_present ? _('Yes') : _('No'))
		])
	]);
}

function renderPreview(data) {
	return E('div', { 'class': 'singbox-manager-section' }, [
		E('h3', {}, _('nftables Preview')),
		E('pre', { 'class': 'singbox-manager-preview' }, valueOrDash(data && data.nftables_preview))
	]);
}

function renderForm() {
	var m = new form.Map('singbox-manager');
	var s = m.section(form.NamedSection, 'tproxy', 'tproxy', _('Transparent Proxy'));
	s.anonymous = true;

	var o = s.option(form.Flag, 'enabled', _('Enable'));
	o.default = '0';
	o.rmempty = false;

	o = s.option(form.DynamicList, 'lan_ifname', _('LAN Interfaces'));
	o.placeholder = 'br-lan';
	o.rmempty = false;

	o = s.option(form.DynamicList, 'include_subnet', _('Include Subnets'));
	o.datatype = 'cidr';
	o.placeholder = '192.168.1.0/24';
	o.rmempty = true;

	o = s.option(form.DynamicList, 'exclude_subnet', _('Exclude Subnets'));
	o.datatype = 'cidr';
	o.placeholder = '192.168.0.0/16';
	o.rmempty = true;

	o = s.option(form.DynamicList, 'include_mac', _('Device MAC Filters'));
	o.datatype = 'macaddr';
	o.placeholder = '00:11:22:33:44:55';
	o.rmempty = true;

	o = s.option(form.Flag, 'dns_hijack', _('DNS Hijacking'));
	o.default = '0';
	o.rmempty = false;

	return m.render();
}

return view.extend({
	load: function() {
		return Promise.all([
			callTProxy(),
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
				'.singbox-manager-list{margin:0;padding-left:18px}',
				'.singbox-manager-preview{box-sizing:border-box;max-width:100%;min-height:180px;overflow:auto;padding:12px;border:1px solid var(--border-color-medium);background:var(--background-color-high);font:12px/1.5 monospace;white-space:pre}',
				'@media(max-width:700px){.singbox-manager-grid{grid-template-columns:1fr}.singbox-manager-preview{white-space:pre-wrap;word-break:break-word}}'
			].join('')),
			renderStatus(data),
			formNode,
			renderPreview(data)
		]);
	}
});
