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

var startAgent = rpc.declare({ object: 'rmm.agent', method: 'start', expect: { code: -1 } });
var restartAgent = rpc.declare({ object: 'rmm.agent', method: 'restart', expect: { code: -1 } });

function isRunning(services) {
	var instances = (services['rmm-agent'] || {}).instances || {};
	return Object.keys(instances).some(function(key) { return instances[key] && instances[key].running === true; });
}

return view.extend({
	load: function() {
		return Promise.all([ uci.load('rmm-agent'), callServiceList('rmm-agent') ]);
	},

	handleServiceAction: function(update, button, status) {
		return (button.dataset.running === 'true' ? restartAgent() : startAgent()).then(function(code) {
			if (code)
				throw new Error(_('Command failed with code %d').format(code));
			ui.addNotification(null, E('p', {}, _('RMM agent action completed.')), 'info');
			return callServiceList('rmm-agent').then(function(services) { update(button, status, services); });
		}).catch(function(error) {
			ui.addNotification(null, E('p', {}, _('Unable to control RMM agent: %s').format(error.message)));
		});
	},

	render: function(data) {
		var services = data[1] || {};
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

		option = section.option(form.Value, 'interval_seconds', _('Heartbeat interval (seconds)'));
		option.datatype = 'range(10,3600)';
		option.default = '30';
		option.rmempty = false;

		option = section.option(form.Value, 'connectivity_check_interval_seconds', _('Connectivity check interval (seconds)'));
		option.datatype = 'range(30,3600)';
		option.default = '300';
		option.rmempty = false;
		option.description = _('HTTPS results are reused between checks while heartbeats continue at their own interval.');

		option = section.option(form.Value, 'https_check_urls', _('HTTPS check URLs'));
		option.placeholder = 'https://detectportal.firefox.com/success.txt https://www.apple.com/library/test/success.html';
		option.description = _('Up to four HTTPS URLs separated by spaces. Leave empty to use the defaults. Older agents use their saved ping targets.');
		option.rmempty = true;

		section = map.section(form.NamedSection, 'main', 'agent', _('Advanced settings'));
		section.anonymous = true;
		section.addremove = false;

		option = section.option(form.Value, 'tunnel_identity_file', _('Tunnel identity file'));
		option.placeholder = '/etc/rmm-agent/tunnel_device_key';
		option.rmempty = false;

		option = section.option(form.Flag, 'allow_insecure_http', _('Allow insecure HTTP'));
		option.description = _('Use only in an isolated lab. Agent credentials can otherwise be intercepted.');

		option = section.option(form.Flag, 'reset_identity', _('Re-enroll on next restart'));
		option.description = _('Deletes the current device identity. Create and enter a fresh one-time grant first.');

		function update(button, status, services) {
			var running = isRunning(services);
			status.textContent = running ? _('The RMM agent is running.') : _('The RMM agent is stopped.');
			button.textContent = running ? _('Restart agent') : _('Start agent');
			button.dataset.running = String(running);
		}
		var state = E('p');
		var button = E('button', { 'type': 'button', 'class': 'btn cbi-button-action' });
		button.addEventListener('click', ui.createHandlerFn(this, 'handleServiceAction', update, button, state));
		update(button, state, services);
		var status = E('div', { 'class': 'cbi-section' }, [
			E('h3', {}, _('Service status')),
			state,
			button
		]);

		return map.render().then(function(node) {
			node.insertBefore(status, node.firstChild);
			return node;
		});
	}
});
