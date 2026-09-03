import React from 'react'
import { View, Text } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { ActionChannel } from '../../types/drama'
import './index.scss'

interface ActionModalProps {
  visible: boolean
  onClose: () => void
  channels: ActionChannel[]
  targetEpisodeNum?: number
  dramaTitle?: string
  totalEpisodes?: number
}

export const ActionModal: React.FC<ActionModalProps> = ({
  visible,
  onClose,
  channels = [],
  targetEpisodeNum,
  dramaTitle,
  totalEpisodes
}) => {
  if (!visible) return null

  // 复制内容并给予反馈
  const handleCopyAction = (channel: ActionChannel) => {
    let copyText = channel.content
    if (channel.fetch_code) {
      copyText = `${channel.content} 提取码: ${channel.fetch_code}`
    }

    Taro.setClipboardData({
      data: copyText,
      success: () => {
        Taro.showModal({
          title: '已复制到剪贴板',
          content: channel.tip_notice || '链接已成功复制，请打开对应软件粘贴即可观看全集！',
          showCancel: false,
          confirmText: '我知道了',
          confirmColor: '#FF9F0A'
        })
      }
    })
  }

  const titleText = targetEpisodeNum
    ? `解锁第 ${targetEpisodeNum} 集与后续全集`
    : (dramaTitle ? `获取《${dramaTitle}》完整版全集` : '获取完整版全集')

  const subtitleText = totalEpisodes
    ? `高清 4K 无删减 · 包含 1~${totalEpisodes} 集大结局`
    : '高清 4K 无删减 · 完整版全集大结局'

  return (
    <View className='action-modal-overlay' onClick={onClose}>
      <View className='action-modal-sheet' onClick={(e) => e.stopPropagation()}>
        {/* 顶部指示条与关闭按钮 */}
        <View className='sheet-handle' />
        <View className='sheet-header'>
          <View className='header-left'>
            <Text className='header-title'>{titleText}</Text>
            <Text className='header-subtitle'>{subtitleText}</Text>
          </View>
          <View className='close-btn apple-press-feedback' onClick={onClose}>
            <Text className='close-icon'>✕</Text>
          </View>
        </View>

        {/* 承接渠道卡片流 */}
        <View className='channels-list'>
          {channels.map((channel, index) => (
            <View key={index} className='channel-card apple-card'>
              <View className='card-left'>
                <View className='badge-tag'>
                  {channel.type === 'pan' ? '推荐' : '快捷'}
                </View>
                <Text className='channel-name'>{channel.name}</Text>
                <Text className='channel-desc'>{channel.desc}</Text>
                {channel.fetch_code && (
                  <View className='code-box'>
                    <Text className='code-label'>提取码：</Text>
                    <Text className='code-val'>{channel.fetch_code}</Text>
                  </View>
                )}
              </View>

              <View
                className='card-action-btn apple-pill-btn'
                onClick={() => handleCopyAction(channel)}
              >
                <Text className='btn-text'>{channel.btn_text || '立即获取'}</Text>
              </View>
            </View>
          ))}
        </View>

        <View className='sheet-footer'>
          <Text className='footer-tip'>🔒 资源由剧方官方授权分享，定期维护保证有效</Text>
        </View>
      </View>
    </View>
  )
}
