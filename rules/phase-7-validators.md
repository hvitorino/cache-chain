# Phase 7: Validators for Data Integrity

## Objective
Implement pluggable validation system for ensuring data integrity at cache layer boundaries. Support both entry (Set) and exit (Get) validation with configurable policies.

## Scope
- Define `Validator` interface for custom validation logic
- Implement common validators (type checking, schema validation, size limits)
- Add validation wrapper for cache layers
- Support validation policies (strict, warn, skip)
- Comprehensive unit tests with various validation scenarios

## Requirements

### 1. Validator Interface
Create `pkg/validator/validator.go`:

```go
// Validator validates cache values
type Validator interface {
    // Validate checks if a value is valid for the given key
    // Returns nil if valid, error describing the problem if invalid
    Validate(ctx context.Context, key string, value interface{}) error
    
    // Name returns the validator identifier (for logging/metrics)
    Name() string
}

// NoOpValidator allows everything (default)
type NoOpValidator struct{}

func (NoOpValidator) Validate(ctx context.Context, key string, value interface{}) error {
    return nil
}
func (NoOpValidator) Name() string { return "noop" }
```

### 2. Common Validators

#### Type Validator
```go
// TypeValidator ensures values are of expected type
type TypeValidator struct {
    allowedTypes []reflect.Type
}

func NewTypeValidator(types ...interface{}) *TypeValidator

// Example: NewTypeValidator(string(""), int(0)) allows only string or int
```

#### Size Validator
```go
// SizeValidator enforces size limits
type SizeValidator struct {
    maxBytes int64
}

func NewSizeValidator(maxBytes int64) *SizeValidator

// Validates:
// - Strings: len(str)
// - Byte slices: len(slice)
// - Serializable: JSON encoding size
```

#### Schema Validator
```go
// SchemaValidator validates struct fields
type SchemaValidator struct {
    requiredFields []string
    validate       func(interface{}) error  // Custom validation function
}

func NewSchemaValidator(requiredFields []string, customFn func(interface{}) error) *SchemaValidator
```

#### Composite Validator
```go
// CompositeValidator chains multiple validators
type CompositeValidator struct {
    validators []Validator
}

func NewCompositeValidator(validators ...Validator) *CompositeValidator

// Runs all validators, collects all errors
func (cv *CompositeValidator) Validate(ctx context.Context, key string, value interface{}) error {
    var errs []error
    for _, v := range cv.validators {
        if err := v.Validate(ctx, key, value); err != nil {
            errs = append(errs, fmt.Errorf("%s: %w", v.Name(), err))
        }
    }
    if len(errs) > 0 {
        return fmt.Errorf("validation failed: %v", errs)
    }
    return nil
}
```

### 3. Validation Policy
```go
type ValidationPolicy int

const (
    // PolicyStrict: validation failure causes operation to fail
    PolicyStrict ValidationPolicy = iota
    
    // PolicyWarn: validation failure is logged but operation succeeds
    PolicyWarn
    
    // PolicySkip: no validation performed
    PolicySkip
)
```

### 4. ValidatedLayer Wrapper
Create `pkg/validator/layer.go`:

```go
type ValidatedLayer struct {
    layer         cache.CacheLayer
    setValidator  Validator  // Validates values being written
    getValidator  Validator  // Validates values being read
    setPolicy     ValidationPolicy
    getPolicy     ValidationPolicy
    metrics       metrics.MetricsCollector  // Track validation failures
}

func NewValidatedLayer(layer cache.CacheLayer, config ValidationConfig) *ValidatedLayer

type ValidationConfig struct {
    SetValidator Validator
    GetValidator Validator
    SetPolicy    ValidationPolicy
    GetPolicy    ValidationPolicy
}

func (vl *ValidatedLayer) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    // Validate before writing
    if err := vl.validateSet(ctx, key, value); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    return vl.layer.Set(ctx, key, value, ttl)
}

func (vl *ValidatedLayer) Get(ctx context.Context, key string) (interface{}, error) {
    value, err := vl.layer.Get(ctx, key)
    if err != nil {
        return nil, err
    }
    
    // Validate after reading
    if err := vl.validateGet(ctx, key, value); err != nil {
        // Policy determines behavior
        switch vl.getPolicy {
        case PolicyStrict:
            return nil, fmt.Errorf("validation failed: %w", err)
        case PolicyWarn:
            // Log warning, return value anyway
            log.Printf("validation warning for key %s: %v", key, err)
            return value, nil
        case PolicySkip:
            return value, nil
        }
    }
    
    return value, nil
}
```

### 5. Integration with Chain
Optional: validate at chain entry point:

```go
type ChainConfig struct {
    Layers            []cache.CacheLayer
    MetricsCollector  metrics.MetricsCollector
    EntryValidator    validator.Validator  // Validate before entering chain
    ExitValidator     validator.Validator  // Validate when exiting chain (last layer)
    ValidationPolicy  validator.ValidationPolicy
}
```

## File Structure
```
pkg/
  validator/
    validator.go       # Interface and NoOpValidator
    validator_test.go  # Interface tests
    type.go            # TypeValidator
    type_test.go       # Type validator tests
    size.go            # SizeValidator
    size_test.go       # Size validator tests
    schema.go          # SchemaValidator
    schema_test.go     # Schema validator tests
    composite.go       # CompositeValidator
    composite_test.go  # Composite tests
    layer.go           # ValidatedLayer wrapper
    layer_test.go      # Layer wrapper tests
```

## Testing Requirements

### Test Cases

#### Type Validator
```go
func TestTypeValidator(t *testing.T) {
    v := NewTypeValidator(string(""), int(0))
    
    tests := []struct {
        name    string
        value   interface{}
        wantErr bool
    }{
        {"string ok", "hello", false},
        {"int ok", 42, false},
        {"float fail", 3.14, true},
        {"struct fail", struct{}{}, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := v.Validate(context.Background(), "key", tt.value)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

#### Size Validator
```go
func TestSizeValidator(t *testing.T) {
    v := NewSizeValidator(10) // 10 bytes max
    
    tests := []struct {
        name    string
        value   interface{}
        wantErr bool
    }{
        {"small string", "hello", false},
        {"large string", strings.Repeat("a", 20), true},
        {"byte slice ok", []byte("short"), false},
        {"byte slice large", make([]byte, 20), true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := v.Validate(context.Background(), "key", tt.value)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

#### ValidatedLayer
```go
func TestValidatedLayer_StrictPolicy(t *testing.T) {
    mock := &mock.MockLayer{
        SetFunc: func(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
            return nil
        },
    }
    
    validator := NewTypeValidator(string(""))
    validated := NewValidatedLayer(mock, ValidationConfig{
        SetValidator: validator,
        SetPolicy:    PolicyStrict,
    })
    
    // Valid type: should succeed
    err := validated.Set(context.Background(), "key", "value", time.Minute)
    if err != nil {
        t.Errorf("Valid set failed: %v", err)
    }
    
    // Invalid type: should fail
    err = validated.Set(context.Background(), "key", 123, time.Minute)
    if err == nil {
        t.Error("Invalid set should fail with strict policy")
    }
}

func TestValidatedLayer_WarnPolicy(t *testing.T) {
    // Similar but validates operation succeeds with warning
}
```

#### Composite Validator
```go
func TestCompositeValidator(t *testing.T) {
    typeV := NewTypeValidator(string(""))
    sizeV := NewSizeValidator(10)
    composite := NewCompositeValidator(typeV, sizeV)
    
    // Fails both: wrong type AND too large
    err := composite.Validate(context.Background(), "key", 12345678901234567890)
    if err == nil {
        t.Error("Should fail composite validation")
    }
    
    // Error message should mention both validators
    if !strings.Contains(err.Error(), "type") {
        t.Error("Error should mention type validator")
    }
    if !strings.Contains(err.Error(), "size") {
        t.Error("Error should mention size validator")
    }
}
```

### Integration Test
```go
func TestChain_WithValidation(t *testing.T) {
    l1 := memory.NewMemoryCache(memory.MemoryCacheConfig{Name: "L1"})
    
    // Wrap with validator
    validator := NewTypeValidator(string(""))
    validated := NewValidatedLayer(l1, ValidationConfig{
        SetValidator: validator,
        SetPolicy:    PolicyStrict,
    })
    
    chain, _ := chain.New(validated)
    defer chain.Close()
    
    // Valid set
    err := chain.Set(context.Background(), "key", "string-value", time.Minute)
    if err != nil {
        t.Errorf("Valid set failed: %v", err)
    }
    
    // Invalid set
    err = chain.Set(context.Background(), "key", 123, time.Minute)
    if err == nil {
        t.Error("Invalid set should fail")
    }
}
```

## Acceptance Criteria
- [ ] Validator interface defined
- [ ] Type, Size, Schema validators implemented
- [ ] Composite validator for chaining
- [ ] ValidatedLayer wrapper with policy support
- [ ] All policies work correctly (strict, warn, skip)
- [ ] Set and Get validation independent
- [ ] Validation errors are descriptive
- [ ] Unit tests with >90% coverage
- [ ] All tests passing: `go test ./pkg/validator/...`
- [ ] Code follows Go instructions
- [ ] Godoc comments on all exported types

## Notes
- Validation should be fast (avoid expensive operations)
- Consider caching validation results for immutable values
- Log validation failures at appropriate levels
- Metrics integration: track validation failures per layer
- Validation is optional - default is NoOpValidator

## Example Usage
```go
// Create validators
typeValidator := validator.NewTypeValidator(string(""))
sizeValidator := validator.NewSizeValidator(1024 * 1024) // 1MB max
composite := validator.NewCompositeValidator(typeValidator, sizeValidator)

// Wrap layer with validation
validated := validator.NewValidatedLayer(l1, validator.ValidationConfig{
    SetValidator: composite,
    GetValidator: validator.NoOpValidator{},
    SetPolicy:    validator.PolicyStrict,
    GetPolicy:    validator.PolicySkip,
})

// Use in chain
chain, _ := chain.New(validated, l2, l3)
```

## Next Phase
Phase 8 will add advanced features like negative caching, bloom filters, and batch operations.
