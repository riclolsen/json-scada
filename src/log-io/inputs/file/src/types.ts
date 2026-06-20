export type FileSizeMap = { [path: string]: number }

// Subset of chokidar v4 watch options that can be supplied via JSON config.
// (chokidar v4 dropped the `disableGlobbing` option along with built-in glob
// support; globs are expanded by the file input itself - see input.ts.)
export type WatcherOptions = {
  persistent?: boolean,
  ignored?: string | string[],
  ignoreInitial?: boolean,
  followSymlinks?: boolean,
  cwd?: string,
  usePolling?: boolean,
  interval?: number,
  binaryInterval?: number,
  alwaysStat?: boolean,
  depth?: number,
  awaitWriteFinish?: boolean | {
    stabilityThreshold?: number,
    pollInterval?: number
  },
  ignorePermissionErrors?: boolean,
  atomic?: boolean | number
}

export type FileInputConfig = {
  source: string,
  stream: string,
  config: {
    path: string,
    watcherOptions?: WatcherOptions,
  },
}

export type InputConfig = {
  messageServer: {
    host: string,
    port: number,
  },
  inputs: Array<FileInputConfig>,
}
