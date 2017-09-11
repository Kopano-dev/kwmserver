const fs = require('fs');
const path = require('path');
const BannerPlugin = require('webpack').BannerPlugin;
const UglifyJsPlugin = require('webpack').optimize.UglifyJsPlugin;
const LicenseWebpackPlugin = require('license-webpack-plugin').LicenseWebpackPlugin;

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
		new LicenseWebpackPlugin({
			pattern: /^(MIT|ISC|BSD.*)$/,
			unacceptablePattern: /GPL/,
			abortOnUnacceptableLicense: true,
			perChunkOutput: false,
			outputFilename: 'kwm.3rdpartylicenses.txt'
		}),
		new BannerPlugin(fs.readFileSync(path.resolve(__dirname, '..', '..', 'LICENSE.txt')).toString())
	]
};
