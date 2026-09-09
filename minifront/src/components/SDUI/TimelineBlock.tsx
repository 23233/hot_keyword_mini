// TimelineBlock.tsx
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { blockClassName } from './style'

interface TimelineBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
  context?: any
}

interface TimelineNode {
  time: string
  title: string
  content?: string
  tag?: string
  is_active?: boolean
  action?: BlockAction
}

/**
 * 苹果 HIG 规范时间线流转积木 (TimelineBlock)
 * 支持热点吃瓜始末、剧情脉络、剧集更新历史、活动里程碑与单节点交互
 */
export const TimelineBlock: React.FC<TimelineBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '剧情始末'
  const rawNodes = props.nodes || props.items || []
  const nodes: TimelineNode[] = Array.isArray(rawNodes) ? rawNodes : []

  const handleNodeClick = (node: TimelineNode, index: number, e: any) => {
    if (node.action && onAction) {
      e.stopPropagation?.()
      onAction(node.action, { item: node, node, index, actionPayload: node, title: node.title, time: node.time })
    }
  }

  return (
    <View
      className={blockClassName('sdui-timeline-block', block.style)}
      onClick={() => {
        if ((block.action || block.events?.tap) && onAction) onAction(block.action)
      }}
      style={block.style?.border_radius ? { borderRadius: block.style.border_radius } : undefined}
    >
      {title && (
        <View className="sdui-timeline-heading">
          <Text className="sdui-timeline-title">{title}</Text>
        </View>
      )}

      <View className="sdui-timeline-nodes">
        {nodes.map((node, index) => {
          const isLast = index === nodes.length - 1
          return (
            <View
              key={index}
              onClick={(e) => handleNodeClick(node, index, e)}
              className={`sdui-timeline-node ${node.action ? 'is-actionable' : ''}`}
            >
              {/* 左侧垂直线与时间圆点 */}
              <View className="sdui-timeline-rail">
                <View className={`sdui-timeline-dot ${node.is_active ? 'is-active' : ''}`} />
                {!isLast && (
                  <View className="sdui-timeline-line" />
                )}
              </View>

              {/* 右侧节点信息 */}
              <View className={`sdui-timeline-copy ${isLast ? 'is-last' : ''}`}>
                <View className="sdui-timeline-meta">
                  <Text className="sdui-timeline-time">{node.time}</Text>
                  {node.tag && (
                    <View className="sdui-timeline-tag">
                      <Text>{node.tag}</Text>
                    </View>
                  )}
                </View>
                <Text className="sdui-timeline-node-title">
                  {node.title}
                </Text>
                {node.content && (
                  <Text className="sdui-timeline-node-content">
                    {node.content}
                  </Text>
                )}
              </View>
            </View>
          )
        })}
      </View>
    </View>
  )
}
