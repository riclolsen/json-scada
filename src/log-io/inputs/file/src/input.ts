import { watch } from 'chokidar'
import fs from 'fs'
import { Socket } from 'net'
import picomatch from 'picomatch'
import { promisify } from 'util'
import {
  FileInputConfig,
  FileSizeMap,
  InputConfig,
  WatcherOptions,
} from './types'

// Normalize path separators to forward slashes so glob matching behaves
// consistently across platforms (chokidar reports native, e.g. Windows, paths).
function normalizePath(p: string): string {
  return p.replace(/\\/g, '/')
}

const openAsync = promisify(fs.open)
const readAsync = promisify(fs.read)
const statAsync = promisify(fs.stat)

const fds: {[filePath: string]: number} = {}

/**
 * Reads new lines from file on disk and sends them to the server
 */
async function sendNewMessages(
  client: Socket,
  streamName: string,
  sourceName: string,
  filePath: string,
  newSize: number,
  oldSize: number,
): Promise<void> {
  let fd = fds[filePath]
  if (!fd) {
    fd = await openAsync(filePath, 'r')
    fds[filePath] = fd
  }
  const offset = Math.max(newSize - oldSize, 0)
  const readBuffer = Buffer.alloc(offset)
  await readAsync(fd, readBuffer, 0, offset, oldSize)
  const messages = readBuffer.toString().split('\r\n').filter((msg) => !!msg.trim())
  messages.forEach((message) => {
    client.write(`+msg|${streamName}|${sourceName}|${message}\0`)
  })
}

/**
 * Sends an input registration to server
 */
async function sendInput(
  client: Socket,
  input: FileInputConfig,
): Promise<void> {
  client.write(`+input|${input.stream}|${input.source}\0`)
}

/**
 * Initializes file watcher for the provided path
 */
async function startFileWatcher(
  client: Socket,
  streamName: string,
  sourceName: string,
  inputPath: string,
  watcherOptions: WatcherOptions,
): Promise<void> {
  const fileSizes: FileSizeMap = {}
  // chokidar v4 removed built-in glob support, so expand globs ourselves: watch
  // the static base directory and filter matched files with picomatch. This
  // keeps the documented glob feature (e.g. "/var/log/**/*.log") working,
  // including picking up newly created files that match the pattern.
  const { ignored: userIgnored, ...restOptions } = watcherOptions
  const normalizedPattern = normalizePath(inputPath)
  const scan = picomatch.scan(normalizedPattern)
  const isGlob = scan.isGlob
  const matchesPattern = isGlob ? picomatch(normalizedPattern, { dot: true }) : null
  // `basename: true` lets slash-less ignore patterns (e.g. "*.txt") match by
  // file name anywhere in the tree, matching chokidar v3's documented behavior.
  const matchesUserIgnored = userIgnored
    ? picomatch(userIgnored, { dot: true, basename: true })
    : null
  const watchTarget = isGlob ? (scan.base || '.') : inputPath
  // chokidar v4's `ignored` accepts a (path, stats) => boolean predicate.
  // Never prune directories so the watcher keeps descending into them.
  const ignored = (filePath: string, stats?: fs.Stats): boolean => {
    if (!stats || stats.isDirectory()) return false
    const normalized = normalizePath(filePath)
    if (matchesUserIgnored && matchesUserIgnored(normalized)) return true
    if (matchesPattern && !matchesPattern(normalized)) return true
    return false
  }
  const watcher = watch(watchTarget, { ...restOptions, ignored })
  // Capture byte size of a new file
  watcher.on('add', async (filePath: string) => {

    console.log(`[${streamName}][${sourceName}] Watching: ${filePath}`)
    fileSizes[filePath] = (await statAsync(filePath)).size
  })
  // Send new lines when a file is changed
  watcher.on('change', async (filePath: string) => {
    try {
      const newSize = (await statAsync(filePath)).size
      await sendNewMessages(
        client,
        streamName,
        sourceName,
        filePath,
        newSize,
        fileSizes[filePath],
      )
      fileSizes[filePath] = newSize
    } catch (err) {

      console.error(err)
    }
  })
  // If a file is removed (or moved), delete its file descriptor & size
  watcher.on('unlink', (filePath: string) => {
    delete fileSizes[filePath]
    delete fds[filePath]
  })
}

/**
 * Async sleep helper
 */
async function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms))
}

/**
 * Start file input process
 */
async function main(config: InputConfig): Promise<void> {
  const { messageServer, inputs } = config
  const serverStr = `${messageServer.host}:${messageServer.port}`
  const client = new Socket()
  let lastConnectionAttempt = new Date().getTime()
  // Register new inputs w/ server
  client.on('connect', async () => {

    console.log(`Connected to server: ${serverStr}`)
    await Promise.all(inputs.map(async (input) => {
      sendInput(client, input)
    }))
  })
  // Reconnect to server if an error occurs while sending a message
  client.on('error', async () => {
    const currTime = new Date().getTime()
    if (currTime - lastConnectionAttempt > 5000) {
      lastConnectionAttempt = new Date().getTime()

      console.error(`Unable to connect to server (${serverStr}), retrying...`)
      await sleep(5000)
      client.connect(messageServer.port, messageServer.host)
    }
  })
  // Connect to server & start watching files for changes
  client.connect(messageServer.port, messageServer.host)
  await Promise.all(inputs.map(async (input) => (
    startFileWatcher(
      client,
      input.stream,
      input.source,
      input.config.path,
      input.config.watcherOptions || {},
    )
  )))
}

export default main
