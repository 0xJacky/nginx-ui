import { fileURLToPath } from 'node:url'
import { spawn, which } from 'bun'
import { expect, test } from 'bun:test'

test.each(['upgradeClient', 'upgradeComponents'])('%s integration', async name => {
  // Bun module mocks are process-wide. Isolate the transport fixture so it
  // cannot replace the user/settings stores used by other unit test files.
  const fixture = fileURLToPath(new URL(`../test-fixtures/${name}.test.ts`, import.meta.url))
  const process = spawn([
    which('bun')!,
    'test',
    fixture,
  ], {
    stdout: 'pipe',
    stderr: 'pipe',
  })
  const timeout = setTimeout(() => process.kill(), 8000)
  try {
    const [exitCode, stdout, stderr] = await Promise.all([
      process.exited,
      new Response(process.stdout).text(),
      new Response(process.stderr).text(),
    ])
    expect({ exitCode, output: stdout + stderr }).toMatchObject({ exitCode: 0 })
  }
  finally {
    clearTimeout(timeout)
  }
}, 10_000)
