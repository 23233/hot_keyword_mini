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

const appId = process.env.WECHAT_AGENT_APPID || process.env.MINIPROGRAM_APPID
if (appId) {
  const projectConfigPath = path.join(projectDir, 'dist', 'project.config.json')
  const projectConfig = JSON.parse(fs.readFileSync(projectConfigPath, 'utf8'))
  projectConfig.appid = appId
  fs.writeFileSync(projectConfigPath, `${JSON.stringify(projectConfig, null, 2)}\n`)
}
