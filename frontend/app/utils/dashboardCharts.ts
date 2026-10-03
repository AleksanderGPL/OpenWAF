import type { EChartsOption } from 'echarts'
import type { DashboardRange, TrafficView, ThreatCategory } from '~/types/dashboard'
import { hourlyTraffic, hourlyBlocked } from '~/data/dashboard'
import { formatDashboardNumber } from '~/utils/dashboard'

export function createTrafficChartOption(range: DashboardRange, trafficView: TrafficView, totalRequests: number, totalBlocked: number): EChartsOption {
  const axisStyle = {
    color: '#94a3b8',
    fontSize: 11,
    fontFamily: 'Inter, system-ui, sans-serif'
  }
  const chartLabels = range === '24h' ? hourlyTraffic.map((_, index) => `${String(index).padStart(2, '0')}:00`) : Array.from({
    length: range === '7d' ? 7 : 30
  }, (_, index) => `Day ${index + 1}`)
  const length = chartLabels.length
  const distribute = (total: number, weights: number[]) => {
    const weightSum = weights.reduce((sum, value) => sum + value, 0)
    const result = weights.map(weight => Math.floor(total * weight / weightSum))
    result[result.length - 1]! += total - result.reduce((sum, value) => sum + value, 0)
    return result
  }
  const weights = Array.from({
    length
  }, (_, i) => 0.7 + hourlyTraffic[i % 24]! / 52800 * 0.5)
  const values = range === '24h' ? hourlyTraffic : distribute(totalRequests, weights)
  const blocked = range === '24h' ? hourlyBlocked : distribute(totalBlocked, weights)
  const bytes = trafficView === 'bandwidth'
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
      valueFormatter: value => bytes ? `${Number(value).toFixed(1)} MB` : formatDashboardNumber(Number(value))
    },
    grid: {
      left: 48,
      right: 16,
      top: 22,
      bottom: 32
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: chartLabels,
      axisLine: {
        show: false
      },
      axisTick: {
        show: false
      },
      axisLabel: {
        ...axisStyle,
        interval: range === '24h' ? 3 : range === '7d' ? 0 : 5,
        margin: 16
      }
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        ...axisStyle,
        formatter: value => bytes ? `${value} MB` : value >= 1000 ? `${value / 1000}k` : String(value)
      },
      splitLine: {
        lineStyle: {
          color: '#f0f2f6',
          type: 'dashed'
        }
      }
    },
    series: [{
      name: bytes ? 'Bandwidth (MB)' : 'Total requests',
      type: 'line',
      smooth: 0.35,
      symbol: 'none',
      data: bytes ? values.map(value => +(value * 0.008).toFixed(1)) : values,
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
      name: bytes ? 'Blocked bandwidth (MB)' : 'Blocked requests',
      type: 'line',
      smooth: 0.35,
      symbol: 'none',
      data: bytes ? blocked.map(value => +(value * 0.008).toFixed(1)) : blocked,
      lineStyle: {
        width: 2
      }
    }]
  }
}
export function createThreatChartOption(attacks: readonly ThreatCategory[], multiplier: number): EChartsOption {
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
        value: attack.value * multiplier
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
