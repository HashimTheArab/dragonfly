package block

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// evaluateBlockCondition evaluates state-only permutation expressions. Unsupported
// queries fail registration instead of silently selecting the wrong geometry.
func evaluateBlockCondition(condition string, state map[string]any) (bool, error) {
	expression, err := parseNetworkCondition(condition)
	if err != nil {
		return false, err
	}
	value, err := blockConditionValue(expression, func(name string, args []any) (any, error) {
		if (name != "block_state" && name != "block_property") || len(args) != 1 {
			return nil, fmt.Errorf("unsupported block query %s", name)
		}
		key, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("block state key must be a string")
		}
		value, ok := state[key]
		if !ok {
			return nil, fmt.Errorf("permutation references missing state %s", key)
		}
		switch v := value.(type) {
		case string:
			return v, nil
		case bool:
			if v {
				return float64(1), nil
			}
			return float64(0), nil
		default:
			return networkComponentNumber(value)
		}
	})
	if err != nil {
		return false, err
	}
	return blockConditionTrue(value), nil
}

// parseNetworkCondition parses bounded state and item-tag expressions without executing code.
func parseNetworkCondition(condition string) (ast.Expr, error) {
	if len(condition) > 4096 {
		return nil, fmt.Errorf("permutation condition exceeds 4096 bytes")
	}
	var text strings.Builder
	for i := 0; i < len(condition); i++ {
		if condition[i] != '\'' {
			text.WriteByte(condition[i])
			continue
		}
		start := i + 1
		i++
		for i < len(condition) && condition[i] != '\'' {
			i++
		}
		if i == len(condition) {
			return nil, fmt.Errorf("unterminated permutation string")
		}
		text.WriteString(strconv.Quote(condition[start:i]))
	}
	expression, err := parser.ParseExpr(text.String())
	if err != nil {
		return nil, fmt.Errorf("unsupported permutation condition: %w", err)
	}
	return expression, nil
}

// blockConditionTrue applies Molang's numeric truth convention.
func blockConditionTrue(v any) bool { n, ok := v.(float64); return ok && n != 0 }

// blockConditionValue evaluates a bounded expression without access to player or process state.
func blockConditionValue(expression ast.Expr, query func(string, []any) (any, error)) (any, error) {
	switch e := expression.(type) {
	case *ast.ParenExpr:
		return blockConditionValue(e.X, query)
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return strconv.Unquote(e.Value)
		}
		return strconv.ParseFloat(e.Value, 64)
	case *ast.Ident:
		if e.Name == "true" {
			return float64(1), nil
		}
		if e.Name == "false" {
			return float64(0), nil
		}
	case *ast.CallExpr:
		selector, ok := e.Fun.(*ast.SelectorExpr)
		if !ok {
			break
		}
		owner, ok := selector.X.(*ast.Ident)
		if !ok || (owner.Name != "q" && owner.Name != "query") {
			break
		}
		args := make([]any, len(e.Args))
		for i, arg := range e.Args {
			value, err := blockConditionValue(arg, query)
			if err != nil {
				return nil, err
			}
			args[i] = value
		}
		return query(selector.Sel.Name, args)
	case *ast.UnaryExpr:
		value, err := blockConditionValue(e.X, query)
		if err != nil {
			return nil, err
		}
		n, ok := value.(float64)
		if !ok {
			break
		}
		switch e.Op {
		case token.NOT:
			if n == 0 {
				return float64(1), nil
			}
			return float64(0), nil
		case token.SUB:
			return -n, nil
		case token.ADD:
			return n, nil
		}
	case *ast.BinaryExpr:
		left, err := blockConditionValue(e.X, query)
		if err != nil {
			return nil, err
		}
		right, err := blockConditionValue(e.Y, query)
		if err != nil {
			return nil, err
		}
		var result bool
		switch e.Op {
		case token.EQL:
			result = left == right
		case token.NEQ:
			result = left != right
		case token.LAND:
			result = blockConditionTrue(left) && blockConditionTrue(right)
		case token.LOR:
			result = blockConditionTrue(left) || blockConditionTrue(right)
		default:
			a, ok := left.(float64)
			if !ok {
				return nil, fmt.Errorf("numeric operator applied to a string")
			}
			b, ok := right.(float64)
			if !ok {
				return nil, fmt.Errorf("numeric operator applied to a string")
			}
			switch e.Op {
			case token.ADD:
				return a + b, nil
			case token.SUB:
				return a - b, nil
			case token.MUL:
				return a * b, nil
			case token.QUO:
				if b == 0 {
					return nil, fmt.Errorf("division by zero in permutation")
				}
				return a / b, nil
			case token.LSS:
				result = a < b
			case token.LEQ:
				result = a <= b
			case token.GTR:
				result = a > b
			case token.GEQ:
				result = a >= b
			default:
				return nil, fmt.Errorf("unsupported permutation operator %s", e.Op)
			}
		}
		if result {
			return float64(1), nil
		}
		return float64(0), nil
	}
	return nil, fmt.Errorf("unsupported permutation expression %T", expression)
}
