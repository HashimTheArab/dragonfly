package block

// RedstoneConnectionType is the visible connection of redstone wire on one side.
type RedstoneConnectionType struct {
	redstoneConnectionType
}

// NoRedstoneConnection returns a side without a wire connection.
func NoRedstoneConnection() RedstoneConnectionType {
	return RedstoneConnectionType{0}
}

// SideRedstoneConnection returns a horizontal wire connection.
func SideRedstoneConnection() RedstoneConnectionType {
	return RedstoneConnectionType{1}
}

// UpRedstoneConnection returns a wire connection climbing the adjacent block.
func UpRedstoneConnection() RedstoneConnectionType {
	return RedstoneConnectionType{2}
}

// RedstoneConnectionTypes returns every encoded wire connection type.
func RedstoneConnectionTypes() []RedstoneConnectionType {
	return []RedstoneConnectionType{NoRedstoneConnection(), SideRedstoneConnection(), UpRedstoneConnection()}
}

type redstoneConnectionType uint8

// Uint8 returns the connection type as a uint8.
func (r redstoneConnectionType) Uint8() uint8 {
	return uint8(r)
}

// String returns the block state value for the connection.
func (r redstoneConnectionType) String() string {
	switch r {
	case 0:
		return "none"
	case 1:
		return "side"
	case 2:
		return "up"
	}
	panic("unknown redstone connection type")
}
