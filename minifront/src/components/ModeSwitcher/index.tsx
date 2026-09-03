import React from 'react'
import { View, Text } from '@tarojs/components'
import { DisplayMode } from '../../types/drama'
import './index.scss'

interface ModeSwitcherProps {
  currentMode: DisplayMode
  onSelectMode: (mode: DisplayMode) => void
}

export const ModeSwitcher: React.FC<ModeSwitcherProps> = ({
  currentMode,
  onSelectMode
}) => {
  const modes: { key: DisplayMode; label: string; tag: string }[] = [
    { key: 'immersive_video', label: '沉浸影音', tag: '视频' },
    { key: 'episode_grid', label: '剧集矩阵', tag: '选集' },
    { key: 'direct_portal', label: '极速直达', tag: '全集' },
    { key: 'gallery_matrix', label: '短剧画廊', tag: '多剧' }
  ]

  return (
    <View className='mode-switcher-wrapper'>
      <View className='apple-segmented-control'>
        {modes.map((item) => {
          const active = currentMode === item.key
          return (
            <View
              key={item.key}
              className={`segmented-item apple-press-feedback ${active ? 'active' : ''}`}
              onClick={() => onSelectMode(item.key)}
            >
              <Text className='item-label'>{item.label}</Text>
              <Text className='item-tag'>{item.tag}</Text>
            </View>
          )
        })}
      </View>
    </View>
  )
}
