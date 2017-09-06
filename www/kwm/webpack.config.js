const path = require('path');

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
	devtool: 'source-map'
};
