#!/bin/sh
set -eu

lan_device="${1:-eth2}"
lan_ip="${2:-192.168.200.1}"
lan_netmask="${3:-255.255.255.0}"
wan_device="${4:-eth1}"
wan_proto="${5:-dhcp}"
wan_ip="${6:-10.0.200.2}"
wan_netmask="${7:-255.255.255.0}"
wan_gateway="${8:-10.0.200.1}"

delete_zone_by_name() {
	local name="$1"
	local section

	while :; do
		section="$(uci -q show firewall | sed -n "s/^firewall\.\([^.]*\)\.name='\{0,1\}${name}'\{0,1\}$/\1/p" | head -n 1)"
		[ -n "$section" ] || break
		uci -q delete "firewall.$section" || break
	done
}

delete_forwardings() {
	local section

	while :; do
		section="$(uci -q show firewall | sed -n 's/^firewall\.\([^.]*\)=forwarding$/\1/p' | head -n 1)"
		[ -n "$section" ] || break
		uci -q delete "firewall.$section" || break
	done
}

uci -q set network.lan.device="$lan_device"
uci -q set network.lan.ipaddr="$lan_ip"
uci -q set network.lan.netmask="$lan_netmask"

uci -q delete network.wan || true
uci -q set network.wan='interface'
uci -q set network.wan.device="$wan_device"
if [ "$wan_proto" = "static" ]; then
	uci -q set network.wan.proto='static'
	uci -q set network.wan.ipaddr="$wan_ip"
	uci -q set network.wan.netmask="$wan_netmask"
	uci -q set network.wan.gateway="$wan_gateway"
	uci -q delete network.wan.dns || true
	uci -q add_list network.wan.dns='1.1.1.1'
	uci -q add_list network.wan.dns='8.8.8.8'
	uci -q delete network.wan6 || true
else
	uci -q set network.wan.proto='dhcp'
	uci -q delete network.wan6 || true
	uci -q set network.wan6='interface'
	uci -q set network.wan6.device="$wan_device"
	uci -q set network.wan6.proto='dhcpv6'
	uci -q set network.wan6.reqaddress='try'
	uci -q set network.wan6.reqprefix='auto'
fi

delete_zone_by_name lan
delete_zone_by_name wan
delete_forwardings

uci -q set firewall.lab_lan='zone'
uci -q set firewall.lab_lan.name='lan'
uci -q delete firewall.lab_lan.network || true
uci -q add_list firewall.lab_lan.network='lan'
uci -q set firewall.lab_lan.input='ACCEPT'
uci -q set firewall.lab_lan.output='ACCEPT'
uci -q set firewall.lab_lan.forward='ACCEPT'

uci -q set firewall.lab_wan='zone'
uci -q set firewall.lab_wan.name='wan'
uci -q delete firewall.lab_wan.network || true
uci -q add_list firewall.lab_wan.network='wan'
uci -q add_list firewall.lab_wan.network='wan6'
uci -q set firewall.lab_wan.input='REJECT'
uci -q set firewall.lab_wan.output='ACCEPT'
uci -q set firewall.lab_wan.forward='REJECT'
uci -q set firewall.lab_wan.masq='1'
uci -q set firewall.lab_wan.mtu_fix='1'

uci -q set firewall.lab_lan_wan='forwarding'
uci -q set firewall.lab_lan_wan.src='lan'
uci -q set firewall.lab_lan_wan.dest='wan'

uci -q commit network
uci -q commit firewall

sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || true
