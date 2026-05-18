export function useMockData() {
  const generateMockData = (max: number, min: number, count: number = 30) =>
    Array(count).fill(0).map(() => Math.random() * max + min)
  
  const getProgressColor = (value: number): string => {
    if (value > 80) return '#F56C6C'
    if (value > 60) return '#E6A23C'
    return '#67C23A'
  }
  
  return { generateMockData, getProgressColor }
}
