// minifront/src/utils/condition.ts
/**
 * SDUI 受控条件表达式求值引擎 (Safe Condition Evaluator)
 * 严格遵循无 eval、受控语法树执行原则，防止任意代码注入
 */

/**
 * 架构规范中允许的受控数据作用域白名单集合
 */
export const KNOWN_SCOPES = new Set([
  'query', 'entity', 'item', 'state', 'page', 'session', 'tenant', 'props', 'result'
])

/**
 * 判断路径是否以合法受控作用域开头 (如 "$entity.status" 或 "entity.status")
 */
export function isKnownScopedPath(path: string): boolean {
  if (!path || typeof path !== 'string') return false
  const raw = path.startsWith('$') ? path.slice(1) : path
  const root = raw.split('.')[0]
  return KNOWN_SCOPES.has(root)
}

/**
 * 解析受控路径 (如 $entity.is_locked, entity.is_locked, $query.id, $props.title)
 */
export function resolveValue(val: any, context: Record<string, any> = {}): any {
  if (typeof val === 'string') {
    const trimmed = val.trim()
    if (trimmed.startsWith('$') || isKnownScopedPath(trimmed)) {
      return resolveScopedPath(trimmed, context)
    }
  }
  if (val && typeof val === 'object' && typeof val.path === 'string') {
    return resolveScopedPath(val.path.trim(), context)
  }
  return val
}

/** 统一解析所有受控作用域，保证字符串和 {path} 两种协议写法语义一致。 */
function resolveScopedPath(path: string, context: Record<string, any>): any {
  if (!path) return undefined
  const normalizedPath = path.startsWith('$') ? path : `$${path}`
  const segments = normalizedPath.split('.')
  const root = segments[0]
  const rawRoot = root.startsWith('$') ? root.slice(1) : root

  // 双向兼容：支持 context 中无论以 $xxx 还是以无 $ 存储均能精准命中文本
  const scopeVal = context[root] ?? context[rawRoot] ?? context[`$${rawRoot}`]
  if (scopeVal === undefined || scopeVal === null) return undefined

  let current = scopeVal
  for (let i = 1; i < segments.length; i++) {
    if (current === undefined || current === null) return undefined
    current = current[segments[i]]
  }
  return current
}

/** 受控宽松等值比较，规则与服务端 compareLooseEqual 保持一致。 */
function isLooseEqual(a: any, b: any): boolean {
  if (a === b) return true
  if (a == null && b == null) return true
  if (a == null || b == null) return false
  const boolA = parseBoolean(a)
  const boolB = parseBoolean(b)
  if (boolA !== undefined && boolB !== undefined) {
    return boolA === boolB
  }
  const numA = parseNumber(a)
  const numB = parseNumber(b)
  if (numA !== undefined && numB !== undefined) {
    return numA === numB
  }
  return String(a) === String(b)
}

/** 只接受协议定义的布尔值与 true/false 字符串，避免 JavaScript 真值转换歧义。 */
function parseBoolean(value: any): boolean | undefined {
  if (typeof value === 'boolean') return value
  if (typeof value !== 'string') return undefined
  const normalized = value.trim().toLowerCase()
  if (normalized === 'true') return true
  if (normalized === 'false') return false
  return undefined
}

/** 只接受有限数字或非空数字字符串，规则与服务端 strconv.ParseFloat 对齐。 */
function parseNumber(value: any): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value !== 'string' || !value.trim()) return undefined
  const result = Number(value)
  return Number.isFinite(result) ? result : undefined
}

/**
 * 受控条件表达式求值器
 * 支持: and, or, not, eq, neq, in, exists, gt, gte, lt, lte
 */
export function evaluateCondition(cond: any, context: Record<string, any> = {}): boolean {
  if (cond === undefined || cond === null) {
    return true // 未配置条件，默认展示
  }

  // 1. 布尔直接值
  if (typeof cond === 'boolean') {
    return cond
  }

  if (typeof cond !== 'object') {
    return true
  }

  // 简易快捷属性: hide: true
  if (cond.hide === true) {
    return false
  }

  // 2. 逻辑连接符: and
  if (Array.isArray(cond.and)) {
    return cond.and.every((sub: any) => evaluateCondition(sub, context))
  }

  // 3. 逻辑连接符: or
  if (Array.isArray(cond.or)) {
    return cond.or.some((sub: any) => evaluateCondition(sub, context))
  }

  // 4. 逻辑非: not
  if (cond.not !== undefined) {
    return !evaluateCondition(cond.not, context)
  }

  // 5. 属性简写: { path: '$state.enabled', eq: true }。
  // 与服务端 IR 同时兼容，避免预览与小程序的条件渲染分叉。
  if (typeof cond.path === 'string' && cond.path.trim()) {
    const left = resolveValue({ path: cond.path }, context)
    if (cond.eq !== undefined) return isLooseEqual(left, resolveValue(cond.eq, context))
    if (cond.neq !== undefined) return !isLooseEqual(left, resolveValue(cond.neq, context))
    if (cond.exists !== undefined) {
      const exists = left !== undefined && left !== null && left !== ''
      return typeof cond.exists === 'boolean' ? exists === cond.exists : exists
    }
    if (cond.in !== undefined) {
      const values = resolveValue(cond.in, context)
      return Array.isArray(values) && values.some((value: any) => isLooseEqual(value, left))
    }
    for (const operator of ['gt', 'gte', 'lt', 'lte'] as const) {
      if (cond[operator] === undefined) continue
      const right = parseNumber(resolveValue(cond[operator], context))
      const leftNumber = parseNumber(left)
      if (leftNumber === undefined || right === undefined) return false
      if (operator === 'gt') return leftNumber > right
      if (operator === 'gte') return leftNumber >= right
      if (operator === 'lt') return leftNumber < right
      return leftNumber <= right
    }
  }

  // 6. 等值比较: eq: [A, B]
  if (Array.isArray(cond.eq) && cond.eq.length >= 2) {
    const left = resolveValue(cond.eq[0], context)
    const right = resolveValue(cond.eq[1], context)
    return isLooseEqual(left, right)
  }

  // 7. 不等比较: neq: [A, B]
  if (Array.isArray(cond.neq) && cond.neq.length >= 2) {
    const left = resolveValue(cond.neq[0], context)
    const right = resolveValue(cond.neq[1], context)
    return !isLooseEqual(left, right)
  }

  // 8. 存在性检查: exists: { path: "..." }
  if (cond.exists !== undefined) {
    const val = resolveValue(cond.exists, context)
    return val !== undefined && val !== null && val !== ''
  }

  // 9. 集合包含: in: [Item, Array] 或 [Array, Item] (双向容错与类型安全宽松等值比较)
  if (Array.isArray(cond.in) && cond.in.length >= 2) {
    const val0 = resolveValue(cond.in[0], context)
    const val1 = resolveValue(cond.in[1], context)
    if (Array.isArray(val1)) {
      return val1.some((element: any) => isLooseEqual(element, val0))
    }
    if (Array.isArray(val0)) {
      return val0.some((element: any) => isLooseEqual(element, val1))
    }
    return false
  }

  // 10. 大于/小于比较: gt, gte, lt, lte
  if (Array.isArray(cond.gt) && cond.gt.length >= 2) {
    const left = parseNumber(resolveValue(cond.gt[0], context))
    const right = parseNumber(resolveValue(cond.gt[1], context))
    return left !== undefined && right !== undefined && left > right
  }
  if (Array.isArray(cond.gte) && cond.gte.length >= 2) {
    const left = parseNumber(resolveValue(cond.gte[0], context))
    const right = parseNumber(resolveValue(cond.gte[1], context))
    return left !== undefined && right !== undefined && left >= right
  }
  if (Array.isArray(cond.lt) && cond.lt.length >= 2) {
    const left = parseNumber(resolveValue(cond.lt[0], context))
    const right = parseNumber(resolveValue(cond.lt[1], context))
    return left !== undefined && right !== undefined && left < right
  }
  if (Array.isArray(cond.lte) && cond.lte.length >= 2) {
    const left = parseNumber(resolveValue(cond.lte[0], context))
    const right = parseNumber(resolveValue(cond.lte[1], context))
    return left !== undefined && right !== undefined && left <= right
  }

  return true
}
