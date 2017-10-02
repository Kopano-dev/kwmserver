const fs = require('fs');
const path = require('path');
const BannerPlugin = require('webpack').BannerPlugin;
const DefinePlugin = require('webpack').DefinePlugin;
const UglifyJsPlugin = require('uglifyjs-webpack-plugin');
const LicenseWebpackPlugin = require('license-webpack-plugin').LicenseWebpackPlugin;
const buildVersion = process.env.BUILD_VERSION || 'v0.0.0-no-proper-build';
const buildDate = process.env.BUILD_DATE || new Date();

module.exports = {
	resolve: {
		extensions: ['.ts', '.js']
	},
	entry: './src/kwm.ts',
	output: {
		filename: 'kwm.js',
		path: path.resolve(__dirname, 'dist'),
		publicPath: '/dist/',
		library: 'KWM',
		libraryExport: 'KWM',
		libraryTarget: 'umd'
	},
	module: {
		rules: [
			{
				test: /\.tsx?$/,
				loaders: ['ts-loader']
			}
		]
	},
	devtool: 'source-map',
	plugins: [
		new UglifyJsPlugin({
			uglifyOptions: {
				ecma: 6,
				warnings: true
			}
		}),
		new DefinePlugin({
			__VERSION__: JSON.stringify(buildVersion)
		}),
		new LicenseWebpackPlugin({
			pattern: /^(MIT|ISC|BSD.*)$/,
			unacceptablePattern: /GPL/,
			abortOnUnacceptableLicense: true,
			perChunkOutput: false,
			outputFilename: 'kwm.3rdpartylicenses.txt'
		}),
		new BannerPlugin(
			fs.readFileSync(path.resolve(__dirname, '..', '..', 'LICENSE.txt')).toString()
			+ '\n\n@version ' + buildVersion + ' (' + buildDate + ')'
		)
	]
};
