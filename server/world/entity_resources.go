package world

// EntityResources contains the client entity, optional behaviour definition, and
// supporting resource files for one custom entity type. JSON identifiers must
// match its network identifier. Asset paths are relative to the resource pack.
type EntityResources struct {
	ClientEntity []byte
	Definition   []byte
	Assets       map[string][]byte
}

// ResourcePackEntityType supplies resources before the server starts listening.
type ResourcePackEntityType interface {
	EntityType
	Resources() EntityResources
}

// EntityNetworkIdentifier returns the client-facing identifier of t.
func EntityNetworkIdentifier(t EntityType) string {
	if network, ok := t.(interface{ NetworkEncodeEntity() string }); ok {
		return network.NetworkEncodeEntity()
	}
	return t.EncodeEntity()
}
