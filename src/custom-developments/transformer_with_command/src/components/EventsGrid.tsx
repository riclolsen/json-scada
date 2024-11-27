import { useState, useEffect } from 'react'
import { Card } from '@/components/ui/card'
import * as scadaOpcApi from '../lib/scadaOpcApi'

export function EventsGrid() {  
  const [events, setEvents] = useState<scadaOpcApi.SoeData[]>([])

  const fetchEvents = async () => {
    try {
      const timeEnd = new Date()
      const timeBegin = new Date(timeEnd.getTime() - 24 * 60 * 60 * 1000) // Last 24 hours
      const data = await scadaOpcApi.getSoeData(
        ['KAW2'], // Filter by station for events
        true, // Use source time
        0, // No aggregation
        50, // Limit to 50 events
        timeBegin,
        null
      )
      // Map SoeData to Event type
      const mappedEvents = data
        .filter((soeData) => soeData.description.includes('TR1'))
        .map((soeData) => ({
          ...soeData,
          value: soeData.valueString, // Add value property
        }))
      setEvents(mappedEvents)
    } catch (error) {
      console.error('Error fetching events:', error)
    }
  }

  useEffect(() => {
    fetchEvents()
    const interval = setInterval(fetchEvents, 10000) // Update every 10 seconds
    return () => clearInterval(interval)
  }, [])

  return (
    <Card className="p-4">
      <h3 className="text-lg font-semibold mb-4">Recent Events</h3>
      <div className="overflow-auto max-h-[300px]">
        <table className="min-w-full text-xs font-mono">
          <thead>
            <tr className="border-b">
              <th className="text-left p-1">Time</th>
              <th className="text-left p-1">Description</th>
              <th className="text-left p-1">Value</th>
              <th className="text-left p-1">Priority</th>
            </tr>
          </thead>
          <tbody>
            {events.map((event, index) => (
              <tr
                key={event.eventId || index}
                className="border-b hover:bg-gray-50"
              >
                <td className="p-1 whitespace-nowrap">
                  {event.sourceTimestamp.toLocaleString()}
                </td>
                <td className="p-1">{event.description}</td>
                <td className="p-1 whitespace-nowrap">{event.valueString}</td>
                <td className="p-1">{event.priority}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  )
}
