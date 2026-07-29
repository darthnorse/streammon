import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const repoDir = resolve(dirname(fileURLToPath(import.meta.url)), '../../..')

// Every file that installs frontend dependencies. Adding an install path here is
// cheaper than discovering it drifted.
const INSTALL_ENTRYPOINTS = ['Dockerfile', 'Makefile']

// Joins backslash line continuations so a wrapped `npm ci \` + `--legacy-peer-deps`
// reads as the single command it actually is.
function joinContinuations(source: string): string {
  return source.replace(/\\\r?\n\s*/g, ' ')
}

const INSTALL_COMMAND = /\bnpm\b[^\n]*\b(ci|install|i)\b/

describe('frontend dependency resolution', () => {
  it.each(INSTALL_ENTRYPOINTS)('%s installs without legacy peer resolution', (entrypoint) => {
    const source = joinContinuations(readFileSync(resolve(repoDir, entrypoint), 'utf8'))

    expect(source).toMatch(INSTALL_COMMAND)
    // Checked against the whole file rather than matched command lines: an install
    // written any other way must not be able to smuggle the flag past this guard.
    expect(source).not.toContain('--legacy-peer-deps')
  })
})
