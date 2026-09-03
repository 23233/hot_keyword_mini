import React from 'react'
import { View, Text, Image, Video, ChannelVideo } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { DramaInfo, EpisodeItem, ActionChannel, PlayMode } from '../../types/drama'
import './index.scss'

interface UniversalPlayerProps {
  drama: DramaInfo
  episode?: EpisodeItem
  panChannel?: ActionChannel
  onOpenActionModal?: (episodeNum?: number) => void
}

export const UniversalPlayer: React.FC<UniversalPlayerProps> = ({
  drama,
  episode,
  panChannel,
  onOpenActionModal
}) => {
  // 决定当前生效的播放模式
  const playMode: PlayMode = episode?.play_mode || drama.play_mode || 'direct_video'
  const videoUrl = episode?.video_url || ''
  const feedId = episode?.channels_feed_id || drama.channels_feed_id || ''
  const finderUserName = episode?.finder_user_name || drama.finder_user_name || 'gh_drama_official'
  const webUrl = episode?.web_url || drama.web_url || ''
  const poster = episode?.cover_url || drama.banner_url || drama.cover_url

  // 视频号跳转
  const handleChannelsJump = () => {
    Taro.showModal({
      title: '前往微信视频号观看',
      content: `即将前往《${drama.title}》官方视频号播放。是否立即前往？`,
      confirmText: '立即打开',
      confirmColor: '#FF9F0A',
      success: (res) => {
        if (res.confirm) {
          if ((Taro as any).openChannelsActivity) {
            ;(Taro as any).openChannelsActivity({
              finderUserName,
              feedId: feedId || 'export/UzFfdHQ5M1F2cTVXWll4eW1GZz09',
              fail: () => {
                Taro.showToast({ title: '已连接视频号专区', icon: 'success' })
              }
            })
          } else {
            Taro.showToast({ title: '已模拟打开视频号', icon: 'success' })
          }
        }
      }
    })
  }

  // 网页直达或复制
  const handleWebJump = () => {
    if (!webUrl) return
    Taro.showActionSheet({
      itemList: ['在小程序内直接打开网页', '复制网页链接去浏览器打开'],
      success: (res) => {
        if (res.tapIndex === 0) {
          Taro.navigateTo({
            url: `/pages/webview/index?url=${encodeURIComponent(webUrl)}&title=${encodeURIComponent(drama.title)}`
          })
        } else if (res.tapIndex === 1) {
          Taro.setClipboardData({
            data: webUrl,
            success: () => {
              Taro.showToast({ title: '网页链接已复制', icon: 'success' })
            }
          })
        }
      }
    })
  }

  // 复制网盘数据
  const handleCopyPan = () => {
    if (!panChannel) {
      if (onOpenActionModal) onOpenActionModal(episode?.episode_num)
      return
    }
    let copyData = panChannel.content
    if (panChannel.fetch_code) {
      copyData = `${panChannel.content} 提取码: ${panChannel.fetch_code}`
    }
    Taro.setClipboardData({
      data: copyData,
      success: () => {
        Taro.showModal({
          title: '网盘链接已复制',
          content: panChannel.tip_notice || '网盘链接与提取码已复制，打开网盘APP即可保存并观看全集！',
          showCancel: false,
          confirmText: '我知道了',
          confirmColor: '#FF9F0A'
        })
      }
    })
  }

  return (
    <View className='universal-player-wrapper apple-card'>
      {/* 形态 1：直接视频源播放 (有 video 播放源) */}
      {playMode === 'direct_video' && videoUrl && (
        <Video
          className='core-video'
          src={videoUrl}
          poster={poster}
          controls
          autoplay={false}
          loop={false}
          showFullscreenBtn
          showPlayBtn
        />
      )}

      {/* 形态 2：微信视频号视频原生内嵌 (免跳转内嵌播放) */}
      {playMode === 'channels_embedded' && feedId && (
        <View className='channels-embedded-box'>
          <ChannelVideo
            className='core-channel-video'
            feedId={feedId}
            finderUserName={finderUserName}
            autoplay={false}
            loop={false}
          />
        </View>
      )}

      {/* 形态 3：微信视频号跳转模式 (带视频号跳转参数) */}
      {playMode === 'channels_video' && (
        <View className='player-card channels-jump-card' onClick={handleChannelsJump}>
          <Image className='card-bg' src={poster} mode='aspectFill' />
          <View className='card-mask'>
            <View className='badge-pill'><Text>微信视频号</Text></View>
            <Text className='card-title'>正版授权 · 视频号高清首播</Text>
            <Text className='card-sub'>已同步关联官方独播剧集动态</Text>
            <View className='action-btn apple-pill-btn'>
              <Text className='btn-txt'>▶ 打开微信视频号观看</Text>
            </View>
          </View>
        </View>
      )}

      {/* 形态 4：外部网页模式 */}
      {playMode === 'web_view' && (
        <View className='player-card web-card' onClick={handleWebJump}>
          <Image className='card-bg' src={poster} mode='aspectFill' />
          <View className='card-mask'>
            <Text className='card-icon'>🌐</Text>
            <Text className='card-title'>官方正版全屏网页影院</Text>
            <Text className='card-sub'>支持 4K 原画免广告播放</Text>
            <View className='action-btn apple-pill-btn'>
              <Text className='btn-txt'>🌐 打开正版网页播放</Text>
            </View>
          </View>
        </View>
      )}

      {/* 形态 5：没有播放链接 / 无法播放 (有网盘数据就显示网盘) */}
      {(playMode === 'none' || (playMode === 'direct_video' && !videoUrl)) && (
        <View className='player-card none-card'>
          <Image className='card-bg' src={poster} mode='aspectFill' />
          <View className='card-mask'>
            <Text className='card-icon'>⏳</Text>
            <Text className='card-title'>当前剧集暂无在线播放源</Text>
            <Text className='card-sub'>已收录正版网盘无删减全集大结局</Text>

            {/* 如果有网盘的数据就显示网盘的 */}
            {panChannel ? (
              <View className='pan-quick-card' onClick={handleCopyPan}>
                <View className='pan-info'>
                  <Text className='pan-name'>{panChannel.name}</Text>
                  {panChannel.fetch_code && (
                    <Text className='pan-code'>提取码: {panChannel.fetch_code}</Text>
                  )}
                </View>
                <View className='pan-btn apple-pill-btn'>
                  <Text className='btn-txt'>{panChannel.btn_text || '一键复制网盘'}</Text>
                </View>
              </View>
            ) : (
              <View
                className='action-btn apple-pill-btn'
                onClick={() => onOpenActionModal && onOpenActionModal(episode?.episode_num)}
              >
                <Text className='btn-txt'>一键获取全集大结局资源</Text>
              </View>
            )}
          </View>
        </View>
      )}
    </View>
  )
}
