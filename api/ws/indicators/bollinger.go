package indicators

import "math"

// BollingerBands 布林带计算结果
type BollingerBands struct {
	Upper  float64 // 上轨
	Middle float64 // 中轨 (SMA)
	Lower  float64 // 下轨
}

// CalculateSMA 计算简单移动平均线
func CalculateSMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	return sum / float64(period)
}

// CalculateStdDev 计算标准差
func CalculateStdDev(prices []float64, period int, mean float64) float64 {
	if len(prices) < period {
		return 0
	}

	variance := 0.0
	for i := 0; i < period; i++ {
		diff := prices[i] - mean
		variance += diff * diff
	}
	variance /= float64(period)
	return math.Sqrt(variance)
}

// CalculateBollingerBands 计算布林带
// prices: 价格数组 (从新到旧排列,prices[0]是最新价格)
// period: 周期
// deviation: 标准差倍数
func CalculateBollingerBands(prices []float64, period int, deviation float64) BollingerBands {
	if len(prices) < period {
		return BollingerBands{}
	}

	// 计算中轨 (SMA)
	middle := CalculateSMA(prices, period)

	// 计算标准差
	stdDev := CalculateStdDev(prices, period, middle)

	// 计算上下轨
	upper := middle + deviation*stdDev
	lower := middle - deviation*stdDev

	return BollingerBands{
		Upper:  upper,
		Middle: middle,
		Lower:  lower,
	}
}

// CalculateBollingerBandsSeries 批量计算布林带序列
// prices: 价格数组 (从旧到新排列)
// period: 周期
// deviation: 标准差倍数
// 返回: 布林带数组 (从旧到新排列,与输入prices对应)
func CalculateBollingerBandsSeries(prices []float64, period int, deviation float64) []BollingerBands {
	n := len(prices)
	if n < period {
		return []BollingerBands{}
	}

	result := make([]BollingerBands, n)

	// 从第period-1个位置开始计算
	for i := period - 1; i < n; i++ {
		// 获取当前位置往前period个价格 (需要反转顺序)
		window := make([]float64, period)
		for j := 0; j < period; j++ {
			window[j] = prices[i-j] // 从新到旧
		}

		result[i] = CalculateBollingerBands(window, period, deviation)
	}

	return result
}
