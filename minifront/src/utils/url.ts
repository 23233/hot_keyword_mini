// url.ts

/** 在目标路径或查询串中设置参数，保留原 query/hash 并覆盖同名键。 */
export function setUrlQueryParams(target: string, params: Record<string, string>): string {
  const [beforeHash, hash = ''] = target.split('#')
  const [pathPart, rawQuery = ''] = beforeHash.split('?')
  const entries = new Map<string, string>()
  rawQuery.split('&').filter(Boolean).forEach((pair) => {
    const [rawKey, ...rest] = pair.split('=')
    entries.set(decodeURIComponent(rawKey), rest.join('='))
  })
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && String(value) !== '') {
      entries.set(key, encodeURIComponent(String(value)))
    }
  })
  const query = Array.from(entries.entries())
    .map(([key, value]) => `${encodeURIComponent(key)}=${value}`)
    .join('&')
  return `${pathPart}${query ? `?${query}` : ''}${hash ? `#${hash}` : ''}`
}


/** 在纯查询串（不带 ? 前缀）中设置参数，供朋友圈分享 query 使用。 */
export function setQueryStringParams(query: string, params: Record<string, string>): string {
  const normalized = query.startsWith('?') ? query.slice(1) : query
  return setUrlQueryParams(`?${normalized}`, params).replace(/^\?/, '')
}
