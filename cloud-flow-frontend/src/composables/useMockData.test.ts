import { describe, it, expect } from 'vitest'
import { useMockData } from './useMockData'

describe('useMockData', () => {
  it('should generate array of specified length', () => {
    const { generateMockData } = useMockData()
    const data = generateMockData(100, 0, 50)
    expect(data).toHaveLength(50)
  })

  it('should generate values within range', () => {
    const { generateMockData } = useMockData()
    const data = generateMockData(100, 50, 100)
    data.forEach(value => {
      expect(value).toBeGreaterThanOrEqual(50)
      expect(value).toBeLessThanOrEqual(150) // max + min
    })
  })

  it('should return correct progress colors', () => {
    const { getProgressColor } = useMockData()
    
    expect(getProgressColor(90)).toBe('#F56C6C')  // red
    expect(getProgressColor(70)).toBe('#E6A23C')  // orange
    expect(getProgressColor(50)).toBe('#67C23A')  // green
  })

  it('should return red for values above 80', () => {
    const { getProgressColor } = useMockData()
    expect(getProgressColor(81)).toBe('#F56C6C')
    expect(getProgressColor(100)).toBe('#F56C6C')
  })

  it('should return orange for values between 60 and 80', () => {
    const { getProgressColor } = useMockData()
    expect(getProgressColor(61)).toBe('#E6A23C')
    expect(getProgressColor(80)).toBe('#E6A23C')
  })

  it('should return green for values at or below 60', () => {
    const { getProgressColor } = useMockData()
    expect(getProgressColor(60)).toBe('#67C23A')
    expect(getProgressColor(0)).toBe('#67C23A')
  })

  it('should use default count of 30 when not specified', () => {
    const { generateMockData } = useMockData()
    const data = generateMockData(100, 0)
    expect(data).toHaveLength(30)
  })

  it('should generate different values on each call', () => {
    const { generateMockData } = useMockData()
    const data1 = generateMockData(100, 0, 10)
    const data2 = generateMockData(100, 0, 10)
    // While theoretically possible to be equal, extremely unlikely
    expect(data1).not.toEqual(data2)
  })
})
