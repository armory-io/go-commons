package maputils

import (
	"golang.org/x/exp/maps"
	"reflect"
	"strings"
)

// MergeSources recursively left merges config sources, omitting any non-map values that are not one of: strings, lists, numbers, or booleans
// un-flattens keys before merging into new map
func MergeSources(sources ...map[string]any) map[string]any {
	m := make(map[string]any)
	for _, unNormalizedSource := range sources {
		source := NormalizeKeys(unNormalizedSource)
		// iterate through key and if the value is a map recurse, else set the key to the value if type is a number, list or boolean
		for key := range source {
			val := source[key]
			cur := m[key]
			if cur == nil {
				m[key] = val
				continue
			}

			curT := reflect.TypeOf(cur)
			valT := reflect.TypeOf(val)
			switch curT.Kind() {
			case reflect.Map:
				typedCur := cur.(map[string]any)
				if valT.Kind() == reflect.Map {
					typedVal := val.(map[string]any)
					m[key] = MergeSources(typedCur, typedVal)
				} else {
					m[key] = val
				}
			case reflect.Array, reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64:
				m[key] = val
			}
		}
	}
	return m
}

func NormalizeKeys(source map[string]any) map[string]any {
	m := make(map[string]any)
	// un-flatten keys, ['foo.bar.bam']=true -> ['foo']['bar']['bam']=true
	for _, key := range maps.Keys(source) {
		normalizedKey := strings.ToLower(key)
		val := source[key]
		if strings.Contains(normalizedKey, ".") {
			parts := strings.Split(normalizedKey, ".")
			SetValue(m, parts, val)
		} else {
			m[normalizedKey] = val
		}
	}
	return m
}

func SetValue(config map[string]any, key []string, value any) {
	if len(key) == 1 {
		config[key[0]] = value
		return
	}
	cur, remaining := pop(key)
	var nested map[string]any
	if config[cur] == nil {
		nested = make(map[string]any)
	} else {
		curNested := config[cur]
		unboxed, ok := curNested.(map[string]any)
		if !ok {
			nested = make(map[string]any)
		} else {
			nested = unboxed
		}
	}
	config[cur] = nested
	SetValue(nested, remaining, value)
}

func pop[T any](array []T) (T, []T) {
	return array[0], array[1:]
}
