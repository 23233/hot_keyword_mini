// BlockRenderer.tsx
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { MediaHeroBlock } from './MediaHeroBlock'
import { ResourceCardBlock } from './ResourceCardBlock'
import { ActionButtonBlock } from './ActionButtonBlock'
import { NoticeBlock } from './NoticeBlock'
import { GameCardBlock } from './GameCardBlock'
import { FormBlock } from './FormBlock'
import { EpisodeListBlock } from './EpisodeListBlock'
import { ItemGridBlock } from './ItemGridBlock'
import { TimelineBlock } from './TimelineBlock'
import { ImageBlock } from './ImageBlock'
import { TextBlock } from './TextBlock'
import { VideoBlock } from './VideoBlock'
import { ContainerBlock, GridBlock, TabsBlock, CarouselBlock, SpacerBlock } from './LayoutBlocks'
import { EmptyBlock, SkeletonBlock } from './StateBlocks'
import { GenericBlock } from './GenericBlock'
import { evaluateCondition } from '../../utils/condition'
import { dispatchEvents, resolveBindingValue, resolveBlockPropsBindings } from '../../utils/action'
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

  // 1. 检查受控条件可见性 (visible_when 多运算符受控求值)
  if (block.visible_when !== undefined) {
    const isVisible = evaluateCondition(block.visible_when, context)
    if (!isVisible) {
      return null
    }
  }

  // 1.1 检查块级局部状态多态 (loading, empty, error)
  const blockState = context?.blockStates?.[block.id] || (block.props as any)?._state
  if (blockState === 'loading' && block.loading) {
    return <BlockRenderer block={block.loading} onAction={onAction} context={context} />
  }
  if (blockState === 'empty' && block.empty) {
    return <BlockRenderer block={block.empty} onAction={onAction} context={context} />
  }
  if ((blockState === 'error' || blockState === 'offline' || blockState === 'out_of_stock') && block.error) {
    return <BlockRenderer block={block.error} onAction={onAction} context={context} />
  }

  // 2. 支持 repeat 数组循环展开渲染
  if (block.repeat && typeof block.repeat === 'object') {
    let repeatList: any[] = []
    if (Array.isArray(block.repeat.items)) {
      repeatList = block.repeat.items
    } else if (block.repeat.path) {
      const resolved = resolveBindingValue({ path: block.repeat.path }, context)
      if (Array.isArray(resolved)) {
        repeatList = resolved
      }
    }

    if (repeatList.length > 0) {
      return (
        <View className="sdui-repeat-container">
          {repeatList.map((itemData, idx) => {
            const childContext = { ...context, item: itemData, index: idx }
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
  }

  // 3. 事件动作拦截器 (优先执行 events 序列，其次回退 action，支持透传 item 子上下文)
  const handleWrappedAction = (act?: BlockAction, extraContext?: Record<string, any>) => {
    const mergedContext = extraContext ? { ...context, ...extraContext } : context
    if (block.events && block.events.tap) {
      dispatchEvents(block.events, 'tap', mergedContext)
    } else if (act) {
      onAction?.(act, mergedContext)
    } else if (block.action) {
      onAction?.(block.action, mergedContext)
    }
  }

  // 4. 解析外层容器样式
  const style = block.style || {}
  const wrapperClass = `sdui-block-wrapper ${style.glass_blur !== false ? 'is-glass' : ''}`
  const wrapperStyle: any = {
    marginTop: style.margin_y || undefined,
    marginBottom: style.margin_y || '24rpx',
    padding: style.padding || undefined,
    minHeight: block.props?._layout_height ? `${block.props._layout_height}px` : undefined
  }

  // 5. 递归求值解析积木 props 中的全部受控数据绑定表达式 ($entity.*, $query.*, $item.*, $state.*)
  const resolvedProps = resolveBlockPropsBindings(block.props || {}, context)
  const resolvedBlock: BlockItem = {
    ...block,
    props: resolvedProps
  }

  // 递归渲染子积木辅助函数
  const renderChildBlock = (child: BlockItem, childCtx?: Record<string, any>) => {
    return <BlockRenderer block={child} onAction={onAction} context={childCtx || context} />
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
        return <ContainerBlock block={resolvedBlock} onAction={handleWrappedAction} context={context} renderBlock={renderChildBlock} />
      case 'grid':
        return <GridBlock block={resolvedBlock} onAction={handleWrappedAction} context={context} renderBlock={renderChildBlock} />
      case 'tabs':
        return <TabsBlock block={resolvedBlock} onAction={handleWrappedAction} context={context} renderBlock={renderChildBlock} />
      case 'carousel':
        return <CarouselBlock block={resolvedBlock} onAction={handleWrappedAction} context={context} renderBlock={renderChildBlock} />
      case 'spacer':
        return <SpacerBlock block={resolvedBlock} />
      case 'list':
        return <ContainerBlock block={resolvedBlock} onAction={handleWrappedAction} context={context} renderBlock={renderChildBlock} />

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
        return <TimelineBlock block={resolvedBlock} />

      // 已纳入协议的业务扩展块使用通用渲染器，确保不会出现空白页
      case 'score_panel':
      case 'coupon_card':
      case 'countdown':
      case 'result_table':
      case 'contact_card':
      case 'map_card':
      case 'game_header':
      case 'redeem_code_card':
      case 'server_status':
      case 'product_card':
      case 'download_card':
      case 'event_card':
      case 'poll':
      case 'feed_list':
        return <GenericBlock block={resolvedBlock} onAction={handleWrappedAction} />

      default:
        // 7. 未知积木组件降级策略 (优先 fallback，无则优雅占位，杜绝整页白屏崩溃)
        if (block.fallback) {
          return <BlockRenderer block={block.fallback} onAction={onAction} context={context} />
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
