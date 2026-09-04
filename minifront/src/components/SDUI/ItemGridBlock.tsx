// ItemGridBlock.tsx
import { View, Text, Image } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'

interface ItemGridBlockProps {
  block: BlockItem
  onAction?: (action?: BlockAction, extraContext?: Record<string, any>) => void
  context?: any
}

interface GridItem {
  id?: string | number
  title: string
  subtitle?: string
  image_url?: string
  badge?: string
  action?: BlockAction
}

/**
 * 苹果 HIG 规范自适应多列网格积木 (ItemGridBlock)
 * 支持 2/3/4 列自适应、画廊卡片、剧照海报、热门榜单
 */
export const ItemGridBlock: React.FC<ItemGridBlockProps> = ({ block, onAction }) => {
  const props = block.props || {}
  const title = props.title || ''
  const columns = Number(props.columns || 3)
  const rawItems = props.items || []
  const items: GridItem[] = Array.isArray(rawItems) ? rawItems : []

  const handleItemClick = (item: GridItem) => {
    const actionToDispatch = item.action || block.action
    if (actionToDispatch && onAction) {
      onAction(actionToDispatch, { item })
    }
  }

  const gridTemplateColumns = `repeat(${columns}, 1fr)`

  return (
    <View
      className="sdui-item-grid-block"
      style={{
        borderRadius: block.style?.border_radius || '28rpx',
        padding: '28rpx',
        background: 'rgba(255, 255, 255, 0.05)',
        backdropFilter: 'blur(20px)',
        border: '1px solid rgba(255, 255, 255, 0.08)'
      }}
    >
      {title && (
        <View style={{ marginBottom: '20rpx' }}>
          <Text style={{ fontSize: '30rpx', fontWeight: 600, color: '#fff' }}>{title}</Text>
        </View>
      )}

      <View
        style={{
          display: 'grid',
          gridTemplateColumns,
          gap: '16rpx'
        }}
      >
        {items.map((item, idx) => (
          <View
            key={item.id || idx}
            onClick={() => handleItemClick(item)}
            style={{
              background: 'rgba(255, 255, 255, 0.04)',
              borderRadius: '20rpx',
              overflow: 'hidden',
              border: '1px solid rgba(255, 255, 255, 0.06)',
              display: 'flex',
              flexDirection: 'column'
            }}
          >
            {item.image_url && (
              <Image
                src={item.image_url}
                mode="aspectFill"
                style={{ width: '100%', height: columns === 2 ? '220rpx' : '160rpx' }}
              />
            )}
            <View style={{ padding: '16rpx' }}>
              <Text
                style={{
                  fontSize: '24rpx',
                  fontWeight: 600,
                  color: '#fff',
                  display: '-webkit-box',
                  WebkitLineClamp: 1,
                  WebkitBoxOrient: 'vertical',
                  overflow: 'hidden'
                }}
              >
                {item.title}
              </Text>
              {item.subtitle && (
                <Text
                  style={{
                    fontSize: '20rpx',
                    color: 'rgba(255, 255, 255, 0.45)',
                    marginTop: '4rpx',
                    display: 'block'
                  }}
                >
                  {item.subtitle}
                </Text>
              )}
            </View>
          </View>
        ))}
      </View>
    </View>
  )
}
