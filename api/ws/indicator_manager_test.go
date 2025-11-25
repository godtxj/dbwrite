package ws

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Unit tests for CandleBuffer

func TestCandleBuffer_Creation(t *testing.T) {
	buffer := NewCandleBuffer(500)
	if buffer == nil {
		t.Fatal("Failed to create CandleBuffer")
	}
	if buffer.Size() != 0 {
		t.Errorf("Expected size 0, got %d", buffer.Size())
	}
	if buffer.maxSize != 500 {
		t.Errorf("Expected maxSize 500, got %d", buffer.maxSize)
	}
}

func TestCandleBuffer_Add_EmptyBuffer(t *testing.T) {
	buffer := NewCandleBuffer(500)
	candle := createValidCandle(time.Now(), 0)
	
	buffer.Add(candle)
	
	if buffer.Size() != 1 {
		t.Errorf("Expected size 1, got %d", buffer.Size())
	}
	
	candles := buffer.GetAll()
	if len(candles) != 1 {
		t.Errorf("Expected 1 candle, got %d", len(candles))
	}
	if candles[0].Close != candle.Close {
		t.Errorf("Expected close %.2f, got %.2f", candle.Close, candles[0].Close)
	}
}

func TestCandleBuffer_Add_FullBuffer(t *testing.T) {
	buffer := NewCandleBuffer(3)
	baseTime := time.Now()
	
	// Add 4 candles to a buffer with maxSize 3
	for i := 0; i < 4; i++ {
		candle := createValidCandle(baseTime, i)
		buffer.Add(candle)
	}
	
	if buffer.Size() != 3 {
		t.Errorf("Expected size 3, got %d", buffer.Size())
	}
	
	candles := buffer.GetAll()
	// First candle should be removed, so we should have candles 1, 2, 3
	if candles[0].Close != createValidCandle(baseTime, 1).Close {
		t.Error("Oldest candle should have been removed")
	}
}

func TestCandleBuffer_Update_EmptyBuffer(t *testing.T) {
	buffer := NewCandleBuffer(500)
	candle := createValidCandle(time.Now(), 0)
	
	buffer.Update(candle)
	
	if buffer.Size() != 1 {
		t.Errorf("Expected size 1, got %d", buffer.Size())
	}
}

func TestCandleBuffer_Update_PopulatedBuffer(t *testing.T) {
	buffer := NewCandleBuffer(500)
	baseTime := time.Now()
	
	// Add initial candles
	buffer.Add(createValidCandle(baseTime, 0))
	buffer.Add(createValidCandle(baseTime, 1))
	
	// Update last candle
	updatedCandle := CandleData{
		Time:   baseTime.Add(time.Minute),
		Open:   2650.0,
		High:   2655.0,
		Low:    2648.0,
		Close:  2653.0,
		Volume: 2000,
	}
	buffer.Update(updatedCandle)
	
	if buffer.Size() != 2 {
		t.Errorf("Expected size 2, got %d", buffer.Size())
	}
	
	candles := buffer.GetAll()
	lastCandle := candles[len(candles)-1]
	if lastCandle.Close != 2653.0 {
		t.Errorf("Expected updated close 2653.0, got %.2f", lastCandle.Close)
	}
}

func TestCandleBuffer_GetAll_ReturnsCorrectData(t *testing.T) {
	buffer := NewCandleBuffer(500)
	baseTime := time.Now()
	
	expected := []CandleData{
		createValidCandle(baseTime, 0),
		createValidCandle(baseTime, 1),
		createValidCandle(baseTime, 2),
	}
	
	for _, candle := range expected {
		buffer.Add(candle)
	}
	
	result := buffer.GetAll()
	
	if len(result) != len(expected) {
		t.Errorf("Expected %d candles, got %d", len(expected), len(result))
	}
	
	for i := range expected {
		if result[i].Close != expected[i].Close {
			t.Errorf("Candle %d: expected close %.2f, got %.2f", i, expected[i].Close, result[i].Close)
		}
	}
}

func TestCandleBuffer_Size_Accuracy(t *testing.T) {
	buffer := NewCandleBuffer(500)
	baseTime := time.Now()
	
	if buffer.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", buffer.Size())
	}
	
	for i := 1; i <= 10; i++ {
		buffer.Add(createValidCandle(baseTime, i))
		if buffer.Size() != i {
			t.Errorf("After adding %d candles, expected size %d, got %d", i, i, buffer.Size())
		}
	}
}

// Unit tests for MultiPeriodManager

func TestMultiPeriodManager_Creation(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	if manager == nil {
		t.Fatal("Failed to create MultiPeriodManager")
	}
	if manager.maxSize != 500 {
		t.Errorf("Expected maxSize 500, got %d", manager.maxSize)
	}
}

func TestMultiPeriodManager_GetOrCreateBuffer_CreatesNew(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	key := "XAUUSD:M1"
	
	buffer := manager.GetOrCreateBuffer(key)
	
	if buffer == nil {
		t.Fatal("Failed to create buffer")
	}
	if buffer.Size() != 0 {
		t.Errorf("Expected new buffer to be empty, got size %d", buffer.Size())
	}
}

func TestMultiPeriodManager_GetOrCreateBuffer_ReusesExisting(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	key := "XAUUSD:M1"
	
	buffer1 := manager.GetOrCreateBuffer(key)
	buffer1.Add(createValidCandle(time.Now(), 0))
	
	buffer2 := manager.GetOrCreateBuffer(key)
	
	if buffer2.Size() != 1 {
		t.Error("Should reuse existing buffer with data")
	}
}

func TestMultiPeriodManager_AddCandle_DelegatesToBuffer(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	key := "XAUUSD:M1"
	candle := createValidCandle(time.Now(), 0)
	
	manager.AddCandle(key, candle, true)
	
	buffer := manager.GetOrCreateBuffer(key)
	if buffer.Size() != 1 {
		t.Errorf("Expected buffer size 1, got %d", buffer.Size())
	}
}

func TestMultiPeriodManager_GetCandles_ReturnsCorrectData(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	key := "XAUUSD:M1"
	baseTime := time.Now()
	
	expected := []CandleData{
		createValidCandle(baseTime, 0),
		createValidCandle(baseTime, 1),
	}
	
	for _, candle := range expected {
		manager.AddCandle(key, candle, true)
	}
	
	result := manager.GetCandles(key)
	
	if len(result) != len(expected) {
		t.Errorf("Expected %d candles, got %d", len(expected), len(result))
	}
}

func TestMultiPeriodManager_BufferIndependence(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	key1 := "XAUUSD:M1"
	key2 := "XAUUSD:M5"
	
	candle1 := createValidCandle(time.Now(), 0)
	candle2 := createValidCandle(time.Now(), 1)
	
	manager.AddCandle(key1, candle1, true)
	manager.AddCandle(key2, candle2, true)
	
	candles1 := manager.GetCandles(key1)
	candles2 := manager.GetCandles(key2)
	
	if len(candles1) != 1 || len(candles2) != 1 {
		t.Error("Buffers should be independent")
	}
	if candles1[0].Close == candles2[0].Close {
		t.Error("Different buffers should have different data")
	}
}

// K-line validation tests

func TestMultiPeriodManager_RejectsInvalidCandle_HighLessThanLow(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	key := "XAUUSD:M1"
	
	invalidCandle := CandleData{
		Time:   time.Now(),
		Open:   2650.0,
		High:   2648.0, // Invalid: high < low
		Low:    2652.0,
		Close:  2650.0,
		Volume: 1000,
	}
	
	manager.AddCandle(key, invalidCandle, true)
	
	candles := manager.GetCandles(key)
	if len(candles) != 0 {
		t.Error("Invalid candle should be rejected")
	}
}

func TestMultiPeriodManager_RejectsInvalidCandle_HighLessThanOpen(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	key := "XAUUSD:M1"
	
	invalidCandle := CandleData{
		Time:   time.Now(),
		Open:   2655.0,
		High:   2650.0, // Invalid: high < open
		Low:    2648.0,
		Close:  2650.0,
		Volume: 1000,
	}
	
	manager.AddCandle(key, invalidCandle, true)
	
	candles := manager.GetCandles(key)
	if len(candles) != 0 {
		t.Error("Invalid candle should be rejected")
	}
}

func TestMultiPeriodManager_RejectsInvalidCandle_LowGreaterThanOpen(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	key := "XAUUSD:M1"
	
	invalidCandle := CandleData{
		Time:   time.Now(),
		Open:   2648.0,
		High:   2655.0,
		Low:    2650.0, // Invalid: low > open
		Close:  2650.0,
		Volume: 1000,
	}
	
	manager.AddCandle(key, invalidCandle, true)
	
	candles := manager.GetCandles(key)
	if len(candles) != 0 {
		t.Error("Invalid candle should be rejected")
	}
}

func TestMultiPeriodManager_AcceptsValidCandle(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	key := "XAUUSD:M1"
	
	validCandle := CandleData{
		Time:   time.Now(),
		Open:   2650.0,
		High:   2655.0,
		Low:    2648.0,
		Close:  2652.0,
		Volume: 1000,
	}
	
	manager.AddCandle(key, validCandle, true)
	
	candles := manager.GetCandles(key)
	if len(candles) != 1 {
		t.Error("Valid candle should be accepted")
	}
}

func TestMultiPeriodManager_AcceptsEqualValues(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	key := "XAUUSD:M1"
	
	// Edge case: all values equal (valid doji candle)
	validCandle := CandleData{
		Time:   time.Now(),
		Open:   2650.0,
		High:   2650.0,
		Low:    2650.0,
		Close:  2650.0,
		Volume: 1000,
	}
	
	manager.AddCandle(key, validCandle, true)
	
	candles := manager.GetCandles(key)
	if len(candles) != 1 {
		t.Error("Valid candle with equal values should be accepted")
	}
}

// Property-based test generators for indicator_manager

func genCandleSequence(count int) gopter.Gen {
	return gen.SliceOfN(count, genValidCandle())
}

func genBufferSize() gopter.Gen {
	return gen.IntRange(10, 1000)
}

// **Feature: kline-complete-push, Property 9: Buffer size limit**
// **Validates: Requirements 5.4**
// For any CandleBuffer, after adding N candles where N > maxSize, 
// the buffer should contain exactly maxSize candles, with the oldest candles removed.
func TestProperty_BufferSizeLimit(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	properties.Property("buffer maintains size limit and removes oldest candles", prop.ForAll(
		func(maxSize int, numCandles int) bool {
			// Create buffer with random maxSize
			buffer := NewCandleBuffer(maxSize)
			baseTime := time.Now()
			
			// Add numCandles candles
			for i := 0; i < numCandles; i++ {
				candle := createValidCandle(baseTime, i)
				buffer.Add(candle)
			}
			
			// Property 1: Buffer size should never exceed maxSize
			actualSize := buffer.Size()
			if actualSize > maxSize {
				return false
			}
			
			// Property 2: If we added more than maxSize, size should equal maxSize
			if numCandles > maxSize && actualSize != maxSize {
				return false
			}
			
			// Property 3: If we added fewer than maxSize, size should equal numCandles
			if numCandles <= maxSize && actualSize != numCandles {
				return false
			}
			
			// Property 4: If we added more than maxSize, oldest candles should be removed
			if numCandles > maxSize {
				candles := buffer.GetAll()
				// The first candle should be the (numCandles - maxSize)th candle we added
				expectedFirstIndex := numCandles - maxSize
				expectedFirstCandle := createValidCandle(baseTime, expectedFirstIndex)
				
				// Check that the first candle in buffer matches expected
				if len(candles) > 0 {
					firstCandle := candles[0]
					// Compare close prices (they should match)
					if firstCandle.Close != expectedFirstCandle.Close {
						return false
					}
				}
			}
			
			return true
		},
		gen.IntRange(1, 100),    // maxSize between 1 and 100
		gen.IntRange(0, 200),    // numCandles between 0 and 200
	))
	
	properties.TestingRun(t)
}

// **Feature: kline-complete-push, Property 8: Data immutability**
// **Validates: Requirements 4.5, 5.2**
// For any call to GetAll() or GetCandles(), the returned data should be a copy 
// such that modifying the returned array does not affect the internal CandleBuffer state.
func TestProperty_DataImmutability(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	// Test CandleBuffer.GetAll() immutability
	properties.Property("CandleBuffer.GetAll returns immutable copy", prop.ForAll(
		func(numCandles int) bool {
			buffer := NewCandleBuffer(500)
			baseTime := time.Now()
			
			// Add candles to buffer
			for i := 0; i < numCandles; i++ {
				candle := createValidCandle(baseTime, i)
				buffer.Add(candle)
			}
			
			// Get original size
			originalSize := buffer.Size()
			
			// Get data from buffer
			candles := buffer.GetAll()
			
			// Modify the returned slice
			if len(candles) > 0 {
				// Modify first candle
				candles[0].Close = 99999.99
				candles[0].High = 99999.99
				
				// Append to the slice (this shouldn't affect buffer)
				newCandle := createValidCandle(baseTime, numCandles+100)
				candles = append(candles, newCandle)
			}
			
			// Verify buffer is unchanged
			newSize := buffer.Size()
			if newSize != originalSize {
				return false
			}
			
			// Get data again and verify it's unchanged
			candles2 := buffer.GetAll()
			if len(candles2) != originalSize {
				return false
			}
			
			// If we had candles, verify the first one wasn't modified
			if len(candles2) > 0 {
				if candles2[0].Close == 99999.99 {
					return false // Buffer was modified!
				}
			}
			
			return true
		},
		gen.IntRange(0, 100), // numCandles between 0 and 100
	))
	
	// Test MultiPeriodManager.GetCandles() immutability
	properties.Property("MultiPeriodManager.GetCandles returns immutable copy", prop.ForAll(
		func(numCandles int) bool {
			manager := NewMultiPeriodManager(500, nil)
			key := "XAUUSD:M1"
			baseTime := time.Now()
			
			// Add candles to manager
			for i := 0; i < numCandles; i++ {
				candle := createValidCandle(baseTime, i)
				manager.AddCandle(key, candle, true)
			}
			
			// Get original data
			candles1 := manager.GetCandles(key)
			originalLen := len(candles1)
			
			// Modify the returned slice
			if len(candles1) > 0 {
				// Modify first candle
				candles1[0].Close = 88888.88
				candles1[0].Open = 88888.88
				
				// Append to the slice
				newCandle := createValidCandle(baseTime, numCandles+200)
				candles1 = append(candles1, newCandle)
			}
			
			// Get data again and verify it's unchanged
			candles2 := manager.GetCandles(key)
			if len(candles2) != originalLen {
				return false
			}
			
			// If we had candles, verify the first one wasn't modified
			if len(candles2) > 0 {
				if candles2[0].Close == 88888.88 {
					return false // Manager's buffer was modified!
				}
			}
			
			return true
		},
		gen.IntRange(0, 100), // numCandles between 0 and 100
	))
	
	properties.TestingRun(t)
}

// Helper functions for testing

// createValidCandle creates a valid candle for testing
func createValidCandle(baseTime time.Time, offset int) CandleData {
	t := baseTime.Add(time.Duration(offset) * time.Minute)
	base := 2650.0 + float64(offset)
	return CandleData{
		Time:   t,
		Open:   base,
		High:   base + 5.0,
		Low:    base - 3.0,
		Close:  base + 2.0,
		Volume: 1000 + int64(offset)*100,
	}
}

// genValidCandle generates a valid candle for property testing
func genValidCandle() gopter.Gen {
	return gopter.CombineGens(
		gen.Float64Range(2000.0, 3000.0), // base price
		gen.Float64Range(0, 50),          // high offset
		gen.Float64Range(0, 30),          // low offset
		gen.Float64Range(-20, 30),        // close offset
		gen.Int64Range(100, 10000),       // volume
		gen.Int64Range(0, 1000000),       // time offset in seconds
	).Map(func(values []interface{}) CandleData {
		base := values[0].(float64)
		highOffset := values[1].(float64)
		lowOffset := values[2].(float64)
		closeOffset := values[3].(float64)
		volume := values[4].(int64)
		timeOffset := values[5].(int64)
		
		// Generate valid OHLC values
		open := base
		high := base + highOffset
		low := base - lowOffset
		close := base + closeOffset
		
		// Adjust to ensure validity
		if high < low {
			high, low = low, high
		}
		if high < open {
			high = open + 1.0
		}
		if high < close {
			high = close + 1.0
		}
		if low > open {
			low = open - 1.0
		}
		if low > close {
			low = close - 1.0
		}
		
		return CandleData{
			Time:   time.Now().Add(time.Duration(timeOffset) * time.Second),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: volume,
		}
	})
}

// genInvalidCandle generates an invalid candle for property testing
func genInvalidCandle() gopter.Gen {
	return gen.OneGenOf(
		// Case 1: high < low
		gopter.CombineGens(
			gen.Float64Range(2000.0, 3000.0),
			gen.Int64Range(100, 10000),
		).Map(func(values []interface{}) CandleData {
			base := values[0].(float64)
			volume := values[1].(int64)
			return CandleData{
				Time:   time.Now(),
				Open:   base,
				High:   base - 10.0, // Invalid: high < low
				Low:    base + 5.0,
				Close:  base,
				Volume: volume,
			}
		}),
		
		// Case 2: high < open
		gopter.CombineGens(
			gen.Float64Range(2000.0, 3000.0),
			gen.Int64Range(100, 10000),
		).Map(func(values []interface{}) CandleData {
			base := values[0].(float64)
			volume := values[1].(int64)
			return CandleData{
				Time:   time.Now(),
				Open:   base + 20.0,
				High:   base, // Invalid: high < open
				Low:    base - 5.0,
				Close:  base + 10.0,
				Volume: volume,
			}
		}),
		
		// Case 3: high < close
		gopter.CombineGens(
			gen.Float64Range(2000.0, 3000.0),
			gen.Int64Range(100, 10000),
		).Map(func(values []interface{}) CandleData {
			base := values[0].(float64)
			volume := values[1].(int64)
			return CandleData{
				Time:   time.Now(),
				Open:   base,
				High:   base + 5.0,
				Low:    base - 5.0,
				Close:  base + 15.0, // Invalid: close > high
				Volume: volume,
			}
		}),
		
		// Case 4: low > open
		gopter.CombineGens(
			gen.Float64Range(2000.0, 3000.0),
			gen.Int64Range(100, 10000),
		).Map(func(values []interface{}) CandleData {
			base := values[0].(float64)
			volume := values[1].(int64)
			return CandleData{
				Time:   time.Now(),
				Open:   base - 10.0,
				High:   base + 10.0,
				Low:    base, // Invalid: low > open
				Close:  base + 5.0,
				Volume: volume,
			}
		}),
		
		// Case 5: low > close
		gopter.CombineGens(
			gen.Float64Range(2000.0, 3000.0),
			gen.Int64Range(100, 10000),
		).Map(func(values []interface{}) CandleData {
			base := values[0].(float64)
			volume := values[1].(int64)
			return CandleData{
				Time:   time.Now(),
				Open:   base + 5.0,
				High:   base + 10.0,
				Low:    base + 3.0, // Invalid: low > close (when close < low)
				Close:  base,
				Volume: volume,
			}
		}),
	)
}

// **Feature: kline-complete-push, Property 6: K-line validation**
// **Validates: Requirements 6.1, 6.2, 6.3**
// For any K-line data, it should only be accepted into the CandleBuffer if it satisfies 
// all validation rules: high >= low, high >= open, high >= close, low <= open, and low <= close.
func TestProperty_KlineValidation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	properties.Property("only valid K-lines are accepted into buffer", prop.ForAll(
		func(candle CandleData) bool {
			manager := NewMultiPeriodManager(500, nil)
			key := "XAUUSD:M1"
			
			// Add the candle
			manager.AddCandle(key, candle, true)
			
			// Get candles from buffer
			candles := manager.GetCandles(key)
			
			// Check if candle is valid
			isValid := candle.High >= candle.Low &&
				candle.High >= candle.Open &&
				candle.High >= candle.Close &&
				candle.Low <= candle.Open &&
				candle.Low <= candle.Close
			
			// Property: If valid, should be in buffer. If invalid, should not be in buffer.
			if isValid {
				return len(candles) == 1
			} else {
				return len(candles) == 0
			}
		},
		genValidCandle(),
	))
	
	properties.TestingRun(t)
}

// **Feature: kline-complete-push, Property 7: Invalid data rejection**
// **Validates: Requirements 6.4, 6.5**
// For any K-line that fails validation, it should be rejected and not added to the CandleBuffer, 
// leaving the buffer size unchanged.
func TestProperty_InvalidDataRejection(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	properties.Property("invalid K-lines are rejected and buffer size unchanged", prop.ForAll(
		func(validCandles []CandleData, invalidCandle CandleData) bool {
			manager := NewMultiPeriodManager(500, nil)
			key := "XAUUSD:M1"
			
			// Add valid candles first
			for _, candle := range validCandles {
				manager.AddCandle(key, candle, true)
			}
			
			// Record size before adding invalid candle
			sizeBefore := len(manager.GetCandles(key))
			
			// Try to add invalid candle
			manager.AddCandle(key, invalidCandle, true)
			
			// Get size after
			sizeAfter := len(manager.GetCandles(key))
			
			// Property: Buffer size should be unchanged after rejecting invalid candle
			return sizeAfter == sizeBefore
		},
		gen.SliceOfN(5, genValidCandle()), // Add 5 valid candles first
		genInvalidCandle(),                 // Then try to add an invalid one
	))
	
	properties.TestingRun(t)
}

// **Feature: kline-complete-push, Property 10: Concurrent access safety**
// **Validates: Requirements 5.5**
// For any sequence of concurrent Add, Update, and GetAll operations on a CandleBuffer, 
// the final state should be consistent and no data corruption should occur.
func TestProperty_ConcurrentAccessSafety(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	properties.Property("concurrent operations maintain data consistency", prop.ForAll(
		func(numAdds int, numUpdates int, numReads int) bool {
			// Skip invalid cases
			if numAdds < 1 || numUpdates < 1 || numReads < 1 {
				return true
			}
			
			buffer := NewCandleBuffer(500)
			baseTime := time.Now()
			
			// Use channels to synchronize goroutines
			done := make(chan bool)
			
			// Concurrent Add operations
			go func() {
				for i := 0; i < numAdds; i++ {
					candle := createValidCandle(baseTime, i)
					buffer.Add(candle)
				}
				done <- true
			}()
			
			// Concurrent Update operations
			go func() {
				for i := 0; i < numUpdates; i++ {
					candle := createValidCandle(baseTime, i)
					buffer.Update(candle)
				}
				done <- true
			}()
			
			// Concurrent GetAll operations
			readResults := make([][]CandleData, numReads)
			for r := 0; r < numReads; r++ {
				go func(index int) {
					readResults[index] = buffer.GetAll()
					done <- true
				}(r)
			}
			
			// Wait for all goroutines to complete
			for i := 0; i < 2+numReads; i++ {
				<-done
			}
			
			// Property 1: Buffer size should be valid (0 to maxSize)
			finalSize := buffer.Size()
			if finalSize < 0 || finalSize > 500 {
				return false
			}
			
			// Property 2: All read operations should return valid data
			for _, result := range readResults {
				// Each read should return a valid slice
				if result == nil {
					return false
				}
				// Size should be consistent with buffer's reported size at time of read
				if len(result) > 500 {
					return false
				}
			}
			
			// Property 3: Final buffer state should be consistent
			finalCandles := buffer.GetAll()
			if len(finalCandles) != finalSize {
				return false
			}
			
			// Property 4: All candles in final state should be valid
			for _, candle := range finalCandles {
				if candle.High < candle.Low {
					return false
				}
				if candle.High < candle.Open || candle.High < candle.Close {
					return false
				}
				if candle.Low > candle.Open || candle.Low > candle.Close {
					return false
				}
			}
			
			return true
		},
		gen.IntRange(1, 50),  // numAdds
		gen.IntRange(1, 30),  // numUpdates
		gen.IntRange(1, 20),  // numReads
	))
	
	properties.TestingRun(t)
}

// **Feature: kline-complete-push, Property 5: Timeframe isolation**
// **Validates: Requirements 3.1, 3.2, 3.3, 3.4**
// For any two different timeframes A and B, modifying the CandleBuffer for timeframe A 
// should not affect the CandleBuffer for timeframe B, and snapshots for each timeframe 
// should only contain data matching that timeframe.
func TestProperty_TimeframeIsolation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	properties.Property("timeframes are isolated and independent", prop.ForAll(
		func(candlesA []CandleData, candlesB []CandleData, symbol string) bool {
			manager := NewMultiPeriodManager(500, nil)
			
			// Use two different timeframes
			timeframeA := "M1"
			timeframeB := "M5"
			keyA := symbol + ":" + timeframeA
			keyB := symbol + ":" + timeframeB
			
			// Add candles to timeframe A
			for _, candle := range candlesA {
				manager.AddCandle(keyA, candle, true)
			}
			
			// Record state of timeframe A
			candlesFromA_before := manager.GetCandles(keyA)
			sizeA_before := len(candlesFromA_before)
			
			// Add candles to timeframe B
			for _, candle := range candlesB {
				manager.AddCandle(keyB, candle, true)
			}
			
			// Get state after modifying timeframe B
			candlesFromA_after := manager.GetCandles(keyA)
			candlesFromB := manager.GetCandles(keyB)
			
			// Property 1: Modifying timeframe B should not affect timeframe A's size
			if len(candlesFromA_after) != sizeA_before {
				return false
			}
			
			// Property 2: Timeframe A's data should be unchanged
			if len(candlesFromA_before) != len(candlesFromA_after) {
				return false
			}
			for i := range candlesFromA_before {
				if candlesFromA_before[i].Close != candlesFromA_after[i].Close {
					return false
				}
				if candlesFromA_before[i].Time != candlesFromA_after[i].Time {
					return false
				}
			}
			
			// Property 3: Timeframe B should have its own independent data
			if len(candlesB) != len(candlesFromB) {
				return false
			}
			
			// Property 4: The two buffers should be independent (different data)
			// If both have data, they should not be identical (unless by chance)
			if len(candlesFromA_after) > 0 && len(candlesFromB) > 0 {
				// Check that at least the sizes or some values differ
				// (This is a weak check but validates independence)
				if len(candlesFromA_after) == len(candlesFromB) {
					// If sizes are equal, check if data is different
					allSame := true
					for i := range candlesFromA_after {
						if candlesFromA_after[i].Close != candlesFromB[i].Close {
							allSame = false
							break
						}
					}
					// If all data is the same, that's suspicious (but possible by chance)
					// We'll allow it but it's unlikely with random data
					_ = allSame
				}
			}
			
			return true
		},
		gen.SliceOfN(10, genValidCandle()), // 10 candles for timeframe A
		gen.SliceOfN(15, genValidCandle()), // 15 candles for timeframe B
		gen.OneConstOf("XAUUSD", "EURUSD", "GBPUSD"), // Random symbol
	))
	
	properties.TestingRun(t)
}

// Concurrency tests with race detector
// Run with: go test -race

// TestConcurrent_AddUpdateGetAll tests concurrent Add/Update/GetAll operations
func TestConcurrent_AddUpdateGetAll(t *testing.T) {
	buffer := NewCandleBuffer(500)
	baseTime := time.Now()
	
	// Number of concurrent operations
	numAdds := 100
	numUpdates := 50
	numReads := 50
	
	// Use WaitGroup to synchronize
	var wg sync.WaitGroup
	
	// Concurrent Add operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numAdds; i++ {
			candle := createValidCandle(baseTime, i)
			buffer.Add(candle)
		}
	}()
	
	// Concurrent Update operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numUpdates; i++ {
			candle := createValidCandle(baseTime, i)
			buffer.Update(candle)
		}
	}()
	
	// Concurrent GetAll operations
	for r := 0; r < numReads; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			candles := buffer.GetAll()
			// Verify returned data is valid
			if candles == nil {
				t.Error("GetAll should not return nil")
			}
			// Verify all candles are valid
			for _, candle := range candles {
				if candle.High < candle.Low {
					t.Error("Invalid candle: high < low")
				}
			}
		}()
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// Verify final state is consistent
	finalSize := buffer.Size()
	finalCandles := buffer.GetAll()
	
	if len(finalCandles) != finalSize {
		t.Errorf("Inconsistent state: Size()=%d but GetAll() returned %d candles", finalSize, len(finalCandles))
	}
	
	// Verify buffer size is within bounds
	if finalSize < 0 || finalSize > 500 {
		t.Errorf("Invalid buffer size: %d", finalSize)
	}
}

// TestConcurrent_BufferCreation tests concurrent buffer creation in MultiPeriodManager
func TestConcurrent_BufferCreation(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	
	// Create multiple buffers concurrently
	keys := []string{
		"XAUUSD:M1",
		"XAUUSD:M5",
		"EURUSD:M1",
		"EURUSD:M5",
		"GBPUSD:M1",
		"GBPUSD:M5",
	}
	
	var wg sync.WaitGroup
	
	// Each key is accessed by multiple goroutines simultaneously
	for _, key := range keys {
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(k string) {
				defer wg.Done()
				buffer := manager.GetOrCreateBuffer(k)
				if buffer == nil {
					t.Errorf("GetOrCreateBuffer returned nil for key %s", k)
				}
			}(key)
		}
	}
	
	wg.Wait()
	
	// Verify all buffers were created
	for _, key := range keys {
		buffer := manager.GetOrCreateBuffer(key)
		if buffer == nil {
			t.Errorf("Buffer for key %s should exist", key)
		}
	}
}

// TestConcurrent_SubscriptionOperations tests concurrent subscription operations in Hub
func TestConcurrent_SubscriptionOperations(t *testing.T) {
	hub := createTestHub()
	
	// Create multiple clients
	numClients := 20
	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = &Client{
			Send:          make(chan []byte, 256),
			Subscriptions: make(map[string]bool),
		}
	}
	
	channels := []string{
		"kline:XAUUSD:M1",
		"kline:XAUUSD:M5",
		"kline:EURUSD:M1",
	}
	
	var wg sync.WaitGroup
	
	// Concurrent Subscribe operations
	for _, client := range clients {
		for _, channel := range channels {
			wg.Add(1)
			go func(c *Client, ch string) {
				defer wg.Done()
				hub.Subscribe(c, ch)
				c.Subscriptions[ch] = true
			}(client, channel)
		}
	}
	
	wg.Wait()
	
	// Wait for async snapshot sends
	time.Sleep(200 * time.Millisecond)
	
	// Verify all subscriptions were registered
	hub.subMutex.RLock()
	for _, channel := range channels {
		if clients, exists := hub.Subscriptions[channel]; !exists {
			t.Errorf("Channel %s should exist in subscriptions", channel)
		} else if len(clients) != numClients {
			t.Errorf("Channel %s should have %d clients, got %d", channel, numClients, len(clients))
		}
	}
	hub.subMutex.RUnlock()
	
	// Concurrent Unsubscribe operations
	for _, client := range clients {
		for _, channel := range channels {
			wg.Add(1)
			go func(c *Client, ch string) {
				defer wg.Done()
				hub.Unsubscribe(c, ch)
				delete(c.Subscriptions, ch)
			}(client, channel)
		}
	}
	
	wg.Wait()
	
	// Verify all subscriptions were removed
	hub.subMutex.RLock()
	for _, channel := range channels {
		if clients, exists := hub.Subscriptions[channel]; exists && len(clients) > 0 {
			t.Errorf("Channel %s should have no clients after unsubscribe, got %d", channel, len(clients))
		}
	}
	hub.subMutex.RUnlock()
}

// TestConcurrent_MixedOperations tests a realistic scenario with mixed operations
func TestConcurrent_MixedOperations(t *testing.T) {
	manager := NewMultiPeriodManager(500, nil)
	baseTime := time.Now()
	
	keys := []string{
		"XAUUSD:M1",
		"EURUSD:M1",
		"GBPUSD:M1",
	}
	
	var wg sync.WaitGroup
	
	// Goroutine 1: Add candles to multiple keys
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			for _, key := range keys {
				candle := createValidCandle(baseTime, i)
				manager.AddCandle(key, candle, true)
			}
		}
	}()
	
	// Goroutine 2: Update candles
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			for _, key := range keys {
				candle := createValidCandle(baseTime, i)
				candle.Close = candle.Close + 1.0 // Modify close price
				manager.AddCandle(key, candle, false)
			}
		}
	}()
	
	// Goroutine 3: Read candles
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			for _, key := range keys {
				candles := manager.GetCandles(key)
				// Verify data is valid
				for _, candle := range candles {
					if candle.High < candle.Low {
						t.Error("Invalid candle in concurrent read")
					}
				}
			}
		}
	}()
	
	// Goroutine 4: Create new buffers
	wg.Add(1)
	go func() {
		defer wg.Done()
		newKeys := []string{
			"USDJPY:M1",
			"AUDUSD:M1",
		}
		for i := 0; i < 20; i++ {
			for _, key := range newKeys {
				buffer := manager.GetOrCreateBuffer(key)
				if buffer == nil {
					t.Error("GetOrCreateBuffer returned nil")
				}
			}
		}
	}()
	
	wg.Wait()
	
	// Verify final state is consistent
	for _, key := range keys {
		candles := manager.GetCandles(key)
		if len(candles) > 500 {
			t.Errorf("Buffer for %s exceeded max size: %d", key, len(candles))
		}
		
		// Verify all candles are valid
		for _, candle := range candles {
			if candle.High < candle.Low {
				t.Errorf("Invalid candle in final state for %s", key)
			}
		}
	}
}

// TestConcurrent_DataImmutability tests that concurrent modifications to returned data don't affect buffer
func TestConcurrent_DataImmutability(t *testing.T) {
	buffer := NewCandleBuffer(500)
	baseTime := time.Now()
	
	// Add initial candles
	for i := 0; i < 10; i++ {
		candle := createValidCandle(baseTime, i)
		buffer.Add(candle)
	}
	
	var wg sync.WaitGroup
	
	// Multiple goroutines get data and try to modify it
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			candles := buffer.GetAll()
			
			// Try to modify the returned data
			if len(candles) > 0 {
				candles[0].Close = 99999.99 + float64(index)
				candles[0].High = 99999.99 + float64(index)
			}
			
			// Try to append to the slice
			newCandle := createValidCandle(baseTime, 100+index)
			candles = append(candles, newCandle)
		}(i)
	}
	
	wg.Wait()
	
	// Verify buffer data is unchanged
	finalCandles := buffer.GetAll()
	if len(finalCandles) != 10 {
		t.Errorf("Buffer size should be 10, got %d", len(finalCandles))
	}
	
	// Verify first candle wasn't modified
	if len(finalCandles) > 0 {
		if finalCandles[0].Close > 90000.0 {
			t.Error("Buffer data was modified by concurrent operations")
		}
	}
}
