import React from 'react'
import { View, Text, Image } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { DramaInfo, ActionChannel } from '../../types/drama'
import './index.scss'

interface DirectPortalViewProps {
  drama: DramaInfo
  channels: ActionChannel[]
}

export const DirectPortalView: React.FC<DirectPortalViewProps> = ({
  drama,
  channels = []
}) => {
  // 一键复制处理
  const handleCopy = (ch: ActionChannel) => {
    let copyText = ch.content
    if (ch.fetch_code) {
      copyText = `${ch.content} 提取码: ${ch.fetch_code}`
    }

    Taro.setClipboardData({
      data: copyText,
      success: () => {
        Taro.showModal({
          title: '已成功复制',
          content: ch.tip_notice || '请打开对应软件粘贴即可观看全集！',
          showCancel: false,
          confirmText: '我知道了',
          confirmColor: '#FF9F0A'
        })
      }
    })
  }

  return (
    <View className='direct-portal-view'>
      {/* 顶部醒目全集大卡片 */}
      <View className='portal-hero-card apple-card'>
        <View className='hero-badge'>
          <Text className='badge-txt'>🔥 搜一搜专属直通车</Text>
        </View>

        <Text className='hero-title'>《{drama.title}》全集完整版</Text>
        <Text className='hero-subtitle'>1~{drama.total_episodes} 集大结局 · 4K 超清 · 免广告无删减</Text>

        <View className='drama-preview-banner'>
          <Image className='banner-img' src={drama.banner_url || drama.cover_url} mode='aspectFill' />
          <View className='banner-glass-bar'>
            <Text className='glass-tag'>{drama.subtitle || '全网爆款热播'}</Text>
            {drama.hot_score > 0 && (
              <Text className='glass-heat'>全网热度: {(drama.hot_score / 10000).toFixed(1)}万</Text>
            )}
          </View>
        </View>
      </View>

      {/* 核心看后续快捷通道卡片组 */}
      <View className='channels-group'>
        <View className='group-header'>
          <Text className='header-label'>请选择您方便的观看通道：</Text>
        </View>

        {channels.map((ch, idx) => (
          <View key={idx} className='channel-tile apple-card apple-press-feedback' onClick={() => handleCopy(ch)}>
            <View className='tile-left'>
              <View className='tile-title-row'>
                <Text className='tile-name'>{ch.name}</Text>
                {idx === 0 && <View className='rec-badge'><Text>首选通道</Text></View>}
              </View>
              <Text className='tile-desc'>{ch.desc}</Text>
              {ch.fetch_code && (
                <View className='fetch-code-tag'>
                  <Text className='code-txt'>提取码: {ch.fetch_code}</Text>
                </View>
              )}
            </View>

            <View className='tile-btn apple-pill-btn'>
              <Text className='btn-text'>{ch.btn_text || '立即复制'}</Text>
            </View>
          </View>
        ))}
      </View>

      {/* 简单 3 步观看指南 */}
      <View className='guide-card apple-card'>
        <Text className='guide-title'>📖 三步极速看全集指南</Text>
        <View className='steps-list'>
          <View className='step-item'>
            <View className='step-circle'><Text>1</Text></View>
            <Text className='step-text'>点击上方按钮一键复制网盘链接与提取码</Text>
          </View>
          <View className='step-item'>
            <View className='step-circle'><Text>2</Text></View>
            <Text className='step-text'>打开手机夸克/百度网盘 APP 或浏览器直接保存</Text>
          </View>
          <View className='step-item'>
            <View className='step-circle'><Text>3</Text></View>
            <Text className='step-text'>即刻开始沉浸式观看 1~{drama.total_episodes} 集无删减大结局</Text>
          </View>
        </View>
      </View>
    </View>
  )
}
