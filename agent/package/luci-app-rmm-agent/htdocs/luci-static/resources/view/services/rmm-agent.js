'use strict';
'require form';
'require rpc';
'require uci';
'require ui';
'require view';

var callServiceList = rpc.declare({
	object: 'service',
	method: 'list',
	params: [ 'name' ],
	expect: { '': {} }
});

var callRcInit = rpc.declare({
	object: 'rc',
	method: 'init',
	params: [ 'name', 'action' ]
});

function validateDiscoveryURL(sectionId, value) {
	if (!value || /^https:\/\/[A-Za-z0-9.-]+(?::[0-9]+)?(?:\/.*)?$/.test(value))
		return true;
	return _('Public IP discovery must use an HTTPS URL without embedded credentials.');
}

return view.extend({
	load: function() {
		return Promise.all([ uci.load('rmm-agent'), callServiceList('rmm-agent') ]);
	},

	handleServiceAction: function(action) {
		return callRcInit('rmm-agent', action).then(function(code) {
			if (code)
				throw new Error(_('Command failed with code %d').format(code));
			ui.addNotification(null, E('p', {}, _('RMM agent action completed.')), 'info');
		}).catch(function(error) {
			ui.addNotification(null, E('p', {}, _('Unable to control RMM agent: %s').format(error.message)));
		});
	},

	render: function(data) {
		var services = data[1] || {};
		var running = !!(services['rmm-agent'] && services['rmm-agent'].instances && Object.keys(services['rmm-agent'].instances).length);
		var map = new form.Map('rmm-agent', _('RMM agent'),
			_('Connect this router to your RMM account. Create a one-time enrollment grant in the RMM control panel, paste it below, save and restart the agent.'));
		var section = map.section(form.NamedSection, 'main', 'agent', _('Connection'));
		section.anonymous = true;
		section.addremove = false;

		var option = section.option(form.Flag, 'enabled', _('Enable agent'));
		option.rmempty = false;

		option = section.option(form.Value, 'server_url', _('RMM server URL'));
		option.placeholder = 'https://rmm.example.com';
		option.rmempty = false;
		option.validate = function(sectionId, value) {
			if (/^https:\/\/[A-Za-z0-9.-]+(?::[0-9]+)?(?:\/.*)?$/.test(value))
				return true;
			if (uci.get('rmm-agent', sectionId, 'allow_insecure_http') === '1' && /^http:\/\//.test(value))
				return true;
			return _('Use an HTTPS URL, or explicitly allow insecure HTTP in advanced settings.');
		};

		option = section.option(form.Value, 'enrollment_token', _('One-time enrollment grant'));
		option.password = true;
		option.rmempty = true;
		option.description = _('The grant is consumed once and removed from the router after successful enrollment.');

		option = section.option(form.Value, 'interval_seconds', _('Polling interval'));
		option.datatype = 'range(10,3600)';
		option.default = '30';
		option.rmempty = false;

		option = section.option(form.Value, 'check_targets', _('Connectivity check targets'));
		option.placeholder = '1.1.1.1 8.8.8.8';
		option.rmempty = false;

		section = map.section(form.NamedSection, 'main', 'agent', _('Direct DNS'));
		section.anonymous = true;
		section.addremove = false;

		option = section.option(form.Flag, 'direct_dns_enabled', _('Update public DNS addresses'));
		option.default = '0';
		option.rmempty = false;
		option.description = _('Opt-in feature for Go agent 0.6.0 or newer. The router periodically reports its public addresses to RMM.');

		option = section.option(form.Value, 'dns_update_interval_seconds', _('Update interval'));
		option.datatype = 'range(60,86400)';
		option.default = '300';
		option.rmempty = false;
		option.depends('direct_dns_enabled', '1');

		option = section.option(form.Value, 'public_ipv4_url', _('Public IPv4 discovery URL'));
		option.placeholder = 'https://api.ipify.org';
		option.default = 'https://api.ipify.org';
		option.rmempty = false;
		option.depends('direct_dns_enabled', '1');
		option.validate = validateDiscoveryURL;

		option = section.option(form.Value, 'public_ipv6_url', _('Public IPv6 discovery URL'));
		option.placeholder = 'https://api6.ipify.org';
		option.default = 'https://api6.ipify.org';
		option.rmempty = true;
		option.description = _('Clear this field on routers without public IPv6.');
		option.depends('direct_dns_enabled', '1');
		option.validate = validateDiscoveryURL;

		section = map.section(form.NamedSection, 'main', 'agent', _('Advanced settings'));
		section.anonymous = true;
		section.addremove = false;

		option = section.option(form.Value, 'tunnel_identity_file', _('Tunnel identity file'));
		option.placeholder = '/etc/rmm-agent/tunnel_key';
		option.rmempty = false;

		option = section.option(form.Flag, 'allow_insecure_http', _('Allow insecure HTTP'));
		option.description = _('Use only in an isolated lab. Agent credentials can otherwise be intercepted.');

		option = section.option(form.Flag, 'reset_identity', _('Re-enroll on next restart'));
		option.description = _('Deletes the current device identity. Create and enter a fresh one-time grant first.');

		var status = E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, _('Service status')),
			E('p', {}, running ? _('The RMM agent is running.') : _('The RMM agent is stopped.')),
			E('button', {
				'class': 'btn cbi-button-action',
				'click': ui.createHandlerFn(this, 'handleServiceAction', running ? 'restart' : 'start')
			}, running ? _('Restart agent') : _('Start agent'))
		]);

		return map.render().then(function(node) {
			node.insertBefore(status, node.firstChild);
			return node;
		});
	}
});
