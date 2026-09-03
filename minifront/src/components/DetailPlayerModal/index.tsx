import React, { useState } from 'react'
import { View, Text, ScrollView } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { DramaInfo, EpisodeItem, ActionChannel, PlayMode } from '../../types/drama'
import { UniversalPlayer } from '../UniversalPlayer'
import './index.scss'

interface DetailPlayerModalProps {
  visible: boolean
  onClose: () => void
  drama: DramaInfo | null
  episodes: EpisodeItem[]
  panChannel?: ActionChannel
  onOpenActionModal: (episodeNum?: number) => void
}

export const DetailPlayerModal: React.FC<DetailPlayerModalProps> = ({
  visible,
  onClose,
  drama,
  episodes = [],
  panChannel,
  onOpenActionModal
}) => {
  if (!visible || !drama) return null

  // 默认选中单集
  const [selectedEp, setSelectedEp] = useState<EpisodeItem>(
    episodes[0] || {
      id: 1,
      drama_id: drama.id,
      episode_num: 1,
      title: '第1集',
      cover_url: drama.cover_url,
      video_url: '',
      is_free: true,
      duration: 120,
      play_mode: drama.play_mode || 'direct_video',
      finder_user_name: drama.finder_user_name,
      channels_feed_id: drama.channels_feed_id,
      web_url: drama.web_url
    }
  )

  const currentPlayMode: PlayMode = selectedEp.play_mode || drama.play_mode || 'direct_video'

  // 点击单集处理通用播放规则
  const handleSelectEpisode = (ep: EpisodeItem) => {
    setSelectedEp(ep)
    const mode = ep.play_mode || drama.play_mode || 'direct_video'

    if (mode === 'direct_video') {
      if (ep.video_url) {
        Taro.showToast({ title: `正在播放第 ${ep.episode_num} 集`, icon: 'none' })
      } else {
        // 无视频链接：引导网盘或全集大结局
        onOpenActionModal(ep.episode_num)
      }
    } else if (mode === 'channels_embedded') {
      Taro.showToast({ title: `视频号内嵌免跳播放第 ${ep.episode_num} 集`, icon: 'none' })
    } else if (mode === 'channels_video') {
      handleChannelsJump(ep.episode_num)
    } else if (mode === 'web_view') {
      handleWebJump()
    } else {
      // none 无播放源
      Taro.showToast({ title: `第 ${ep.episode_num} 集暂无播放源，已为您推荐网盘`, icon: 'none' })
      onOpenActionModal(ep.episode_num)
    }
  }

  // 微信视频号跳转
  const handleChannelsJump = (epNum: number) => {
    Taro.showModal({
      title: '前往微信视频号观看',
      content: `即将前往《${drama.title}》官方视频号播放第 ${epNum} 集。是否立即打开？`,
      confirmText: '立即打开',
      confirmColor: '#FF9F0A',
      success: (res) => {
        if (res.confirm) {
          if ((Taro as any).openChannelsActivity) {
            ;(Taro as any).openChannelsActivity({
              finderUserName: selectedEp.finder_user_name || drama.finder_user_name || 'gh_drama_official',
              feedId: selectedEp.channels_feed_id || drama.channels_feed_id || 'export/UzFfdHQ5M1F2cTVXWll4eW1GZz09',
              fail: () => {
                Taro.showToast({ title: '已连接视频号专区', icon: 'success' })
              }
            })
          } else {
            Taro.showToast({ title: '已模拟调起微信视频号', icon: 'success' })
          }
        }
      }
    })
  }

  // 网页外链模式
  const handleWebJump = () => {
    const targetUrl = selectedEp.web_url || drama.web_url || ''
    if (!targetUrl) return
    Taro.setClipboardData({
      data: targetUrl,
      success: () => {
        Taro.showModal({
          title: '已复制官方播放网页',
          content: '正版授权网页链接已复制到剪贴板，请在浏览器中打开即可直接在线播放！',
          showCancel: false,
          confirmText: '我知道了',
          confirmColor: '#FF9F0A'
        })
      }
    })
  }

  const tagsList = (drama.tags || '').split(',').filter(Boolean)

  return (
    <View className='detail-modal-overlay' onClick={onClose}>
      <View className='detail-modal-sheet' onClick={(e) => e.stopPropagation()}>
        {/* 顶部把手与关闭 */}
        <View className='modal-handle' />
        <View className='modal-nav'>
          <View className='nav-left'>
            <Text className='nav-title'>{drama.title}</Text>
            <View className='nav-mode-badge'>
              <Text className='mode-txt'>
                {currentPlayMode === 'direct_video' && '🎬 播放源直接播放'}
                {currentPlayMode === 'channels_embedded' && '📺 视频号内嵌免跳'}
                {currentPlayMode === 'channels_video' && '📺 微信视频号跳转'}
                {currentPlayMode === 'web_view' && '🌐 外部网页直达'}
                {currentPlayMode === 'none' && '⏳ 暂无播放链接'}
              </Text>
            </View>
          </View>
          <View className='modal-close-btn apple-press-feedback' onClick={onClose}>
            <Text className='close-x'>✕</Text>
          </View>
        </View>

        <ScrollView className='modal-body-scroll' scrollY>
          {/* 通用播放引擎 (规则一致) */}
          <UniversalPlayer
            drama={drama}
            episode={selectedEp}
            panChannel={panChannel}
            onOpenActionModal={onOpenActionModal}
          />

          {/* 剧集概况 */}
          <View className='modal-meta-card apple-card'>
            <View className='meta-row'>
              <Text className='drama-name'>{drama.title}</Text>
              {drama.rating > 0 && (
                <View className='score-badge'>
                  <Text className='score-txt'>★ {drama.rating}</Text>
                </View>
              )}
            </View>
            {drama.subtitle && <Text className='drama-sub'>{drama.subtitle}</Text>}
            {tagsList.length > 0 && (
              <View className='tags-wrap'>
                {tagsList.map((tag, idx) => (
                  <View key={idx} className='apple-tag'>
                    <Text>{tag}</Text>
                  </View>
                ))}
              </View>
            )}
            {drama.description && <Text className='drama-desc-txt'>{drama.description}</Text>}
          </View>

          {/* 完整选集网格 */}
          <View className='modal-episodes-section apple-card'>
            <View className='ep-head'>
              <Text className='head-title'>选集列表 (全 {drama.total_episodes || episodes.length} 集)</Text>
              <Text className='head-tip'>
                {currentPlayMode === 'direct_video' && '点击选集直接播放'}
                {currentPlayMode === 'channels_embedded' && '点击选集内嵌播放'}
                {currentPlayMode === 'channels_video' && '点击选集视频号播放'}
                {currentPlayMode === 'none' && '无播放源，支持网盘全集'}
              </Text>
            </View>

            <View className='ep-grid'>
              {episodes.map((ep) => {
                const isSelected = selectedEp.episode_num === ep.episode_num
                const epMode = ep.play_mode || currentPlayMode
                return (
                  <View
                    key={ep.id || ep.episode_num}
                    className={`ep-item apple-press-feedback ${isSelected ? 'selected' : ''}`}
                    onClick={() => handleSelectEpisode(ep)}
                  >
                    <Text className='ep-num'>{ep.episode_num}</Text>
                    <Text className='ep-tag'>
                      {ep.is_free ? '试看' : epMode === 'channels_video' ? '视频号' : '全集'}
                    </Text>
                  </View>
                )
              })}
            </View>
          </View>

          {/* 底部获取资源按钮 */}
          <View className='action-collaborate-row'>
            <View
              className='collab-btn apple-pill-btn'
              onClick={() => onOpenActionModal()}
            >
              <Text className='collab-txt'>🚀 获取完整版全集大结局</Text>
            </View>
          </View>
        </ScrollView>
      </View>
    </View>
  )
}
