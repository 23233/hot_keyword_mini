/**
 * SDUI 受控条件表达式求值引擎 (Safe Condition Evaluator)
 * 严格遵循无 eval、受控语法树执行原则，防止任意代码注入
 */

/**
 * 解析受控路径 (如 $entity.is_locked, $query.id, $props.title)
 */
export function resolveValue(val: any, context: Record<string, any> = {}): any {
  if (typeof val === 'string') {
    const trimmed = val.trim()
    if (trimmed.startsWith('$')) {
      const segments = trimmed.split('.')
      let current: any = context
      if (segments[0] === '$query') current = context.query || {}
      else if (segments[0] === '$entity') current = context.entity || {}
      else if (segments[0] === '$item') current = context.item || {}
      else if (segments[0] === '$state') current = context.state || {}
      else if (segments[0] === '$page') current = context.page || {}
      else if (segments[0] === '$session') current = context.session || {}
      else if (segments[0] === '$tenant') current = context.tenant || {}
      for (let i = 1; i < segments.length; i++) {
        if (current === undefined || current === null) return undefined
        current = current[segments[i]]
      }
      return current
    }
  }
  if (val && typeof val === 'object' && typeof val.path === 'string') {
    const pathStr: string = val.path.trim()
    const segments = pathStr.split('.')
    const rootKey = segments[0] // 如 $entity, $query, $session, $props

    let current: any
    if (rootKey === '$query') {
      current = context.query || {}
    } else if (rootKey === '$entity') {
      current = context.entity || {}
    } else if (rootKey === '$session') {
      current = context.session || {}
    } else if (rootKey === '$props') {
      current = context.props || {}
    } else {
      current = context
    }

    for (let i = 1; i < segments.length; i++) {
      if (current === undefined || current === null) return undefined
      current = current[segments[i]]
    }
    return current
  }
  return val
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

  // 5. 等值比较: eq: [A, B]
  if (Array.isArray(cond.eq) && cond.eq.length >= 2) {
    const left = resolveValue(cond.eq[0], context)
    const right = resolveValue(cond.eq[1], context)
    return String(left) === String(right)
  }

  // 6. 不等比较: neq: [A, B]
  if (Array.isArray(cond.neq) && cond.neq.length >= 2) {
    const left = resolveValue(cond.neq[0], context)
    const right = resolveValue(cond.neq[1], context)
    return String(left) !== String(right)
  }

  // 7. 存在性检查: exists: { path: "..." }
  if (cond.exists !== undefined) {
    const val = resolveValue(cond.exists, context)
    return val !== undefined && val !== null && val !== ''
  }

  // 8. 集合包含: in: [Item, Array]
  if (Array.isArray(cond.in) && cond.in.length >= 2) {
    const item = resolveValue(cond.in[0], context)
    const list = resolveValue(cond.in[1], context)
    if (Array.isArray(list)) {
      return list.some((value: any) => String(value) === String(item))
    }
    return false
  }

  // 9. 大于/小于比较: gt, gte, lt, lte
  if (Array.isArray(cond.gt) && cond.gt.length >= 2) {
    return Number(resolveValue(cond.gt[0], context)) > Number(resolveValue(cond.gt[1], context))
  }
  if (Array.isArray(cond.gte) && cond.gte.length >= 2) {
    return Number(resolveValue(cond.gte[0], context)) >= Number(resolveValue(cond.gte[1], context))
  }
  if (Array.isArray(cond.lt) && cond.lt.length >= 2) {
    return Number(resolveValue(cond.lt[0], context)) < Number(resolveValue(cond.lt[1], context))
  }
  if (Array.isArray(cond.lte) && cond.lte.length >= 2) {
    return Number(resolveValue(cond.lte[0], context)) <= Number(resolveValue(cond.lte[1], context))
  }

  return true
}
