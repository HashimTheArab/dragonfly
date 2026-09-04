package networkoffset

import "testing"

func TestPlayerUsesPoseSpecificOffset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pose PlayerPose
		want float64
	}{
		{name: "standing", want: 1.62001},
		{name: "sneaking", pose: PlayerPose{Sneaking: true}, want: 1.27001},
		{name: "swimming", pose: PlayerPose{Swimming: true}, want: 0.4},
		{name: "crawling", pose: PlayerPose{Crawling: true}, want: 0.4},
		{name: "gliding", pose: PlayerPose{Gliding: true}, want: 0.4},
		{name: "sleeping", pose: PlayerPose{Sleeping: true, Sneaking: true}, want: 0.2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Player(test.pose); got != test.want {
				t.Fatalf("Player() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestEntityUsesProtocolTypeOffset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		identifier string
		want       float64
	}{
		{identifier: "minecraft:item", want: 0.5},
		{identifier: "minecraft:falling_block", want: 0.5},
		{identifier: "minecraft:tnt", want: 0.49},
		{identifier: "minecraft:minecart", want: 0.5},
		{identifier: "minecraft:chest_minecart", want: 0.5},
		{identifier: "minecraft:command_block_minecart", want: 0.5},
		{identifier: "minecraft:hopper_minecart", want: 0.5},
		{identifier: "minecraft:tnt_minecart", want: 0.5},
		{identifier: "minecraft:boat", want: 0.375},
		{identifier: "minecraft:chest_boat", want: 0.375},
		{identifier: "minecraft:zombie"},
	}
	for _, test := range tests {
		t.Run(test.identifier, func(t *testing.T) {
			if got := Entity(test.identifier); got != test.want {
				t.Fatalf("Entity(%q) = %v, want %v", test.identifier, got, test.want)
			}
		})
	}
}
