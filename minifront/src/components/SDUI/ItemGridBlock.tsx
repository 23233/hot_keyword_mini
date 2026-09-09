// ItemGridBlock.tsx
import { View, Text, Image } from '@tarojs/components'
import { BlockItem, BlockAction } from '../../types/sdui'
import { blockClassName } from './style'

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
  const configuredColumns = Number(props.columns || 3)
  const columns = Number.isFinite(configuredColumns) ? Math.min(4, Math.max(1, Math.floor(configuredColumns))) : 3
  const rawItems = props.items || []
  const items: GridItem[] = Array.isArray(rawItems) ? rawItems : []

  const handleItemClick = (item: GridItem, e?: any) => {
    e?.stopPropagation?.()
    const actionToDispatch = item.action || block.action
    if ((actionToDispatch || block.events?.tap) && onAction) {
      onAction(actionToDispatch, { item, actionPayload: item })
    }
  }

  const gridTemplateColumns = `repeat(${columns}, 1fr)`

  return (
    <View className={blockClassName('sdui-item-grid-block', block.style)}>
      {title && (
        <View className="sdui-grid-heading">
          <Text className="sdui-grid-title">{title}</Text>
        </View>
      )}

      <View className="sdui-grid-items" style={{ gridTemplateColumns }}>
        {items.map((item, idx) => (
          <View
            key={item.id || idx}
            onClick={(e) => handleItemClick(item, e)}
            className="sdui-grid-item"
          >
            <View className="sdui-grid-media-wrap">
              {item.image_url && (
                <Image
                  src={item.image_url}
                  mode="aspectFill"
                  className={`sdui-grid-media sdui-grid-media-${columns}`}
                />
              )}
              {item.badge && (
                <View className="sdui-grid-badge">
                  <Text>{item.badge}</Text>
                </View>
              )}
            </View>
            <View className="sdui-grid-copy">
              <Text className="sdui-grid-item-title">
                {item.title}
              </Text>
              {item.subtitle && (
                <Text className="sdui-grid-item-subtitle">
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
