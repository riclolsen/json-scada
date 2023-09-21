import InputRegistry from './inputs'
import socketio from 'socket.io'

export type ServerConfig = {
  messageServer: {
    port: number,
    host: string
  },
  httpServer: {
    port: number,
    host: string
  },
  debug?: boolean,
  basicAuth?: {
    realm: string,
    users: {
      [username: string]: string,
    }
  },
}

export type MessageHandlerFunction = (
  config: ServerConfig,
  inputs: InputRegistry,
  io: socketio.Server,
  msgParts: Array<string>
) => Promise<void>

export type MessageHandlers = {
  [messageType: string]: MessageHandlerFunction,
}
