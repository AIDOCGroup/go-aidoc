

import React, {Component} from 'react';

import withStyles from 'material-ui/styles/withStyles';
import Typography from 'material-ui/Typography';
import Grid from 'material-ui/Grid';
import {ResponsiveContainer, AreaChart, Area, Tooltip} from 'recharts';

import ChartRow from './ChartRow';
import CustomTooltip, {bytePlotter, bytePerSecPlotter, percentPlotter, multiplier} from './CustomTooltip';
import {styles as commonStyles} from '../common';
import type {General, System} from '../types/content';

const FOOTER_SYNC_ID = 'footerSyncId';

const CPU     = 'cpu';
const MEMORY  = 'memory';
const DISK    = 'disk';
const TRAFFIC = 'traffic';

const TOP = 'Top';
const BOTTOM = 'Bottom';

// styles 包含组件的常量样式。
const styles = {
	footer: {
		maxWidth: '100%',
		flexWrap: 'nowrap',
		margin:   0,
	},
	chartRowWrapper: {
		height:  '100%',
		padding: 0,
	},
	doubleChartWrapper: {
		height: '100%',
		width:  '99%',
	},
};

// themeStyles 返回从组件主题生成的样式。
const themeStyles: Object = (theme: Object) => ({
	footer: {
		backgroundColor: theme.palette.grey[900],
		color:           theme.palette.getContrastText(theme.palette.grey[900]),
		zIndex:          theme.zIndex.appBar,
		height:          theme.spacing.unit * 10,
	},
});

export type Props = {
	classes: Object, // 注入withStyles()
	theme: Object,
	general: General,
	system: System,
	shouldUpdate: Object,
};

// 页脚呈现仪表板的页脚。
class Footer extends Component<Props> {
	shouldComponentUpdate(nextProps) {
		return typeof nextProps.shouldUpdate.general !== 'undefined' || typeof nextProps.shouldUpdate.system !== 'undefined';
	}

	// halfHeightChart 使用其父级高度的一半渲染面积图。
	halfHeightChart = (chartProps, tooltip, areaProps) => (
		<ResponsiveContainer width='100%' height='50%'>
			<AreaChart {...chartProps} >
				{!tooltip || (<Tooltip cursor={false} content={<CustomTooltip tooltip={tooltip} />} />)}
				<Area isAnimationActive={false} type='monotone' {...areaProps} />
			</AreaChart>
		</ResponsiveContainer>
	);

	// doubleChart 呈现由基线分隔的一对图表。
	doubleChart = (syncId, chartKey, topChart, bottomChart) => {
		if (!Array.isArray(topChart.data) || !Array.isArray(bottomChart.data)) {
			return null;
		}
		const topDefault = topChart.default || 0;
		const bottomDefault = bottomChart.default || 0;
		const topKey = `${chartKey}${TOP}`;
		const bottomKey = `${chartKey}${BOTTOM}`;
		const topColor = '#8884d8';
		const bottomColor = '#82ca9d';

		return (
			<div style={styles.doubleChartWrapper}>
				{this.halfHeightChart(
					{
						syncId,
						data:   topChart.data.map(({value}) => ({[topKey]: value || topDefault})),
						margin: {top: 5, right: 5, bottom: 0, left: 5},
					},
					topChart.tooltip,
					{dataKey: topKey, stroke: topColor, fill: topColor},
				)}
				{this.halfHeightChart(
					{
						syncId,
						data:   bottomChart.data.map(({value}) => ({[bottomKey]: -value || -bottomDefault})),
						margin: {top: 0, right: 5, bottom: 5, left: 5},
					},
					bottomChart.tooltip,
					{dataKey: bottomKey, stroke: bottomColor, fill: bottomColor},
				)}
			</div>
		);
	};

	render() {
		const {general, system} = this.props;

		return (
			<Grid container className={this.props.classes.footer} direction='row' alignItems='center' style={styles.footer}>
				<Grid item xs style={styles.chartRowWrapper}>
					<ChartRow>
						{this.doubleChart(
							FOOTER_SYNC_ID,
							CPU,
							{data: system.processCPU, tooltip: percentPlotter('Process load')},
							{data: system.systemCPU, tooltip: percentPlotter('System load', multiplier(-1))},
						)}
						{this.doubleChart(
							FOOTER_SYNC_ID,
							MEMORY,
							{data: system.activeMemory, tooltip: bytePlotter('Active memory')},
							{data: system.virtualMemory, tooltip: bytePlotter('Virtual memory', multiplier(-1))},
						)}
						{this.doubleChart(
							FOOTER_SYNC_ID,
							DISK,
							{data: system.diskRead, tooltip: bytePerSecPlotter('Disk read')},
							{data: system.diskWrite, tooltip: bytePerSecPlotter('Disk write', multiplier(-1))},
						)}
						{this.doubleChart(
							FOOTER_SYNC_ID,
							TRAFFIC,
							{data: system.networkIngress, tooltip: bytePerSecPlotter('Download')},
							{data: system.networkEgress, tooltip: bytePerSecPlotter('Upload', multiplier(-1))},
						)}
					</ChartRow>
				</Grid>
				<Grid item >
					<Typography type='caption' color='inherit'>
						<span style={commonStyles.light}>Gaidoc</span> {general.version}
					</Typography>
					{general.commit && (
						<Typography type='caption' color='inherit'>
							<span style={commonStyles.light}>{'Commit '}</span>
							<a href={`https://github.com/aidoc/go-aidoc/commit/${general.commit}`} target='_blank' style={{color: 'inherit', textDecoration: 'none'}} >
								{general.commit.substring(0, 8)}
							</a>
						</Typography>
					)}
				</Grid>
			</Grid>
		);
	}
}

export default withStyles(themeStyles)(Footer);
