// copy-agent.mjs
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const projectDir = path.resolve(scriptDir, '..')
const sourceDir = path.join(projectDir, 'src', 'agent')
const targetDir = path.join(projectDir, 'dist', 'agent')

fs.rmSync(targetDir, { recursive: true, force: true })
fs.cpSync(sourceDir, targetDir, { recursive: true })
