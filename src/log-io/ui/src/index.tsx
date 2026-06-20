import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './components/app'
import { initializeState } from './reducers'
import { createSocket } from './socket'

import './index.scss'

const Root: React.FC = () => {
  const socket = createSocket()
  const initialState = initializeState(socket)
  return (
    <App
      socket={socket}
      initialState={initialState}
    />
  )
}

const container = document.getElementById('root')
if (!container) {
  throw new Error('Root element #root not found')
}
createRoot(container).render(<Root />)
