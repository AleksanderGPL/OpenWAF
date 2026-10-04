import type { EChartsOption } from 'echarts'
import type { CountryRequests } from '~/types/telemetry'
import { MapChart } from 'echarts/charts'
import { GeoComponent } from 'echarts/components'
import { registerMap, use } from 'echarts/core'

export type GeoMapMode = 'all' | 'blocked'

const mercatorLimit = 85 * Math.PI / 180

function mercatorProject(point: number[]) {
  const longitude = (point[0] ?? 0) * Math.PI / 180
  const latitude = Math.min(mercatorLimit, Math.max(-mercatorLimit, (point[1] ?? 0) * Math.PI / 180))
  return [longitude, -Math.log(Math.tan(Math.PI / 4 + latitude / 2))]
}

function mercatorUnproject(point: number[]) {
  return [
    (point[0] ?? 0) * 180 / Math.PI,
    (2 * Math.atan(Math.exp(-(point[1] ?? 0))) - Math.PI / 2) * 180 / Math.PI
  ]
}

const countriesOnMap = new Set<string>()
let loading: Promise<void> | null = null

export function mapHasCountry(code: string | null) {
  return !!code && countriesOnMap.has(code)
}

export function loadWorldMap() {
  if (!loading) {
    loading = fetch('/geo/world.json')
      .then(response => {
        if (!response.ok) throw new Error('map')
        return response.json() as Promise<{ features?: { properties?: { iso_a2?: string } }[] }>
      })
      .then(world => {
        use([MapChart, GeoComponent])
        registerMap('world', world as never)
        for (const feature of world.features ?? []) {
          const code = feature.properties?.iso_a2
          if (code) countriesOnMap.add(code)
        }
      })
      .catch(error => {
        loading = null
        throw error
      })
  }
  return loading
}

export function createGeoMapOption(
  items: CountryRequests[],
  mode: GeoMapMode,
  nameOf: (code: string) => string,
  copy: { requests: string, format: (value: number) => string }
): EChartsOption {
  const areaColor = mode === 'blocked' ? '#fb923c' : '#38bdf8'
  const points = items.flatMap(item => item.countryCode && item.requests > 0 && countriesOnMap.has(item.countryCode)
    ? [{ name: item.countryCode, value: item.requests, itemStyle: { areaColor } }]
    : [])
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: '#0b1220',
      borderColor: '#1e293b',
      textStyle: { color: '#e2e8f0', fontSize: 12, fontFamily: 'Figtree, sans-serif' },
      formatter: raw => {
        const params = (Array.isArray(raw) ? raw[0] : raw) as { name?: string, value?: number }
        const requests = Number(params?.value ?? 0)
        if (!params?.name || !requests) return ''
        return `${nameOf(params.name)}<br/>${copy.format(requests)} ${copy.requests}`
      }
    },
    series: [{
      type: 'map',
      map: 'world',
      nameProperty: 'iso_a2',
      roam: false,
      left: 0,
      right: 0,
      top: 0,
      bottom: 0,
      aspectScale: 1,
      boundingCoords: [[-180, 85], [180, -62]],
      showLegendSymbol: false,
      clip: true,
      projection: {
        project: mercatorProject,
        unproject: mercatorUnproject
      },
      selectedMode: false,
      itemStyle: {
        areaColor: '#1b3146',
        borderColor: '#4d7394',
        borderWidth: 0.7
      },
      emphasis: {
        label: { show: false },
        itemStyle: {
          areaColor: mode === 'blocked' ? '#fdba74' : '#7dd3fc',
          borderColor: '#f8fafc',
          borderWidth: 1
        }
      },
      data: points
    }]
  }
}
