import { useState, useEffect } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { CircleIcon } from '@radix-ui/react-icons'
import { type HistoricalData, getHistoricalData } from '../lib/scadaOpcApi'

interface HistoricalDataPlotProps {
  selectedPoints: string[]
  onRemovePoint: (pointName: string) => void
}

interface PointData {
  pointName: string
  data: {
    timestamp: string
    value: number
  }[]
}

export function HistoricalDataPlot({
  selectedPoints,
  onRemovePoint,
}: HistoricalDataPlotProps) {
  const [data, setData] = useState<PointData[]>([])
  const [error, setError] = useState<Error | null>(null)

  useEffect(() => {
    if (selectedPoints.length === 0) {
      setData([])
      return
    }

    const fetchHistoricalData = async () => {
      try {
        // Calculate time 1 hour ago
        const timeBegin = new Date()
        timeBegin.setHours(timeBegin.getHours() - 1)

        // Fetch historical data for each point
        const historicalDataPromises = selectedPoints.map((point) =>
          getHistoricalData(point, timeBegin, null)
        )

        const historicalDataResults = await Promise.all(historicalDataPromises)

        // Process data for each point separately
        const pointsData = historicalDataResults.map((pointData, index) => {
          const pointName = selectedPoints[index]
          const filteredData = pointData
            .filter(Boolean)
            .map((dataPoint) => ({
              timestamp: dataPoint.serverTimestamp.toISOString(),
              value: dataPoint.value,
            }))
            .sort(
              (a, b) =>
                new Date(a.timestamp).getTime() -
                new Date(b.timestamp).getTime()
            )

          return {
            pointName,
            data: filteredData,
          }
        })

        setData(pointsData)
      } catch (err) {
        setError(err as Error)
      }
    }

    // Initial fetch
    fetchHistoricalData()

    // Set up interval for periodic updates
    const interval = setInterval(fetchHistoricalData, 60000) // Update every minute

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
        Select points from the station list to display historical data
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
      <div className="grid grid-cols-1 gap-6">
        {data.map(({ pointName, data }) => (
          <div
            key={pointName}
            className="bg-gray-800/40 backdrop-blur-md rounded-xl border border-cyan-500/20 p-6 shadow-lg hover:shadow-cyan-500/10 transition-all duration-300"
          >
            <h3 className="text-center text-cyan-400 text-lg font-medium mb-4">
              {pointName}
            </h3>
            <div className="h-[300px]">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={data}>
                  <CartesianGrid
                    strokeDasharray="3 3"
                    stroke="rgba(6, 182, 212, 0.1)"
                    vertical={false}
                  />
                  <XAxis
                    dataKey="timestamp"
                    style={{ fontSize: '0.75rem' }}
                    tick={{ fill: '#06b6d4' }}
                    axisLine={{ stroke: '#0891b2' }}
                    tickFormatter={(value) => {
                      const date = new Date(value)
                      return date.toLocaleTimeString()
                    }}
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
                    labelFormatter={(value) => {
                      const date = new Date(value)
                      return date.toLocaleString()
                    }}
                  />
                  <Line
                    type="monotone"
                    dataKey="value"
                    stroke="#06b6d4"
                    dot={false}
                    strokeWidth={2}
                  />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
