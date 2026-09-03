import React, { useState } from 'react'
import { View, Text, Image, ScrollView } from '@tarojs/components'
import { DramaInfo, EpisodeItem } from '../../types/drama'
import './index.scss'

interface EpisodeGridViewProps {
  drama: DramaInfo
  episodes: EpisodeItem[]
  recommendations: DramaInfo[]
  onOpenActionModal: (episodeNum?: number) => void
}

export const EpisodeGridView: React.FC<EpisodeGridViewProps> = ({
  drama,
  episodes = [],
  recommendations = [],
  onOpenActionModal
}) => {
  // 分页 Tab (每30集一组)
  const [activeTab, setActiveTab] = useState(0)

  // 计算 Tab 列表
  const tabs = [
    { label: '1 - 30 集', start: 1, end: 30 },
    { label: '31 - 60 集', start: 31, end: 60 },
    { label: '61 - 80 集', start: 61, end: 80 }
  ]

  const currentTabInfo = tabs[activeTab] || tabs[0]
  const displayedEpisodes = episodes.filter(
    (ep) => ep.episode_num >= currentTabInfo.start && ep.episode_num <= currentTabInfo.end
  )

  const tagsList = (drama.tags || '').split(',').filter(Boolean)

  return (
    <View className='episode-grid-view'>
      {/* 顶部电影海报 Hero 质感大卡片 */}
      <View className='poster-hero-card apple-card'>
        <Image className='poster-img' src={drama.cover_url} mode='aspectFill' />
        <View className='poster-meta'>
          <View className='meta-top'>
            <Text className='drama-title'>{drama.title}</Text>
            {drama.subtitle && <Text className='drama-subtitle'>{drama.subtitle}</Text>}
          </View>

          {tagsList.length > 0 && (
            <View className='meta-tags'>
              {tagsList.map((tag, idx) => (
                <View key={idx} className='apple-tag'>
                  <Text>{tag}</Text>
                </View>
              ))}
            </View>
          )}

          <View className='meta-footer'>
            {drama.rating > 0 && (
              <View className='rating-badge'>
                <Text className='star'>★</Text>
                <Text className='score'>{drama.rating}</Text>
              </View>
            )}
            <Text className='ep-status'>全{drama.total_episodes}集</Text>
          </View>
        </View>
      </View>

      {/* 快捷解锁全集引导横幅 */}
      <View className='fast-unlock-banner apple-card apple-press-feedback' onClick={() => onOpenActionModal()}>
        <View className='banner-left'>
          <Text className='banner-icon'>🎁</Text>
          <View className='banner-text-group'>
            <Text className='banner-title'>点击领取《{drama.title}》完整版全集</Text>
            <Text className='banner-desc'>高清无删减 · 网盘直取 · 无需等待</Text>
          </View>
        </View>
        <View className='banner-btn apple-pill-btn'>
          <Text className='btn-text'>立即获取</Text>
        </View>
      </View>

      {/* 选集网格选择器 */}
      <View className='grid-section apple-card'>
        <View className='section-header'>
          <Text className='section-title'>全集选集列表</Text>
          <Text className='section-tip'>点击剧集直接观看或获取后续</Text>
        </View>

        {/* 选集分组 Tabs */}
        <View className='tabs-row'>
          {tabs.map((tab, idx) => (
            <View
              key={idx}
              className={`tab-btn apple-press-feedback ${activeTab === idx ? 'active' : ''}`}
              onClick={() => setActiveTab(idx)}
            >
              <Text className='tab-text'>{tab.label}</Text>
            </View>
          ))}
        </View>

        {/* 选集五列网格 */}
        <View className='episodes-grid'>
          {displayedEpisodes.map((ep) => (
            <View
              key={ep.id || ep.episode_num}
              className={`grid-item apple-press-feedback ${ep.is_free ? 'free' : 'locked'}`}
              onClick={() => {
                if (ep.is_free) {
                  onOpenActionModal(ep.episode_num)
                } else {
                  onOpenActionModal(ep.episode_num)
                }
              }}
            >
              <Text className='item-num'>{ep.episode_num}</Text>
              <View className='item-status'>
                <Text className='status-text'>{ep.is_free ? '免费' : '专享'}</Text>
              </View>
            </View>
          ))}
        </View>
      </View>

      {/* 同类爆款推荐短剧 */}
      {recommendations.length > 0 && (
        <View className='recom-section apple-card'>
          <View className='recom-header'>
            <Text className='recom-title'>🔥 剧迷还在看</Text>
          </View>
          <ScrollView className='recom-scroll' scrollX enableFlex showScrollbar={false}>
            <View className='recom-list'>
              {recommendations.map((item) => (
                <View
                  key={item.id}
                  className='recom-item apple-press-feedback'
                  onClick={() => onOpenActionModal()}
                >
                  <Image className='recom-cover' src={item.cover_url} mode='aspectFill' />
                  <Text className='recom-name'>{item.title}</Text>
                  <Text className='recom-score'>评分 {item.rating}</Text>
                </View>
              ))}
            </View>
          </ScrollView>
        </View>
      )}
    </View>
  )
}
