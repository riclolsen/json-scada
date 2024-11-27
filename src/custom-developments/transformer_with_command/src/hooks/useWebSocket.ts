import { useState, useEffect } from 'react'

export function useWebSocket() {
  const [lastMessage, setLastMessage] = useState<string | null>(null)
  const [socket, setSocket] = useState<WebSocket | null>(null)

  useEffect(() => {
    // Replace with your WebSocket server URL
    const ws = new WebSocket('ws://localhost:8080')

    ws.onopen = () => {
      console.log('WebSocket Connected')
      // Subscribe to specific tags
      ws.send(
        JSON.stringify({
          action: 'subscribe',
          tags: ['KAW2TR1-2MTWT', 'KAW2TR1-2MTVR'],
        })
      )
    }

    ws.onmessage = (event) => {
      setLastMessage(event.data)
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }

    ws.onclose = () => {
      console.log('WebSocket disconnected')
    }

    setSocket(ws)

    return () => {
      ws.close()
    }
  }, [])

  return { lastMessage, socket }
}
