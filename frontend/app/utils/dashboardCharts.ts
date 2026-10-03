import type { EChartsOption } from 'echarts'
import type { TrafficPoint, TrafficView, ThreatCategory } from '~/types/dashboard'
import { formatBytes, formatDashboardNumber } from '~/utils/dashboard'

export function createTrafficChartOption(points: readonly TrafficPoint[], trafficView: TrafficView, labels: { totalRequests: string, blockedRequests: string, requestBody: string, responseBody: string }): EChartsOption {
  const axisStyle = {
    color: '#94a3b8',
    fontSize: 11,
    fontFamily: 'Inter, system-ui, sans-serif'
  }
  const bytes = trafficView === 'bandwidth'
  const labelInterval = points.length <= 8 ? 0 : Math.ceil(points.length / 8) - 1
  return {
    animationDuration: 450,
    color: ['#6366f1', '#f59e0b'],
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#fff',
      borderColor: '#e2e8f0',
      textStyle: {
        color: '#334155',
        fontSize: 12
      },
      valueFormatter: value => bytes ? formatBytes(Number(value)) : formatDashboardNumber(Number(value))
    },
    grid: {
      left: bytes ? 64 : 48,
      right: 16,
      top: 22,
      bottom: 32
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: points.map(point => point.label),
      axisLine: {
        show: false
      },
      axisTick: {
        show: false
      },
      axisLabel: {
        ...axisStyle,
        interval: labelInterval,
        margin: 16
      }
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        ...axisStyle,
        formatter: value => bytes ? formatBytes(Number(value)) : Number(value) >= 1000 ? `${Number(value) / 1000}k` : String(value)
      },
      splitLine: {
        lineStyle: {
          color: '#f0f2f6',
          type: 'dashed'
        }
      }
    },
    series: [{
      name: bytes ? labels.requestBody : labels.totalRequests,
      type: 'line',
      smooth: 0.35,
      symbol: 'none',
      data: points.map(point => bytes ? point.requestBytes : point.totalRequests),
      lineStyle: {
        width: 2.5
      },
      areaStyle: {
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [{
            offset: 0,
            color: '#6366f12b'
          }, {
            offset: 1,
            color: '#6366f101'
          }]
        }
      }
    }, {
      name: bytes ? labels.responseBody : labels.blockedRequests,
      type: 'line',
      smooth: 0.35,
      symbol: 'none',
      data: points.map(point => bytes ? point.responseBytes : point.blockedRequests),
      lineStyle: {
        width: 2
      }
    }]
  }
}
export function createThreatChartOption(attacks: readonly ThreatCategory[]): EChartsOption {
  return {
    color: attacks.map(attack => attack.color),
    tooltip: {
      trigger: 'item',
      formatter: '{b}: {c} ({d}%)'
    },
    series: [{
      type: 'pie',
      radius: ['70%', '91%'],
      center: ['50%', '50%'],
      avoidLabelOverlap: true,
      label: {
        show: false
      },
      itemStyle: {
        borderColor: '#fff',
        borderWidth: 4,
        borderRadius: 5
      },
      emphasis: {
        scaleSize: 4
      },
      data: attacks.map(attack => ({
        name: attack.name,
        value: attack.value
      }))
    }]
  }
}
export const createSparklineOption = (values: number[], color: string): EChartsOption => ({
  grid: {
    left: 0,
    right: 0,
    top: 5,
    bottom: 0
  },
  xAxis: {
    type: 'category',
    show: false
  },
  yAxis: {
    type: 'value',
    show: false
  },
  series: [{
    type: 'line',
    data: values,
    smooth: true,
    symbol: 'none',
    lineStyle: {
      color,
      width: 2
    },
    areaStyle: {
      color: `${color}0d`
    }
  }]
})
