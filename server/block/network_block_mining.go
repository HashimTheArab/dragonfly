package block

import (
	"fmt"
	"go/ast"
	"slices"
	"strings"

	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/item/recipe"
)

type networkMiningRule struct {
	name       string
	aux        int16
	tag        string
	expression ast.Expr
	hardness   float64
}

// decodeNetworkMiningRules reads the 26.30 ItemDescriptor compound and its hardness override.
func decodeNetworkMiningRules(raw any) ([]networkMiningRule, error) {
	entries, err := customBlockMaps(raw, "item_specific_speeds")
	if err != nil {
		return nil, err
	}
	rules := make([]networkMiningRule, 0, len(entries))
	for _, entry := range entries {
		hardness, err := networkComponentNumber(entry["destroy_speed"])
		if err != nil {
			return nil, err
		}
		rule := networkMiningRule{hardness: hardness, aux: -1}
		descriptor, ok := entry["item"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("mining item descriptor must be a compound")
		}
		switch {
		case descriptor["Tags"] != nil:
			expression, ok := descriptor["Tags"].(string)
			if !ok {
				return nil, fmt.Errorf("item Tags must be a string")
			}
			rule.expression, err = parseNetworkCondition(expression)
			if err != nil {
				return nil, err
			}
			// Validate every query during setup, even if a particular held item would short circuit it.
			if _, err = blockConditionValue(rule.expression, func(name string, args []any) (any, error) { return miningTagQuery(name, args, item.Stack{}) }); err != nil {
				return nil, err
			}
		case descriptor["ItemTag"] != nil:
			rule.tag, ok = descriptor["ItemTag"].(string)
			if !ok || rule.tag == "" {
				return nil, fmt.Errorf("invalid ItemTag")
			}
		default:
			rule.name, ok = descriptor["Name"].(string)
			if !ok || rule.name == "" {
				return nil, fmt.Errorf("mining descriptor has no Name")
			}
			if raw, exists := descriptor["Aux"]; exists {
				aux, ok := raw.(int16)
				if !ok {
					return nil, fmt.Errorf("item Aux must be a short")
				}
				rule.aux = aux
			}
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// breakInfoForItem uses the first matching override, as PlayerDestroy does in 26.30.
func (b mineableNetworkBlock) breakInfoForItem(stack item.Stack) BreakInfo {
	hardness := b.hardness
	for _, rule := range b.rules {
		if rule.matches(stack) {
			hardness = rule.hardness
			break
		}
	}
	return newBreakInfo(hardness, alwaysHarvestable, b.effectiveTool, simpleDrops())
}

// matches compares item identity, an item tag, or a validated tag expression.
func (r networkMiningRule) matches(stack item.Stack) bool {
	if r.expression != nil {
		value, err := blockConditionValue(r.expression, func(name string, args []any) (any, error) { return miningTagQuery(name, args, stack) })
		return err == nil && blockConditionTrue(value)
	}
	if r.tag != "" {
		return networkItemHasTag(stack, r.tag)
	}
	name, aux := "minecraft:air", int16(0)
	if !stack.Empty() {
		name, aux = stack.Item().EncodeItem()
	}
	return name == r.name && (r.aux == -1 || r.aux == aux)
}

// miningTagQuery supports the item-tag queries accepted by network item descriptors.
func miningTagQuery(name string, args []any, stack item.Stack) (any, error) {
	if (name != "any_tag" && name != "all_tags") || len(args) == 0 {
		return nil, fmt.Errorf("unsupported item-tag query %s", name)
	}
	result := name == "all_tags"
	for _, arg := range args {
		tag, ok := arg.(string)
		if !ok {
			return nil, fmt.Errorf("item tag must be a string")
		}
		matches := networkItemHasTag(stack, tag)
		if name == "all_tags" {
			result = result && matches
		} else {
			result = result || matches
		}
	}
	if result {
		return float64(1), nil
	}
	return float64(0), nil
}

// networkItemHasTag reads the canonical vanilla tag registry and optional item-owned tags.
func networkItemHasTag(stack item.Stack, tag string) bool {
	if stack.Empty() {
		return false
	}
	if tagged, ok := stack.Item().(interface{ Tags() []string }); ok && slices.Contains(tagged.Tags(), tag) {
		return true
	}
	name, _ := stack.Item().EncodeItem()
	return recipe.NewItemTag(tag, 1).Contains(name)
}

// effectiveTool derives tool effectiveness from the advertised destructible tags and
// the canonical item-tag registry, rather than keeping another list of tool types.
func (b networkBlock) effectiveTool(tool item.Tool) bool {
	encoded, ok := tool.(interface{ EncodeItem() (string, int16) })
	if !ok {
		return false
	}
	name, _ := encoded.EncodeItem()
	for _, tag := range b.tags {
		if itemTag, ok := strings.CutSuffix(tag, "_item_destructible"); ok && recipe.NewItemTag(itemTag, 1).Contains(name) {
			return true
		}
	}
	return false
}
