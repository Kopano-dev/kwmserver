#!/usr/bin/env node

/*
 * Copyright 2017 Kopano and its licensors
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, version 3,
 * as published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

'use strict';

const pjson = require('./package.json');
const program = require('commander');
const winston = require('winston');
const puppeteer = require('puppeteer');

program
	.version(pjson.version)
	.option('-U, --webapp-url <url>', 'ChromiuMCU webapp URL', 'http://localhost:8844/chromiumcu/')
	.option('-u, --websocket-uri <uri>', 'websocket URI to KWM server mcu', 'ws://localhost:8778/api/v1/mcu/websocket')
	.option('-r, --auto-restart', 'automatically restart/reconnect after errors', false)
	.option('--log-level <level>', 'log verbosity level', /^(error|warn|info|verbose|debug|silly)$/i, 'info')
	.option('--insecure', 'ignore HTTPS errors', false)
	.option('--visible', 'launch ChromiuMCU visible/not headless', false)

	.parse(process.argv);

winston.cli();
winston.level = program.logLevel;

winston.log('info', 'Starting ChromiuMCU ...');
winston.log('info', 'Webapp URL:', program.webappUrl);

let browser = null;

function wait(ms) {
	return new Promise(r => setTimeout(r, ms));
}

async function cleanup() {
	if (browser !== null) {
		winston.log('info', 'closing browser ...');
		try {
			await browser.close();
		} catch (err) {
			winston.log('error', 'failed to close browser', err);
		}
	}

	process.exit();
}

async function launch() {
	browser = await puppeteer.launch({ignoreHTTPSErrors: !!program.insecure, headless: !program.visible, args: ['--enable-experimental-web-platform-features', '--disable-gpu']});
	winston.log('info', 'Launced chromium version:', await browser.version());

	const page = await browser.newPage();
	page.on('console', (...args) => {
		winston.log('debug', 'Webapp - %j', args);
	});
	page.on('error', err => {
		winston.log('error', 'Webapp error', err);
		browser.close();
		browser = null;
		throw Error('webapp error');
	});

	try {
		await page.goto(program.webappUrl + '?uri=' + program.websocketUri, {waitUntil: 'networkidle', timeout: 5000});
	} catch (err) {
		browser.close();
		browser = null;
		throw err;
	}

	winston.log('info', 'Ready ...');
}

process.once('SIGINT', cleanup);
process.once('SIGTERM', cleanup);

(async() => {
	while (true) {
		try {
			winston.log('info', 'Launching ...');
			await launch();
			break;
		} catch (err) {
			winston.log('warn', 'Unexpected exit', err);
			if (!program.autoRestart) {
				break;
			}
			winston.log('warn', 'Will restart ...');
			await wait(5000);
		}
	}
})();
