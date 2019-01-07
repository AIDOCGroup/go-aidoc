

import React, {Component} from 'react';

import withStyles from 'material-ui/styles/withStyles';
import AppBar from 'material-ui/AppBar';
import Toolbar from 'material-ui/Toolbar';
import IconButton from 'material-ui/IconButton';
import Icon from 'material-ui/Icon';
import MenuIcon from 'material-ui-icons/Menu';
import Typography from 'material-ui/Typography';

// themeStyles 返回从组件主题生成的样式。
const themeStyles = (theme: Object) => ({
	header: {
		backgroundColor: theme.palette.grey[900],
		color:           theme.palette.getContrastText(theme.palette.grey[900]),
		zIndex:          theme.zIndex.appBar,
	},
	toolbar: {
		paddingLeft:  theme.spacing.unit,
		paddingRight: theme.spacing.unit,
	},
	title: {
		paddingLeft: theme.spacing.unit,
		fontSize:    3 * theme.spacing.unit,
	},
});

export type Props = {
	classes: Object, // 注入 withStyles()
	switchSideBar: () => void,
};

// 标题呈现仪表板的标题。
class Header extends Component<Props> {
	render() {
		const {classes} = this.props;

		return (
			<AppBar position='static' className={classes.header}>
				<Toolbar className={classes.toolbar}>
					<IconButton onClick={this.props.switchSideBar}>
						<Icon>
							<MenuIcon />
						</Icon>
					</IconButton>
					<Typography type='title' color='inherit' noWrap className={classes.title}>
						Go aidoc Dashboard
					</Typography>
				</Toolbar>
			</AppBar>
		);
	}
}

export default withStyles(themeStyles)(Header);
