// BlockErrorBoundary.tsx
import React from 'react'
import { Text, View } from '@tarojs/components'
import { BlockItem } from '../../types/sdui'

interface BlockErrorBoundaryProps {
  // 当前积木，用于日志与块切换时重置错误态。
  block: BlockItem
  // 子积木渲染内容。
  children: React.ReactNode
}

interface BlockErrorBoundaryState {
  // 当前子树是否已捕获异常。
  hasError: boolean
}

/**
 * 块级错误边界：单个积木渲染异常时只替换该块，不让整页白屏。
 */
export class BlockErrorBoundary extends React.Component<BlockErrorBoundaryProps, BlockErrorBoundaryState> {
  state: BlockErrorBoundaryState = { hasError: false }

  static getDerivedStateFromError(): BlockErrorBoundaryState {
    return { hasError: true }
  }

  componentDidCatch(error: any) {
    console.error('[SDUI 积木渲染异常]', this.props.block?.type, error)
  }

  componentDidUpdate(prevProps: BlockErrorBoundaryProps) {
    // 页面刷新后积木 ID 变化时允许重新尝试渲染。
    if (this.state.hasError && prevProps.block?.id !== this.props.block?.id) {
      this.setState({ hasError: false })
    }
  }

  render() {
    if (this.state.hasError) {
      return (
        <View className='sdui-block-error'>
          <Text className='sdui-block-error-text'>组件暂时无法显示，请稍后重试</Text>
        </View>
      )
    }
    return this.props.children
  }
}