import { useState, useEffect } from 'react'
import { StationList } from './StationList'
import { RealTimeBarGraph } from './RealTimeBarGraph'
import { RealTimeArcGauge } from './RealTimeArcGauge'
import { HistoricalDataPlot } from './HistoricalDataPlot'

const STORAGE_KEY = 'selectedGraphPoints'

export function DataVisualization() {
  const [selectedPoints, setSelectedPoints] = useState<string[]>(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored ? JSON.parse(stored) : []
  })

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(selectedPoints))
  }, [selectedPoints])

  const handlePointSelect = (pointName: string) => {
    if (!selectedPoints.includes(pointName)) {
      setSelectedPoints((prev) => [...prev, pointName])
    }
  }

  const handlePointRemove = (pointName: string) => {
    setSelectedPoints((prev) => prev.filter((p) => p !== pointName))
  }

  return (
    <div className="flex h-screen">
      <StationList
        selectedPoints={selectedPoints}
        onPointSelect={handlePointSelect}
      />
      <main className="flex-1 p-4 overflow-auto bg-gray-900">
        <h1 className="text-3xl font-bold mb-4 text-cyan-400">
          Real-Time Data Visualization
        </h1>
        <div className="space-y-6">
          <div className="bg-gray-800/40 backdrop-blur-md rounded-xl border border-cyan-500/20 shadow-lg">
            <h2 className="text-xl font-semibold mb-4 p-4 text-cyan-400">
              Bar Graph View
            </h2>
            <RealTimeBarGraph
              selectedPoints={selectedPoints}
              onRemovePoint={handlePointRemove}
            />
          </div>
          <div className="bg-gray-800/40 backdrop-blur-md rounded-xl border border-cyan-500/20 shadow-lg">
            <h2 className="text-xl font-semibold mb-4 p-4 text-cyan-400">
              Gauge View
            </h2>
            <RealTimeArcGauge
              selectedPoints={selectedPoints}
              onRemovePoint={handlePointRemove}
            />
          </div>
          <div className="bg-gray-800/40 backdrop-blur-md rounded-xl border border-cyan-500/20 shadow-lg">
            <h2 className="text-xl font-semibold mb-4 p-4 text-cyan-400">
              Historical Data View
            </h2>
            <HistoricalDataPlot
              selectedPoints={selectedPoints}
              onRemovePoint={handlePointRemove}
            />
          </div>
        </div>
      </main>
    </div>
  )
}
