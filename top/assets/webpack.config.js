const webpack = require('webpack');
const path = require('path');

module.exports = {
	resolve: {
		extensions: ['.js', '.jsx'],
	},
	entry:  './index',
	output: {
		path:     path.resolve(__dirname, ''),
		filename: 'bundle.js',
	},
	plugins: [
		new webpack.optimize.UglifyJsPlugin({
			comments: false,
			mangle:   false,
			beautify: true,
		}),
		new webpack.DefinePlugin({
			PROD: process.env.NODE_ENV === 'production',
		}),
	],
	module: {
		rules: [
			{
				test:    /\.jsx$/, // JSX文件的正则表达式
				exclude: /node_modules/,
				use:     [ // order: 从下到上
					{
						loader:  'babel-loader',
						options: {
							plugins: [ // order: 从上到下
								// 'transform-decorators-legacy', // @withStyles, @withTheme
								'transform-class-properties', // static defaultProps
								'transform-flow-strip-types',
							],
							presets: [ // order: 从下到上
								'env',
								'react',
								'stage-0',
							],
						},
					},
					// 'eslint-loader', // 不仅在编辑器中显示错误，还在控制台中显示错误
				],
			},
			{
				test: /font-awesome\.css$/,
				use:  [
					'style-loader',
					'css-loader',
					path.resolve(__dirname, './fa-only-woff-loader.js'),
				],
			},
			{
				test: /\.woff2?$/, // 字体很棒的图标
				use:  'url-loader',
			},
		],
	},
};
