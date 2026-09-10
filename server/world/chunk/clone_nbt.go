package chunk

import "reflect"

// cloneNBT copies mutable NBT containers without enumerating scalar or list
// element types. NBT is an acyclic tree, so no reference-cycle tracking is needed.
func cloneNBT(value any) any {
	if value == nil {
		return nil
	}
	return cloneNBTValue(reflect.ValueOf(value)).Interface()
}

// cloneNBTValue preserves concrete Go types while detaching the containers that
// the NBT encoder can traverse, including typed lists and nested compounds.
func cloneNBTValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copied := reflect.New(value.Type()).Elem()
		if value.Kind() == reflect.Pointer {
			copied = reflect.New(value.Type().Elem())
			copied.Elem().Set(cloneNBTValue(value.Elem()))
		} else {
			copied.Set(cloneNBTValue(value.Elem()))
		}
		return copied
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copied := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			copied.SetMapIndex(iter.Key(), cloneNBTValue(iter.Value()))
		}
		return copied
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		copied := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			copied.Index(i).Set(cloneNBTValue(value.Index(i)))
		}
		return copied
	case reflect.Array:
		copied := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			copied.Index(i).Set(cloneNBTValue(value.Index(i)))
		}
		return copied
	case reflect.Struct:
		copied := reflect.New(value.Type()).Elem()
		copied.Set(value)
		for i := 0; i < value.NumField(); i++ {
			if copied.Field(i).CanSet() && value.Field(i).CanInterface() {
				copied.Field(i).Set(cloneNBTValue(value.Field(i)))
			}
		}
		return copied
	default:
		return value
	}
}
