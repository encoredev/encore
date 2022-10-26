package config

// SetValueForTest changes the value of cfg to newValue within the current test and any subtests.
func SetValueForTest[T any](value Value[T], newValue T) {
	// Check we're running in a test
	req := Singleton.rt.Current().Req
	if req == nil || req.Test == nil {
		panic("et.SetCfg called outside of a unit test")
	}

	// Get the value ID
	valueID, _ := GetMetaForValue(value)

	Singleton.testMutex.Lock()
	defer Singleton.testMutex.Unlock()

	// Get the overrides map for this test
	overrides, found := Singleton.testOverrides[req.Test.Current]
	if !found {
		overrides = make(map[ValueID]any)
		Singleton.testOverrides[req.Test.Current] = overrides
	}

	overrides[valueID] = newValue
}

// testOverrideOrValue returns an overridden value if one exists for this test or it's parents
// otherwise it returns the originalValue.
//
// If we're not in a unit test, it returns the originalValue.
func testOverrideOrValue[T any](valueID ValueID, originalValue T) T {
	req := Singleton.rt.Current().Req
	if req == nil || req.Test == nil {
		// Not in a unit test
		return originalValue
	}
	testData := req.Test

	Singleton.testMutex.RLock()
	defer Singleton.testMutex.RUnlock()

	// Get the overrides map for this test or any of the parent tests
	for testData != nil && testData.Current != nil {
		// Check if this test has overrides
		overrides, found := Singleton.testOverrides[testData.Current]
		if found {
			if overriddenValue, found := overrides[valueID]; found {
				return overriddenValue.(T)
			}
		}

		// Iterate up the test parents
		if testData.Parent == nil {
			break
		}
		testData = testData.Parent.Test
	}

	return originalValue
}
