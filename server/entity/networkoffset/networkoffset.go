// Package networkoffset owns the vertical offsets used by Bedrock entity movement packets.
package networkoffset

import "github.com/df-mc/dragonfly/server/entity"

const (
	playerStanding   = 1.62001
	playerSneaking   = 1.27001
	playerHorizontal = 0.4
	playerSleeping   = 0.2
	// The Bedrock wire offset is 0.5. Some server implementations expose a
	// separate 0.35 gameplay base offset for minecarts; that is not this value.
	minecart = 0.5
	boat     = 0.375
)

// PlayerPose contains the player states that change the network position offset.
type PlayerPose struct {
	Sleeping bool
	Swimming bool
	Crawling bool
	Gliding  bool
	Sneaking bool
}

// Player returns the network offset for a player's current pose.
func Player(pose PlayerPose) float64 {
	switch {
	case pose.Sleeping:
		return playerSleeping
	case pose.Swimming || pose.Crawling || pose.Gliding:
		return playerHorizontal
	case pose.Sneaking:
		return playerSneaking
	default:
		return playerStanding
	}
}

// Entity returns the fixed network offset for a built-in entity identifier.
func Entity(identifier string) float64 {
	switch identifier {
	case entity.ItemType.EncodeEntity():
		return entity.ItemType.NetworkOffset()
	case entity.FallingBlockType.EncodeEntity():
		return entity.FallingBlockType.NetworkOffset()
	case entity.TNTType.EncodeEntity():
		return entity.TNTType.NetworkOffset()
	case "minecraft:minecart", "minecraft:chest_minecart", "minecraft:command_block_minecart", "minecraft:hopper_minecart", "minecraft:tnt_minecart":
		return minecart
	case "minecraft:boat", "minecraft:chest_boat":
		return boat
	default:
		return 0
	}
}
