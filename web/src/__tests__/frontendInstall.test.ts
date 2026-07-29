import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const repoDir = resolve(dirname(fileURLToPath(import.meta.url)), '../../..')

// Every file that installs frontend dependencies. Adding an install path here is
// cheaper than discovering it drifted.
const INSTALL_ENTRYPOINTS = ['Dockerfile', 'Makefile']

// Joins backslash line continuations so a wrapped `npm ci \` + `--legacy-peer-deps`
// is inspected as the single command it actually is.
function installCommands(source: string): string[] {
  return source
    .replace(/\\\r?\n\s*/g, ' ')
    .split('\n')
    .filter((line) => /npm (ci|install|i)\b/.test(line))
}

describe('frontend dependency resolution', () => {
  it.each(INSTALL_ENTRYPOINTS)('%s installs without legacy peer resolution', (entrypoint) => {
    const commands = installCommands(readFileSync(resolve(repoDir, entrypoint), 'utf8'))

    expect(commands.length).toBeGreaterThan(0)
    for (const command of commands) {
      expect(command).not.toContain('--legacy-peer-deps')
    }
  })
})
