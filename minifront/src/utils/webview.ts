// webview.ts
import { setUrlQueryParams } from './url'

/** 将当前微信字体设置与主题作为受控参数传给 WebView，供目标网页自行适配。 */
export function buildWebViewDisplayURL(target: string, options: { fontSizeSetting?: number; theme?: string }): string {
  const setting = Number(options.fontSizeSetting || 16)
  const safeSetting = Number.isFinite(setting) ? setting : 16
  const scale = Math.min(1.35, Math.max(1, safeSetting / 16))
  return setUrlQueryParams(target, {
    sdui_client: 'weapp',
    sdui_font_size: String(safeSetting),
    sdui_font_scale: scale.toFixed(2),
    sdui_theme: options.theme || ''
  })
}
