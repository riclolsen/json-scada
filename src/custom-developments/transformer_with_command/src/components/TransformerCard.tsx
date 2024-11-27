import React, { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import * as scadaOpcApi from '../lib/scadaOpcApi'
import { EventsGrid } from './EventsGrid'

interface Measurements {
  hvMw: number
  hvMvar: number
  lvMw: number
  lvMvar: number
}

export function TransformerCard() {
  const [tapPosition, setTapPosition] = useState<number>(0)
  const [measurements, setMeasurements] = useState<Measurements>({
    hvMw: 0,
    hvMvar: 0,
    lvMw: 0,
    lvMvar: 0,
  })
  const [isLoading, setIsLoading] = useState<{
    up: boolean
    down: boolean
  }>({ up: false, down: false })
  const [dataStatus, setDataStatus] = useState<
    'idle' | 'fetching' | 'success' | 'error'
  >('idle')
  const [lastUpdateTime, setLastUpdateTime] = useState<Date | null>(null)

  const formatTime = (date: Date) => {
    return date.toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    })
  }

  const fetchRealtimeData = async () => {
    try {
      setDataStatus('fetching')
      const data = await scadaOpcApi.readRealTimeData([
        'KAW2TR1-2MTWT', // HV MW
        'KAW2TR1-2MTVR', // HV Mvar
        'KAW2TR1-0MTWT', // LV MW
        'KAW2TR1-0MTVR', // LV Mvar
        'KAW2TR1--YTAP', // Tap Position
      ])

      data.forEach((item) => {
        switch (item.name) {
          case 'KAW2TR1-2MTWT':
            setMeasurements((prev) => ({ ...prev, hvMw: item.value }))
            break
          case 'KAW2TR1-2MTVR':
            setMeasurements((prev) => ({ ...prev, hvMvar: item.value }))
            break
          case 'KAW2TR1-0MTWT':
            setMeasurements((prev) => ({ ...prev, lvMw: item.value }))
            break
          case 'KAW2TR1-0MTVR':
            setMeasurements((prev) => ({ ...prev, lvMvar: item.value }))
            break
          case 'KAW2TR1--YTAP':
            setTapPosition(item.value)
            break
        }
      })
      setLastUpdateTime(new Date())
      setDataStatus('success')
    } catch (error) {
      console.error('Error fetching realtime data:', error)
      setDataStatus('error')
    }
  }

  const handleTapChange = async (direction: 'up' | 'down') => {
    setIsLoading((prev) => ({ ...prev, [direction]: true }))
    try {
      await scadaOpcApi.issueCommand(
        'KAW2TR1--YTAP--------K',
        direction === 'up' ? 1 : 0
      )

      // Refresh data after command
      await fetchRealtimeData()
    } catch (error) {
      console.error('Error issuing tap command:', error)
      setDataStatus('error')
    } finally {
      setIsLoading((prev) => ({ ...prev, [direction]: false }))
    }
  }

  useEffect(() => {
    // Initial fetch
    fetchRealtimeData()

    // Set up polling interval (e.g., every 5 seconds)
    const interval = setInterval(fetchRealtimeData, 5000)

    return () => clearInterval(interval)
  }, [])

  return (
    <>
      <Card className="p-4">
        <div className="flex flex-col items-center space-y-4">
          {/* Status Indicator */}
          <div className="absolute top-6 right-6 flex items-center space-x-2">
            <div
              className={`h-3 w-3 rounded-full ${
                dataStatus === 'idle' ? 'bg-gray-400'
                : dataStatus === 'fetching' ? 'bg-blue-400 animate-pulse'
                : dataStatus === 'success' ? 'bg-green-400'
                : 'bg-red-400'
              }`}
            />
            <span className="text-xs text-gray-500">
              {dataStatus === 'fetching' ?
                'Updating...'
              : dataStatus === 'error' ?
                'Error'
              : dataStatus === 'success' && lastUpdateTime ?
                `Updated at ${formatTime(lastUpdateTime)}`
              : 'Idle'}
            </span>
          </div>

          {/* Transformer SVG Drawing */}
          <div className="relative w-[400px] h-[300px]">
            <svg
              width="100%"
              height="100%"
              viewBox="0 0 400 300"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
              className="transform scale-100"
            >
              {/* Background Structure */}
              <rect
                x="30"
                y="20"
                width="340"
                height="280"
                rx="5"
                fill="#f8fafc"
                stroke="#94a3b8"
                strokeWidth="1"
              />

              {/* Substation and Transformer Labels */}
              <text
                x="35"
                y="35"
                className="text-[12px] font-bold"
                fill="#1e293b"
              >
                KAW2
              </text>
              <text
                x="340"
                y="35"
                className="text-[12px] font-bold text-right"
                fill="#1e293b"
                textAnchor="end"
              >
                TR1
              </text>

              {/* HV Line with Insulators and Label */}
              <line
                x1="50"
                y1="40"
                x2="350"
                y2="40"
                stroke="black"
                strokeWidth="3"
              />
              <text
                x="35"
                y="50"
                className="text-[10px] font-semibold"
                fill="#1e293b"
              >
                230kV
              </text>
              {[120, 200, 280].map((x) => (
                <React.Fragment key={x}>
                  <circle cx={x} cy="40" r="4" fill="#e2e8f0" />
                  <circle cx={x} cy="40" r="2" fill="#94a3b8" />
                </React.Fragment>
              ))}

              {/* HV Bushings with Insulators */}
              {[120, 200, 280].map((x) => (
                <React.Fragment key={x}>
                  <line
                    x1={x}
                    y1="40"
                    x2={x}
                    y2="80"
                    stroke="black"
                    strokeWidth="3"
                  />
                  {[50, 60, 70].map((y) => (
                    <React.Fragment key={y}>
                      <line
                        x1={x - 8}
                        y1={y}
                        x2={x + 8}
                        y2={y}
                        stroke="#64748b"
                        strokeWidth="2"
                      />
                      <circle cx={x} cy={y} r="2" fill="#94a3b8" />
                    </React.Fragment>
                  ))}
                </React.Fragment>
              ))}

              {/* Transformer Body with Gradient */}
              <path
                d="M80 80 L320 80 L300 240 L100 240 Z"
                fill="url(#transformerGradient)"
                stroke="black"
                strokeWidth="2"
              />

              {/* Gradient Definition */}
              <defs>
                <linearGradient
                  id="transformerGradient"
                  x1="0%"
                  y1="0%"
                  x2="100%"
                  y2="0%"
                >
                  <stop
                    offset="0%"
                    style={{ stopColor: '#cbd5e1', stopOpacity: 1 }}
                  />
                  <stop
                    offset="50%"
                    style={{ stopColor: '#e2e8f0', stopOpacity: 1 }}
                  />
                  <stop
                    offset="100%"
                    style={{ stopColor: '#cbd5e1', stopOpacity: 1 }}
                  />
                </linearGradient>
              </defs>

              {/* Cooling Fins with Detail */}
              {[90, 105, 120, 280, 295, 310].map((x) => (
                <React.Fragment key={x}>
                  <path
                    d={`M${x} 100 L${x} 220`}
                    stroke="black"
                    strokeWidth="2"
                  />
                  {Array.from({ length: 12 }).map((_, i) => (
                    <line
                      key={i}
                      x1={x - 3}
                      y1={100 + i * 10}
                      x2={x + 3}
                      y2={100 + i * 10}
                      stroke="#94a3b8"
                      strokeWidth="1"
                    />
                  ))}
                </React.Fragment>
              ))}

              {/* Tap Changer with Details */}
              <rect
                x="320"
                y="120"
                width="40"
                height="80"
                fill="#e2e8f0"
                stroke="black"
                strokeWidth="2"
              />
              <rect
                x="325"
                y="130"
                width="30"
                height="60"
                fill="#cbd5e1"
                stroke="#64748b"
                strokeWidth="1"
              />
              <text
                x="327"
                y="150"
                className="text-[10px] font-semibold"
                fill="black"
              >
                TAP
              </text>
              <text
                x="327"
                y="160"
                className="text-[10px] font-semibold"
                fill="black"
              >
                CHANGER
              </text>
              {/* Control wheel */}
              <circle cx="340" cy="180" r="8" fill="#94a3b8" stroke="black" />
              <path
                d="M340 175 L340 185 M335 180 L345 180"
                stroke="white"
                strokeWidth="2"
              />

              {/* LV Bushings with Insulators */}
              {[120, 200, 280].map((x) => (
                <React.Fragment key={x}>
                  <line
                    x1={x}
                    y1="240"
                    x2={x}
                    y2="280"
                    stroke="black"
                    strokeWidth="3"
                  />
                  {[250, 260, 270].map((y) => (
                    <React.Fragment key={y}>
                      <line
                        x1={x - 8}
                        y1={y}
                        x2={x + 8}
                        y2={y}
                        stroke="#64748b"
                        strokeWidth="2"
                      />
                      <circle cx={x} cy={y} r="2" fill="#94a3b8" />
                    </React.Fragment>
                  ))}
                </React.Fragment>
              ))}

              {/* LV Line with Insulators and Label */}
              <line
                x1="50"
                y1="280"
                x2="350"
                y2="280"
                stroke="black"
                strokeWidth="3"
              />
              <text
                x="35"
                y="290"
                className="text-[10px] font-semibold"
                fill="#1e293b"
              >
                69kV
              </text>
              {[120, 200, 280].map((x) => (
                <React.Fragment key={x}>
                  <circle cx={x} cy="280" r="4" fill="#e2e8f0" />
                  <circle cx={x} cy="280" r="2" fill="#94a3b8" />
                </React.Fragment>
              ))}

              {/* HV Measurements Box */}
              <g transform="translate(0, 80)">
                <rect
                  x="0"
                  y="0"
                  width="100"
                  height="70"
                  rx="5"
                  fill="white"
                  stroke="#64748b"
                  strokeWidth="1.5"
                />
                <path d="M0 20 L100 20" stroke="#64748b" strokeWidth="1" />
                <text
                  x="5"
                  y="15"
                  className="text-[10px] font-semibold"
                  fill="#1e293b"
                >
                  230kV SIDE
                </text>
                <text
                  x="10"
                  y="35"
                  className="text-sm font-medium"
                  fill="black"
                >
                  MW: {measurements.hvMw.toFixed(2)}
                </text>
                <text
                  x="10"
                  y="55"
                  className="text-sm font-medium"
                  fill="black"
                >
                  Mvar: {measurements.hvMvar.toFixed(2)}
                </text>
              </g>

              {/* LV Measurements Box */}
              <g transform="translate(0, 170)">
                <rect
                  x="0"
                  y="0"
                  width="100"
                  height="70"
                  rx="5"
                  fill="white"
                  stroke="#64748b"
                  strokeWidth="1.5"
                />
                <path d="M0 20 L100 20" stroke="#64748b" strokeWidth="1" />
                <text
                  x="5"
                  y="15"
                  className="text-[10px] font-semibold"
                  fill="#1e293b"
                >
                  69kV SIDE
                </text>
                <text
                  x="10"
                  y="35"
                  className="text-sm font-medium"
                  fill="black"
                >
                  MW: {measurements.lvMw.toFixed(2)}
                </text>
                <text
                  x="10"
                  y="55"
                  className="text-sm font-medium"
                  fill="black"
                >
                  Mvar: {measurements.lvMvar.toFixed(2)}
                </text>
              </g>

              {/* Oil Level Indicator */}
              <rect
                x="300"
                y="100"
                width="15"
                height="40"
                fill="white"
                stroke="#64748b"
                strokeWidth="1"
              />
              <rect
                x="303"
                y="105"
                width="9"
                height="30"
                fill="#fde68a"
                stroke="#64748b"
                strokeWidth="1"
              />
              <text x="290" y="95" className="text-[8px]" fill="black">
                OIL
              </text>

              {/* Radiator Details */}
              <path
                d="M135 130 L265 130 L265 190 L135 190 Z"
                fill="none"
                stroke="#64748b"
                strokeWidth="1"
                strokeDasharray="4 2"
              />
              <text x="180" y="165" className="text-[10px]" fill="#64748b">
                RADIATOR
              </text>

              {/* Tap Position Box */}
              <g transform="translate(330, 80)">
                <rect
                  x="0"
                  y="0"
                  width="60"
                  height="30"
                  rx="5"
                  fill="white"
                  stroke="#64748b"
                  strokeWidth="1.5"
                />
                <path d="M0 15 L60 15" stroke="#64748b" strokeWidth="1" />
                <text
                  x="5"
                  y="12"
                  className="text-[10px] font-semibold"
                  fill="#1e293b"
                >
                  TAP POS
                </text>
                <text x="5" y="25" className="text-sm font-medium" fill="black">
                  {Math.round(tapPosition)}
                </text>
              </g>
            </svg>
          </div>

          {/* Tap Control Buttons */}
          <div className="flex space-x-4">
            <Button
              onClick={() => handleTapChange('down')}
              variant="outline"
              disabled={isLoading.down || isLoading.up}
              className={`min-w-[100px] ${isLoading.down ? 'animate-pulse bg-gray-100' : ''}`}
            >
              {isLoading.down ? 'Lowering...' : 'Lower Tap'}
            </Button>
            <Button
              onClick={() => handleTapChange('up')}
              variant="outline"
              disabled={isLoading.up || isLoading.down}
              className={`min-w-[100px] ${isLoading.up ? 'animate-pulse bg-gray-100' : ''}`}
            >
              {isLoading.up ? 'Raising...' : 'Raise Tap'}
            </Button>
          </div>
        </div>
      </Card>
      <div className="mt-4">
        <EventsGrid />
      </div>
    </>
  )
}
