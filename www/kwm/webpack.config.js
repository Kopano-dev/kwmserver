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
		library: 'kwmjs',
		libraryTarget: 'var'
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
