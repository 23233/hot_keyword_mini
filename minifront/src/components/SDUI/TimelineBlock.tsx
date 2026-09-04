// TimelineBlock.tsx
import { View, Text } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface TimelineBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction) => void
  context?: any
}

interface TimelineNode {
  time: string
  title: string
  content?: string
  tag?: string
  is_active?: boolean
}

/**
 * 苹果 HIG 规范时间线流转积木 (TimelineBlock)
 * 支持热点吃瓜始末、剧情脉络、剧集更新历史、活动里程碑
 */
export const TimelineBlock: React.FC<TimelineBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || '剧情始末'
  const rawNodes = props.nodes || props.items || []
  const nodes: TimelineNode[] = Array.isArray(rawNodes) ? rawNodes : []

  return (
    <View
      className="sdui-timeline-block"
      onClick={() => {
        if ((block.action || block.events?.tap) && onAction) onAction(block.action)
      }}
      style={{
        borderRadius: block.style?.border_radius || '28rpx',
        padding: '28rpx',
        background: 'rgba(255, 255, 255, 0.05)',
        backdropFilter: 'blur(20px)',
        border: '1px solid rgba(255, 255, 255, 0.08)'
      }}
    >
      {title && (
        <View style={{ marginBottom: '24rpx' }}>
          <Text style={{ fontSize: '30rpx', fontWeight: 600, color: '#fff' }}>{title}</Text>
        </View>
      )}

      <View style={{ display: 'flex', flexDirection: 'column', gap: '24rpx' }}>
        {nodes.map((node, index) => {
          const isLast = index === nodes.length - 1
          return (
            <View key={index} style={{ display: 'flex', gap: '20rpx', position: 'relative' }}>
              {/* 左侧垂直线与时间圆点 */}
              <View style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', width: '32rpx' }}>
                <View
                  style={{
                    width: '18rpx',
                    height: '18rpx',
                    borderRadius: '50%',
                    background: node.is_active ? '#FF9F0A' : 'rgba(255, 255, 255, 0.4)',
                    boxShadow: node.is_active ? '0 0 12rpx rgba(255, 159, 10, 0.8)' : 'none',
                    marginTop: '8rpx'
                  }}
                />
                {!isLast && (
                  <View
                    style={{
                      flex: 1,
                      width: '2rpx',
                      background: 'rgba(255, 255, 255, 0.1)',
                      margin: '8rpx 0'
                    }}
                  />
                )}
              </View>

              {/* 右侧节点信息 */}
              <View style={{ flex: 1, paddingBottom: isLast ? '0' : '16rpx' }}>
                <View style={{ display: 'flex', alignItems: 'center', gap: '12rpx' }}>
                  <Text style={{ fontSize: '22rpx', color: 'rgba(255, 255, 255, 0.45)' }}>{node.time}</Text>
                  {node.tag && (
                    <View
                      style={{
                        padding: '2rpx 10rpx',
                        borderRadius: '8rpx',
                        background: 'rgba(255, 159, 10, 0.15)',
                        border: '1px solid rgba(255, 159, 10, 0.3)'
                      }}
                    >
                      <Text style={{ fontSize: '18rpx', color: '#FF9F0A' }}>{node.tag}</Text>
                    </View>
                  )}
                </View>
                <Text style={{ fontSize: '26rpx', fontWeight: 600, color: '#fff', marginTop: '6rpx', display: 'block' }}>
                  {node.title}
                </Text>
                {node.content && (
                  <Text style={{ fontSize: '22rpx', color: 'rgba(255, 255, 255, 0.6)', marginTop: '6rpx', display: 'block', lineHeight: 1.5 }}>
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
