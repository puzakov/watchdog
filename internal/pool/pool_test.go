package pool

import (
	"sync"
	"testing"
)

// testObject is a simple struct that implements Reseter via a pointer receiver.
// It tracks how many times Reset() has been called.
type testObject struct {
	mu         sync.Mutex
	ID         int
	Name       string
	Items      []int
	Nested     *nestedObject
	resetCount int
}

type nestedObject struct {
	Value string
}

func (o *testObject) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.ID = 0
	o.Name = ""
	o.Items = o.Items[:0]
	if o.Nested != nil {
		o.Nested.Value = ""
	}
	o.resetCount++
}

func (o *testObject) ResetCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.resetCount
}

func TestNew_PoolIsNotNil(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{ID: 42, Name: "initial"}
	})
	if p == nil {
		t.Fatal("New() returned nil")
	}
}

func TestGet_ReturnsNonNil(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{ID: 1, Name: "fresh"}
	})

	obj := p.Get()
	if obj == nil {
		t.Fatal("Get() returned nil")
	}
}

func TestGet_CreatesNewObjectWhenPoolEmpty(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{ID: 99, Name: "factory"}
	})

	obj := p.Get()
	if obj.ID != 99 || obj.Name != "factory" {
		t.Errorf("expected factory-built object (ID=99, Name='factory'), got (ID=%d, Name=%q)",
			obj.ID, obj.Name)
	}
}

func TestPut_ResetsObject(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{}
	})

	obj := p.Get()
	obj.ID = 100
	obj.Name = "dirty"
	obj.Items = append(obj.Items, 1, 2, 3)

	p.Put(obj)

	if obj.ID != 0 {
		t.Errorf("expected ID=0 after reset, got %d", obj.ID)
	}
	if obj.Name != "" {
		t.Errorf("expected empty Name after reset, got %q", obj.Name)
	}
	if len(obj.Items) != 0 {
		t.Errorf("expected empty Items after reset, got %v", obj.Items)
	}
}

func TestPut_IncrementsResetCount(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{}
	})

	obj := p.Get()
	before := obj.ResetCount()

	p.Put(obj)

	after := obj.ResetCount()
	if after <= before {
		t.Errorf("expected resetCount to increase, before=%d after=%d", before, after)
	}
}

func TestGet_ReusesRecycledObject(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{}
	})

	// Get, mutate, Put — object goes back to pool.
	obj1 := p.Get()
	obj1.ID = 7
	obj1.Name = "recycled"
	p.Put(obj1)

	// Get again — should ideally receive the same pointer (reset to zero state).
	obj2 := p.Get()
	if obj2 != obj1 {
		t.Log("note: got a different pointer; sync.Pool does not guarantee reuse")
	}

	// Whether same pointer or not, the object must be in clean state.
	if obj2.ID != 0 {
		t.Errorf("expected ID=0 after reuse, got %d", obj2.ID)
	}
	if obj2.Name != "" {
		t.Errorf("expected empty Name after reuse, got %q", obj2.Name)
	}
}

func TestPut_NilSliceTruncated(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{}
	})

	obj := p.Get()
	obj.Items = []int{10, 20, 30}
	p.Put(obj)

	if len(obj.Items) != 0 {
		t.Errorf("expected empty Items after reset, got len=%d", len(obj.Items))
	}
}

func TestPut_NestedPointerReset(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{Nested: &nestedObject{}}
	})

	obj := p.Get()
	obj.Nested.Value = "hello"
	p.Put(obj)

	if obj.Nested.Value != "" {
		t.Errorf("expected empty nested Value after reset, got %q", obj.Nested.Value)
	}
}

func TestMultipleGetPut_Cycles(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{}
	})

	const cycles = 10

	for i := range cycles {
		obj := p.Get()
		// The object should always be clean.
		if obj.ID != 0 || obj.Name != "" {
			t.Fatalf("cycle %d: expected clean object, got ID=%d Name=%q", i, obj.ID, obj.Name)
		}

		obj.ID = i
		obj.Name = "cycle"
		p.Put(obj)
	}
}

func TestConcurrentAccess(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{}
	})

	const goroutines = 20
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				obj := p.Get()
				// The object must be clean.
				if obj.ID != 0 || obj.Name != "" {
					t.Errorf("concurrent: expected clean object, got ID=%d Name=%q", obj.ID, obj.Name)
				}
				obj.ID = 1
				obj.Name = "conc"
				p.Put(obj)
			}
		}()
	}

	wg.Wait()
}

func TestPutAfterGet_ResetsNestedPointerBeforeReuse(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{Nested: &nestedObject{}}
	})

	obj := p.Get()
	obj.Nested.Value = "set"
	p.Put(obj)

	obj2 := p.Get()
	if obj2.Nested.Value != "" {
		t.Errorf("expected nested field to be reset, got %q", obj2.Nested.Value)
	}
}

func TestMultiplePut_DifferentObjects_AllReset(t *testing.T) {
	p := New(func() *testObject {
		return &testObject{}
	})

	objects := make([]*testObject, 5)
	for i := range objects {
		objects[i] = p.Get()
		objects[i].ID = i + 1
		objects[i].Name = "obj"
	}

	for _, obj := range objects {
		p.Put(obj)
	}

	for _, obj := range objects {
		if obj.ID != 0 || obj.Name != "" {
			t.Errorf("expected all fields reset after Put, got ID=%d Name=%q", obj.ID, obj.Name)
		}
	}
}
