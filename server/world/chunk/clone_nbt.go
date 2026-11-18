package chunk

import "reflect"

// CloneBlockEntityData detaches raw block entity NBT compounds and lists. Scalars
// and fixed numeric arrays are already values; arbitrary Go pointers and structs
// are not raw NBT. Nil stays nil.
func CloneBlockEntityData(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	return cloneCompound(data)
}

// cloneCompound copies an untyped compound, the shape the NBT decoder produces.
func cloneCompound(data map[string]any) map[string]any {
	copied := make(map[string]any, len(data))
	for k, v := range data {
		copied[k] = cloneNBT(v)
	}
	return copied
}

// cloneNBT detaches one NBT value. Untyped compounds and lists are copied
// directly and every other value that holds no container is returned as is, so
// scalars keep the interface they arrived in instead of being boxed again.
// Other container types go through cloneNBTValue to keep their concrete type.
func cloneNBT(value any) any {
	switch v := value.(type) {
	case map[string]any:
		if v == nil {
			return value
		}
		return cloneCompound(v)
	case []any:
		if v == nil {
			return value
		}
		copied := make([]any, len(v))
		for i, e := range v {
			copied[i] = cloneNBT(e)
		}
		return copied
	}
	switch rv := reflect.ValueOf(value); rv.Kind() {
	case reflect.Interface, reflect.Map, reflect.Slice:
		return cloneNBTValue(rv).Interface()
	default:
		return value
	}
}

// cloneNBTValue detaches containers while preserving concrete list types.
// Reflection avoids a separate case for every possible typed NBT list.
func cloneNBTValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		copied := reflect.New(value.Type()).Elem()
		copied.Set(cloneNBTValue(value.Elem()))
		return copied
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		copied := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			copied.SetMapIndex(iter.Key(), cloneNBTValue(iter.Value()))
		}
		return copied
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		copied := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			copied.Index(i).Set(cloneNBTValue(value.Index(i)))
		}
		return copied
	default:
		return value
	}
}
