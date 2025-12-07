# Phase 5: Circuit Breaker and Timeout Resilience

## Objective
Add resilience mechanisms to prevent cascade failures and protect the system from slow or failing cache layers. Implement per-layer circuit breakers and configurable timeouts.

## Scope
- Implement circuit breaker pattern (closed/open/half-open states)
- Add per-operation timeout support via context
- Create `ResilientLayer` wrapper that adds resilience to any `CacheLayer`
- Integrate with Chain to make all layers resilient by default
- Comprehensive unit tests including failure scenarios

## Requirements

### 1. Circuit Breaker Implementation
Create `pkg/resilience/circuitbreaker.go`:

```go
type CircuitBreaker struct {
    maxFailures  uint32        // Failures before opening
    resetTimeout time.Duration // Time before half-open attempt
    halfOpenSuccesses uint32    // Successes needed to close from half-open
    
    state        State
    failures     uint32
    successes    uint32
    lastFailTime time.Time
    mu           sync.RWMutex
}

type State int

const (
    StateClosed State = iota   // Normal operation
    StateOpen                  // Rejecting calls
    StateHalfOpen              // Testing if backend recovered
)

func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker

type CircuitBreakerConfig struct {
    MaxFailures       uint32
    ResetTimeout      time.Duration
    HalfOpenSuccesses uint32  // Default: 2
}
```

### 2. Circuit Breaker Operations
```go
// Execute wraps a function with circuit breaker logic
func (cb *CircuitBreaker) Execute(fn func() error) error

// GetState returns current circuit state
func (cb *CircuitBreaker) GetState() State

// Reset manually resets circuit to closed (for testing/ops)
func (cb *CircuitBreaker) Reset()

// State machine:
// Closed --[max failures]--> Open
// Open --[reset timeout]--> Half-Open
// Half-Open --[success * N]--> Closed
// Half-Open --[failure]--> Open
```

### 3. ResilientLayer Wrapper
Create `pkg/resilience/layer.go`:

```go
type ResilientLayer struct {
    layer   cache.CacheLayer
    cb      *CircuitBreaker
    timeout time.Duration
}

// NewResilientLayer wraps a layer with circuit breaker and timeout
func NewResilientLayer(layer cache.CacheLayer, config ResilientConfig) *ResilientLayer

type ResilientConfig struct {
    Timeout              time.Duration
    CircuitBreakerConfig CircuitBreakerConfig
}

// Implements CacheLayer interface with resilience
func (rl *ResilientLayer) Get(ctx context.Context, key string) (interface{}, error) {
    // Apply timeout
    ctx, cancel := context.WithTimeout(ctx, rl.timeout)
    defer cancel()
    
    // Execute through circuit breaker
    var result interface{}
    var err error
    
    cbErr := rl.cb.Execute(func() error {
        result, err = rl.layer.Get(ctx, key)
        return err
    })
    
    if cbErr != nil {
        return nil, cbErr
    }
    return result, err
}

// Similar for Set, Delete, etc.
```

### 4. Error Types
Add to `pkg/cache/errors.go`:

```go
var (
    ErrCircuitOpen = errors.New("cache: circuit breaker open")
    ErrTimeout     = errors.New("cache: operation timeout")
)

func IsCircuitOpen(err error) bool
```

### 5. Integration with Chain
Modify `pkg/chain/chain.go` to use resilient layers:

```go
func New(layers ...CacheLayer) (*Chain, error) {
    // Option 1: Wrap each layer automatically
    resilientLayers := make([]CacheLayer, len(layers))
    for i, layer := range layers {
        resilientLayers[i] = resilience.NewResilientLayer(layer, defaultConfig)
    }
    
    // Option 2: Accept ResilientConfig per layer
    // (more flexible, document in NewWithConfig)
    
    return &Chain{
        layers: resilientLayers,
        // ... rest of setup
    }, nil
}
```

## File Structure
```
pkg/
  resilience/
    circuitbreaker.go      # Circuit breaker implementation
    circuitbreaker_test.go # CB unit tests
    layer.go               # ResilientLayer wrapper
    layer_test.go          # Wrapper tests
    config.go              # Configuration types
```

## Testing Requirements

### Test Cases

#### Circuit Breaker Tests
1. **State Transitions**
   - Closed → Open after max failures
   - Open → Half-Open after reset timeout
   - Half-Open → Closed after successes
   - Half-Open → Open on failure

2. **Closed State**
   - Allows all calls through
   - Counts failures
   - Resets failure count on success

3. **Open State**
   - Rejects all calls immediately with ErrCircuitOpen
   - Doesn't call underlying function
   - Transitions to half-open after timeout

4. **Half-Open State**
   - Allows limited calls through
   - Closes on consecutive successes
   - Opens immediately on failure

#### ResilientLayer Tests
1. **Timeout Enforcement**
   - Operation completes within timeout: success
   - Operation exceeds timeout: ErrTimeout
   - Timeout counted as failure in CB

2. **Circuit Breaker Integration**
   - Circuit open: return ErrCircuitOpen
   - Circuit closed: execute normally
   - Failures trigger circuit

3. **Context Cancellation**
   - Parent context cancelled: propagate immediately
   - Don't override parent timeout with shorter one

### Test Example
```go
func TestCircuitBreaker_StateTransitions(t *testing.T) {
    cb := NewCircuitBreaker(CircuitBreakerConfig{
        MaxFailures:  3,
        ResetTimeout: 100 * time.Millisecond,
    })
    
    // Initial: Closed
    if cb.GetState() != StateClosed {
        t.Error("Initial state should be Closed")
    }
    
    // Trigger failures
    for i := 0; i < 3; i++ {
        cb.Execute(func() error { return errors.New("fail") })
    }
    
    // Should be Open
    if cb.GetState() != StateOpen {
        t.Error("Should be Open after max failures")
    }
    
    // Should reject calls
    err := cb.Execute(func() error { return nil })
    if !errors.Is(err, ErrCircuitOpen) {
        t.Errorf("Expected ErrCircuitOpen, got %v", err)
    }
    
    // Wait for reset timeout
    time.Sleep(150 * time.Millisecond)
    
    // Should be Half-Open (verified on next call)
    cb.Execute(func() error { return nil })
    if cb.GetState() != StateHalfOpen {
        t.Error("Should be Half-Open after timeout")
    }
}

func TestResilientLayer_Timeout(t *testing.T) {
    slowLayer := &mock.MockLayer{
        GetFunc: func(ctx context.Context, key string) (interface{}, error) {
            time.Sleep(200 * time.Millisecond)
            return "value", nil
        },
    }
    
    resilient := NewResilientLayer(slowLayer, ResilientConfig{
        Timeout: 50 * time.Millisecond,
    })
    
    _, err := resilient.Get(context.Background(), "key")
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("Expected timeout error, got %v", err)
    }
    
    // Verify circuit opened after repeated timeouts
    for i := 0; i < 3; i++ {
        resilient.Get(context.Background(), "key")
    }
    
    _, err = resilient.Get(context.Background(), "key")
    if !IsCircuitOpen(err) {
        t.Error("Circuit should be open after repeated timeouts")
    }
}
```

### Integration Test
```go
func TestChain_ResilientLayers(t *testing.T) {
    // L1: Always fails
    l1 := &mock.MockLayer{
        GetFunc: func(ctx context.Context, key string) (interface{}, error) {
            return nil, errors.New("L1 always fails")
        },
    }
    
    // L2: Works fine
    l2 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L2"})
    l2.Set(context.Background(), "key", "value", time.Minute)
    
    chain, _ := chain.New(l1, l2)
    defer chain.Close()
    
    // First few calls: L1 fails, falls back to L2
    for i := 0; i < 5; i++ {
        value, err := chain.Get(context.Background(), "key")
        if err != nil {
            t.Fatalf("Call %d failed: %v", i, err)
        }
        if value != "value" {
            t.Errorf("Wrong value: %v", value)
        }
    }
    
    // After enough failures, L1 circuit should open
    // Verify we're skipping L1 entirely (faster response)
    // This would be visible in metrics (Phase 6)
}
```

## Acceptance Criteria
- [ ] Circuit breaker implements all three states correctly
- [ ] State transitions work as specified
- [ ] ResilientLayer wraps any CacheLayer
- [ ] Timeouts enforced via context
- [ ] Circuit breaker integrates with timeouts
- [ ] Chain automatically wraps layers with resilience
- [ ] Unit tests with >90% coverage
- [ ] Time-based tests use time.Sleep or test clocks
- [ ] All tests passing: `go test ./pkg/resilience/...`
- [ ] Code follows Go instructions
- [ ] Godoc comments on all exported types/functions

## Notes
- Circuit breaker parameters should be tunable per layer
- Consider exponential backoff for reset timeout (future enhancement)
- Timeout should be separate from circuit breaker to allow fine-tuning
- Context deadline takes precedence if shorter than configured timeout
- Use `sync.RWMutex` for circuit breaker state (many reads, few writes)

## Configuration Guidelines
- **MaxFailures**: 3-10 depending on layer reliability
- **ResetTimeout**: 5-30 seconds (longer for critical backends)
- **HalfOpenSuccesses**: 2-3 (enough to verify recovery)
- **Timeout**: Based on layer characteristics:
  - In-memory: 10-100ms
  - Redis: 100-500ms
  - Database: 500-2000ms

## Next Phase
Phase 6 will add comprehensive metrics collection and exporters for observability.
