import { useState, useEffect } from 'react'
import GaugeChart from 'react-gauge-chart'
import { type DataPoint, getRealTimeData } from '../lib/scadaOpcApi'
import { CircleIcon } from '@radix-ui/react-icons'

interface RealTimeArcGaugeProps {
  selectedPoints: string[]
  onRemovePoint: (pointName: string) => void
}

export function RealTimeArcGauge({
  selectedPoints,
  onRemovePoint,
}: RealTimeArcGaugeProps) {
  const [data, setData] = useState<DataPoint[]>([])
  const [error, setError] = useState<Error | null>(null)

  useEffect(() => {
    if (selectedPoints.length === 0) {
      setData([])
      return
    }

    const fetchData = async () => {
      try {
        const result = await getRealTimeData(selectedPoints)
        const filteredResult = result.filter(Boolean)

        // Only update if values have changed
        setData((prevData) => {
          if (prevData.length !== filteredResult.length) return filteredResult

          const hasChanged = filteredResult.some((newPoint, index) => {
            const prevPoint = prevData[index]
            return prevPoint.value !== newPoint.value
          })

          return hasChanged ? filteredResult : prevData
        })
      } catch (err) {
        setError(err as Error)
      }
    }

    // Initial fetch
    fetchData()

    // Set up interval for periodic updates with slower refresh rate
    const interval = setInterval(fetchData, 3000)

    // Cleanup interval on component unmount
    return () => clearInterval(interval)
  }, [selectedPoints])

  if (error)
    return (
      <div className="text-red-500">Failed to load data: {error.message}</div>
    )
  if (selectedPoints.length === 0)
    return (
      <div className="w-full h-[400px] flex items-center justify-center text-gray-500">
        Select points from the station list to display data
      </div>
    )
  if (!data.length) return <div className="text-gray-500">Loading...</div>

  return (
    <div className="w-full p-4 bg-gray-900">
      <div className="mb-4 flex flex-wrap gap-2">
        {selectedPoints.map((point) => (
          <div
            key={point}
            className="flex items-center gap-2 bg-gray-800/50 backdrop-blur-sm border border-cyan-500/20 text-cyan-400 px-3 py-1.5 rounded-full shadow-lg shadow-cyan-500/20"
          >
            <span className="text-sm font-medium">{point}</span>
            <button
              onClick={() => onRemovePoint(point)}
              className="hover:text-red-400 transition-colors"
              title="Remove point"
            >
              <CircleIcon className="h-4 w-4" />
            </button>
          </div>
        ))}
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-6">
        {data.map((point) => (
          <div
            key={point.name}
            className="bg-gray-800/40 backdrop-blur-md rounded-xl border border-cyan-500/20 p-6 shadow-lg hover:shadow-cyan-500/10 transition-all duration-300"
            style={{
              boxShadow: '0 0 20px rgba(6, 182, 212, 0.1)',
            }}
          >
            <h3 className="text-center text-cyan-400 text-sm font-medium mb-4">
              {point.name}
            </h3>
            <div className="relative">
              <div className="absolute inset-0 bg-gradient-to-b from-cyan-500/10 to-transparent rounded-full" />
              <GaugeChart
                id={`gauge-${point.name}`}
                nrOfLevels={20}
                percent={point.value / 10000}
                textColor="#06b6d4"
                colors={['#0891b2', '#06b6d4', '#22d3ee']}
                formatTextValue={(value) => `${(value * 100).toFixed(0)}`}
                animate={false}
                cornerRadius={6}
                arcPadding={0.03}
                needleColor="#0891b2"
                needleBaseColor="#06b6d4"
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
