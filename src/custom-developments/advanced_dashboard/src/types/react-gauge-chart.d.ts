declare module 'react-gauge-chart' {
  import { ReactNode } from 'react'

  interface GaugeChartProps {
    id?: string
    percent: number
    nrOfLevels?: number
    colors?: string[]
    arcWidth?: number
    arcPadding?: number
    cornerRadius?: number
    textColor?: string
    needleColor?: string
    needleBaseColor?: string
    hideText?: boolean
    animate?: boolean
    animDelay?: number
    formatTextValue?: (value: number) => string | ReactNode
  }

  declare const GaugeChart: React.FC<GaugeChartProps>
  export default GaugeChart
}
