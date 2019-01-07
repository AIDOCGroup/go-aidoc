// @flow



import React, {Component} from 'react';
import type {ChildrenArray} from 'react';

import Grid from 'material-ui/Grid';

// styles包含组件的常量样式。
const styles = {
	container: {
		flexWrap: 'nowrap',
		height:   '100%',
		maxWidth: '100%',
		margin:   0,
	},
	item: {
		flex:    1,
		padding: 0,
	},
}

export type Props = {
	children: ChildrenArray<React$Element<any>>,
};

// ChartRow呈现一行大小相同的响应图表。
class ChartRow extends Component<Props> {
	render() {
		return (
			<Grid container direction='row' style={styles.container} justify='space-between'>
				{React.Children.map(this.props.children, child => (
					<Grid item xs style={styles.item}>
						{child}
					</Grid>
				))}
			</Grid>
		);
	}
}

export default ChartRow;
