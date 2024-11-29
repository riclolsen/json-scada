import { useEffect, useState, useCallback } from 'react'
import {
  type DataPoint,
  getGroup1List,
  getRealtimeFilteredData,
} from '../lib/scadaOpcApi'
import { TreeView, TreeItem } from './ui/tree-view'

interface StationData {
  name: string
  points: DataPoint[]
  loading: boolean
}

interface StationListProps {
  selectedPoints: string[]
  onPointSelect: (pointName: string) => void
}

export function StationList({
  selectedPoints,
  onPointSelect,
}: StationListProps) {
  const [stations, setStations] = useState<StationData[]>([])

  useEffect(() => {
    const fetchStations = async () => {
      const stationList = await getGroup1List()
      setStations(
        stationList.map((name) => ({
          name,
          points: [],
          loading: false,
        }))
      )
    }

    fetchStations()
  }, [])

  const handleStationClick = useCallback(
    async (station: StationData) => {
      if (station.points.length > 0) return

      const stationIndex = stations.findIndex((s) => s.name === station.name)
      if (stationIndex === -1) return

      setStations((prev) =>
        prev.map((s, i) => (i === stationIndex ? { ...s, loading: true } : s))
      )

      try {
        const points = await getRealtimeFilteredData(station.name, '', false)
        setStations((prev) =>
          prev.map((s, i) =>
            i === stationIndex ?
              {
                ...s,
                points: points.filter(Boolean),
                loading: false,
              }
            : s
          )
        )
      } catch (error) {
        console.error('Error loading points:', error)
        setStations((prev) =>
          prev.map((s, i) =>
            i === stationIndex ? { ...s, loading: false } : s
          )
        )
      }
    },
    [stations]
  )

  const handlePointClick = useCallback(
    (point: DataPoint) => {
      onPointSelect(point.name)
    },
    [onPointSelect]
  )

  const formatPointValue = useCallback((point: DataPoint) => {
    if (point.valueString) {
      return point.valueString
    }
    return typeof point.value === 'number' ?
        point.value.toFixed(2)
      : point.value
  }, [])

  return (
    <div className="h-screen w-64 bg-gray-800 text-white p-4">
      <h2 className="text-xl font-semibold mb-4">Stations</h2>
      <div className="h-[calc(100vh-8rem)] overflow-y-auto">
        <TreeView>
          {stations.map((station) => (
            <TreeItem
              key={station.name}
              title={station.name}
              onClick={() => handleStationClick(station)}
              loading={station.loading}
            >
              {station.points.map((point) => (
                <TreeItem
                  key={point.name}
                  title={`${point.name} (${formatPointValue(point)})`}
                  onClick={() => handlePointClick(point)}
                  className={
                    selectedPoints.includes(point.name) ?
                      'bg-blue-900 hover:bg-blue-800'
                    : ''
                  }
                />
              ))}
            </TreeItem>
          ))}
        </TreeView>
      </div>
    </div>
  )
}
