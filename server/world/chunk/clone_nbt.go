package chunk

import "reflect"

// cloneNBT copies raw NBT compounds and lists. Scalars and fixed numeric arrays
// are already values; arbitrary Go pointers and structs are not raw NBT.
func cloneNBT(value any) any {
	if value == nil {
		return nil
	}
	return cloneNBTValue(reflect.ValueOf(value)).Interface()
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
