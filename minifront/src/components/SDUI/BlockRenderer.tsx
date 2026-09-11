// BlockRenderer.tsx
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { MediaHeroBlock } from './MediaHeroBlock'
import { ResourceCardBlock } from './ResourceCardBlock'
import { TextBlock } from './TextBlock'
import { ActionButtonBlock } from './ActionButtonBlock'
import { NoticeBlock } from './NoticeBlock'
import { GameCardBlock } from './GameCardBlock'
import { FormBlock } from './FormBlock'
import { EpisodeListBlock } from './EpisodeListBlock'
import { ItemGridBlock } from './ItemGridBlock'
import { TimelineBlock } from './TimelineBlock'
import { ImageBlock } from './ImageBlock'
import { VideoBlock } from './VideoBlock'
import { ContainerBlock, GridBlock, TabsBlock, CarouselBlock, SpacerBlock } from './LayoutBlocks'
import { EmptyBlock, SkeletonBlock } from './StateBlocks'
import { GenericBlock } from './GenericBlock'
import {
  ScorePanelBlock,
  CouponCardBlock,
  CountdownBlock,
  ServerStatusBlock,
  ProductCardBlock,
  DownloadCardBlock
} from './BusinessBlocks'
import { CollectionNavBlock, ContentFeedBlock, ContentDetailBlock, DiscussionThreadBlock, OfferListBlock } from './ContentBlocks'
import {
  BottomNavBlock,
  ContactServiceBlock,
  FloatingActionBlock,
  GameHeaderBlock,
  MediaGalleryBlock,
  MediaPickerBlock,
  TencentMapBlock,
  UploadProgressBlock,
  WebViewEntryBlock
} from './PlatformBlocks'
import {
  AfterSaleFormBlock,
  AdSlotBlock,
  ChatThreadBlock,
  EvidenceListBlock,
  LogisticsTrackBlock,
  MembershipCardBlock,
  MessageComposerBlock,
  MessageListBlock,
  OrderCardBlock,
  OrderSummaryBlock,
  OrderTimelineBlock,
  QuoteCardBlock,
  SchedulePickerBlock,
  ServiceCardBlock,
  TaskCardBlock,
  UnreadBadgeBlock,
  WalletCardBlock,
  WithdrawFormBlock
} from './DomainBlocks'
import { evaluateCondition } from '../../utils/condition'
import { resolveBindingValue, resolveBlockPropsBindings } from '../../utils/action'
import { utilityClasses } from './style'
import './sdui.scss'

interface BlockRendererProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
  context?: Record<string, any>
}

/**
 * 动态原子积木调度渲染器 (BlockRenderer)
 * 具备受控条件求值、repeat 列表循环展开、props 数据绑定求值、events 序列调度与未知组件自动优雅降级保护
 */
export const BlockRenderer: React.FC<BlockRendererProps> = ({ block, onAction, context }) => {
  if (!block || !block.type) return null

  // 当前积木独立作用域，供 visible_when、动作参数和子积木绑定使用 (优先接管 _repeat_item 循环展开项)
  const injectedItem = (block.props as any)?._repeat_item
  const injectedIndex = (block.props as any)?._repeat_index
  const rawBlockContext: Record<string, any> = {
    ...context,
    ...(injectedItem !== undefined ? { item: injectedItem, $item: injectedItem, index: injectedIndex } : {}),
    props: block.props || {}
  }
  const resolvedProps = resolveBlockPropsBindings(block.props || {}, rawBlockContext, block.type)
  const blockContext: Record<string, any> = { ...rawBlockContext, props: resolvedProps }

  // 1.1 检查块级局部状态多态 (loading, empty, error，覆盖库存不足、过期、离线等场景并提供优雅兜底)
  const blockState = context?.blockStates?.[block.id] || (block.props as any)?._state
  if (blockState === 'loading') {
    if (block.loading) {
      return <BlockRenderer block={block.loading} onAction={onAction} context={blockContext} />
    }
    return (
      <View className="sdui-block-wrapper is-glass u-space-y-6 u-padding-6">
        <SkeletonBlock block={{ id: `${block.id}_loading`, type: 'skeleton', props: { rows: 2, hero: false } }} />
      </View>
    )
  }
  if (blockState === 'empty') {
    if (block.empty) {
      return <BlockRenderer block={block.empty} onAction={onAction} context={blockContext} />
    }
    return (
      <View className="sdui-block-wrapper is-glass u-space-y-6 u-padding-6">
        <EmptyBlock block={{ id: `${block.id}_empty`, type: 'empty', props: { state: 'empty', title: '暂无数据', desc: '当前卡片内容暂不可用' } }} onAction={onAction} />
      </View>
    )
  }
  if (blockState === 'error' || blockState === 'offline' || blockState === 'out_of_stock' || blockState === 'expired') {
    if (block.error) {
      return <BlockRenderer block={block.error} onAction={onAction} context={blockContext} />
    }
    return (
      <View className="sdui-block-wrapper is-glass u-space-y-6 u-padding-6">
        <EmptyBlock
          block={{
            id: `${block.id}_error`,
            type: 'empty',
            props: {
              state: blockState === 'out_of_stock' ? 'out_of_stock' : (blockState === 'expired' ? 'expired' : 'offline'),
              btn_text: '刷新重试'
            },
            action: { type: 'refresh' }
          }}
          onAction={onAction}
        />
      </View>
    )
  }

  // 2. 支持 repeat 数组循环展开渲染
  if (block.repeat && typeof block.repeat === 'object') {
    let repeatList: any[] = []
    let hasExplicitRepeat = false
    if (Array.isArray(block.repeat.items)) {
      hasExplicitRepeat = true
      repeatList = block.repeat.items
    } else if (block.repeat.path) {
      hasExplicitRepeat = true
      const resolved = resolveBindingValue({ path: block.repeat.path }, blockContext)
      if (Array.isArray(resolved)) {
        repeatList = resolved
      }
    }

    if (repeatList.length > 0) {
      return (
        <View className="sdui-repeat-container">
          {repeatList.map((itemData, idx) => {
            const childContext = { ...blockContext, item: itemData, index: idx }
            const clonedBlock = { ...block, id: `${block.id}_${idx}`, repeat: undefined }
            return (
              <BlockRenderer
                key={`${block.id}_${idx}`}
                block={clonedBlock}
                onAction={onAction}
                context={childContext}
              />
            )
          })}
        </View>
      )
    }

    // 若显式配置了 repeat 但计算出的列表为空，优先渲染 empty 积木，杜绝幽灵未解析积木被渲染
    if (hasExplicitRepeat) {
      if (block.empty) {
        return <BlockRenderer block={block.empty} onAction={onAction} context={blockContext} />
      }
      return null
    }
  }

  // 循环展开后再求值条件，确保 visible_when 可以读取当前 $item。
  if (block.visible_when !== undefined && !evaluateCondition(block.visible_when, blockContext)) {
    return null
  }

  // 3. 事件动作拦截器 (优先区分内部特定子动作与积木主体动作，主体动作优先执行 events 序列，最后回退 block.action)
  const handleWrappedAction = (act?: BlockAction, extraContext?: Record<string, any>) => {
    let finalItem: any = undefined
    const baseItem = blockContext?.item
    const extItem = extraContext?.item
    if (extItem !== undefined && (typeof extItem !== 'object' || extItem === null)) {
      finalItem = extItem
    } else if (baseItem !== undefined && (typeof baseItem !== 'object' || baseItem === null)) {
      finalItem = extItem !== undefined ? extItem : baseItem
    } else if (baseItem || extItem) {
      finalItem = { ...(baseItem || {}), ...(extItem || {}) }
    }

    const mergedContext = extraContext
      ? {
          ...blockContext,
          ...extraContext,
          item: finalItem,
          $item: finalItem
        }
      : {
          ...blockContext,
          item: finalItem ?? blockContext?.item,
          $item: finalItem ?? blockContext?.$item ?? blockContext?.item
        }

    // 若调用方（如卡片内部独立的复制按钮或特定子动作）显式传入了不同于积木主体 action 的独立动作，优先派发该动作
    const isDistinctSubAction = act && act.type && act !== block.action
    if (isDistinctSubAction) {
      onAction?.(act, mergedContext)
      return
    }

    // 积木主体触发时，优先分发 events 动作流序列 (如 events.tap 或 events.submit)
    const eventName = (extraContext && extraContext.__event) || (block.events?.tap ? 'tap' : (block.events?.submit ? 'submit' : (block.events ? Object.keys(block.events)[0] : undefined)))
    if (block.events && eventName && block.events[eventName]) {
      const targetActions = Array.isArray(block.events[eventName]) ? (block.events[eventName] as BlockAction[]) : [block.events[eventName] as BlockAction]
      const enrichedActions = targetActions.map((eventAction) => ({
        ...eventAction,
        payload: {
          ...(eventAction.payload || {}),
          ...((mergedContext as any)?.actionPayload || {}),
          ...((act && act.payload) || {})
        }
      }))
      onAction?.(enrichedActions as any, mergedContext)
    } else if (act && act.type) {
      onAction?.(act, mergedContext)
    } else if (block.action) {
      onAction?.(block.action, mergedContext)
    }
  }

  // 4. 解析外层容器样式与嵌套上下文感知 (防止嵌套子积木多层毛玻璃背景重叠以及间距破坏)
  const isNested = !!context?.isNested
  const isSpacer = block.type === 'spacer'
  const style = block.style || {}

  const flatUtility = style.utilities?.includes('layout/flat')
  const cardUtility = style.utilities?.includes('layout/card')
  const contentBlock = ['collection_nav', 'category_nav', 'content_feed', 'article_feed', 'content_detail', 'article_detail', 'offer_list', 'membership_plan_list', 'discussion_thread', 'comment_thread'].includes(block.type)
  const shouldApplyGlass = !isSpacer && !flatUtility && !cardUtility && (
    style.glass_blur !== undefined
      ? !!style.glass_blur
      : (!isNested && !contentBlock && !['text', 'rich_text', 'image', 'video', 'action_button', 'spacer'].includes(block.type))
  )

  // 嵌套子积木上下外边距默认归零，完全遵循父级容器 (Flex/Grid) 的 gap 布局规范，杜绝双重叠加破坏排版
  const wrapperClass = `sdui-block-wrapper type-${block.type} ${utilityClasses(style.utilities)} ${shouldApplyGlass ? 'is-glass' : ''} ${isNested ? 'is-nested' : ''}`
  const wrapperStyle: any = undefined

  // 5. 递归求值解析积木 props 中的全部受控数据绑定表达式 ($entity.*, $query.*, $item.*, $state.*)
  const resolvedBlock: BlockItem = {
    ...block,
    // 协议样式只在外层 Wrapper 应用一次，内部组件保留自身静态样式。
    style: undefined,
    props: resolvedProps
  }

  // 递归渲染子积木辅助函数 (透传 isNested 嵌套标志)
  const renderChildBlock = (child: BlockItem, childCtx?: Record<string, any>) => {
    return <BlockRenderer block={child} onAction={onAction} context={{ ...blockContext, isNested: true, ...childCtx }} />
  }

  // 6. 按照全套积木注册表匹配对应积木组件
  const renderInner = () => {
    switch (block.type) {
      // 内容块
      case 'image':
        return <ImageBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'text':
      case 'rich_text':
        return <TextBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'video':
        return <VideoBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'empty':
        return <EmptyBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'skeleton':
        return <SkeletonBlock block={resolvedBlock} />

      // 布局块
      case 'container':
      case 'stack':
      case 'list':
        return <ContainerBlock block={resolvedBlock} onAction={handleWrappedAction} context={blockContext} renderBlock={renderChildBlock} />
      case 'grid':
        return <GridBlock block={resolvedBlock} onAction={handleWrappedAction} context={blockContext} renderBlock={renderChildBlock} />
      case 'tabs':
        return <TabsBlock block={resolvedBlock} onAction={handleWrappedAction} context={blockContext} renderBlock={renderChildBlock} />
      case 'carousel':
        return <CarouselBlock block={resolvedBlock} onAction={handleWrappedAction} context={blockContext} renderBlock={renderChildBlock} />
      case 'spacer':
        return <SpacerBlock block={resolvedBlock} />

      // 通用内容消费组件：数据源、字段映射和动作都由协议声明，不绑定任何行业。
      case 'collection_nav':
      case 'category_nav':
        return <CollectionNavBlock block={resolvedBlock} onAction={handleWrappedAction} context={blockContext} />
      case 'content_feed':
      case 'article_feed':
        return <ContentFeedBlock block={resolvedBlock} onAction={handleWrappedAction} context={blockContext} />
      case 'content_detail':
      case 'article_detail':
        return <ContentDetailBlock block={resolvedBlock} onAction={handleWrappedAction} context={blockContext} />
      case 'offer_list':
      case 'membership_plan_list':
        return <OfferListBlock block={resolvedBlock} onAction={handleWrappedAction} context={blockContext} />
      case 'discussion_thread':
      case 'comment_thread':
        return <DiscussionThreadBlock block={resolvedBlock} onAction={handleWrappedAction} context={blockContext} />

      // 经典业务块
      case 'media_hero':
        return <MediaHeroBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'resource_card':
        return <ResourceCardBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'action_button':
        return <ActionButtonBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'notice':
        return <NoticeBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'game_card':
        return <GameCardBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'form':
        return <FormBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'episode_list':
        return <EpisodeListBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'item_grid':
        return <ItemGridBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'timeline':
        return <TimelineBlock block={resolvedBlock} onAction={handleWrappedAction} />

      // 业务专用积木
      case 'score_panel':
        return <ScorePanelBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'coupon_card':
      case 'redeem_code_card':
        return <CouponCardBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'countdown':
        return <CountdownBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'server_status':
        return <ServerStatusBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'product_card':
        return <ProductCardBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'download_card':
        return <DownloadCardBlock block={resolvedBlock} onAction={handleWrappedAction} />

      // 跨小程序复用的平台能力块
      case 'bottom_nav':
        return <BottomNavBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'floating_action':
        return <FloatingActionBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'game_header':
        return <GameHeaderBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'media_picker':
        return <MediaPickerBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'upload_progress':
        return <UploadProgressBlock block={resolvedBlock} />
      case 'media_gallery':
        return <MediaGalleryBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'map_card':
      case 'location_picker':
        return <TencentMapBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'contact_card':
      case 'service_entry':
        return <ContactServiceBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'webview_entry':
      case 'webview_state':
        return <WebViewEntryBlock block={resolvedBlock} onAction={handleWrappedAction} />

      // 通用聊天、交易、履约、会员和资金能力块。业务数据与动作均由协议提供。
      case 'chat_thread':
        return <ChatThreadBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'message_list':
        return <MessageListBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'message_composer':
        return <MessageComposerBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'unread_badge':
        return <UnreadBadgeBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'order_card':
        return <OrderCardBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'order_summary':
        return <OrderSummaryBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'order_timeline':
        return <OrderTimelineBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'logistics_track':
        return <LogisticsTrackBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'after_sale_form':
        return <AfterSaleFormBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'evidence_list':
        return <EvidenceListBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'service_card':
        return <ServiceCardBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'task_card':
        return <TaskCardBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'quote_card':
        return <QuoteCardBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'schedule_picker':
        return <SchedulePickerBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'membership_card':
        return <MembershipCardBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'ad_slot':
        return <AdSlotBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'wallet_card':
        return <WalletCardBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'withdraw_form':
        return <WithdrawFormBlock block={resolvedBlock} onAction={handleWrappedAction} />
      case 'payment_result':
      case 'price_breakdown':
      case 'address_card':
      case 'qr_code':
      case 'feature_gate':
      case 'compliance_panel':
        return <GenericBlock block={resolvedBlock} onAction={handleWrappedAction} />

      // 已纳入协议的业务扩展块使用通用渲染器，确保不会出现空白页
      case 'result_table':
      case 'event_card':
      case 'poll':
      case 'feed_list':
      case 'custom':
      case 'custom_block':
        return <GenericBlock block={resolvedBlock} onAction={handleWrappedAction} />

      default:
        // 7. 未知积木组件降级策略 (优先 fallback，无则优雅占位，杜绝整页白屏崩溃)
        if (block.fallback) {
          return <BlockRenderer block={block.fallback} onAction={onAction} context={blockContext} />
        }
        return (
          <View className="sdui-fallback-block">
            <Text className="fallback-hint">暂未支持的组件类型: {block.type}</Text>
          </View>
        )
    }
  }

  return (
    <View className={wrapperClass} style={wrapperStyle}>
      {renderInner()}
    </View>
  )
}
