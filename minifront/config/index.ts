// index.ts
import { defineConfig, type UserConfigExport } from '@tarojs/cli'
import path from 'path'
import fs from 'fs'

// 智能加载 .env / .env.production 中的环境变量配置
let resolvedApiBaseUrl = process.env.TARO_APP_API_BASE_URL || ''
if (!resolvedApiBaseUrl) {
  try {
    const envFileName = process.env.NODE_ENV === 'development' ? '.env' : '.env.production'
    const envFilePath = path.resolve(__dirname, '..', envFileName)
    const fallbackPath = path.resolve(__dirname, '..', '.env')
    const targetPath = fs.existsSync(envFilePath) ? envFilePath : (fs.existsSync(fallbackPath) ? fallbackPath : '')
    if (targetPath) {
      const content = fs.readFileSync(targetPath, 'utf-8')
      for (const line of content.split('\n')) {
        const trimmed = line.trim()
        if (trimmed.startsWith('TARO_APP_API_BASE_URL=')) {
          resolvedApiBaseUrl = trimmed.substring('TARO_APP_API_BASE_URL='.length).trim()
          break
        }
      }
    }
  } catch (e) {
    // ignore
  }
}

if (process.env.NODE_ENV !== 'development' && !resolvedApiBaseUrl) {
  throw new Error('生产/体验构建必须注入 TARO_APP_API_BASE_URL')
}

export default defineConfig(async (merge) => {
  const baseConfig: UserConfigExport = {
    projectName: 'hot-keyword-mini',
    date: '2026-9-3',
    designWidth: 750,
    deviceRatio: {
      640: 2.34 / 2,
      750: 1,
      375: 2,
      828: 1.81 / 2
    },
    sourceRoot: 'src',
    outputRoot: 'dist',
    plugins: ['@tarojs/plugin-framework-react'],
    defineConstants: {
      'process.env.TARO_APP_API_BASE_URL': JSON.stringify(resolvedApiBaseUrl),
      'process.env.NODE_ENV': JSON.stringify(process.env.NODE_ENV || 'production')
    },
    copy: {
      patterns: [],
      options: {}
    },
    framework: 'react',
    compiler: 'webpack5',
    cache: {
      enable: false
    },
    alias: {
      '@': path.resolve(__dirname, '..', 'src')
    },
    mini: {
      postcss: {
        pxtransform: {
          enable: true,
          config: {}
        },
        url: {
          enable: true,
          config: {
            limit: 1024
          }
        },
        cssModules: {
          enable: false,
          config: {
            namingPattern: 'module',
            generateScopedName: '[name]__[local]___[hash:base64:5]'
          }
        }
      }
    }
  }

  if (process.env.NODE_ENV === 'development') {
    return merge({}, baseConfig, require('./dev').default)
  }
  return merge({}, baseConfig, require('./prod').default)
})
