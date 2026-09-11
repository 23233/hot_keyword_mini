// DomainBlocks.tsx
import React, { useState } from 'react'
import { Button, Input, Text, Textarea, View } from '@tarojs/components'
import { BlockAction, BlockItem } from '../../types/sdui'

interface DomainBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
}

const actionFor = (block: BlockItem, fallbackType: BlockAction['type'], payload: Record<string, any> = {}): BlockAction => {
  if (!block.action) return { type: fallbackType, payload }
  if (Object.keys(payload).length === 0) return block.action
  return { ...block.action, payload: { ...(block.action.payload || {}), ...payload } }
}

const emit = (block: BlockItem, onAction: DomainBlockProps['onAction'], action: BlockAction, payload: Record<string, any> = {}) => {
  onAction?.(action, { item: block.props || {}, actionPayload: payload })
}

const listOf = (props: Record<string, any>, key = 'items'): any[] => Array.isArray(props[key]) ? props[key] : []

/** 通用消息列表，消息结构由协议字段映射提供。 */
export const MessageListBlock: React.FC<DomainBlockProps> = ({ block }) => {
  const props = block.props || {}
  const messages = listOf(props, 'messages').length > 0 ? listOf(props, 'messages') : listOf(props)
  return <View className='sdui-message-list'>
    {messages.length === 0 && <Text className='domain-empty-text'>{String(props.empty_text || '暂无消息')}</Text>}
    {messages.map((message: any, index: number) => {
      const mine = message.mine === true || message.role === 'self' || message.sender === 'me'
      return <View key={String(message.id || index)} className={`sdui-message-row ${mine ? 'is-mine' : ''}`}>
        <View className='sdui-message-bubble'>
          {message.sender_name && <Text className='sdui-message-sender'>{String(message.sender_name)}</Text>}
          <Text className='sdui-message-content'>{String(message.content || message.text || '')}</Text>
          {message.time && <Text className='sdui-message-time'>{String(message.time)}</Text>}
        </View>
      </View>
    })}
  </View>
}

/** 通用消息输入框，发送内容通过标准 send_message Action 提交。 */
export const MessageComposerBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const [value, setValue] = useState('')
  const send = () => {
    const content = value.trim()
    if (!content) return
    const action = actionFor({ ...block, action: props.send_action || block.action }, 'send_message', { content })
    emit(block, onAction, action, { ...props, content })
    setValue('')
  }
  return <View className='sdui-message-composer'>
    <Input className='sdui-message-input' value={value} onInput={(event) => setValue(event.detail.value)} placeholder={String(props.placeholder || '输入消息')} />
    <Button className='domain-primary-button' onClick={send}>{String(props.send_text || '发送')}</Button>
  </View>
}

/** 通用客服/聊天线程，组合消息列表、未读摘要和消息输入。 */
export const ChatThreadBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const refreshAction = props.refresh_action || actionFor(block, 'poll_messages', { thread_id: props.thread_id })
  return <View className='sdui-domain-card sdui-chat-thread'>
    <View className='domain-heading'><View><Text className='domain-title'>{String(props.title || props.peer_name || '在线客服')}</Text>{props.status && <Text className='domain-meta'>{String(props.status)}</Text>}</View><Button className='domain-inline-button' onClick={() => emit(block, onAction, refreshAction, props)}>刷新</Button></View>
    {Number(props.unread_count || 0) > 0 && <Text className='domain-unread'>未读 {Number(props.unread_count)}</Text>}
    <MessageListBlock block={{ ...block, props: { ...props, messages: props.messages || props.items } }} />
    <MessageComposerBlock block={{ ...block, action: undefined, props: { ...props, send_action: props.send_action || { type: 'send_message', payload: { thread_id: props.thread_id } } } }} onAction={onAction} />
  </View>
}

/** 独立未读数徽标。 */
export const UnreadBadgeBlock: React.FC<DomainBlockProps> = ({ block }) => {
  const count = Number(block.props?.count || block.props?.unread_count || 0)
  if (count <= 0 && block.props?.hide_zero !== false) return null
  return <View className='sdui-unread-badge'><Text>{count > 99 ? '99+' : count}</Text></View>
}

const StatusRow: React.FC<{ label: string; value: any }> = ({ label, value }) => <View className='domain-status-row'><Text className='domain-label'>{label}</Text><Text className='domain-value'>{String(value ?? '--')}</Text></View>

/** 通用订单摘要卡片。 */
export const OrderCardBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const status = String(props.status || 'pending')
  return <View className='sdui-domain-card sdui-order-card'>
    <View className='domain-heading'><Text className='domain-title'>{String(props.title || '订单')}</Text><Text className={`domain-status status-${status}`}>{String(props.status_text || status)}</Text></View>
    {props.order_no && <StatusRow label='订单号' value={props.order_no} />}
    {props.amount !== undefined && <StatusRow label='金额' value={props.amount} />}
    {props.updated_at && <StatusRow label='更新时间' value={props.updated_at} />}
    {props.action_text && <Button className='domain-primary-button' onClick={() => emit(block, onAction, actionFor(block, 'confirm_order', { order_id: props.order_id || props.id }), props)}>{String(props.action_text)}</Button>}
  </View>
}

/** 通用订单金额与操作摘要。 */
export const OrderSummaryBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  return <View className='sdui-domain-card sdui-order-summary'><Text className='domain-title'>{String(props.title || '订单摘要')}</Text><View className='domain-total-row'><Text>合计</Text><Text className='domain-total'>{String(props.total || props.amount || '¥0.00')}</Text></View>{props.action_text && <Button className='domain-primary-button' onClick={() => emit(block, onAction, actionFor(block, 'create_order', props), props)}>{String(props.action_text)}</Button>}</View>
}

/** 通用订单状态时间线。 */
export const OrderTimelineBlock: React.FC<DomainBlockProps> = ({ block }) => {
  const props = block.props || {}
  const nodes = listOf(props, 'nodes').length > 0 ? listOf(props, 'nodes') : listOf(props, 'timeline')
  return <View className='sdui-domain-card sdui-order-timeline'><Text className='domain-title'>{String(props.title || '订单进度')}</Text>{nodes.map((node: any, index: number) => <View key={String(node.id || index)} className={`domain-timeline-node ${node.active ? 'is-active' : ''}`}><View className='domain-timeline-dot' /><View className='domain-timeline-copy'><Text className='domain-timeline-title'>{String(node.title || node.status || '')}</Text>{node.time && <Text className='domain-meta'>{String(node.time)}</Text>}{node.content && <Text className='domain-description'>{String(node.content)}</Text>}</View></View>)}</View>
}

/** 通用物流轨迹卡片。 */
export const LogisticsTrackBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const tracks = listOf(props, 'tracks').length > 0 ? listOf(props, 'tracks') : listOf(props, 'items')
  return <View className='sdui-domain-card sdui-logistics-track'><View className='domain-heading'><Text className='domain-title'>{String(props.title || '物流轨迹')}</Text><Button className='domain-inline-button' onClick={() => emit(block, onAction, actionFor(block, 'refresh_logistics', { order_id: props.order_id }), props)}>刷新</Button></View>{props.company && <Text className='domain-meta'>{String(props.company)} {String(props.tracking_no || '')}</Text>}{tracks.map((track: any, index: number) => <View key={String(track.id || index)} className='domain-track-row'><Text className='domain-track-time'>{String(track.time || '')}</Text><View><Text className='domain-track-status'>{String(track.status || track.title || '')}</Text>{track.location && <Text className='domain-meta'>{String(track.location)}</Text>}</View></View>)}</View>
}

/** 通用售后申请表单，提交由受控 Action 负责。 */
export const AfterSaleFormBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const [reason, setReason] = useState(String(props.reason || ''))
  const submit = () => emit(block, onAction, actionFor(block, 'apply_after_sale', { order_id: props.order_id, reason }), { ...props, reason })
  return <View className='sdui-domain-card sdui-after-sale-form'><Text className='domain-title'>{String(props.title || '申请售后')}</Text><Textarea className='domain-textarea' value={reason} onInput={(event) => setReason(event.detail.value)} placeholder={String(props.placeholder || '请填写申请原因')} maxlength={500} /><Button className='domain-primary-button' onClick={submit}>{String(props.submit_text || '提交申请')}</Button></View>
}

/** 通用证据附件列表。 */
export const EvidenceListBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const items = listOf(props, 'evidence').length > 0 ? listOf(props, 'evidence') : listOf(props)
  return <View className='sdui-domain-card sdui-evidence-list'><Text className='domain-title'>{String(props.title || '凭证材料')}</Text>{items.map((item: any, index: number) => <View key={String(item.id || index)} className='domain-evidence-row'><Text>{String(item.name || item.title || `材料 ${index + 1}`)}</Text>{item.status && <Text className='domain-meta'>{String(item.status)}</Text>}</View>)}{props.allow_upload && <Button className='domain-secondary-button' onClick={() => emit(block, onAction, actionFor(block, 'upload_evidence', { order_id: props.order_id }), props)}>添加材料</Button>}</View>
}

/** 通用服务/斗师资料卡片。 */
export const ServiceCardBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  return <View className='sdui-domain-card sdui-service-card'><View className='domain-heading'><Text className='domain-title'>{String(props.title || props.name || '服务项目')}</Text>{props.status && <Text className='domain-status'>{String(props.status)}</Text>}</View>{props.description && <Text className='domain-description'>{String(props.description)}</Text>}{props.price && <Text className='domain-price'>{String(props.price)}</Text>}{props.action_text && <Button className='domain-primary-button' onClick={() => emit(block, onAction, actionFor(block, 'accept_task', { task_id: props.task_id || props.id }), props)}>{String(props.action_text)}</Button>}</View>
}

/** 通用任务接单卡片。 */
export const TaskCardBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => <ServiceCardBlock block={{ ...block, props: { ...(block.props || {}), title: block.props?.title || block.props?.task_title, action_text: block.props?.action_text || '接受任务' } }} onAction={onAction} />

/** 通用报价卡片。 */
export const QuoteCardBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => <View className='sdui-domain-card sdui-quote-card'><Text className='domain-title'>{String(block.props?.title || '服务报价')}</Text><StatusRow label='报价' value={block.props?.price || block.props?.amount || '--'} /><StatusRow label='有效期' value={block.props?.valid_until || '--'} />{block.props?.action_text && <Button className='domain-primary-button' onClick={() => emit(block, onAction, actionFor(block, 'submit_quote', block.props || {}), block.props || {})}>{String(block.props.action_text)}</Button>}</View>

/** 通用排期选择器，使用原生日期输入能力。 */
export const SchedulePickerBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const [value, setValue] = useState(String(props.value || ''))
  return <View className='sdui-domain-card sdui-schedule-picker'><Text className='domain-title'>{String(props.title || '选择服务时间')}</Text><Input className='domain-input' type='text' value={value} onInput={(event) => setValue(event.detail.value)} placeholder={String(props.placeholder || '例如：2026-09-12 14:00')} /><Button className='domain-primary-button' onClick={() => emit(block, onAction, actionFor(block, 'update_service_status', { schedule: value }), { ...props, schedule: value })}>{String(props.confirm_text || '确认时间')}</Button></View>
}

/** 通用会员权益卡片。 */
export const MembershipCardBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  return <View className='sdui-domain-card sdui-membership-card'><View className='domain-heading'><Text className='domain-title'>{String(props.title || props.level_name || '会员权益')}</Text><Text className='domain-status'>{String(props.status || 'inactive')}</Text></View>{props.expire_at && <StatusRow label='有效期至' value={props.expire_at} />}{Array.isArray(props.benefits) && <View className='domain-chip-list'>{props.benefits.map((benefit: any, index: number) => <Text key={index} className='domain-chip'>{String(benefit)}</Text>)}</View>}{props.action_text && <Button className='domain-primary-button' onClick={() => emit(block, onAction, actionFor(block, 'open_membership', props), props)}>{String(props.action_text)}</Button>}</View>
}

/** 通用受控广告位，占位内容由广告策略和状态决定。 */
export const AdSlotBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  return <View className={`sdui-domain-card sdui-ad-slot ${props.disabled ? 'is-disabled' : ''}`} onClick={() => !props.disabled && emit(block, onAction, actionFor(block, 'load_ad', { slot_id: props.slot_id }), props)}><Text className='domain-ad-label'>{String(props.label || '广告')}</Text><Text className='domain-title'>{String(props.title || '内容推荐')}</Text>{props.description && <Text className='domain-meta'>{String(props.description)}</Text>}</View>
}

/** 通用钱包余额卡片。 */
export const WalletCardBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => <View className='sdui-domain-card sdui-wallet-card'><Text className='domain-meta'>{String(block.props?.title || '可用余额')}</Text><Text className='domain-wallet-amount'>{String(block.props?.balance || '¥0.00')}</Text>{block.props?.action_text && <Button className='domain-primary-button' onClick={() => emit(block, onAction, actionFor(block, 'refresh_wallet', block.props || {}), block.props || {})}>{String(block.props.action_text)}</Button>}</View>

/** 通用提现表单，金额校验和审核由后端负责。 */
export const WithdrawFormBlock: React.FC<DomainBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const [amount, setAmount] = useState('')
  return <View className='sdui-domain-card sdui-withdraw-form'><Text className='domain-title'>{String(props.title || '申请提现')}</Text><Input className='domain-input' type='number' value={amount} onInput={(event) => setAmount(event.detail.value)} placeholder={String(props.placeholder || '输入提现金额')} /><Button className='domain-primary-button' onClick={() => emit(block, onAction, actionFor(block, 'request_withdraw', { amount }), { ...props, amount })}>{String(props.submit_text || '提交提现申请')}</Button></View>
}
