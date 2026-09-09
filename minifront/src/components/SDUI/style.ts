// style.ts
import type { BlockStyle } from '../../types/sdui'

// 与后端 protocol_validator.go 保持同一份受控样式契约。
export const SDUI_STYLE_TOKENS = new Set([
  'layout/flat', 'layout/card', 'surface/canvas', 'surface/base', 'surface/raised', 'surface/ink',
  'border/none', 'border/subtle', 'border/strong', 'radius/none', 'radius/sm', 'radius/md', 'radius/lg', 'radius/xl', 'radius/full',
  'space/y-0', 'space/y-2', 'space/y-4', 'space/y-6', 'space/y-8', 'space/y-10', 'space/y-12',
  'padding/none', 'padding/0', 'padding/2', 'padding/4', 'padding/5', 'padding/6', 'padding/8',
  'padding/x-4', 'padding/x-5', 'padding/x-6', 'padding/y-4', 'padding/y-6',
  'gap/2', 'gap/4', 'gap/6', 'gap/8', 'text/display', 'text/section', 'text/muted', 'text/inverse',
  'accent/blue', 'accent/ink', 'accent/amber', 'accent/green', 'elevation/none', 'elevation/sm', 'elevation/md', 'media/rounded'
])

/** 将服务端受控令牌转换为稳定的工具类名，禁止把任意字符串当作 CSS 注入。 */
export function utilityClasses(utilities?: string[]): string {
  return [...new Set(utilities || [])]
    .filter((token) => SDUI_STYLE_TOKENS.has(token))
    .map((token) => `u-${token.replace('/', '-')}`)
    .join(' ')
}

/** 组合积木根节点和服务端工具令牌，保持组件样式可组合。 */
export function blockClassName(base: string, style?: BlockStyle, ...extra: Array<string | false | undefined>): string {
  return [base, utilityClasses(style?.utilities), ...extra].filter(Boolean).join(' ')
}
