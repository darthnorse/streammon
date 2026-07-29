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

// Both heatmap packages are personal-scope forks from one maintainer, published
// 2024-07-04 and untouched since. Pin them exactly so a hijacked release cannot
// arrive via a routine `npm i`. The fork declares its own simpleheat dep with a
// caret, so the direct pin alone would leave that one floating — hence the override.
const PINNED_FORKS = ['@pauloak_/react-leaflet-heatmap-layer', '@pauloak_/simpleheat']
const EXACT_VERSION = /^\d+\.\d+\.\d+$/

interface WebManifest {
  dependencies?: Record<string, string>
  overrides?: Record<string, string>
}

describe('frontend dependency resolution', () => {
  it.each(INSTALL_ENTRYPOINTS)('%s installs without legacy peer resolution', (entrypoint) => {
    const source = joinContinuations(readFileSync(resolve(repoDir, entrypoint), 'utf8'))

    expect(source).toMatch(INSTALL_COMMAND)
    // Checked against the whole file rather than matched command lines: an install
    // written any other way must not be able to smuggle the flag past this guard.
    expect(source).not.toContain('--legacy-peer-deps')
  })

  it.each(PINNED_FORKS)('%s is pinned to an exact version', (name) => {
    const manifest: WebManifest = JSON.parse(
      readFileSync(resolve(repoDir, 'web', 'package.json'), 'utf8')
    )
    const declared = manifest.dependencies?.[name] ?? manifest.overrides?.[name]

    expect(declared).toMatch(EXACT_VERSION)
  })
})
