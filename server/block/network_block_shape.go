package block

import (
	"fmt"
	"math"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl64"
)

// networkBlockVector accepts homogeneous NBT lists without losing their numeric types.
func networkBlockVector(raw any) (mgl64.Vec3, error) {
	values, ok := customBlockEnumValues(raw)
	if !ok || len(values) != 3 {
		return mgl64.Vec3{}, fmt.Errorf("expected a three-number vector")
	}
	var vector mgl64.Vec3
	for i, v := range values {
		n, err := networkComponentNumber(v)
		if err != nil {
			return vector, err
		}
		vector[i] = n
	}
	return vector, nil
}

// networkBlockBoxes decodes the absolute-pixel and origin/size wire shapes.
func networkBlockBoxes(raw any, fallback []cube.BBox) ([]cube.BBox, error) {
	if raw == nil {
		return fallback, nil
	}
	if enabled, err := customBlockTraitEnabled(raw); err == nil {
		if enabled {
			return fallback, nil
		}
		return []cube.BBox{}, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected collision compound, got %T", raw)
	}
	if value, ok := m["enabled"]; ok {
		enabled, err := customBlockTraitEnabled(value)
		if err != nil {
			return nil, err
		}
		if !enabled {
			return []cube.BBox{}, nil
		}
	}
	if value, exists := m["boxes"]; exists {
		boxes, err := customBlockMaps(value, "boxes")
		if err != nil {
			return nil, err
		}
		result := make([]cube.BBox, 0, len(boxes))
		for _, box := range boxes {
			var lo, hi mgl64.Vec3
			for i, axis := range []string{"X", "Y", "Z"} {
				n, e := networkComponentNumber(box["min"+axis])
				if e != nil {
					return nil, e
				}
				lo[i] = n / 16
				n, e = networkComponentNumber(box["max"+axis])
				if e != nil {
					return nil, e
				}
				hi[i] = n / 16
			}
			b, err := checkedNetworkBox(lo, hi)
			if err != nil {
				return nil, err
			}
			result = append(result, b)
		}
		return result, nil
	}
	origin, hasOrigin := m["origin"]
	size, hasSize := m["size"]
	if !hasOrigin && !hasSize {
		return fallback, nil
	}
	lo, err := networkBlockVector(origin)
	if err != nil {
		return nil, err
	}
	dimensions, err := networkBlockVector(size)
	if err != nil {
		return nil, err
	}
	lo = lo.Add(mgl64.Vec3{8, 0, 8}).Mul(1.0 / 16)
	hi := lo.Add(dimensions.Mul(1.0 / 16))
	box, err := checkedNetworkBox(lo, hi)
	if err != nil {
		return nil, err
	}
	if lo[0] == hi[0] || lo[1] == hi[1] || lo[2] == hi[2] {
		return []cube.BBox{}, nil
	}
	return []cube.BBox{box}, nil
}

// checkedNetworkBox rejects invalid extents instead of normalising inverted network coordinates.
func checkedNetworkBox(lo, hi mgl64.Vec3) (cube.BBox, error) {
	for i := range lo {
		if hi[i] < lo[i] || math.Abs(lo[i]) > 16 || math.Abs(hi[i]) > 16 {
			return cube.BBox{}, fmt.Errorf("invalid block box %v..%v", lo, hi)
		}
	}
	return cube.Box(lo[0], lo[1], lo[2], hi[0], hi[1], hi[2]), nil
}

// transformNetworkBlockModel applies the wire's axis-aligned quarter-turn transforms.
func transformNetworkBlockModel(model networkBlockModel, raw any) (networkBlockModel, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return model, fmt.Errorf("transformation must be a compound")
	}
	rotation, translation, scale := mgl64.Vec3{}, mgl64.Vec3{}, mgl64.Vec3{1, 1, 1}
	for name, dst := range map[string]*mgl64.Vec3{"rotation": &rotation, "translation": &translation, "scale": &scale} {
		if value, ok := m[name]; ok {
			v, err := networkBlockVector(value)
			if err != nil {
				return model, err
			}
			*dst = v
		}
	}
	for i, axis := range []string{"X", "Y", "Z"} {
		for prefix, dst := range map[string]*mgl64.Vec3{"R": &rotation, "T": &translation, "S": &scale} {
			if value, ok := m[prefix+axis]; ok {
				n, err := networkComponentNumber(value)
				if err != nil {
					return model, err
				}
				if prefix == "R" {
					n *= 90
				}
				dst[i] = n
			}
		}
		if math.Mod(rotation[i], 90) != 0 {
			return model, fmt.Errorf("non-quarter-turn block rotation")
		}
	}
	pivot := mgl64.Vec3{0.5, 0.5, 0.5}
	rotationPivot, scalePivot := pivot, pivot
	for name, dst := range map[string]*mgl64.Vec3{"rotation_pivot": &rotationPivot, "scale_pivot": &scalePivot} {
		if value, ok := m[name]; ok {
			v, err := networkBlockVector(value)
			if err != nil {
				return model, err
			}
			*dst = v.Add(pivot)
		}
	}
	matrix := mgl64.HomogRotate3DZ(rotation[2] * math.Pi / 180).Mul4(mgl64.HomogRotate3DY(rotation[1] * math.Pi / 180)).Mul4(mgl64.HomogRotate3DX(rotation[0] * math.Pi / 180))
	transform := func(boxes []cube.BBox) ([]cube.BBox, error) {
		result := make([]cube.BBox, 0, len(boxes))
		for _, box := range boxes {
			lo, hi := mgl64.Vec3{math.Inf(1), math.Inf(1), math.Inf(1)}, mgl64.Vec3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
			for corner := 0; corner < 8; corner++ {
				v := box.Min()
				for i := range v {
					if corner&(1<<i) != 0 {
						v[i] = box.Max()[i]
					}
					v[i] = (v[i]-scalePivot[i])*scale[i] + scalePivot[i]
				}
				v = matrix.Mul4x1(v.Sub(rotationPivot).Vec4(1)).Vec3().Add(rotationPivot).Add(translation)
				for i := range v {
					v[i] = math.Round(v[i]*1e12) / 1e12
					lo[i] = min(lo[i], v[i])
					hi[i] = max(hi[i], v[i])
				}
			}
			transformed, err := checkedNetworkBox(lo, hi)
			if err != nil {
				return nil, err
			}
			result = append(result, transformed)
		}
		return result, nil
	}
	var err error
	model.collision, err = transform(model.collision)
	if err != nil {
		return model, err
	}
	model.selection, err = transform(model.selection)
	return model, err
}
