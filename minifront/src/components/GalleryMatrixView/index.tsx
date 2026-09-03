import React, { useState } from 'react'
import { View, Text, Image } from '@tarojs/components'
import { DramaInfo } from '../../types/drama'
import './index.scss'

interface GalleryMatrixViewProps {
  dramaList: DramaInfo[]
  onSelectDrama: (drama: DramaInfo) => void
  onOpenActionModal: () => void
}

export const GalleryMatrixView: React.FC<GalleryMatrixViewProps> = ({
  dramaList = [],
  onSelectDrama,
  onOpenActionModal
}) => {
  // 分类筛选
  const [activeCategory, setActiveCategory] = useState<string>('all')

  const categories = [
    { key: 'all', label: '🔥 全部精选' },
    { key: '战神', label: '⚔️ 战神逆袭' },
    { key: '修真', label: '⚡ 修仙无敌' },
    { key: '神豪', label: '💰 都市神豪' }
  ]

  const filteredList = dramaList.filter((item) => {
    if (activeCategory === 'all') return true
    return (item.tags || '').includes(activeCategory)
  })

  return (
    <View className='gallery-matrix-view'>
      {/* 顶部画廊介绍横幅 */}
      <View className='gallery-hero-banner apple-card'>
        <View className='hero-left'>
          <Text className='hero-badge'>剧场精选画廊</Text>
          <Text className='hero-title'>全网热搜短剧集萃</Text>
          <Text className='hero-desc'>涵盖热搜榜单与正版授权爆款好剧</Text>
        </View>
        <View className='hero-action apple-pill-btn' onClick={onOpenActionModal}>
          <Text className='btn-txt'>领取全集资源</Text>
        </View>
      </View>

      {/* 分类切换胶囊栏 */}
      <View className='category-pills-row'>
        {categories.map((c) => (
          <View
            key={c.key}
            className={`cat-pill apple-press-feedback ${activeCategory === c.key ? 'active' : ''}`}
            onClick={() => setActiveCategory(c.key)}
          >
            <Text className='cat-text'>{c.label}</Text>
          </View>
        ))}
      </View>

      {/* 双列海报画廊网格 */}
      <View className='gallery-grid'>
        {filteredList.map((drama) => {
          const mode = drama.play_mode || 'direct_video'
          return (
            <View
              key={drama.id}
              className='gallery-card apple-card apple-press-feedback'
              onClick={() => onSelectDrama(drama)}
            >
              {/* 海报与播放模式角标 */}
              <View className='card-poster-wrap'>
                <Image className='card-poster' src={drama.cover_url} mode='aspectFill' />
                <View className='play-mode-pill'>
                  <Text className='mode-icon'>
                    {mode === 'direct_video' && '🎬 直接播放'}
                    {mode === 'channels_video' && '📺 视频号首播'}
                    {mode === 'web_view' && '🌐 网页专区'}
                    {mode === 'none' && '⏳ 转码中'}
                  </Text>
                </View>
                <View className='rating-tag'>
                  <Text className='rating-val'>★ {drama.rating || '9.8'}</Text>
                </View>
              </View>

              {/* 剧集文字信息 */}
              <View className='card-info'>
                <Text className='card-title'>{drama.title}</Text>
                <Text className='card-subtitle'>{drama.subtitle}</Text>

                <View className='card-footer'>
                  <Text className='ep-total'>全 {drama.total_episodes} 集</Text>
                  <Text className='heat-num'>🔥 {(drama.hot_score / 10000).toFixed(1)}万</Text>
                </View>
              </View>
            </View>
          )
        })}
      </View>
    </View>
  )
}
