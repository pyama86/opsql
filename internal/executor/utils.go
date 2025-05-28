package executor

import (
	"fmt"
	"reflect"
)

func compareValues(actual, expected interface{}) bool {
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	actualValue := reflect.ValueOf(actual)
	expectedValue := reflect.ValueOf(expected)

	if actualValue.Type() != expectedValue.Type() {
		actualStr := fmt.Sprintf("%v", actual)
		expectedStr := fmt.Sprintf("%v", expected)
		return actualStr == expectedStr
	}

	return reflect.DeepEqual(actual, expected)
}
