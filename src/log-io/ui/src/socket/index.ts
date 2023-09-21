import { Dispatch } from 'react'
import socketIO from 'socket.io-client'
import { Socket } from 'socket.io-client'
import { InputActions } from '../reducers/inputs/types'
import { MessageActions } from '../reducers/messages/types'
import { ActionTypes } from '../reducers/types'

import { MessageEvent, PingEvent, RegistrationEvent } from './types'

/**
 * Creates a new socket.io connection to the server
 */
export const createSocket = (): Socket =>
  socketIO()

/**
 * Receives a new input event and adds it to relevant state
 */
export const registerNewInput = (
  socket: Socket,
  dispatch: Dispatch<ActionTypes>
): void => {
  socket.on('+input', (input: RegistrationEvent) => {
    const { inputName, stream, source } = input
    dispatch({ type: InputActions.ENSURE_INPUT, inputName, stream, source })
  })
}

export const unregisterNewInput = (socket: Socket): void => {
  socket.off('+input')
}

/**
 * Receives a remove input event and removes it from relevant state
 */
export const registerRemoveInput = (
  socket: Socket,
  dispatch: Dispatch<ActionTypes>
): void => {
  socket.on('-input', (input: RegistrationEvent) => {
    const { inputName, stream, source } = input
    dispatch({ type: InputActions.REMOVE_INPUT, inputName, stream, source })
  })
}

export const unregisterRemoveInput = (socket: Socket): void => {
  socket.off('-input')
}

/**
 * Receives a ping event and updates input state
 */
export const registerPing = (
  socket: Socket,
  dispatch: Dispatch<ActionTypes>
): void => {
  socket.on('+ping', (input: PingEvent) => {
    const { inputName, stream, source } = input
    dispatch({ type: InputActions.ENSURE_INPUT, inputName, stream, source })
    dispatch({ type: InputActions.PING, inputName, stream, source })
  })
}

export const unregisterPing = (socket: Socket): void => {
  socket.off('+ping')
}

/**
 * Receives a new message event and adds it to state
 */
export const registerNewMessage = (
  socket: Socket,
  dispatch: Dispatch<ActionTypes>
): void => {
  socket.on('+msg', (data: MessageEvent) => {
    const { inputName, msg } = data
    dispatch({ type: MessageActions.ADD_MESSAGE, inputName, msg })
  })
}

export const unregisterNewMessage = (socket: Socket): void => {
  socket.off('+msg')
}

export const sendBindInput = (
  socket: Socket,
  inputName: string,
): void => {
  socket.emit('+activate', inputName)
}

export const sendUnbindInput = (
  socket: Socket,
  inputName: string,
): void => {
  socket.emit('-activate', inputName)
}