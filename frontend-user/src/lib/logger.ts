type Level = 'debug' | 'info' | 'warn' | 'error'

const rank: Record<Level, number> = { debug: 10, info: 20, warn: 30, error: 40 }
const current: Level = import.meta.env.DEV ? 'debug' : 'info'

function emit(level: Level, msg: string, extra?: Record<string, unknown>) {
  if (rank[level] < rank[current]) return
  const rec = { t: new Date().toISOString(), level, msg, ...extra }
  if (level === 'error') console.error(JSON.stringify(rec))
  else if (level === 'warn') console.warn(JSON.stringify(rec))
  else console.info(JSON.stringify(rec))
}

export const log = {
  debug: (m: string, e?: Record<string, unknown>) => emit('debug', m, e),
  info: (m: string, e?: Record<string, unknown>) => emit('info', m, e),
  warn: (m: string, e?: Record<string, unknown>) => emit('warn', m, e),
  error: (m: string, e?: Record<string, unknown>) => emit('error', m, e),
}
