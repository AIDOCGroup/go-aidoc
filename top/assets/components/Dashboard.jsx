

import React, {Component} from 'react';

import withStyles from 'material-ui/styles/withStyles';

import Header from './Header';
import Body from './Body';
import {MENU} from '../common';
import type {Content} from '../types/content';

// deepUpdate更新与给定更新数据相对应的对象，该对象具有与原始对象相同结构的形状。
// updater也具有相同的结构，除了它包含需要更新原始数据的函数。 这些函数用于处理更新。
//
// 由于消息具有与状态内容相同的形状，因此该方法允许消息处理的泛化。
// 唯一必要的是为状态的每个路径设置处理函数，以便最大化更新的灵活性。
const deepUpdate = (updater: Object, update: Object, prev: Object): $Shape<Content> => {
	if (typeof update === 'undefined') {
		// TODO (kurkomisi): originally this was deep copy, investigate it.
		return prev;
	}
	if (typeof updater === 'function') {
		return updater(update, prev);
	}
	const updated = {};
	Object.keys(prev).forEach((key) => {
		updated[key] = deepUpdate(updater[key], update[key], prev[key]);
	});

	return updated;
};

// shouldUpdate返回消息的结构。
// 它用于防止不必要的渲染方法触发。
// 在受影响组件的shouldComponentUpdate方法中，可以通过检查消息结构来检查所涉及的数据是否已更改。
//
// 我们也可以自己返回消息，但不提供访问权限更安全。
const shouldUpdate = (updater: Object, msg: Object) => {
	const su = {};
	Object.keys(msg).forEach((key) => {
		su[key] = typeof updater[key] !== 'function' ? shouldUpdate(updater[key], msg[key]) : true;
	});

	return su;
};

// replacer 是一个状态更新程序功能，它取代了原始数据。
const replacer = <T>(update: T) => update;

// appender 是状态更新程序功能，它将更新数据附加到现有数据。
// limit 定义创建的数组的最大允许大小，mapper映射更新数据。
const appender = <T>(limit: number, mapper = replacer) => (update: Array<T>, prev: Array<T>) => [
	...prev,
	...update.map(sample => mapper(sample)),
].slice(-limit);

// defaultContent 是状态内容的初始值。
const defaultContent: Content = {
	general: {
		version: null,
		commit:  null,
	},
	home:    {},
	chain:   {},
	txpool:  {},
	network: {},
	system:  {
		activeMemory:   [],
		virtualMemory:  [],
		networkIngress: [],
		networkEgress:  [],
		processCPU:     [],
		systemCPU:      [],
		diskRead:       [],
		diskWrite:      [],
	},
	logs:    {
		log: [],
	},
};

// updaters 包含状态的每个路径的状态更新程序功能。...
//
// TODO (kurkomisi):定义一个包含内容和更新程序的棘手类型。
const updaters = {
	general: {
		version: replacer,
		commit:  replacer,
	},
	home:    null,
	chain:   null,
	txpool:  null,
	network: null,
	system:  {
		activeMemory:   appender(200),
		virtualMemory:  appender(200),
		networkIngress: appender(200),
		networkEgress:  appender(200),
		processCPU:     appender(200),
		systemCPU:      appender(200),
		diskRead:       appender(200),
		diskWrite:      appender(200),
	},
	logs: {
		log: appender(200),
	},
};

// styles包含组件的常量样式。
const styles = {
	dashboard: {
		display:  'flex',
		flexFlow: 'column',
		width:    '100%',
		height:   '100%',
		zIndex:   1,
		overflow: 'hidden',
	},
};

// themeStyles 返回从组件主题生成的样式。
const themeStyles: Object = (theme: Object) => ({
	dashboard: {
		background: theme.palette.background.default,
	},
});

export type Props = {
	classes: Object, // injected by withStyles()
};

type State = {
	active: string, // 活动菜单
	sideBar: boolean, // 如果侧边栏打开，则为true
	content: Content, // 可视化数据
	shouldUpdate: Object, // 组件的标签，需要根据传入的消息重新呈现
};

// Dashboard is the main component, which renders the whole page, makes connection with the server and
// listens for messages. When there is an incoming message, updates the page's content correspondingly.
class Dashboard extends Component<Props, State> {
	constructor(props: Props) {
		super(props);
		this.state = {
			active:       MENU.get('home').id,
			sideBar:      true,
			content:      defaultContent,
			shouldUpdate: {},
		};
	}

	// componentDidMount 在呈现组件后启动第一个 websocket 连接的建立。
	componentDidMount() {
		this.reconnect();
	}

	// reconnect 与服务器建立 websocket 连接，侦听传入的消息并尝试重新连接连接丢失。
	reconnect = () => {
		// PROD is defined by webpack.
		const server = new WebSocket(`${((window.location.protocol === 'https:') ? 'wss://' : 'ws://')}${PROD ? window.location.host : 'localhost:8080'}/api`);
		server.onopen = () => {
			this.setState({content: defaultContent, shouldUpdate: {}});
		};
		server.onmessage = (event) => {
			const msg: $Shape<Content> = JSON.parse(event.data);
			if (!msg) {
				console.error(`Incoming message is ${msg}`);
				return;
			}
			this.update(msg);
		};
		server.onclose = () => {
			setTimeout(this.reconnect, 3000);
		};
	};

	//  update 更新与传入消息对应的内容。
	update = (msg: $Shape<Content>) => {
		this.setState(prevState => ({
			content:      deepUpdate(updaters, msg, prevState.content),
			shouldUpdate: shouldUpdate(updaters, msg),
		}));
	};

	// changeContent 设置活动标签，用于内容呈现。
	changeContent = (newActive: string) => {
		this.setState(prevState => (prevState.active !== newActive ? {active: newActive} : {}));
	};

	// switchSideBar 打开或关闭侧边栏的状态。
	switchSideBar = () => {
		this.setState(prevState => ({sideBar: !prevState.sideBar}));
	};

	render() {
		return (
			<div className={this.props.classes.dashboard} style={styles.dashboard}>
				<Header
					switchSideBar={this.switchSideBar}
				/>
				<Body
					opened={this.state.sideBar}
					changeContent={this.changeContent}
					active={this.state.active}
					content={this.state.content}
					shouldUpdate={this.state.shouldUpdate}
				/>
			</div>
		);
	}
}

export default withStyles(themeStyles)(Dashboard);
