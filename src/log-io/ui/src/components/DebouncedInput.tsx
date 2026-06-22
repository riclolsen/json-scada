import React, { useEffect, useRef, useState } from 'react'

interface DebouncedInputProps {
  minLength?: number,
  debounceTimeout?: number,
  placeholder?: string,
  onChange: (event: { target: { value: string } }) => void,
}

/**
 * A minimal debounced text input. Replaces the unmaintained
 * `react-debounce-input` package. Notifies `onChange` after `debounceTimeout`
 * ms, but only once the value is empty or reaches `minLength` characters.
 */
const DebouncedInput: React.FC<DebouncedInputProps> = ({
  minLength = 0,
  debounceTimeout = 100,
  placeholder,
  onChange,
}) => {
  const [value, setValue] = useState('')
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
    const next = e.target.value
    setValue(next)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => {
      if (next.length === 0 || next.length >= minLength) {
        onChange({ target: { value: next } })
      }
    }, debounceTimeout)
  }

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current)
  }, [])

  return (
    <input
      type="text"
      value={value}
      placeholder={placeholder}
      onChange={handleChange}
    />
  )
}

export default DebouncedInput
