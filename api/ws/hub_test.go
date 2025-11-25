package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/redis/go-redis/v9"
)

// MockRedisClient is a mock implementation of Redis client for testing
type MockRedisClient struct {
	*redis.Client
	publishedMessages []MockPublishedMessage
}

type MockPublishedMessage struct {
	Channel string
	Payload interface{}
}

func (m *MockRedisClient) Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	m.publishedMessages = append(m.publishedMessages, MockPublishedMessage{
		Channel: channel,
		Payload: message,
	})
	return redis.NewIntCmd(ctx)
}

// MockDB is a mock implementation of database for testing
type MockDB struct {
	*sqlx.DB
	candles map[string][]CandleData
}

func NewMockDB() *MockDB {
	return &MockDB{
		candles: make(map[string][]CandleData),
	}
}

func (m *MockDB) AddMockCandles(symbol, timeframe string, candles []CandleData) {
	key := symbol + ":" + timeframe
	m.candles[key] = candles
}

// Test helper functions

func createTestHub() *Hub {
	mockRedis := &MockRedisClient{}
	mockDB := NewMockDB()
	return NewHub(500, &mockRedis.Client, &mockDB.DB)
}

func createTestCandle(t time.Time, open, high, low, close float64, volume int64) CandleData {
	return CandleData{
		Time:   t,
		Open:   open,
		High:   high,
		Low:    low,
		Close:  close,
		Volume: volume,
	}
}

func createValidCandle(baseTime time.Time, offset int) CandleData {
	t := baseTime.Add(time.Duration(offset) * time.Minute)
	base := 2650.0 + float64(offset)*0.1
	return CandleData{
		Time:   t,
		Open:   base,
		High:   base + 1.0,
		Low:    base - 1.0,
		Close:  base + 0.5,
		Volume: 1000 + int64(offset)*10,
	}
}

// Basic unit tests

func TestHub_Creation(t *testing.T) {
	hub := createTestHub()
	if hub == nil {
		t.Fatal("Failed to create hub")
	}
	if hub.Clients == nil {
		t.Error("Hub.Clients should not be nil")
	}
	if hub.Subscriptions == nil {
		t.Error("Hub.Subscriptions should not be nil")
	}
	if hub.indicatorManager == nil {
		t.Error("Hub.indicatorManager should not be nil")
	}
}

func TestHub_Subscribe(t *testing.T) {
	hub := createTestHub()
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	
	channel := "kline:XAUUSD:M1"
	hub.Subscribe(client, channel)
	
	// Give some time for async snapshot send
	time.Sleep(100 * time.Millisecond)
	
	hub.subMutex.RLock()
	clients, exists := hub.Subscriptions[channel]
	hub.subMutex.RUnlock()
	
	if !exists {
		t.Error("Channel should exist in subscriptions")
	}
	if !clients[client] {
		t.Error("Client should be subscribed to channel")
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	hub := createTestHub()
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	
	channel := "kline:XAUUSD:M1"
	hub.Subscribe(client, channel)
	time.Sleep(50 * time.Millisecond)
	
	hub.Unsubscribe(client, channel)
	
	hub.subMutex.RLock()
	clients, exists := hub.Subscriptions[channel]
	hub.subMutex.RUnlock()
	
	if exists && clients[client] {
		t.Error("Client should not be subscribed after unsubscribe")
	}
}

func TestSnapshotMessage_Serialization(t *testing.T) {
	baseTime := time.Now()
	candles := []CandleData{
		createValidCandle(baseTime, 0),
		createValidCandle(baseTime, 1),
	}
	
	snapshot := SnapshotMessage{
		Type:      "snapshot",
		Symbol:    "XAUUSD",
		Timeframe: "M1",
		Data:      candles,
	}
	
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot: %v", err)
	}
	
	var decoded SnapshotMessage
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}
	
	if decoded.Type != "snapshot" {
		t.Errorf("Expected type 'snapshot', got '%s'", decoded.Type)
	}
	if decoded.Symbol != "XAUUSD" {
		t.Errorf("Expected symbol 'XAUUSD', got '%s'", decoded.Symbol)
	}
	if decoded.Timeframe != "M1" {
		t.Errorf("Expected timeframe 'M1', got '%s'", decoded.Timeframe)
	}
	if len(decoded.Data) != 2 {
		t.Errorf("Expected 2 candles, got %d", len(decoded.Data))
	}
}

// Property-based test generators

func genValidCandle() gopter.Gen {
	return gopter.CombineGens(
		gen.Int64Range(1000, 10000),     // volume
		gen.Float64Range(2600.0, 2700.0), // base price
		gen.Float64Range(0.1, 5.0),       // spread
	).Map(func(values []interface{}) CandleData {
		volume := values[0].(int64)
		base := values[1].(float64)
		spread := values[2].(float64)
		
		open := base
		high := base + spread
		low := base - spread
		close := base + spread/2
		
		return CandleData{
			Time:   time.Now(),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close,
			Volume: volume,
		}
	})
}

func genInvalidCandle() gopter.Gen {
	return gen.OneGenOf(
		// High < Low
		gopter.CombineGens(
			gen.Float64Range(2600.0, 2700.0),
			gen.Float64Range(0.1, 5.0),
		).Map(func(values []interface{}) CandleData {
			base := values[0].(float64)
			spread := values[1].(float64)
			return CandleData{
				Time:   time.Now(),
				Open:   base,
				High:   base - spread, // Invalid: high < low
				Low:    base + spread,
				Close:  base,
				Volume: 1000,
			}
		}),
		// High < Open
		gopter.CombineGens(
			gen.Float64Range(2600.0, 2700.0),
			gen.Float64Range(0.1, 5.0),
		).Map(func(values []interface{}) CandleData {
			base := values[0].(float64)
			spread := values[1].(float64)
			return CandleData{
				Time:   time.Now(),
				Open:   base + spread,
				High:   base, // Invalid: high < open
				Low:    base - spread,
				Close:  base,
				Volume: 1000,
			}
		}),
	)
}

// **Feature: kline-complete-push, Property 1: Complete snapshot delivery**
// **Validates: Requirements 1.1, 2.1, 2.2, 2.3**
// For any K-line update (UPDATE or CLOSE) received from Redis, the API Hub should send 
// a complete snapshot containing all candles in the buffer to all subscribed clients.
func TestProperty_CompleteSnapshotDelivery(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	properties.Property("complete snapshot sent on K-line update", prop.ForAll(
		func(numCandles int, isNew bool) bool {
			hub := createTestHub()
			baseTime := time.Now()
			symbol := "XAUUSD"
			timeframe := "M1"
			key := symbol + ":" + timeframe
			
			// Pre-populate buffer with candles
			for i := 0; i < numCandles; i++ {
				candle := createValidCandle(baseTime, i)
				hub.indicatorManager.AddCandle(key, candle, true)
			}
			
			// Create mock clients
			client1 := &Client{
				Send:          make(chan []byte, 256),
				Subscriptions: make(map[string]bool),
			}
			client2 := &Client{
				Send:          make(chan []byte, 256),
				Subscriptions: make(map[string]bool),
			}
			
			// Subscribe clients to channel
			channel := "kline:" + symbol + ":" + timeframe
			hub.Subscribe(client1, channel)
			hub.Subscribe(client2, channel)
			
			// Wait for initial snapshots to be sent
			time.Sleep(50 * time.Millisecond)
			
			// Clear the channels
			for len(client1.Send) > 0 {
				<-client1.Send
			}
			for len(client2.Send) > 0 {
				<-client2.Send
			}
			
			// Create a K-line update message
			status := "UPDATE"
			if isNew {
				status = "CLOSE"
			}
			
			newCandle := createValidCandle(baseTime, numCandles)
			msg := map[string]interface{}{
				"status": status,
				"candle": map[string]interface{}{
					"symbol":     symbol,
					"timeframe":  timeframe,
					"start_time": newCandle.Time,
					"open":       newCandle.Open,
					"high":       newCandle.High,
					"low":        newCandle.Low,
					"close":      newCandle.Close,
					"volume":     newCandle.Volume,
				},
			}
			
			payload, _ := json.Marshal(msg)
			redisMsg := &redis.Message{
				Channel: channel,
				Payload: string(payload),
			}
			
			// Process the message
			hub.handleKlineMessage(redisMsg)
			
			// Wait for messages to be sent
			time.Sleep(50 * time.Millisecond)
			
			// Property 1: Both clients should receive a message
			if len(client1.Send) == 0 || len(client2.Send) == 0 {
				return false
			}
			
			// Get the messages
			msg1 := <-client1.Send
			msg2 := <-client2.Send
			
			// Property 2: Messages should be snapshots
			var snapshot1, snapshot2 SnapshotMessage
			if err := json.Unmarshal(msg1, &snapshot1); err != nil {
				return false
			}
			if err := json.Unmarshal(msg2, &snapshot2); err != nil {
				return false
			}
			
			// Property 3: Snapshots should have type "snapshot"
			if snapshot1.Type != "snapshot" || snapshot2.Type != "snapshot" {
				return false
			}
			
			// Property 4: Snapshots should contain all candles in buffer
			expectedCount := numCandles
			if isNew {
				expectedCount++ // New candle was added
			}
			if len(snapshot1.Data) != expectedCount || len(snapshot2.Data) != expectedCount {
				return false
			}
			
			// Property 5: Both clients should receive identical snapshots
			if len(snapshot1.Data) != len(snapshot2.Data) {
				return false
			}
			
			return true
		},
		gen.IntRange(1, 50),  // numCandles between 1 and 50
		gen.Bool(),           // isNew (UPDATE or CLOSE)
	))
	
	properties.TestingRun(t)
}

// **Feature: kline-complete-push, Property 2: Snapshot completeness and ordering**
// **Validates: Requirements 1.2, 1.3, 1.5**
// For any snapshot sent to a client, the data array should contain all available candles 
// (up to 500) in chronological order from oldest to newest, with no gaps in the time sequence.
func TestProperty_SnapshotCompletenessAndOrdering(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	properties.Property("snapshots are complete and chronologically ordered", prop.ForAll(
		func(numCandles int) bool {
			hub := createTestHub()
			baseTime := time.Now()
			symbol := "XAUUSD"
			timeframe := "M1"
			key := symbol + ":" + timeframe
			
			// Add candles to buffer
			for i := 0; i < numCandles; i++ {
				candle := createValidCandle(baseTime, i)
				hub.indicatorManager.AddCandle(key, candle, true)
			}
			
			// Create a client
			client := &Client{
				Send:          make(chan []byte, 256),
				Subscriptions: make(map[string]bool),
			}
			
			// Subscribe to trigger snapshot
			channel := "kline:" + symbol + ":" + timeframe
			hub.Subscribe(client, channel)
			
			// Wait for snapshot
			time.Sleep(50 * time.Millisecond)
			
			// Get the snapshot message
			if len(client.Send) == 0 {
				return numCandles == 0 // If no candles, no snapshot is expected
			}
			
			msg := <-client.Send
			var snapshot SnapshotMessage
			if err := json.Unmarshal(msg, &snapshot); err != nil {
				return false
			}
			
			// Property 1: Snapshot should contain all candles (up to maxSize)
			expectedCount := numCandles
			if expectedCount > 500 {
				expectedCount = 500
			}
			if len(snapshot.Data) != expectedCount {
				return false
			}
			
			// Property 2: Candles should be in chronological order (oldest to newest)
			for i := 1; i < len(snapshot.Data); i++ {
				if !snapshot.Data[i].Time.After(snapshot.Data[i-1].Time) {
					return false // Not in chronological order
				}
			}
			
			// Property 3: Time sequence should have no gaps (1 minute intervals for M1)
			if len(snapshot.Data) > 1 {
				for i := 1; i < len(snapshot.Data); i++ {
					expectedTime := snapshot.Data[i-1].Time.Add(time.Minute)
					if !snapshot.Data[i].Time.Equal(expectedTime) {
						return false // Gap in time sequence
					}
				}
			}
			
			return true
		},
		gen.IntRange(0, 600), // numCandles between 0 and 600 (tests buffer limit)
	))
	
	properties.TestingRun(t)
}

// **Feature: kline-complete-push, Property 3: Message structure consistency**
// **Validates: Requirements 1.4, 4.2**
// For any snapshot message, it should contain exactly the fields "type" (with value "snapshot"), 
// "symbol", "timeframe", and "data" (as an array).
func TestProperty_MessageStructureConsistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	properties.Property("snapshot messages have consistent structure", prop.ForAll(
		func(numCandles int, symbol string, timeframe string) bool {
			hub := createTestHub()
			baseTime := time.Now()
			key := symbol + ":" + timeframe
			
			// Add candles to buffer
			for i := 0; i < numCandles; i++ {
				candle := createValidCandle(baseTime, i)
				hub.indicatorManager.AddCandle(key, candle, true)
			}
			
			// Create a client
			client := &Client{
				Send:          make(chan []byte, 256),
				Subscriptions: make(map[string]bool),
			}
			
			// Subscribe to trigger snapshot
			channel := "kline:" + symbol + ":" + timeframe
			hub.Subscribe(client, channel)
			
			// Wait for snapshot
			time.Sleep(50 * time.Millisecond)
			
			// Get the snapshot message
			if len(client.Send) == 0 {
				return numCandles == 0 // If no candles, no snapshot is expected
			}
			
			msg := <-client.Send
			
			// Parse as generic map to check structure
			var msgMap map[string]interface{}
			if err := json.Unmarshal(msg, &msgMap); err != nil {
				return false
			}
			
			// Property 1: Must have "type" field with value "snapshot"
			typeVal, hasType := msgMap["type"]
			if !hasType || typeVal != "snapshot" {
				return false
			}
			
			// Property 2: Must have "symbol" field
			symbolVal, hasSymbol := msgMap["symbol"]
			if !hasSymbol || symbolVal != symbol {
				return false
			}
			
			// Property 3: Must have "timeframe" field
			timeframeVal, hasTimeframe := msgMap["timeframe"]
			if !hasTimeframe || timeframeVal != timeframe {
				return false
			}
			
			// Property 4: Must have "data" field as an array
			dataVal, hasData := msgMap["data"]
			if !hasData {
				return false
			}
			
			// Check that data is an array
			_, isArray := dataVal.([]interface{})
			if !isArray {
				return false
			}
			
			// Property 5: Should have exactly these 4 fields (no extra fields)
			if len(msgMap) != 4 {
				return false
			}
			
			return true
		},
		gen.IntRange(1, 50),                                  // numCandles
		gen.OneConstOf("XAUUSD", "EURUSD", "GBPUSD"),        // symbol
		gen.OneConstOf("M1", "M5", "M15", "M30", "H1", "H4"), // timeframe
	))
	
	properties.TestingRun(t)
}

// **Feature: kline-complete-push, Property 11: Subscription persistence**
// **Validates: Requirements 3.5**
// For any client that unsubscribes from a channel, the CandleBuffer for that channel 
// should continue to receive and store updates for other subscribed clients.
func TestProperty_SubscriptionPersistence(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	properties.Property("buffer persists after client unsubscribes", prop.ForAll(
		func(numInitialCandles int, numUpdates int) bool {
			// Skip invalid cases
			if numInitialCandles < 1 || numUpdates < 1 {
				return true
			}
			
			hub := createTestHub()
			baseTime := time.Now()
			symbol := "XAUUSD"
			timeframe := "M1"
			key := symbol + ":" + timeframe
			channel := "kline:" + symbol + ":" + timeframe
			
			// Pre-populate buffer with candles
			for i := 0; i < numInitialCandles; i++ {
				candle := createValidCandle(baseTime, i)
				hub.indicatorManager.AddCandle(key, candle, true)
			}
			
			// Create two clients and subscribe them both
			client1 := &Client{
				Send:          make(chan []byte, 256),
				Subscriptions: make(map[string]bool),
			}
			client2 := &Client{
				Send:          make(chan []byte, 256),
				Subscriptions: make(map[string]bool),
			}
			
			hub.Subscribe(client1, channel)
			hub.Subscribe(client2, channel)
			client1.Subscriptions[channel] = true
			client2.Subscriptions[channel] = true
			
			// Wait for initial snapshots
			time.Sleep(100 * time.Millisecond)
			
			// Clear channels
			for len(client1.Send) > 0 {
				<-client1.Send
			}
			for len(client2.Send) > 0 {
				<-client2.Send
			}
			
			// Get initial buffer size
			initialBufferSize := len(hub.indicatorManager.GetCandles(key))
			
			// Client1 unsubscribes
			hub.Unsubscribe(client1, channel)
			delete(client1.Subscriptions, channel)
			
			// Send updates to the buffer
			for i := 0; i < numUpdates; i++ {
				newCandle := createValidCandle(baseTime, numInitialCandles+i)
				msg := map[string]interface{}{
					"status": "CLOSE",
					"candle": map[string]interface{}{
						"symbol":     symbol,
						"timeframe":  timeframe,
						"start_time": newCandle.Time,
						"open":       newCandle.Open,
						"high":       newCandle.High,
						"low":        newCandle.Low,
						"close":      newCandle.Close,
						"volume":     newCandle.Volume,
					},
				}
				
				payload, _ := json.Marshal(msg)
				redisMsg := &redis.Message{
					Channel: channel,
					Payload: string(payload),
				}
				
				hub.handleKlineMessage(redisMsg)
			}
			
			// Wait for messages to be processed
			time.Sleep(100 * time.Millisecond)
			
			// Property 1: Buffer should have received all updates
			finalBufferSize := len(hub.indicatorManager.GetCandles(key))
			expectedSize := initialBufferSize + numUpdates
			if expectedSize > 500 {
				expectedSize = 500 // Buffer has max size
			}
			if finalBufferSize != expectedSize {
				t.Logf("Expected buffer size %d, got %d", expectedSize, finalBufferSize)
				return false
			}
			
			// Property 2: Client2 (still subscribed) should receive updates
			if len(client2.Send) != numUpdates {
				t.Logf("Client2 should receive %d updates, got %d", numUpdates, len(client2.Send))
				return false
			}
			
			// Property 3: Client1 (unsubscribed) should NOT receive updates
			if len(client1.Send) > 0 {
				t.Logf("Client1 should not receive updates after unsubscribe")
				return false
			}
			
			// Property 4: Client2's snapshots should contain the updated buffer
			for i := 0; i < numUpdates; i++ {
				msg := <-client2.Send
				var snapshot SnapshotMessage
				if err := json.Unmarshal(msg, &snapshot); err != nil {
					return false
				}
				
				// Each snapshot should have the correct number of candles
				expectedCandlesInSnapshot := initialBufferSize + i + 1
				if expectedCandlesInSnapshot > 500 {
					expectedCandlesInSnapshot = 500
				}
				if len(snapshot.Data) != expectedCandlesInSnapshot {
					return false
				}
			}
			
			return true
		},
		gen.IntRange(1, 50),  // numInitialCandles
		gen.IntRange(1, 20),  // numUpdates
	))
	
	properties.TestingRun(t)
}

// **Feature: kline-complete-push, Property 4: Multi-client broadcast**
// **Validates: Requirements 2.4**
// For any K-line update, if N clients are subscribed to the same channel, 
// all N clients should receive identical snapshot messages.
func TestProperty_MultiClientBroadcast(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	
	properties := gopter.NewProperties(parameters)
	
	properties.Property("all subscribed clients receive identical snapshots", prop.ForAll(
		func(numClients int, numCandles int, isNew bool) bool {
			// Skip invalid cases
			if numClients < 1 || numCandles < 1 {
				return true
			}
			
			hub := createTestHub()
			baseTime := time.Now()
			symbol := "XAUUSD"
			timeframe := "M1"
			key := symbol + ":" + timeframe
			channel := "kline:" + symbol + ":" + timeframe
			
			// Pre-populate buffer with candles
			for i := 0; i < numCandles; i++ {
				candle := createValidCandle(baseTime, i)
				hub.indicatorManager.AddCandle(key, candle, true)
			}
			
			// Create N clients and subscribe them all to the same channel
			clients := make([]*Client, numClients)
			for i := 0; i < numClients; i++ {
				clients[i] = &Client{
					Send:          make(chan []byte, 256),
					Subscriptions: make(map[string]bool),
				}
				hub.Subscribe(clients[i], channel)
			}
			
			// Wait for initial snapshots to be sent
			time.Sleep(100 * time.Millisecond)
			
			// Clear all client channels
			for _, client := range clients {
				for len(client.Send) > 0 {
					<-client.Send
				}
			}
			
			// Create a K-line update message
			status := "UPDATE"
			if isNew {
				status = "CLOSE"
			}
			
			newCandle := createValidCandle(baseTime, numCandles)
			msg := map[string]interface{}{
				"status": status,
				"candle": map[string]interface{}{
					"symbol":     symbol,
					"timeframe":  timeframe,
					"start_time": newCandle.Time,
					"open":       newCandle.Open,
					"high":       newCandle.High,
					"low":        newCandle.Low,
					"close":      newCandle.Close,
					"volume":     newCandle.Volume,
				},
			}
			
			payload, _ := json.Marshal(msg)
			redisMsg := &redis.Message{
				Channel: channel,
				Payload: string(payload),
			}
			
			// Process the message
			hub.handleKlineMessage(redisMsg)
			
			// Wait for messages to be sent
			time.Sleep(100 * time.Millisecond)
			
			// Property 1: All clients should receive a message
			for i, client := range clients {
				if len(client.Send) == 0 {
					t.Logf("Client %d did not receive message", i)
					return false
				}
			}
			
			// Collect all snapshots
			snapshots := make([]SnapshotMessage, numClients)
			for i, client := range clients {
				msg := <-client.Send
				if err := json.Unmarshal(msg, &snapshots[i]); err != nil {
					t.Logf("Failed to unmarshal snapshot for client %d: %v", i, err)
					return false
				}
			}
			
			// Property 2: All snapshots should be identical
			firstSnapshot := snapshots[0]
			for i := 1; i < numClients; i++ {
				// Check type
				if snapshots[i].Type != firstSnapshot.Type {
					return false
				}
				// Check symbol
				if snapshots[i].Symbol != firstSnapshot.Symbol {
					return false
				}
				// Check timeframe
				if snapshots[i].Timeframe != firstSnapshot.Timeframe {
					return false
				}
				// Check data length
				if len(snapshots[i].Data) != len(firstSnapshot.Data) {
					return false
				}
				// Check each candle
				for j := 0; j < len(snapshots[i].Data); j++ {
					c1 := snapshots[i].Data[j]
					c2 := firstSnapshot.Data[j]
					if !c1.Time.Equal(c2.Time) || c1.Open != c2.Open || 
					   c1.High != c2.High || c1.Low != c2.Low || 
					   c1.Close != c2.Close || c1.Volume != c2.Volume {
						return false
					}
				}
			}
			
			return true
		},
		gen.IntRange(1, 10),  // numClients between 1 and 10
		gen.IntRange(1, 50),  // numCandles between 1 and 50
		gen.Bool(),           // isNew (UPDATE or CLOSE)
	))
	
	properties.TestingRun(t)
}

// Unit tests for Hub snapshot operations

// TestHub_HandleKlineMessage_UPDATE tests handling of UPDATE status messages
func TestHub_HandleKlineMessage_UPDATE(t *testing.T) {
	hub := createTestHub()
	baseTime := time.Now()
	symbol := "XAUUSD"
	timeframe := "M1"
	key := symbol + ":" + timeframe
	
	// Pre-populate buffer with candles
	for i := 0; i < 5; i++ {
		candle := createValidCandle(baseTime, i)
		hub.indicatorManager.AddCandle(key, candle, true)
	}
	
	// Create a client and subscribe
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	channel := "kline:" + symbol + ":" + timeframe
	hub.Subscribe(client, channel)
	
	// Wait for initial snapshot
	time.Sleep(50 * time.Millisecond)
	
	// Clear the channel
	for len(client.Send) > 0 {
		<-client.Send
	}
	
	// Create UPDATE message
	updateCandle := createValidCandle(baseTime, 4) // Update last candle
	updateCandle.Close = 2700.0 // Change close price
	
	msg := map[string]interface{}{
		"status": "UPDATE",
		"candle": map[string]interface{}{
			"symbol":     symbol,
			"timeframe":  timeframe,
			"start_time": updateCandle.Time,
			"open":       updateCandle.Open,
			"high":       updateCandle.High,
			"low":        updateCandle.Low,
			"close":      updateCandle.Close,
			"volume":     updateCandle.Volume,
		},
	}
	
	payload, _ := json.Marshal(msg)
	redisMsg := &redis.Message{
		Channel: channel,
		Payload: string(payload),
	}
	
	// Process the message
	hub.handleKlineMessage(redisMsg)
	
	// Wait for message to be sent
	time.Sleep(50 * time.Millisecond)
	
	// Verify snapshot was sent
	if len(client.Send) == 0 {
		t.Fatal("Expected snapshot message to be sent")
	}
	
	msg1 := <-client.Send
	var snapshot SnapshotMessage
	if err := json.Unmarshal(msg1, &snapshot); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}
	
	// Verify snapshot properties
	if snapshot.Type != "snapshot" {
		t.Errorf("Expected type 'snapshot', got '%s'", snapshot.Type)
	}
	if snapshot.Symbol != symbol {
		t.Errorf("Expected symbol '%s', got '%s'", symbol, snapshot.Symbol)
	}
	if snapshot.Timeframe != timeframe {
		t.Errorf("Expected timeframe '%s', got '%s'", timeframe, snapshot.Timeframe)
	}
	if len(snapshot.Data) != 5 {
		t.Errorf("Expected 5 candles, got %d", len(snapshot.Data))
	}
	
	// Verify last candle was updated
	lastCandle := snapshot.Data[len(snapshot.Data)-1]
	if lastCandle.Close != 2700.0 {
		t.Errorf("Expected updated close 2700.0, got %.2f", lastCandle.Close)
	}
}

// TestHub_HandleKlineMessage_CLOSE tests handling of CLOSE status messages
func TestHub_HandleKlineMessage_CLOSE(t *testing.T) {
	hub := createTestHub()
	baseTime := time.Now()
	symbol := "EURUSD"
	timeframe := "M5"
	key := symbol + ":" + timeframe
	
	// Pre-populate buffer with candles
	for i := 0; i < 3; i++ {
		candle := createValidCandle(baseTime, i)
		hub.indicatorManager.AddCandle(key, candle, true)
	}
	
	// Create a client and subscribe
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	channel := "kline:" + symbol + ":" + timeframe
	hub.Subscribe(client, channel)
	
	// Wait for initial snapshot
	time.Sleep(50 * time.Millisecond)
	
	// Clear the channel
	for len(client.Send) > 0 {
		<-client.Send
	}
	
	// Create CLOSE message (new candle)
	newCandle := createValidCandle(baseTime, 3)
	
	msg := map[string]interface{}{
		"status": "CLOSE",
		"candle": map[string]interface{}{
			"symbol":     symbol,
			"timeframe":  timeframe,
			"start_time": newCandle.Time,
			"open":       newCandle.Open,
			"high":       newCandle.High,
			"low":        newCandle.Low,
			"close":      newCandle.Close,
			"volume":     newCandle.Volume,
		},
	}
	
	payload, _ := json.Marshal(msg)
	redisMsg := &redis.Message{
		Channel: channel,
		Payload: string(payload),
	}
	
	// Process the message
	hub.handleKlineMessage(redisMsg)
	
	// Wait for message to be sent
	time.Sleep(50 * time.Millisecond)
	
	// Verify snapshot was sent
	if len(client.Send) == 0 {
		t.Fatal("Expected snapshot message to be sent")
	}
	
	msg1 := <-client.Send
	var snapshot SnapshotMessage
	if err := json.Unmarshal(msg1, &snapshot); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}
	
	// Verify snapshot properties
	if snapshot.Type != "snapshot" {
		t.Errorf("Expected type 'snapshot', got '%s'", snapshot.Type)
	}
	if len(snapshot.Data) != 4 {
		t.Errorf("Expected 4 candles (3 old + 1 new), got %d", len(snapshot.Data))
	}
}

// TestHub_SendSnapshot_PopulatedBuffer tests sending snapshot with data
func TestHub_SendSnapshot_PopulatedBuffer(t *testing.T) {
	hub := createTestHub()
	baseTime := time.Now()
	symbol := "GBPUSD"
	timeframe := "H1"
	key := symbol + ":" + timeframe
	
	// Add candles to buffer
	expectedCount := 10
	for i := 0; i < expectedCount; i++ {
		candle := createValidCandle(baseTime, i)
		hub.indicatorManager.AddCandle(key, candle, true)
	}
	
	// Create a client
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	
	// Call sendSnapshot directly
	channel := "kline:" + symbol + ":" + timeframe
	hub.sendSnapshot(client, channel)
	
	// Wait for message
	time.Sleep(50 * time.Millisecond)
	
	// Verify snapshot was sent
	if len(client.Send) == 0 {
		t.Fatal("Expected snapshot message to be sent")
	}
	
	msg := <-client.Send
	var snapshot SnapshotMessage
	if err := json.Unmarshal(msg, &snapshot); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}
	
	// Verify snapshot content
	if snapshot.Type != "snapshot" {
		t.Errorf("Expected type 'snapshot', got '%s'", snapshot.Type)
	}
	if snapshot.Symbol != symbol {
		t.Errorf("Expected symbol '%s', got '%s'", symbol, snapshot.Symbol)
	}
	if snapshot.Timeframe != timeframe {
		t.Errorf("Expected timeframe '%s', got '%s'", timeframe, snapshot.Timeframe)
	}
	if len(snapshot.Data) != expectedCount {
		t.Errorf("Expected %d candles, got %d", expectedCount, len(snapshot.Data))
	}
	
	// Verify candles are in chronological order
	for i := 1; i < len(snapshot.Data); i++ {
		if !snapshot.Data[i].Time.After(snapshot.Data[i-1].Time) {
			t.Error("Candles should be in chronological order")
		}
	}
}

// TestHub_SendSnapshot_EmptyBuffer tests sending snapshot with no data
func TestHub_SendSnapshot_EmptyBuffer(t *testing.T) {
	hub := createTestHub()
	symbol := "USDJPY"
	timeframe := "M15"
	
	// Create a client (don't add any candles to buffer)
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	
	// Call sendSnapshot directly
	channel := "kline:" + symbol + ":" + timeframe
	hub.sendSnapshot(client, channel)
	
	// Wait for message
	time.Sleep(50 * time.Millisecond)
	
	// Verify no snapshot was sent (empty buffer)
	if len(client.Send) > 0 {
		t.Error("Should not send snapshot for empty buffer")
	}
}

// TestHub_MessageSerializationFormat tests the JSON serialization format
func TestHub_MessageSerializationFormat(t *testing.T) {
	hub := createTestHub()
	baseTime := time.Now()
	symbol := "XAUUSD"
	timeframe := "M1"
	key := symbol + ":" + timeframe
	
	// Add a few candles
	for i := 0; i < 3; i++ {
		candle := createValidCandle(baseTime, i)
		hub.indicatorManager.AddCandle(key, candle, true)
	}
	
	// Create a client and subscribe
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	channel := "kline:" + symbol + ":" + timeframe
	hub.Subscribe(client, channel)
	
	// Wait for snapshot
	time.Sleep(50 * time.Millisecond)
	
	if len(client.Send) == 0 {
		t.Fatal("Expected snapshot message")
	}
	
	msg := <-client.Send
	
	// Parse as generic map to verify structure
	var msgMap map[string]interface{}
	if err := json.Unmarshal(msg, &msgMap); err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}
	
	// Verify required fields exist
	if _, ok := msgMap["type"]; !ok {
		t.Error("Message should have 'type' field")
	}
	if _, ok := msgMap["symbol"]; !ok {
		t.Error("Message should have 'symbol' field")
	}
	if _, ok := msgMap["timeframe"]; !ok {
		t.Error("Message should have 'timeframe' field")
	}
	if _, ok := msgMap["data"]; !ok {
		t.Error("Message should have 'data' field")
	}
	
	// Verify field values
	if msgMap["type"] != "snapshot" {
		t.Errorf("Expected type 'snapshot', got '%v'", msgMap["type"])
	}
	if msgMap["symbol"] != symbol {
		t.Errorf("Expected symbol '%s', got '%v'", symbol, msgMap["symbol"])
	}
	if msgMap["timeframe"] != timeframe {
		t.Errorf("Expected timeframe '%s', got '%v'", timeframe, msgMap["timeframe"])
	}
	
	// Verify data is an array
	dataArray, ok := msgMap["data"].([]interface{})
	if !ok {
		t.Error("'data' field should be an array")
	}
	if len(dataArray) != 3 {
		t.Errorf("Expected 3 candles in data array, got %d", len(dataArray))
	}
	
	// Verify each candle has required fields
	for i, item := range dataArray {
		candleMap, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("Candle %d should be an object", i)
			continue
		}
		
		requiredFields := []string{"time", "open", "high", "low", "close", "volume"}
		for _, field := range requiredFields {
			if _, ok := candleMap[field]; !ok {
				t.Errorf("Candle %d should have '%s' field", i, field)
			}
		}
	}
}

// Unit tests for subscription management

// TestHub_Subscribe_AddsClientToMap tests that Subscribe adds client to subscription map
func TestHub_Subscribe_AddsClientToMap(t *testing.T) {
	hub := createTestHub()
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	
	channel := "kline:XAUUSD:M1"
	hub.Subscribe(client, channel)
	
	// Wait for async operations
	time.Sleep(50 * time.Millisecond)
	
	// Verify client is in subscription map
	hub.subMutex.RLock()
	clients, exists := hub.Subscriptions[channel]
	hub.subMutex.RUnlock()
	
	if !exists {
		t.Error("Channel should exist in subscriptions after Subscribe")
	}
	if !clients[client] {
		t.Error("Client should be in subscription map after Subscribe")
	}
}

// TestHub_Unsubscribe_RemovesClientFromMap tests that Unsubscribe removes client
func TestHub_Unsubscribe_RemovesClientFromMap(t *testing.T) {
	hub := createTestHub()
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	
	channel := "kline:EURUSD:M5"
	
	// Subscribe first
	hub.Subscribe(client, channel)
	time.Sleep(50 * time.Millisecond)
	
	// Verify client is subscribed
	hub.subMutex.RLock()
	_, exists := hub.Subscriptions[channel]
	hub.subMutex.RUnlock()
	if !exists {
		t.Fatal("Client should be subscribed before unsubscribe")
	}
	
	// Unsubscribe
	hub.Unsubscribe(client, channel)
	
	// Verify client is removed
	hub.subMutex.RLock()
	clients, exists := hub.Subscriptions[channel]
	hub.subMutex.RUnlock()
	
	if exists && clients[client] {
		t.Error("Client should not be in subscription map after Unsubscribe")
	}
}

// TestHub_Unsubscribe_CleansUpEmptyChannel tests that empty channels are removed
func TestHub_Unsubscribe_CleansUpEmptyChannel(t *testing.T) {
	hub := createTestHub()
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	
	channel := "kline:GBPUSD:H1"
	
	// Subscribe
	hub.Subscribe(client, channel)
	time.Sleep(50 * time.Millisecond)
	
	// Unsubscribe (only client on this channel)
	hub.Unsubscribe(client, channel)
	
	// Verify channel is removed from subscriptions
	hub.subMutex.RLock()
	_, exists := hub.Subscriptions[channel]
	hub.subMutex.RUnlock()
	
	if exists {
		t.Error("Empty channel should be removed from subscriptions")
	}
}

// TestHub_CleanUpSubscriptions_OnDisconnect tests cleanup when client disconnects
func TestHub_CleanUpSubscriptions_OnDisconnect(t *testing.T) {
	hub := createTestHub()
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	
	// Subscribe to multiple channels
	channels := []string{
		"kline:XAUUSD:M1",
		"kline:XAUUSD:M5",
		"kline:EURUSD:M1",
	}
	
	for _, channel := range channels {
		hub.Subscribe(client, channel)
		client.Subscriptions[channel] = true
	}
	
	time.Sleep(50 * time.Millisecond)
	
	// Verify all subscriptions exist
	hub.subMutex.RLock()
	for _, channel := range channels {
		if _, exists := hub.Subscriptions[channel]; !exists {
			t.Errorf("Channel %s should exist before cleanup", channel)
		}
	}
	hub.subMutex.RUnlock()
	
	// Clean up subscriptions (simulating disconnect)
	hub.cleanUpSubscriptions(client)
	
	// Verify all subscriptions are removed
	hub.subMutex.RLock()
	for _, channel := range channels {
		if clients, exists := hub.Subscriptions[channel]; exists && clients[client] {
			t.Errorf("Client should not be subscribed to %s after cleanup", channel)
		}
	}
	hub.subMutex.RUnlock()
}

// TestHub_Subscribe_SendsSnapshot tests that snapshot is sent on subscription
func TestHub_Subscribe_SendsSnapshot(t *testing.T) {
	hub := createTestHub()
	baseTime := time.Now()
	symbol := "XAUUSD"
	timeframe := "M1"
	key := symbol + ":" + timeframe
	
	// Add candles to buffer
	numCandles := 5
	for i := 0; i < numCandles; i++ {
		candle := createValidCandle(baseTime, i)
		hub.indicatorManager.AddCandle(key, candle, true)
	}
	
	// Create client and subscribe
	client := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	
	channel := "kline:" + symbol + ":" + timeframe
	hub.Subscribe(client, channel)
	
	// Wait for snapshot to be sent
	time.Sleep(100 * time.Millisecond)
	
	// Verify snapshot was sent
	if len(client.Send) == 0 {
		t.Fatal("Expected snapshot to be sent on subscription")
	}
	
	msg := <-client.Send
	var snapshot SnapshotMessage
	if err := json.Unmarshal(msg, &snapshot); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}
	
	// Verify snapshot content
	if snapshot.Type != "snapshot" {
		t.Errorf("Expected type 'snapshot', got '%s'", snapshot.Type)
	}
	if snapshot.Symbol != symbol {
		t.Errorf("Expected symbol '%s', got '%s'", symbol, snapshot.Symbol)
	}
	if snapshot.Timeframe != timeframe {
		t.Errorf("Expected timeframe '%s', got '%s'", timeframe, snapshot.Timeframe)
	}
	if len(snapshot.Data) != numCandles {
		t.Errorf("Expected %d candles in snapshot, got %d", numCandles, len(snapshot.Data))
	}
}

// TestHub_BufferPersistsAfterUnsubscribe tests that buffer continues after unsubscribe
func TestHub_BufferPersistsAfterUnsubscribe(t *testing.T) {
	hub := createTestHub()
	baseTime := time.Now()
	symbol := "EURUSD"
	timeframe := "M5"
	key := symbol + ":" + timeframe
	
	// Add initial candles
	for i := 0; i < 3; i++ {
		candle := createValidCandle(baseTime, i)
		hub.indicatorManager.AddCandle(key, candle, true)
	}
	
	// Create two clients
	client1 := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	client2 := &Client{
		Send:          make(chan []byte, 256),
		Subscriptions: make(map[string]bool),
	}
	
	channel := "kline:" + symbol + ":" + timeframe
	
	// Both subscribe
	hub.Subscribe(client1, channel)
	hub.Subscribe(client2, channel)
	client1.Subscriptions[channel] = true
	client2.Subscriptions[channel] = true
	
	time.Sleep(50 * time.Millisecond)
	
	// Clear channels
	for len(client1.Send) > 0 {
		<-client1.Send
	}
	for len(client2.Send) > 0 {
		<-client2.Send
	}
	
	// Client1 unsubscribes
	hub.Unsubscribe(client1, channel)
	delete(client1.Subscriptions, channel)
	
	// Add a new candle
	newCandle := createValidCandle(baseTime, 3)
	msg := map[string]interface{}{
		"status": "CLOSE",
		"candle": map[string]interface{}{
			"symbol":     symbol,
			"timeframe":  timeframe,
			"start_time": newCandle.Time,
			"open":       newCandle.Open,
			"high":       newCandle.High,
			"low":        newCandle.Low,
			"close":      newCandle.Close,
			"volume":     newCandle.Volume,
		},
	}
	
	payload, _ := json.Marshal(msg)
	redisMsg := &redis.Message{
		Channel: channel,
		Payload: string(payload),
	}
	
	hub.handleKlineMessage(redisMsg)
	
	time.Sleep(50 * time.Millisecond)
	
	// Verify buffer was updated (should have 4 candles now)
	candles := hub.indicatorManager.GetCandles(key)
	if len(candles) != 4 {
		t.Errorf("Expected buffer to have 4 candles, got %d", len(candles))
	}
	
	// Verify client2 received the update
	if len(client2.Send) == 0 {
		t.Error("Client2 should receive update even after client1 unsubscribed")
	}
	
	// Verify client1 did NOT receive the update
	if len(client1.Send) > 0 {
		t.Error("Client1 should not receive update after unsubscribe")
	}
}
