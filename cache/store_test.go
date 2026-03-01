package cache

import (
	"sync"
	"testing"
)

func TestStore_SetGetDelete(t *testing.T) {
	store := NewStore()

	// Test valid types
	err := store.Set("str", "value")
	if err != nil {
		t.Errorf("Expected no error for string, got %v", err)
	}
	err = store.Set("int", 123)
	if err != nil {
		t.Errorf("Expected no error for int, got %v", err)
	}
	err = store.Set("bool", true)
	if err != nil {
		t.Errorf("Expected no error for bool, got %v", err)
	}
	err = store.Set("float", 123.45)
	if err != nil {
		t.Errorf("Expected no error for float64, got %v", err)
	}

	// Test invalid type
	err = store.Set("invalid", []int{1, 2, 3})
	if err == nil {
		t.Errorf("Expected error for slice type, got nil")
	}

	// Test Get
	val, ok := store.Get("str")
	if !ok || val != "value" {
		t.Errorf("Expected string 'value', got %v", val)
	}

	// Test Delete
	store.Delete("str")
	_, ok = store.Get("str")
	if ok {
		t.Errorf("Expected key 'str' to be deleted")
	}
}

func TestStore_Concurrency(t *testing.T) {
	store := NewStore()
	var wg sync.WaitGroup

	numGoroutines := 100
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			key := "key"
			store.Set(key, id)
			store.Get(key)
		}(i)
	}

	wg.Wait()
	_, ok := store.Get("key")
	if !ok {
		t.Errorf("Expected key to exist after concurrent writes")
	}
}
