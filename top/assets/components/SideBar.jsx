

import React, {Component} from 'react';

import withStyles from 'material-ui/styles/withStyles';
import List, {ListItem, ListItemIcon, ListItemText} from 'material-ui/List';
import Icon from 'material-ui/Icon';
import Transition from 'react-transition-group/Transition';
import {Icon as FontAwesome} from 'react-fa';

import {MENU, DURATION} from '../common';

// styles 包含组件的常量样式。
const styles = {
	menu: {
		default: {
			transition: `margin-left ${DURATION}ms`,
		},
		transition: {
			entered: {marginLeft: -200},
		},
	},
};

// themeStyles 返回从组件主题生成的样式。
const themeStyles = theme => ({
	list: {
		background: theme.palette.grey[900],
	},
	listItem: {
		minWidth: theme.spacing.unit * 7,
	},
	icon: {
		fontSize: theme.spacing.unit * 3,
	},
});

export type Props = {
	classes: Object, // 注入withStyles()
	opened: boolean,
	changeContent: string => void,
};

// SideBar 呈现仪表板的侧边栏。
class SideBar extends Component<Props> {
	shouldComponentUpdate(nextProps) {
		return nextProps.opened !== this.props.opened;
	}

	// clickOn 返回给定菜单项的单击事件处理函数。
	clickOn = menu => (event) => {
		event.preventDefault();
		this.props.changeContent(menu);
	};

	// menuItems 返回与侧边栏状态对应的菜单项。
	menuItems = (transitionState) => {
		const {classes} = this.props;
		const children = [];
		MENU.forEach((menu) => {
			children.push((
				<ListItem button key={menu.id} onClick={this.clickOn(menu.id)} className={classes.listItem}>
					<ListItemIcon>
						<Icon className={classes.icon}>
							<FontAwesome name={menu.icon} />
						</Icon>
					</ListItemIcon>
					<ListItemText
						primary={menu.title}
						style={{
							...styles.menu.default,
							...styles.menu.transition[transitionState],
							padding: 0,
						}}
					/>
				</ListItem>
			));
		});
		return children;
	};

	// 菜单呈现菜单项的列表。
	menu = (transitionState: Object) => (
		<div className={this.props.classes.list}>
			<List>
				{this.menuItems(transitionState)}
			</List>
		</div>
	);

	render() {
		return (
			<Transition mountOnEnter in={this.props.opened} timeout={{enter: DURATION}}>
				{this.menu}
			</Transition>
		);
	}
}

export default withStyles(themeStyles)(SideBar);
