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
import { type DataPoint, readRealTimeData } from '../lib/scadaOpcApi'

export function RealTimeBarGraph() {
  const [data, setData] = useState<DataPoint[]>([])
  const [error, setError] = useState<Error | null>(null)

  useEffect(() => {
    const fetchData = async () => {
      try {
        const result = await readRealTimeData([
          'KAW2TR1-2MTWT',
          'KAW2TR1-2MTVR',
          'KAW2TR1-0MTWT',
          'KAW2TR1-0MTVR',
          'KAW2TR1--YTAP',
        ])
        setData(result)
      } catch (err) {
        setError(err as Error)
      }
    }

    // Initial fetch
    fetchData()

    // Set up interval for periodic updates
    const interval = setInterval(fetchData, 1000)

    // Cleanup interval on component unmount
    return () => clearInterval(interval)
  }, [])

  if (error) return <div>Failed to load data: {error.message}</div>
  if (!data.length) return <div>Loading...</div>

  return (
    <div className="w-full h-[400px] p-4">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="name" style={{ fontSize: '0.75rem' }} interval={0} />
          <YAxis />
          <Tooltip />
          <Bar dataKey="value" fill="#3b82f6" />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
