// ContentBlocks.tsx
import React, { useEffect, useMemo, useState } from 'react'
import { Image, Input, RichText, ScrollView, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { BlockAction, BlockItem } from '../../types/sdui'
import { resolveRemoteUrl } from '../../config/env'
import { ensureSession } from '../../utils/auth'
import { resolveBindingValue } from '../../utils/action'
import { request } from '../../utils/request'

interface ContentBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, context?: Record<string, any>) => void
  context?: Record<string, any>
}

type RecordValue = Record<string, any>

function readPath(source: any, path?: string): any {
  if (!path) return source
  return String(path).split('.').reduce((value, key) => value == null ? undefined : value[key], source)
}

function fieldValue(item: RecordValue, fields: RecordValue | undefined, name: string, fallback: any = ''): any {
  const path = fields?.[name] || name
  const value = readPath(item, typeof path === 'string' ? path : name)
  return value === undefined || value === null ? fallback : value
}

function itemsFrom(props: RecordValue): RecordValue[] {
  const value = props.items ?? props.items_path
  return Array.isArray(value) ? value : []
}

function markdownHTML(markdown: string): string {
  const escape = (value: string) => value.replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char] || char))
  return String(markdown || '').split(/\r?\n/).filter(Boolean).map((line) => {
    const image = line.match(/^!\[([^\]]*)\]\(([^)]+)\)$/)
    if (image && /^(https?:\/\/|\/)/.test(image[2].trim())) {
      return `<img class="sdui-markdown-image" src="${escape(resolveRemoteUrl(image[2].trim()))}" alt="${escape(image[1])}" />`
    }
    if (line.startsWith('# ')) return `<h1 class="sdui-markdown-heading">${escape(line.slice(2))}</h1>`
    if (line.startsWith('## ')) return `<h2 class="sdui-markdown-heading">${escape(line.slice(3))}</h2>`
    return `<p class="sdui-markdown-paragraph">${escape(line).replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')}</p>`
  }).join('')
}

/** 通用集合导航：字段、选中态和跳转动作均由积木协议配置。 */
export const CollectionNavBlock: React.FC<ContentBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const fields = props.fields || {}
  const items = itemsFrom(props)
  const activeValue = String(props.active_value ?? '')
  const keyField = String(fields.key || 'key')
  const labelField = String(fields.label || 'label')
  return <View className="sdui-collection-nav">
    {props.title && <Text className="sdui-content-section-title">{props.title}</Text>}
    <ScrollView scrollX className="sdui-collection-scroll">
      <View className="sdui-collection-row">
        {items.map((item, index) => {
          const key = String(readPath(item, keyField) ?? index)
          const label = String(readPath(item, labelField) ?? '')
          return <View key={key} className={`sdui-collection-item ${activeValue === key || (!activeValue && index === 0) ? 'is-active' : ''}`} onClick={() => onAction?.(block.action, { item })}><Text>{label}</Text></View>
        })}
      </View>
    </ScrollView>
  </View>
}

/** 通用内容流：显示字段、筛选、排序、布局与动作全部由页面协议决定。 */
export const ContentFeedBlock: React.FC<ContentBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const fields = props.fields || {}
  const visibleItems = useMemo(() => {
    let result = itemsFrom(props)
    const filter = props.filter as RecordValue | undefined
    if (filter?.field && filter.value != null && filter.value !== '' && filter.value !== filter.all_value) {
      result = result.filter((item) => String(readPath(item, filter.field)) === String(filter.value))
    }
    const sort = props.sort as RecordValue | undefined
    if (sort?.field) {
      const direction = sort.direction === 'asc' ? 1 : -1
      result = [...result].sort((left, right) => String(readPath(left, sort.field) || '').localeCompare(String(readPath(right, sort.field) || '')) * direction)
    }
    const limit = Number(props.limit)
    return limit > 0 ? result.slice(0, limit) : result
  }, [props])
  const layout = props.layout === 'feature' ? 'feature' : 'rows'
  const open = (item: RecordValue) => onAction?.(block.action, { item })
  if (layout === 'feature') {
    const item = visibleItems[0]
    if (!item) return null
    const image = fieldValue(item, fields, 'image')
    return <View className="sdui-content-feed sdui-content-feed-feature">
      {props.title && <Text className="sdui-content-section-title">{props.title}</Text>}
      <View className="sdui-content-feature" onClick={() => open(item)}>
        {image && <Image className="sdui-content-feature-image" src={resolveRemoteUrl(String(image))} mode="aspectFill" />}
        <View className="sdui-content-feature-copy">
          {fieldValue(item, fields, 'eyebrow') && <Text className="sdui-content-eyebrow">{fieldValue(item, fields, 'eyebrow')}</Text>}
          <Text className="sdui-content-feature-title">{fieldValue(item, fields, 'title')}</Text>
          {fieldValue(item, fields, 'summary') && <Text className="sdui-content-feature-summary">{fieldValue(item, fields, 'summary')}</Text>}
          {fieldValue(item, fields, 'meta') && <Text className="sdui-content-meta">{fieldValue(item, fields, 'meta')}</Text>}
        </View>
      </View>
    </View>
  }
  return <View className="sdui-content-feed">
    {props.title && <Text className="sdui-content-section-title">{props.title}</Text>}
    {visibleItems.map((item, index) => {
      const key = String(fieldValue(item, fields, 'key', index))
      const image = fieldValue(item, fields, 'image')
      const badge = fieldValue(item, fields, 'badge')
      return <View key={key} className="sdui-content-row" onClick={() => open(item)}>
        <View className="sdui-content-row-copy">
          <View className="sdui-content-row-heading"><Text className="sdui-content-row-title">{fieldValue(item, fields, 'title')}</Text>{badge && <Text className="sdui-content-badge">{badge}</Text>}</View>
          {fieldValue(item, fields, 'summary') && <Text className="sdui-content-row-summary">{fieldValue(item, fields, 'summary')}</Text>}
          {fieldValue(item, fields, 'meta') && <Text className="sdui-content-meta">{fieldValue(item, fields, 'meta')}</Text>}
        </View>
        {image && <Image className="sdui-content-row-image" src={resolveRemoteUrl(String(image))} mode="aspectFill" />}
      </View>
    })}
  </View>
}

/** 通用资源详情：资源 URL、字段映射、试读态和解锁动作均由协议给出。 */
export const ContentDetailBlock: React.FC<ContentBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const fields = props.fields || {}
  const resourceURL = String(props.resource_url || '')
  const [resource, setResource] = useState<RecordValue | null>(null)
  const [error, setError] = useState('')
  useEffect(() => {
    if (!resourceURL) return
    let active = true
    setError('')
    request<RecordValue>({ url: resourceURL }).then((value) => active && setResource(value)).catch((err: Error) => active && setError(err.message || String(props.error_text || '内容加载失败')))
    return () => { active = false }
  }, [resourceURL, props.error_text])
  if (error) return <View className="sdui-content-detail"><Text className="sdui-content-error">{error}</Text></View>
  if (!resource) return <View className="sdui-content-detail"><Text className="sdui-content-loading">{props.loading_text || '正在加载内容'}</Text></View>
  const title = fieldValue(resource, fields, 'title')
  const meta = fieldValue(resource, fields, 'meta')
  const image = fieldValue(resource, fields, 'image')
  const content = fieldValue(resource, fields, 'content')
  const preview = fieldValue(resource, fields, 'preview')
  const readable = Boolean(fieldValue(resource, fields, 'readable'))
  const lockedReason = fieldValue(resource, fields, 'locked_reason')
  const action = props.locked_action as BlockAction | undefined
  const purchaseAction = props.purchase_action as BlockAction | undefined
  const purchaseValue = fieldValue(resource, fields, 'purchase_value')
  const unlockAction = purchaseValue ? purchaseAction : action
  const lockedTitle = fieldValue(resource, fields, 'locked_title') || props.locked_title || '解锁完整内容'
  const unlockLabel = fieldValue(resource, fields, 'unlock_label') || props.locked_action_text || '继续'
  return <View className="sdui-content-detail">
    <Text className="sdui-content-detail-title">{title}</Text>
    {meta && <Text className="sdui-content-detail-meta">{meta}</Text>}
    {image && <Image className="sdui-content-detail-image" src={resolveRemoteUrl(String(image))} mode="aspectFill" />}
    {readable && content ? <RichText nodes={markdownHTML(String(content))} /> : <View>
      {preview && <View className="sdui-content-preview"><Text className="sdui-content-preview-label">{props.preview_label || '试读'}</Text><RichText nodes={markdownHTML(String(preview))} /></View>}
      <View className="sdui-content-lock">
        <Text className="sdui-content-lock-title">{lockedTitle}</Text>
        {lockedReason && <Text className="sdui-content-lock-desc">{lockedReason}</Text>}
        {unlockAction && <View className="sdui-content-lock-action" onClick={() => onAction?.(unlockAction, { resource, item: resource })}><Text>{unlockLabel}</Text></View>}
      </View>
    </View>}
  </View>
}

/** 通用套餐列表：价格和购买 SKU 只读取服务端展示模型，前端不参与业务定价。 */
export const OfferListBlock: React.FC<ContentBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const fields = props.fields || {}
  const action = props.purchase_action as BlockAction | undefined
  return <View className="sdui-offer-list">
    {props.title && <Text className="sdui-content-section-title">{props.title}</Text>}
    {itemsFrom(props).map((item, index) => <View className="sdui-offer-row" key={String(fieldValue(item, fields, 'key', index))}>
      <View className="sdui-offer-copy"><Text className="sdui-offer-title">{fieldValue(item, fields, 'title')}</Text>{fieldValue(item, fields, 'summary') && <Text className="sdui-offer-summary">{fieldValue(item, fields, 'summary')}</Text>}{fieldValue(item, fields, 'caption') && <Text className="sdui-offer-caption">{fieldValue(item, fields, 'caption')}</Text>}</View>
      <View className="sdui-offer-action" onClick={() => onAction?.(action, { item })}><Text className="sdui-offer-price">{fieldValue(item, fields, 'price')}</Text><Text className="sdui-offer-action-label">{props.purchase_text || '开通'}</Text></View>
    </View>)}
  </View>
}

/** 通用两级讨论串：端点、字段和提交字段均由协议配置，二级内容按需请求。 */
export const DiscussionThreadBlock: React.FC<ContentBlockProps> = ({ block, context }) => {
  const props = block.props || {}
  const fields = props.fields || {}
  const listURL = String(props.list_url || '')
  const [comments, setComments] = useState<RecordValue[]>([])
  const [replies, setReplies] = useState<Record<string, RecordValue[]>>({})
  const [input, setInput] = useState('')
  const [target, setTarget] = useState<RecordValue | null>(null)
  const [sending, setSending] = useState(false)
  const load = async () => { if (listURL) setComments(await request<RecordValue[]>({ url: listURL })) }
  useEffect(() => { load().catch(() => setComments([])) }, [listURL])
  const urlFor = (template: any, item: RecordValue) => String(resolveBindingValue(template, { ...context, item }))
  const loadReplies = async (comment: RecordValue) => {
    const id = String(fieldValue(comment, fields, 'key'))
    if (replies[id]) return
    const url = urlFor(props.replies_url, comment)
    if (url) {
      const result = await request<RecordValue[]>({ url })
      setReplies((previous) => ({ ...previous, [id]: result }))
    }
  }
  const submit = async () => {
    const content = input.trim()
    if (!content || sending) return
    if (props.require_auth !== false && !(await ensureSession())) { Taro.showToast({ title: props.login_text || '请先完成微信登录', icon: 'none' }); return }
    setSending(true)
    try {
      const body: RecordValue = { [props.content_field || 'content']: content }
      if (target) body[props.parent_field || 'parent_id'] = fieldValue(target, fields, 'key')
      await request({ url: String(props.submit_url || listURL), method: 'POST', data: body })
      setInput('')
      setTarget(null)
      await load()
      Taro.showToast({ title: props.submitted_text || '已提交', icon: 'none' })
    } catch (err: any) { Taro.showToast({ title: err.message || String(props.submit_error_text || '提交失败'), icon: 'none' }) } finally { setSending(false) }
  }
  return <View className="sdui-discussion-thread">
    {props.title && <Text className="sdui-content-section-title">{props.title}</Text>}
    {comments.map((comment) => {
      const id = String(fieldValue(comment, fields, 'key'))
      const replyCount = Number(fieldValue(comment, fields, 'reply_count', 0))
      return <View key={id} className="sdui-discussion-comment">
        <Text className="sdui-discussion-author">{fieldValue(comment, fields, 'author')}</Text><Text className="sdui-discussion-content">{fieldValue(comment, fields, 'content')}</Text>
        <View className="sdui-discussion-actions"><Text onClick={() => setTarget(comment)}>{props.reply_text || '回复'}</Text>{replyCount > 0 && <Text onClick={() => loadReplies(comment)}>{String(props.replies_text || '查看 {count} 条回复').replace('{count}', String(replyCount))}</Text>}</View>
        {replies[id]?.map((reply, index) => <View key={String(fieldValue(reply, fields, 'key', index))} className="sdui-discussion-reply"><Text>{fieldValue(reply, fields, 'reply_to') ? `${props.mention_prefix || '@'}${fieldValue(reply, fields, 'reply_to')} ` : ''}{fieldValue(reply, fields, 'content')}</Text></View>)}
      </View>
    })}
    <View className="sdui-discussion-input">{target && <Text className="sdui-discussion-target">{props.mention_prefix || '@'}{fieldValue(target, fields, 'author')}</Text>}<Input value={input} onInput={(event) => setInput(event.detail.value)} placeholder={target ? `${props.reply_text || '回复'} ${fieldValue(target, fields, 'author')}` : (props.placeholder || '说点什么')} /><View onClick={submit}><Text>{sending ? (props.sending_text || '提交中') : (props.submit_text || '发送')}</Text></View></View>
  </View>
}
