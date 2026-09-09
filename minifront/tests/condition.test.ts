// condition.test.ts
import assert from 'node:assert/strict'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { evaluateCondition } from '../src/utils/condition.ts'
import { blockClassName, utilityClasses, SDUI_STYLE_TOKENS } from '../src/components/SDUI/style.ts'

const context = {
  state: {
    enabled: true,
    score: 88,
    role: 'member',
    tags: ['new', 'verified']
  }
}

test('SDUI 条件简写与数组语法保持同一结果', () => {
  const cases = [
    { condition: { path: '$state.enabled', eq: true }, expected: true },
    { condition: { path: '$state.enabled', eq: false }, expected: false },
    { condition: { path: '$state.role', neq: 'guest' }, expected: true },
    { condition: { path: '$state.role', exists: true }, expected: true },
    { condition: { path: '$state.missing', exists: false }, expected: true },
    { condition: { path: '$state.role', in: ['guest', 'member'] }, expected: true },
    { condition: { path: '$state.score', gt: 60 }, expected: true },
    { condition: { path: '$state.score', gte: 88 }, expected: true },
    { condition: { path: '$state.score', lt: 100 }, expected: true },
    { condition: { path: '$state.score', lte: 88 }, expected: true },
    { condition: { eq: [{ path: '$state.enabled' }, true] }, expected: true }
  ]

  for (const { condition, expected } of cases) {
    assert.equal(evaluateCondition(condition, context), expected, JSON.stringify(condition))
  }
})

test('SDUI 工具令牌去重并保持安全类名', () => {
  assert.equal(utilityClasses(['space/y-6', 'space/y-6', 'radius/lg', 'bad token']), 'u-space-y-6 u-radius-lg')
  assert.equal(blockClassName('sdui-card', { utilities: ['layout/card'] }), 'sdui-card u-layout-card')
})

test('Schema、前端令牌与 CSS 工具类全量一致', () => {
  const schema = JSON.parse(readFileSync(new URL('../../schema/sdui.schema.json', import.meta.url), 'utf8'))
  const tokens = schema.definitions.BlockItem.properties.style.properties.utilities.items.enum
  assert.deepEqual([...SDUI_STYLE_TOKENS].sort(), [...tokens].sort())
  const css = readFileSync(new URL('../src/components/SDUI/sdui.scss', import.meta.url), 'utf8')
  for (const token of tokens) assert.ok(css.includes(`.u-${token.replace('/', '-')}`), `缺少 CSS: ${token}`)
})
