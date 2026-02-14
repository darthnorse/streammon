/** Callbacks for processing SSE events from a fetch Response. */
export interface SSEHandlers {
  /** Called for each unnamed `data:` line. */
  onData?: (data: string) => void
  /** Called for named events (e.g. `event: complete` followed by `data:`). */
  onEvent?: (event: string, data: string) => void
}

/**
 * Reads an SSE stream from a fetch Response, parsing `data:` and `event:` lines.
 * Handles buffering, line splitting, and draining any residual buffer on stream end.
 */
export async function readSSEStream(response: Response, handlers: SSEHandlers): Promise<void> {
  const reader = response.body?.getReader()
  if (!reader) throw new Error('No response body')

  const decoder = new TextDecoder()
  let buffer = ''
  let currentEvent: string | null = null

  const processLine = (line: string) => {
    if (line.startsWith(':')) {
      return
    }
    if (line.startsWith('event: ')) {
      currentEvent = line.slice(7)
    } else if (line.startsWith('data: ')) {
      const data = line.slice(6)
      if (currentEvent && handlers.onEvent) {
        handlers.onEvent(currentEvent, data)
      } else if (handlers.onData) {
        handlers.onData(data)
      }
      currentEvent = null
    } else if (line === '') {
      currentEvent = null
    }
  }

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''

    for (const line of lines) {
      processLine(line)
    }
  }

  // Process any remaining data in buffer
  for (const line of buffer.split('\n')) {
    processLine(line)
  }
}
