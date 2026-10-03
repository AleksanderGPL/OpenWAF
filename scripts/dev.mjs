import { spawn, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const directory = mkdtempSync(join(tmpdir(), 'openwaf-dev-'))
const binary = join(directory, process.platform === 'win32' ? 'openwaf.exe' : 'openwaf')
const children = []
let stopping = false
function stop(code) {
  if (stopping) return
  stopping = true
  for (const child of children) {
    if (!child.pid) continue
    try {
      if (process.platform === 'win32') child.kill()
      else process.kill(-child.pid, 'SIGTERM')
    } catch (error) {
      if (error.code !== 'ESRCH') console.error(error)
    }
  }
  Promise.all(children.map(child => !child.pid || child.exitCode !== null || child.signalCode !== null
    ? Promise.resolve()
    : new Promise(resolve => child.once('exit', resolve))))
    .then(() => {
      rmSync(directory, { recursive: true, force: true })
      process.exit(code)
    })
}
process.on('SIGINT', () => stop(130))
process.on('SIGTERM', () => stop(143))
const build = spawnSync('go', ['build', '-tags=dev', '-o', binary, '.'], {
  cwd: root, stdio: 'inherit'
})
if (build.error || build.status !== 0) {
  if (build.error) console.error(build.error.message)
  rmSync(directory, { recursive: true, force: true })
  process.exit(build.status || 1)
}
function start(command, args, cwd) {
  const child = spawn(command, args, {
    cwd, stdio: 'inherit', detached: process.platform !== 'win32'
  })
  children.push(child)
  child.on('error', error => { console.error(error.message); stop(1) })
  child.on('exit', code => stop(code ?? 1))
}
start(binary, [], root)
start(process.execPath, ['node_modules/nuxt/bin/nuxt.mjs', 'dev', '--host', '127.0.0.1', '--port', '3000'], join(root, 'frontend'))
