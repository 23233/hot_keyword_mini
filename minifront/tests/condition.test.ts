// condition.test.ts
import assert from 'node:assert/strict'
import test from 'node:test'
import { evaluateCondition } from '../src/utils/condition.ts'

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
