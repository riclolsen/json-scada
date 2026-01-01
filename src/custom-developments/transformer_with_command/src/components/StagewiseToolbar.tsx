import React from 'react'

export function StagewiseToolbarComponent(): JSX.Element {
  return (
    <div className="flex items-center gap-2 mb-4">
      <div className="flex items-center space-x-2">
        <button className="px-3 py-1 rounded bg-blue-600 text-white hover:bg-blue-700">
          Stage 1
        </button>
        <button className="px-3 py-1 rounded bg-blue-600 text-white hover:bg-blue-700">
          Stage 2
        </button>
        <button className="px-3 py-1 rounded bg-blue-600 text-white hover:bg-blue-700">
          Stage 3
        </button>
      </div>
      <div className="ml-4 text-sm text-gray-600">Mode: Manual</div>
    </div>
  )
}

export default StagewiseToolbarComponent
