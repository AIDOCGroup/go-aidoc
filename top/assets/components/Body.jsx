

import React, {Component} from 'react';

import SideBar from './SideBar';
import Main from './Main';
import type {Content} from '../types/content';

// styles包含组件的常量样式。
const styles = {
	body: {
		display: 'flex',
		width:   '100%',
		height:  '100%',
	},
};

export type Props = {
	opened: boolean,
	changeContent: string => void,
	active: string,
	content: Content,
	shouldUpdate: Object,
};

// Body渲染仪表板的主体。
class Body extends Component<Props> {
	render() {
		return (
			<div style={styles.body}>
				<SideBar
					opened={this.props.opened}
					changeContent={this.props.changeContent}
				/>
				<Main
					active={this.props.active}
					content={this.props.content}
					shouldUpdate={this.props.shouldUpdate}
				/>
			</div>
		);
	}
}

export default Body;
