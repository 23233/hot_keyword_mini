import React, { useEffect, useRef, useState, useCallback, useMemo } from 'react'
import { View, Text, Canvas } from '@tarojs/components'
import Taro from '@tarojs/taro'
import './index.scss'

interface MotionLabProps {
  onRetry?: () => void
}

export type EffectType =
  | 'aurora'
  | 'particles'
  | 'blackhole'
  | 'solar'
  | 'waves'
  | 'plasma'
  | 'warp'
  | 'grid'
  | 'orb'
  | 'dna'
  | 'water'
  | 'fireworks'

type CategoryType = 'cosmic' | 'cyber' | 'physics'

interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  radius: number
  color: string
  alpha: number
  life?: number
  maxLife?: number
}

interface StarWarp {
  x: number
  y: number
  z: number
  pz: number
}

interface FireworkParticle {
  x: number
  y: number
  vx: number
  vy: number
  color: string
  alpha: number
  life: number
}

export const MotionLab: React.FC<MotionLabProps> = ({ onRetry }) => {
  // 1. UI 状态
  const [activeCategory, setActiveCategory] = useState<CategoryType>('cosmic')
  const [activeEffect, setActiveEffect] = useState<EffectType>('aurora')
  const [speedMultiplier, setSpeedMultiplier] = useState<number>(1)
  const [glowIntensity, setGlowIntensity] = useState<'soft' | 'vivid'>('vivid')
  const [fps, setFps] = useState<number>(60)

  // 2. 实时 Ref 镜像 (供单例渲染循环 60 FPS 读取，杜绝任何并发重绘卡死)
  const activeEffectRef = useRef<EffectType>('aurora')
  const speedMultiplierRef = useRef<number>(1)
  const glowIntensityRef = useRef<'soft' | 'vivid'>('vivid')

  useEffect(() => {
    activeEffectRef.current = activeEffect
  }, [activeEffect])

  useEffect(() => {
    speedMultiplierRef.current = speedMultiplier
  }, [speedMultiplier])

  useEffect(() => {
    glowIntensityRef.current = glowIntensity
  }, [glowIntensity])

  // 3. 12 大 ShaderToy 经典着色器预设数据库
  const allPresets = useMemo(
    () => [
      // 类别 A: 宇宙天体
      { key: 'aurora', name: '极光流体', icon: '🌌', desc: '正弦弥散着色器', category: 'cosmic' },
      { key: 'particles', name: '星轨粒子', icon: '✨', desc: '引力重力场', category: 'cosmic' },
      { key: 'blackhole', name: '黑洞视界', icon: '🌀', desc: '时空引力透镜', category: 'cosmic' },
      { key: 'solar', name: '耀斑日冕', icon: '☀️', desc: '核聚变对流辐射', category: 'cosmic' },

      // 类别 B: 赛博未来
      { key: 'waves', name: '赛博声波', icon: '🌊', desc: '能量频谱脉冲', category: 'cyber' },
      { key: 'plasma', name: '等离子场', icon: '⚡', desc: '多相位流体电弧', category: 'cyber' },
      { key: 'warp', name: '时空隧道', icon: '🪐', desc: '超光速星流跃迁', category: 'cyber' },
      { key: 'grid', name: '量子网格', icon: '💠', desc: '复古未来透视', category: 'cyber' },

      // 类别 C: 物理生命
      { key: 'orb', name: '脉冲光球', icon: '🔮', desc: '苹果微光呼吸', category: 'physics' },
      { key: 'dna', name: 'DNA双螺旋', icon: '🧬', desc: '3D分子旋转键', category: 'physics' },
      { key: 'water', name: '流光水波', icon: '💧', desc: '焦散折射网络', category: 'physics' },
      { key: 'fireworks', name: '烟花盛宴', icon: '🎆', desc: '重力物理散裂', category: 'physics' }
    ],
    []
  )

  // 当前分类下的 4 个着色器
  const currentCategoryPresets = useMemo(() => {
    return allPresets.filter((p) => p.category === activeCategory)
  }, [allPresets, activeCategory])

  // 原生 Canvas 引用与物理数据
  const canvasNodeRef = useRef<any>(null)
  const ctxRef = useRef<any>(null)
  const widthRef = useRef<number>(340)
  const heightRef = useRef<number>(260)

  const isRunningRef = useRef<boolean>(false)
  const animationIdRef = useRef<any>(null)
  const touchPosRef = useRef<{ x: number; y: number } | null>(null)
  const timeRef = useRef<number>(0)

  // 粒子与特效专用池
  const particlesRef = useRef<Particle[]>([])
  const warpStarsRef = useRef<StarWarp[]>([])
  const fireworksRef = useRef<FireworkParticle[]>([])

  // 初始化通用粒子
  const initParticles = useCallback((w: number, h: number) => {
    const list: Particle[] = []
    const colors = ['#FF9F0A', '#FF375F', '#5E5CE6', '#0A84FF', '#30D158']
    for (let i = 0; i < 70; i++) {
      list.push({
        x: Math.random() * w,
        y: Math.random() * h,
        vx: (Math.random() - 0.5) * 1.8,
        vy: (Math.random() - 0.5) * 1.8,
        radius: Math.random() * 3 + 1.5,
        color: colors[Math.floor(Math.random() * colors.length)],
        alpha: Math.random() * 0.7 + 0.3
      })
    }
    particlesRef.current = list
  }, [])

  // 初始化时空隧道星点
  const initWarpStars = useCallback((w: number, h: number) => {
    const stars: StarWarp[] = []
    for (let i = 0; i < 120; i++) {
      stars.push({
        x: (Math.random() - 0.5) * w * 2,
        y: (Math.random() - 0.5) * h * 2,
        z: Math.random() * w,
        pz: Math.random() * w
      })
    }
    warpStarsRef.current = stars
  }, [])

  // 切换分类
  const handleSwitchCategory = (cat: CategoryType) => {
    setActiveCategory(cat)
    // 默认激活该分类下的第 1 个预设
    const firstOfCat = allPresets.find((p) => p.category === cat)
    if (firstOfCat) {
      handleSelectPreset(firstOfCat.key as EffectType)
    }
  }

  // 切换预设
  const handleSelectPreset = (key: EffectType) => {
    setActiveEffect(key)
    activeEffectRef.current = key

    const w = widthRef.current
    const h = heightRef.current

    if (key === 'particles') {
      initParticles(w, h)
    } else if (key === 'warp') {
      initWarpStars(w, h)
    } else if (key === 'fireworks') {
      fireworksRef.current = []
    }
  }

  // FPS 监控
  const frameCountRef = useRef<number>(0)
  const lastTimeRef = useRef<number>(Date.now())

  // 核心单例逐帧渲染函数
  const renderFrame = useCallback(() => {
    if (!isRunningRef.current) return

    const ctx = ctxRef.current
    const w = widthRef.current
    const h = heightRef.current

    if (ctx && w > 0 && h > 0) {
      const currentSpeed = speedMultiplierRef.current
      const currentEffect = activeEffectRef.current
      const currentGlow = glowIntensityRef.current

      timeRef.current += 0.02 * currentSpeed

      // FPS 稳定每秒统计
      frameCountRef.current++
      const now = Date.now()
      if (now - lastTimeRef.current >= 1000) {
        setFps(frameCountRef.current)
        frameCountRef.current = 0
        lastTimeRef.current = now
      }

      // 1. 背景微弱拖影残影
      ctx.fillStyle = currentGlow === 'vivid' ? 'rgba(8, 8, 12, 0.22)' : 'rgba(8, 8, 12, 0.4)'
      ctx.fillRect(0, 0, w, h)

      const t = timeRef.current
      const touch = touchPosRef.current

      // 2. 12 大经典 Shader 逻辑分支
      if (currentEffect === 'aurora') {
        // 🌌 1. 极光流体: 4 层正弦波色彩弥散
        const waveColors = [
          'rgba(255, 159, 10, 0.4)',
          'rgba(255, 55, 95, 0.35)',
          'rgba(94, 92, 230, 0.4)',
          'rgba(10, 132, 255, 0.35)'
        ]
        for (let i = 0; i < 4; i++) {
          ctx.beginPath()
          ctx.moveTo(0, h)
          for (let x = 0; x <= w; x += 6) {
            const freq = 0.006 + i * 0.002
            const amp = 45 + i * 20
            const touchBias = touch ? Math.sin((x - touch.x) * 0.035) * 35 : 0
            const y = h * 0.52 + Math.sin(x * freq + t * 1.2 + i * 1.6) * amp + Math.cos(x * 0.012 - t) * 18 + touchBias
            ctx.lineTo(x, y)
          }
          ctx.lineTo(w, h)
          ctx.closePath()

          const grad = ctx.createLinearGradient(0, h * 0.15, 0, h)
          grad.addColorStop(0, waveColors[i])
          grad.addColorStop(1, 'rgba(0, 0, 0, 0)')
          ctx.fillStyle = grad
          ctx.fill()
        }
      } else if (currentEffect === 'particles') {
        // ✨ 2. 星轨粒子: 引力连线与触控排斥
        const list = particlesRef.current
        for (let i = 0; i < list.length; i++) {
          const p = list[i]
          p.x += p.vx * currentSpeed
          p.y += p.vy * currentSpeed
          if (p.x < 0 || p.x > w) p.vx *= -1
          if (p.y < 0 || p.y > h) p.vy *= -1

          if (touch) {
            const dx = p.x - touch.x
            const dy = p.y - touch.y
            const dist = Math.hypot(dx, dy)
            if (dist < 90 && dist > 0) {
              p.x += (dx / dist) * 5
              p.y += (dy / dist) * 5
            }
          }

          ctx.beginPath()
          ctx.arc(p.x, p.y, p.radius, 0, Math.PI * 2)
          ctx.fillStyle = p.color
          ctx.globalAlpha = p.alpha
          ctx.fill()
          ctx.globalAlpha = 1.0

          for (let j = i + 1; j < list.length; j++) {
            const p2 = list[j]
            const dist = Math.hypot(p.x - p2.x, p.y - p2.y)
            if (dist < 55) {
              ctx.beginPath()
              ctx.moveTo(p.x, p.y)
              ctx.lineTo(p2.x, p2.y)
              ctx.strokeStyle = `rgba(255, 255, 255, ${0.28 * (1 - dist / 55)})`
              ctx.lineWidth = 0.8
              ctx.stroke()
            }
          }
        }
      } else if (currentEffect === 'blackhole') {
        // 🌀 3. 黑洞视界: 引力透镜光环 + 倾斜吸积盘
        const cx = touch ? touch.x : w * 0.5
        const cy = touch ? touch.y : h * 0.48
        const rHole = 38

        // 爱因斯坦引力透镜外发光
        const gradLens = ctx.createRadialGradient(cx, cy, rHole * 0.8, cx, cy, rHole * 2.8)
        gradLens.addColorStop(0, '#FF9F0A')
        gradLens.addColorStop(0.3, 'rgba(255, 55, 95, 0.6)')
        gradLens.addColorStop(0.7, 'rgba(94, 92, 230, 0.3)')
        gradLens.addColorStop(1, 'rgba(0, 0, 0, 0)')
        ctx.fillStyle = gradLens
        ctx.beginPath()
        ctx.arc(cx, cy, rHole * 2.8, 0, Math.PI * 2)
        ctx.fill()

        // 旋转倾斜吸积盘 (椭圆轨道点阵)
        ctx.save()
        ctx.translate(cx, cy)
        ctx.rotate(-0.35)
        for (let i = 0; i < 45; i++) {
          const angle = t * 1.5 + (i / 45) * Math.PI * 2
          const rx = 85 + Math.sin(i * 3 + t) * 15
          const ry = 26 + Math.cos(i * 2) * 6
          const px = Math.cos(angle) * rx
          const py = Math.sin(angle) * ry
          ctx.beginPath()
          ctx.arc(px, py, 2.5, 0, Math.PI * 2)
          ctx.fillStyle = i % 2 === 0 ? '#FFD60A' : '#FF375F'
          ctx.shadowBlur = currentGlow === 'vivid' ? 10 : 4
          ctx.shadowColor = '#FF9F0A'
          ctx.fill()
        }
        ctx.restore()

        // 绝对事件视界 (纯黑核心)
        ctx.beginPath()
        ctx.arc(cx, cy, rHole, 0, Math.PI * 2)
        ctx.fillStyle = '#000000'
        ctx.shadowBlur = 0
        ctx.fill()
        ctx.lineWidth = 2
        ctx.strokeStyle = 'rgba(255, 159, 10, 0.8)'
        ctx.stroke()
      } else if (currentEffect === 'solar') {
        // ☀️ 4. 耀斑日冕: 恒星聚变对流核 + 日珥热浪
        const cx = touch ? touch.x : w * 0.5
        const cy = touch ? touch.y : h * 0.48
        const rCore = 46

        // 28 条向外喷射扭动的日冕热浪
        ctx.save()
        ctx.translate(cx, cy)
        for (let i = 0; i < 28; i++) {
          const angle = (i / 28) * Math.PI * 2 + t * 0.4
          const rayLen = rCore + 25 + Math.sin(t * 3 + i * 2) * 18 + Math.cos(t * 1.5 + i) * 10
          ctx.beginPath()
          ctx.moveTo(Math.cos(angle) * rCore, Math.sin(angle) * rCore)
          ctx.lineTo(Math.cos(angle) * rayLen, Math.sin(angle) * rayLen)
          ctx.strokeStyle = i % 2 === 0 ? 'rgba(255, 159, 10, 0.5)' : 'rgba(255, 69, 58, 0.4)'
          ctx.lineWidth = 4
          ctx.stroke()
        }
        ctx.restore()

        // 聚变核心渐变
        const gradSun = ctx.createRadialGradient(cx, cy, 5, cx, cy, rCore)
        gradSun.addColorStop(0, '#FFFFFF')
        gradSun.addColorStop(0.4, '#FFD60A')
        gradSun.addColorStop(0.8, '#FF453A')
        gradSun.addColorStop(1, '#FF375F')
        ctx.beginPath()
        ctx.arc(cx, cy, rCore, 0, Math.PI * 2)
        ctx.fillStyle = gradSun
        ctx.shadowBlur = currentGlow === 'vivid' ? 24 : 10
        ctx.shadowColor = '#FF9F0A'
        ctx.fill()
        ctx.shadowBlur = 0
      } else if (currentEffect === 'waves') {
        // 🌊 5. 赛博声波: 36 根跳动频谱
        const bars = 36
        const barW = w / bars
        for (let i = 0; i < bars; i++) {
          const x = i * barW
          const waveH = Math.abs(Math.sin(t * 2 + i * 0.28) * Math.cos(t * 1.1 + i * 0.18)) * (h * 0.45) + 12
          const grad = ctx.createLinearGradient(x, h * 0.5 - waveH, x, h * 0.5 + waveH)
          grad.addColorStop(0, '#FF375F')
          grad.addColorStop(0.5, '#FF9F0A')
          grad.addColorStop(1, '#5E5CE6')
          ctx.fillStyle = grad
          ctx.fillRect(x + 2, h * 0.5 - waveH, barW - 4, waveH * 2)
        }
      } else if (currentEffect === 'plasma') {
        // ⚡ 6. 等离子场: 经典正余弦多相位交融
        const cols = 24
        const rows = 16
        const cellW = w / cols
        const cellH = h / rows
        for (let r = 0; r < rows; r++) {
          for (let c = 0; c < cols; c++) {
            const v1 = Math.sin(c * 0.3 + t)
            const v2 = Math.sin(r * 0.3 + t * 1.2)
            const v3 = Math.sin((c + r) * 0.2 + t * 0.8)
            const dist = Math.hypot(c - cols / 2, r - rows / 2)
            const v4 = Math.sin(dist * 0.4 - t * 1.5)
            const val = (v1 + v2 + v3 + v4) * 0.25

            const hue = (val * 120 + t * 30) % 360
            ctx.fillStyle = `hsla(${hue}, 85%, 55%, 0.35)`
            ctx.fillRect(c * cellW, r * cellH, cellW + 1, cellH + 1)
          }
        }
      } else if (currentEffect === 'warp') {
        // 🪐 7. 时空隧道: 超光速星流穿梭向外喷射
        const cx = touch ? touch.x : w * 0.5
        const cy = touch ? touch.y : h * 0.5
        const stars = warpStarsRef.current
        for (let i = 0; i < stars.length; i++) {
          const s = stars[i]
          s.z -= 6 * currentSpeed
          if (s.z <= 0) {
            s.z = w
            s.pz = w
            s.x = (Math.random() - 0.5) * w * 2
            s.y = (Math.random() - 0.5) * h * 2
          }

          const k = 128 / s.z
          const px = s.x * k + cx
          const py = s.y * k + cy

          const pk = 128 / s.pz
          const ppx = s.x * pk + cx
          const ppy = s.y * pk + cy
          s.pz = s.z

          if (px >= 0 && px <= w && py >= 0 && py <= h) {
            ctx.beginPath()
            ctx.moveTo(ppx, ppy)
            ctx.lineTo(px, py)
            ctx.strokeStyle = i % 3 === 0 ? '#30D158' : i % 2 === 0 ? '#0A84FF' : '#FF9F0A'
            ctx.lineWidth = Math.min(2.5, (1 - s.z / w) * 3)
            ctx.stroke()
          }
        }
      } else if (currentEffect === 'grid') {
        // 💠 8. 量子网格: 80s 赛博朋克透视网格地形
        const horizon = h * 0.45
        // 绘制地平线夕阳半圆
        const cx = touch ? touch.x : w * 0.5
        const gradSun = ctx.createLinearGradient(cx, horizon - 50, cx, horizon)
        gradSun.addColorStop(0, '#FF375F')
        gradSun.addColorStop(1, '#FF9F0A')
        ctx.fillStyle = gradSun
        ctx.beginPath()
        ctx.arc(cx, horizon, 55, Math.PI, 0, false)
        ctx.fill()

        // 地面纵向透视延伸线
        ctx.strokeStyle = 'rgba(255, 55, 95, 0.45)'
        ctx.lineWidth = 1
        for (let x = -w * 0.5; x <= w * 1.5; x += 36) {
          ctx.beginPath()
          ctx.moveTo(cx, horizon)
          ctx.lineTo(x, h)
          ctx.stroke()
        }

        // 地面横向无限前行网格线
        const speedOffset = (t * 40) % 30
        for (let y = horizon + 4; y <= h; y += 14 + (y - horizon) * 0.3) {
          const actualY = y + speedOffset * ((y - horizon) / (h - horizon))
          if (actualY <= h) {
            ctx.beginPath()
            ctx.moveTo(0, actualY)
            ctx.lineTo(w, actualY)
            ctx.strokeStyle = `rgba(10, 132, 255, ${0.15 + (actualY - horizon) / (h - horizon) * 0.6})`
            ctx.stroke()
          }
        }
      } else if (currentEffect === 'orb') {
        // 🔮 9. 脉冲光球: 苹果呼吸微光环
        const cx = touch ? touch.x : w * 0.5
        const cy = touch ? touch.y : h * 0.48
        const baseR = 50 + Math.sin(t * 2.2) * 16

        for (let r = 4; r >= 1; r--) {
          ctx.beginPath()
          const currentR = baseR + r * 28 + Math.sin(t * 1.4 + r) * 12
          ctx.arc(cx, cy, currentR, 0, Math.PI * 2)
          const grad = ctx.createRadialGradient(cx, cy, currentR * 0.25, cx, cy, currentR)
          grad.addColorStop(0, `rgba(255, 159, 10, ${0.45 / r})`)
          grad.addColorStop(0.6, `rgba(255, 55, 95, ${0.28 / r})`)
          grad.addColorStop(1, 'rgba(0, 0, 0, 0)')
          ctx.fillStyle = grad
          ctx.fill()
        }

        ctx.beginPath()
        ctx.arc(cx, cy, baseR * 0.45, 0, Math.PI * 2)
        ctx.fillStyle = '#FFFFFF'
        ctx.shadowBlur = 30
        ctx.shadowColor = '#FF9F0A'
        ctx.fill()
        ctx.shadowBlur = 0
      } else if (currentEffect === 'dna') {
        // 🧬 10. DNA双螺旋: 3D分子发光双链与旋转横键
        const cx = touch ? touch.x : w * 0.5
        const nodes = 22
        const spacing = (h * 0.8) / nodes
        const startY = h * 0.1

        for (let i = 0; i < nodes; i++) {
          const y = startY + i * spacing
          const phase = t * 2 + i * 0.35
          const amp = 65
          const x1 = cx + Math.sin(phase) * amp
          const x2 = cx + Math.sin(phase + Math.PI) * amp
          const z1 = Math.cos(phase)
          const z2 = Math.cos(phase + Math.PI)

          // 绘制碱基对横键
          ctx.beginPath()
          ctx.moveTo(x1, y)
          ctx.lineTo(x2, y)
          ctx.strokeStyle = `rgba(255, 255, 255, 0.25)`
          ctx.lineWidth = 1
          ctx.stroke()

          // 链 1 节点
          ctx.beginPath()
          ctx.arc(x1, y, 3 + z1 * 1.5, 0, Math.PI * 2)
          ctx.fillStyle = '#0A84FF'
          ctx.fill()

          // 链 2 节点
          ctx.beginPath()
          ctx.arc(x2, y, 3 + z2 * 1.5, 0, Math.PI * 2)
          ctx.fillStyle = '#FF9F0A'
          ctx.fill()
        }
      } else if (currentEffect === 'water') {
        // 💧 11. 流光水波: 水下焦散波纹
        for (let i = 0; i < 6; i++) {
          ctx.beginPath()
          for (let x = 0; x <= w; x += 12) {
            const waveY =
              h * 0.5 +
              Math.sin(x * 0.02 + t + i) * 35 +
              Math.cos(x * 0.015 - t * 0.8 + i * 1.5) * 25
            if (x === 0) ctx.moveTo(x, waveY)
            else ctx.lineTo(x, waveY)
          }
          ctx.strokeStyle = `rgba(48, 209, 88, ${0.15 + i * 0.08})`
          ctx.lineWidth = 2.5
          ctx.stroke()
        }
      } else if (currentEffect === 'fireworks') {
        // 🎆 12. 烟花盛宴: 重力物理粒子散裂
        const list = fireworksRef.current
        // 定时发射新烟花
        if (Math.random() < 0.08 * currentSpeed) {
          const spawnX = Math.random() * (w * 0.8) + w * 0.1
          const spawnY = Math.random() * (h * 0.5) + h * 0.15
          const colorList = ['#FF375F', '#FF9F0A', '#30D158', '#0A84FF', '#BF5AF2']
          const chosenColor = colorList[Math.floor(Math.random() * colorList.length)]
          for (let i = 0; i < 35; i++) {
            const angle = Math.random() * Math.PI * 2
            const speed = Math.random() * 3.5 + 1
            list.push({
              x: spawnX,
              y: spawnY,
              vx: Math.cos(angle) * speed,
              vy: Math.sin(angle) * speed,
              color: chosenColor,
              alpha: 1.0,
              life: 0
            })
          }
        }

        // 模拟粒子重力下坠
        for (let i = list.length - 1; i >= 0; i--) {
          const p = list[i]
          p.x += p.vx * currentSpeed
          p.y += p.vy * currentSpeed
          p.vy += 0.05 * currentSpeed // 重力加速度
          p.alpha -= 0.018 * currentSpeed
          p.life++

          if (p.alpha <= 0) {
            list.splice(i, 1)
            continue
          }

          ctx.beginPath()
          ctx.arc(p.x, p.y, 2, 0, Math.PI * 2)
          ctx.fillStyle = p.color
          ctx.globalAlpha = p.alpha
          ctx.fill()
          ctx.globalAlpha = 1.0
        }
      }

      // 触控共振波纹指示圈
      if (touch) {
        ctx.beginPath()
        ctx.arc(touch.x, touch.y, 22 + Math.sin(t * 6) * 5, 0, Math.PI * 2)
        ctx.strokeStyle = 'rgba(255, 255, 255, 0.55)'
        ctx.lineWidth = 1.5
        ctx.stroke()
      }
    }

    // 单一实例驱动下一帧
    if (!isRunningRef.current) return
    const canvas = canvasNodeRef.current
    if (canvas && typeof canvas.requestAnimationFrame === 'function') {
      animationIdRef.current = canvas.requestAnimationFrame(renderFrame)
    } else if (typeof requestAnimationFrame === 'function') {
      animationIdRef.current = requestAnimationFrame(renderFrame)
    } else {
      animationIdRef.current = setTimeout(renderFrame, 16)
    }
  }, [allPresets])

  // 单例初始化，仅在挂载时执行一次
  useEffect(() => {
    isRunningRef.current = true

    const initTimer = setTimeout(() => {
      Taro.createSelectorQuery()
        .select('#motionCanvas')
        .fields({ node: true, size: true })
        .exec((res) => {
          if (res && res[0] && res[0].node) {
            const canvas = res[0].node
            const ctx = canvas.getContext('2d')
            const dpr = Taro.getSystemInfoSync().pixelRatio || 2

            canvasNodeRef.current = canvas
            ctxRef.current = ctx

            const w = res[0].width || (Taro.getSystemInfoSync().windowWidth - 32)
            const h = res[0].height || 260

            widthRef.current = w
            heightRef.current = h

            canvas.width = w * dpr
            canvas.height = h * dpr
            ctx.scale(dpr, dpr)

            initParticles(w, h)
            initWarpStars(w, h)
            renderFrame()
          } else {
            const fallbackWidth = Taro.getSystemInfoSync().windowWidth - 32
            widthRef.current = fallbackWidth
            heightRef.current = 260
            initParticles(fallbackWidth, 260)
            initWarpStars(fallbackWidth, 260)
          }
        })
    }, 200)

    return () => {
      isRunningRef.current = false
      clearTimeout(initTimer)
      const canvas = canvasNodeRef.current
      if (canvas && typeof canvas.cancelAnimationFrame === 'function') {
        canvas.cancelAnimationFrame(animationIdRef.current)
      } else if (typeof cancelAnimationFrame === 'function') {
        cancelAnimationFrame(animationIdRef.current)
      } else {
        clearTimeout(animationIdRef.current)
      }
    }
  }, [initParticles, initWarpStars, renderFrame])

  // 处理触控事件
  const handleTouchMove = (e: any) => {
    if (e.touches && e.touches[0]) {
      touchPosRef.current = {
        x: e.touches[0].x,
        y: e.touches[0].y
      }
    }
  }

  const handleTouchEnd = () => {
    touchPosRef.current = null
  }

  return (
    <View className='motion-lab-container'>
      {/* 1. 顶部灵动状态胶囊栏 */}
      <View className='status-header-bar'>
        <View className='left-badge'>
          <View className='pulse-dot' />
          <Text className='badge-text'>灵动视界 Lab</Text>
        </View>
        <View className='right-meta'>
          <View className='fps-pill'>
            <Text className='fps-num'>{fps}</Text>
            <Text className='fps-label'>FPS</Text>
          </View>
          {onRetry && (
            <View className='reconnect-btn apple-press-feedback' onClick={onRetry}>
              <Text className='icon'>⚡</Text>
              <Text className='btn-txt'>连接剧场</Text>
            </View>
          )}
        </View>
      </View>

      {/* 2. 核心画布互动视窗 */}
      <View className={`canvas-viewport apple-card effect-${activeEffect}`}>
        <View className='ambient-glow-layer' />
        <Canvas
          type='2d'
          id='motionCanvas'
          className='motion-canvas'
          onTouchStart={handleTouchMove}
          onTouchMove={handleTouchMove}
          onTouchEnd={handleTouchEnd}
        />
        <View className='viewport-glass-tip'>
          <Text className='tip-text'>👆 轻触滑动屏幕，激发引力共振与光波涟漪</Text>
        </View>
      </View>

      {/* 3. 12 大 ShaderToy 预设分类选择器 (分类分段器 + 2x2 精工四宫格) */}
      <View className='presets-section'>
        <View className='section-title-row'>
          <Text className='sec-title'>着色器预设 (12 Shader Presets)</Text>
          <Text className='sec-sub'>60 FPS 物理与流体</Text>
        </View>

        {/* 顶部三分类选择胶囊 (宇宙天体 / 赛博未来 / 物理生命) */}
        <View className='category-segmented-bar'>
          {[
            { key: 'cosmic', txt: '🌌 宇宙天体' },
            { key: 'cyber', txt: '⚡ 赛博未来' },
            { key: 'physics', txt: '🔮 物理生命' }
          ].map((cat) => (
            <View
              key={cat.key}
              className={`cat-tab apple-press-feedback ${activeCategory === cat.key ? 'active' : ''}`}
              onClick={() => handleSwitchCategory(cat.key as CategoryType)}
            >
              <Text className='cat-text'>{cat.txt}</Text>
            </View>
          ))}
        </View>

        {/* 当前分类下的 4 个着色器 2x2 网格 */}
        <View className='presets-grid'>
          {currentCategoryPresets.map((item) => (
            <View
              key={item.key}
              className={`preset-grid-card apple-press-feedback ${
                activeEffect === item.key ? 'active' : ''
              }`}
              onClick={() => handleSelectPreset(item.key as EffectType)}
            >
              <Text className='card-icon'>{item.icon}</Text>
              <View className='card-text-group'>
                <Text className='card-name'>{item.name}</Text>
                <Text className='card-desc'>{item.desc}</Text>
              </View>
            </View>
          ))}
        </View>
      </View>

      {/* 4. 物理参数控制台 (通栏分段器) */}
      <View className='controls-card apple-card'>
        {/* 参数项 A: 流速控制 */}
        <View className='control-group'>
          <View className='group-header'>
            <Text className='group-title'>流速控制 (Speed Factor)</Text>
            <Text className='group-status'>{speedMultiplier.toFixed(1)}x 速率</Text>
          </View>
          <View className='segmented-track'>
            {[
              { val: 0.5, txt: '0.5x 缓流' },
              { val: 1.0, txt: '1.0x 标准' },
              { val: 2.0, txt: '2.0x 极速' }
            ].map((s) => (
              <View
                key={s.val}
                className={`seg-item apple-press-feedback ${speedMultiplier === s.val ? 'selected' : ''}`}
                onClick={() => setSpeedMultiplier(s.val)}
              >
                <Text className='seg-text'>{s.txt}</Text>
              </View>
            ))}
          </View>
        </View>

        {/* 参数项 B: 色彩辉光 */}
        <View className='control-group'>
          <View className='group-header'>
            <Text className='group-title'>色彩辉光 (Glow Bloom)</Text>
            <Text className='group-status'>{glowIntensity === 'vivid' ? '绚丽幻彩' : '柔和深邃'}</Text>
          </View>
          <View className='segmented-track'>
            {[
              { val: 'soft', txt: '柔和深邃' },
              { val: 'vivid', txt: '绚丽幻彩' }
            ].map((g) => (
              <View
                key={g.val}
                className={`seg-item apple-press-feedback ${glowIntensity === g.val ? 'selected' : ''}`}
                onClick={() => setGlowIntensity(g.val as any)}
              >
                <Text className='seg-text'>{g.txt}</Text>
              </View>
            ))}
          </View>
        </View>
      </View>

      {/* 5. 底部合规声明 */}
      <View className='compliance-footer'>
        <Text className='footer-txt'>灵动视界 Studio · 12 大 ShaderToy 经典物理动效实验室</Text>
        <Text className='footer-sub'>原生 Canvas 2D 纯客户端计算 · 零网络流量消耗 · 绿色合规</Text>
      </View>
    </View>
  )
}
