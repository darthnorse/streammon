declare module 'react-leaflet-heatmap-layer-v3' {
  import type { LayerProps } from 'react-leaflet'

  export interface HeatmapLayerProps<T> extends LayerProps {
    points: T[]
    latitudeExtractor: (point: T) => number
    longitudeExtractor: (point: T) => number
    intensityExtractor: (point: T) => number
    gradient?: Record<number, string>
    radius?: number
    blur?: number
    minOpacity?: number
    maxZoom?: number
    max?: number
  }

  export function HeatmapLayer<T>(props: HeatmapLayerProps<T>): JSX.Element
}
