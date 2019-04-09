

import React from 'react';
import {render} from 'react-dom';

import MuiThemeProvider from 'material-ui/styles/MuiThemeProvider';
import createMuiTheme from 'material-ui/styles/createMuiTheme';

import Dashboard from './components/Dashboard';

const theme: Object = createMuiTheme({
	palette: {
		type: 'dark',
	},
});
const dashboard = document.getElementById('dashboard');
if (dashboard) {
	// 渲染整个仪表板。
	render(
		<MuiThemeProvider theme={theme}>
			<Dashboard />
		</MuiThemeProvider>,
		dashboard,
	);
}
