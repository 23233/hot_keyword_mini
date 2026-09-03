import React, { useState } from 'react'
import { View, Text, ScrollView } from '@tarojs/components'
import { DramaInfo, EpisodeItem, ActionChannel } from '../../types/drama'
import { UniversalPlayer } from '../UniversalPlayer'
import './index.scss'

interface ImmersiveVideoViewProps {
  drama: DramaInfo
  episodes: EpisodeItem[]
  panChannel?: ActionChannel
  onOpenActionModal: (episodeNum?: number) => void
}

export const ImmersiveVideoView: React.FC<ImmersiveVideoViewProps> = ({
  drama,
  episodes = [],
  panChannel,
  onOpenActionModal
}) => {
  // 当前选中的剧集 (默认第1集)
  const [currentEpisode, setCurrentEpisode] = useState<EpisodeItem>(
    episodes[0] || {
      id: 1,
      drama_id: drama.id,
      episode_num: 1,
      title: '第1集',
      cover_url: drama?.cover_url || '',
      video_url: '',
      is_free: true,
      duration: 120,
      play_mode: drama.play_mode,
      finder_user_name: drama.finder_user_name,
      channels_feed_id: drama.channels_feed_id,
      web_url: drama.web_url
    }
  )

  // 处理点击选集
  const handleSelectEpisode = (ep: EpisodeItem) => {
    setCurrentEpisode(ep)
    const mode = ep.play_mode || drama.play_mode || 'direct_video'
    if (mode === 'direct_video' && !ep.video_url) {
      onOpenActionModal(ep.episode_num)
    } else if (mode === 'none') {
      onOpenActionModal(ep.episode_num)
    }
  }

  const tagsList = (drama.tags || '').split(',').filter(Boolean)

  return (
    <View className='immersive-video-view'>
      {/* 顶部通用播放器 (通用规则) */}
      <UniversalPlayer
        drama={drama}
        episode={currentEpisode}
        panChannel={panChannel}
        onOpenActionModal={onOpenActionModal}
      />

      {/* 剧集基本信息栏 */}
      <View className='drama-header-info apple-card'>
        <View className='title-row'>
          <Text className='drama-title'>{drama.title}</Text>
          {drama.rating > 0 && (
            <View className='rating-box'>
              <Text className='rating-num'>{drama.rating}</Text>
              <Text className='rating-label'>分</Text>
            </View>
          )}
        </View>

        {drama.subtitle && <Text className='drama-subtitle'>{drama.subtitle}</Text>}

        <View className='tags-row'>
          {tagsList.map((tag, idx) => (
            <View key={idx} className='apple-tag'>
              <Text>{tag}</Text>
            </View>
          ))}
          {drama.hot_score > 0 && (
            <View className='hot-indicator'>
              <Text className='fire-icon'>🔥</Text>
              <Text className='hot-num'>热度 {(drama.hot_score / 10000).toFixed(1)}万</Text>
            </View>
          )}
        </View>
      </View>

      {/* 横向选集列表 */}
      {episodes.length > 0 && (
        <View className='episode-section apple-card'>
          <View className='section-header'>
            <View className='header-left'>
              <Text className='section-title'>剧集选集</Text>
              <Text className='ep-count'>全 {drama.total_episodes || episodes.length} 集</Text>
            </View>
            <View className='header-action apple-press-feedback' onClick={() => onOpenActionModal()}>
              <Text className='all-btn-text'>获取全集</Text>
              <Text className='arrow'>›</Text>
            </View>
          </View>

          <ScrollView className='episode-scroll-x' scrollX enableFlex showScrollbar={false}>
            <View className='episode-list-x'>
              {episodes.map((ep) => {
                const isSelected = currentEpisode.episode_num === ep.episode_num
                return (
                  <View
                    key={ep.id || ep.episode_num}
                    className={`episode-chip apple-press-feedback ${isSelected ? 'selected' : ''} ${
                      ep.is_free ? 'free' : 'locked'
                    }`}
                    onClick={() => handleSelectEpisode(ep)}
                  >
                    <Text className='ep-num'>{ep.episode_num}</Text>
                    <View className='ep-badge'>
                      <Text className='badge-text'>{ep.is_free ? '试看' : '全集'}</Text>
                    </View>
                  </View>
                )
              })}
            </View>
          </ScrollView>
        </View>
      )}

      {/* 精彩看点与简介 */}
      {(drama.highlights || drama.description) && (
        <View className='highlights-card apple-card'>
          <View className='hl-header'>
            <Text className='hl-icon'>⚡</Text>
            <Text className='hl-title'>剧情看点</Text>
          </View>
          {drama.highlights && <Text className='hl-content'>{drama.highlights}</Text>}
          {drama.description && <Text className='hl-desc'>{drama.description}</Text>}
        </View>
      )}

      {/* 吸底快捷全集通道 */}
      <View className='bottom-floating-bar'>
        <View className='bar-inner apple-card'>
          <View className='bar-left'>
            <Text className='bar-main'>想看后续大结局？</Text>
            <Text className='bar-sub'>高清未删减 · 完整版全集免费获取</Text>
          </View>
          <View className='bar-btn apple-pill-btn' onClick={() => onOpenActionModal()}>
            <Text className='btn-txt'>看后续全集</Text>
          </View>
        </View>
      </View>
    </View>
  )
}
