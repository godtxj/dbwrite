package indicators

import "math"

// GreenArrowParams 绿箭侠指标参数
type GreenArrowParams struct {
	Length    int     // 布林带周期
	Deviation int     // 布林带偏差
	MoneyRisk float64 // 风险系数
	Signal    int     // 信号模式 (1=显示信号, 2=仅趋势线)
	Line      int     // 显示趋势线 (1=显示, 0=隐藏)
}

// GreenArrowResult 绿箭侠指标单个K线的计算结果
type GreenArrowResult struct {
	UpStop     float64 `json:"up_stop"`      // 上升趋势止损点
	DownStop   float64 `json:"down_stop"`    // 下降趋势止损点
	UpSignal   float64 `json:"up_signal"`    // 上升信号
	DownSignal float64 `json:"down_signal"`  // 下降信号
	UpLine     float64 `json:"up_line"`      // 上升趋势线
	DownLine   float64 `json:"down_line"`    // 下降趋势线
	Trend      int     `json:"trend"`        // 当前趋势 (1=上升, -1=下降, 0=无趋势)
	IsSignal   bool    `json:"is_signal"`    // 是否为新信号
}

// Candle K线数据
type Candle struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

const (
	EMPTY_VALUE = math.MaxFloat64 // 空值标记
)

// CalculateGreenArrow 计算绿箭侠指标
// candles: K线数组 (从旧到新排列, candles[0]是最旧的)
// params: 指标参数
// 返回: 指标结果数组 (从旧到新排列,与输入candles对应)
func CalculateGreenArrow(candles []Candle, params GreenArrowParams) []GreenArrowResult {
	n := len(candles)
	if n < params.Length {
		return []GreenArrowResult{}
	}

	// 提取收盘价
	closes := make([]float64, n)
	for i := 0; i < n; i++ {
		closes[i] = candles[i].Close
	}

	// 计算布林带序列
	bbSeries := CalculateBollingerBandsSeries(closes, params.Length, float64(params.Deviation))

	// 初始化结果数组
	results := make([]GreenArrowResult, n)
	for i := 0; i < n; i++ {
		results[i] = GreenArrowResult{
			UpStop:     -1.0,
			DownStop:   -1.0,
			UpSignal:   -1.0,
			DownSignal: -1.0,
			UpLine:     EMPTY_VALUE,
			DownLine:   EMPTY_VALUE,
			Trend:      0,
			IsSignal:   false,
		}
	}

	// 趋势状态
	trend := 0

	// 用于平滑的布林带和止损位
	upperBand := make([]float64, n)
	lowerBand := make([]float64, n)
	upperStop := make([]float64, n)
	lowerStop := make([]float64, n)

	// 主计算循环 (从旧到新)
	for i := params.Length - 1; i < n; i++ {
		// 1. 获取原始布林带
		upperBand[i] = bbSeries[i].Upper
		lowerBand[i] = bbSeries[i].Lower

		// 2. 判断趋势
		if i > 0 {
			if closes[i] > upperBand[i-1] {
				trend = 1
			}
			if closes[i] < lowerBand[i-1] {
				trend = -1
			}
		}

		// 3. 布林带平滑处理 (关键步骤)
		if i > 0 {
			if trend > 0 && lowerBand[i] < lowerBand[i-1] {
				lowerBand[i] = lowerBand[i-1]
			}
			if trend < 0 && upperBand[i] > upperBand[i-1] {
				upperBand[i] = upperBand[i-1]
			}
		}

		// 4. 计算动态止损位
		bandWidth := upperBand[i] - lowerBand[i]
		riskFactor := (params.MoneyRisk - 1.0) / 2.0
		upperStop[i] = upperBand[i] + riskFactor*bandWidth
		lowerStop[i] = lowerBand[i] - riskFactor*bandWidth

		// 5. 止损位平滑处理
		if i > 0 {
			if trend > 0 && lowerStop[i] < lowerStop[i-1] {
				lowerStop[i] = lowerStop[i-1]
			}
			if trend < 0 && upperStop[i] > upperStop[i-1] {
				upperStop[i] = upperStop[i-1]
			}
		}

		// 6. 更新指标缓冲区
		results[i].Trend = trend
		updateBuffers(&results[i], i, upperStop, lowerStop, trend, params, results)
	}

	return results
}

// updateBuffers 更新指标缓冲区
func updateBuffers(result *GreenArrowResult, index int, upperStop, lowerStop []float64, trend int, params GreenArrowParams, results []GreenArrowResult) {
	if trend > 0 {
		// 上升趋势
		updateUpTrendBuffers(result, index, lowerStop[index], params, results)
		clearDownTrendBuffers(result)
	} else if trend < 0 {
		// 下降趋势
		updateDownTrendBuffers(result, index, upperStop[index], params, results)
		clearUpTrendBuffers(result)
	}
}

// updateUpTrendBuffers 更新上升趋势缓冲区
func updateUpTrendBuffers(result *GreenArrowResult, index int, stopLevel float64, params GreenArrowParams, results []GreenArrowResult) {
	// 判断是否为新信号
	// MQ4: (shift == Nbars - 1 || UpStopBuffer[shift + 1] == -1.0)
	// Go: (index == params.Length - 1 || results[index-1].UpStop == -1.0)
	isNewSignal := false
	if params.Signal > 0 {
		if index == params.Length-1 {
			// 第一根可计算的K线
			isNewSignal = true
		} else if results[index-1].UpStop == -1.0 {
			// 前一根K线没有上升趋势
			isNewSignal = true
		}
	}

	result.IsSignal = isNewSignal

	if isNewSignal {
		result.UpSignal = stopLevel
		result.UpStop = stopLevel
		if params.Line > 0 {
			result.UpLine = stopLevel
		}
	} else {
		result.UpStop = stopLevel
		if params.Line > 0 {
			result.UpLine = stopLevel
		}
		result.UpSignal = -1.0
	}

	// 如果Signal==2,隐藏止损点
	if params.Signal == 2 {
		result.UpStop = 0
	}
}

// updateDownTrendBuffers 更新下降趋势缓冲区
func updateDownTrendBuffers(result *GreenArrowResult, index int, stopLevel float64, params GreenArrowParams, results []GreenArrowResult) {
	// 判断是否为新信号
	// MQ4: (shift == Nbars - 1 || DownStopBuffer[shift + 1] == -1.0)
	// Go: (index == params.Length - 1 || results[index-1].DownStop == -1.0)
	isNewSignal := false
	if params.Signal > 0 {
		if index == params.Length-1 {
			// 第一根可计算的K线
			isNewSignal = true
		} else if results[index-1].DownStop == -1.0 {
			// 前一根K线没有下降趋势
			isNewSignal = true
		}
	}

	result.IsSignal = isNewSignal

	if isNewSignal {
		result.DownSignal = stopLevel
		result.DownStop = stopLevel
		if params.Line > 0 {
			result.DownLine = stopLevel
		}
	} else {
		result.DownStop = stopLevel
		if params.Line > 0 {
			result.DownLine = stopLevel
		}
		result.DownSignal = -1.0
	}

	// 如果Signal==2,隐藏止损点
	if params.Signal == 2 {
		result.DownStop = 0
	}
}

// clearUpTrendBuffers 清除上升趋势缓冲区
func clearUpTrendBuffers(result *GreenArrowResult) {
	result.UpSignal = -1.0
	result.UpStop = -1.0
	result.UpLine = EMPTY_VALUE
}

// clearDownTrendBuffers 清除下降趋势缓冲区
func clearDownTrendBuffers(result *GreenArrowResult) {
	result.DownSignal = -1.0
	result.DownStop = -1.0
	result.DownLine = EMPTY_VALUE
}
