import { useState, useEffect } from 'react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { type DataPoint, getRealTimeData } from '../lib/scadaOpcApi'
import { CircleIcon } from '@radix-ui/react-icons'

interface RealTimeBarGraphProps {
  selectedPoints: string[]
  onRemovePoint: (pointName: string) => void
}

export function RealTimeBarGraph({
  selectedPoints,
  onRemovePoint,
}: RealTimeBarGraphProps) {
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
        setData(result.filter(Boolean))
      } catch (err) {
        setError(err as Error)
      }
    }

    // Initial fetch
    fetchData()

    // Set up interval for periodic updates
    const interval = setInterval(fetchData, 1500)

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
    <div className="w-full p-4">
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
      <div className="h-[300px]">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data}>
            <CartesianGrid
              strokeDasharray="3 3"
              stroke="rgba(6, 182, 212, 0.1)"
              vertical={false}
            />
            <XAxis
              dataKey="name"
              style={{ fontSize: '0.75rem' }}
              interval={0}
              tick={{ fill: '#06b6d4' }}
              axisLine={{ stroke: '#0891b2' }}
            />
            <YAxis
              tick={{ fill: '#06b6d4' }}
              axisLine={{ stroke: '#0891b2' }}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: 'rgba(17, 24, 39, 0.8)',
                border: '1px solid rgba(6, 182, 212, 0.2)',
                borderRadius: '0.5rem',
                backdropFilter: 'blur(8px)',
                color: '#06b6d4',
              }}
            />
            <Bar
              dataKey="value"
              fill="rgba(6, 182, 212, 0.6)"
              stroke="rgba(6, 182, 212, 0.8)"
              strokeWidth={1}
              radius={[4, 4, 0, 0]}
            />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
