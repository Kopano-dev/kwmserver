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

import { VideoCall as JanusVideoCallPlugin } from './janus/plugins/videocall';
import { KWM } from './kwm';

/**
 * A Iplugin<T> is the generic interface for plugins.
 */
export interface IPlugin<T> {
	/**
	 * Constructor for plugins.
	 *
	 * @param kwm Reference to an instance the plugin can use.
	 * @param callbacks flexible parameter, plugin specific.
	 * @returns The plugin instance, created with the provided parameters.
	 */
	new(kwm: KWM, callbacks: any): T;

	/**
	 * Provides the name of the plugin.
	 *
	 * @returns Unique id of the plugin used as name.
	 */
	getName(): string;
}

/**
 * Plugins implement the registry for plugins.
 */
export class Plugins {
	/**
	 * Registers the provided plugin class in the registry.
	 *
	 * @param plugin Factory class of the plugin to register.
	 */
	public static register(plugin: IPlugin<any>) {
		this.registry.set(plugin.getName(), plugin);
	}

	/**
	 * Fetch a registered plugin class by name.
	 *
	 * @returns Plugin factory.
	 */
	public static get(name: string): IPlugin<any> | undefined {
		return this.registry.get(name);
	}

	private static registry = new Map<string, IPlugin<any>>();
}

Plugins.register(JanusVideoCallPlugin);
