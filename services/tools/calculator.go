package tools

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// CalculatorArgs represents the arguments for calculate tool
type CalculatorArgs struct {
	Expression string `json:"expression"`
}

// ExecuteCalculatorTool evaluates a mathematical expression
func ExecuteCalculatorTool(argsJSON string) (string, error) {
	var args CalculatorArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse calculator args: %w", err)
	}

	// Evaluate the expression
	result, err := evaluateExpression(args.Expression)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate expression: %w", err)
	}

	// Return result as JSON
	resultData := map[string]any{
		"expression": args.Expression,
		"result":     result,
	}

	resultJSON, err := json.Marshal(resultData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal calculator result: %w", err)
	}

	return string(resultJSON), nil
}

// evaluateExpression evaluates a simple mathematical expression
// Supports: +, -, *, /, ^, sqrt, and parentheses
func evaluateExpression(expr string) (float64, error) {
	// Remove whitespace
	expr = strings.ReplaceAll(expr, " ", "")

	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}

	// Parse and evaluate
	result, _, err := parseExpression(expr, 0)
	return result, err
}

// parseExpression parses an expression with operator precedence
func parseExpression(expr string, pos int) (float64, int, error) {
	left, newPos, err := parseTerm(expr, pos)
	if err != nil {
		return 0, newPos, err
	}

	for newPos < len(expr) {
		op := expr[newPos]
		if op != '+' && op != '-' {
			break
		}
		newPos++

		right, nextPos, err := parseTerm(expr, newPos)
		if err != nil {
			return 0, nextPos, err
		}

		if op == '+' {
			left += right
		} else {
			left -= right
		}
		newPos = nextPos
	}

	return left, newPos, nil
}

// parseTerm handles multiplication and division
func parseTerm(expr string, pos int) (float64, int, error) {
	left, newPos, err := parseFactor(expr, pos)
	if err != nil {
		return 0, newPos, err
	}

	for newPos < len(expr) {
		op := expr[newPos]
		if op != '*' && op != '/' {
			break
		}
		newPos++

		right, nextPos, err := parseFactor(expr, newPos)
		if err != nil {
			return 0, nextPos, err
		}

		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, nextPos, fmt.Errorf("division by zero")
			}
			left /= right
		}
		newPos = nextPos
	}

	return left, newPos, nil
}

// parseFactor handles numbers, parentheses, and functions
func parseFactor(expr string, pos int) (float64, int, error) {
	if pos >= len(expr) {
		return 0, pos, fmt.Errorf("unexpected end of expression")
	}

	// Handle negative numbers
	if expr[pos] == '-' {
		val, newPos, err := parseFactor(expr, pos+1)
		return -val, newPos, err
	}

	// Handle positive sign
	if expr[pos] == '+' {
		return parseFactor(expr, pos+1)
	}

	// Handle parentheses
	if expr[pos] == '(' {
		val, newPos, err := parseExpression(expr, pos+1)
		if err != nil {
			return 0, newPos, err
		}
		if newPos >= len(expr) || expr[newPos] != ')' {
			return 0, newPos, fmt.Errorf("mismatched parentheses")
		}
		return val, newPos + 1, nil
	}

	// Handle sqrt function
	if strings.HasPrefix(expr[pos:], "sqrt(") {
		val, newPos, err := parseExpression(expr, pos+5)
		if err != nil {
			return 0, newPos, err
		}
		if newPos >= len(expr) || expr[newPos] != ')' {
			return 0, newPos, fmt.Errorf("mismatched parentheses in sqrt")
		}
		if val < 0 {
			return 0, newPos, fmt.Errorf("square root of negative number")
		}
		return math.Sqrt(val), newPos + 1, nil
	}

	// Handle power operator (^)
	numStr := ""
	startPos := pos
	for pos < len(expr) && (isDigit(expr[pos]) || expr[pos] == '.') {
		numStr += string(expr[pos])
		pos++
	}

	if numStr == "" {
		return 0, pos, fmt.Errorf("expected number at position %d", startPos)
	}

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, pos, fmt.Errorf("invalid number: %s", numStr)
	}

	// Handle exponentiation
	if pos < len(expr) && expr[pos] == '^' {
		exponent, newPos, err := parseFactor(expr, pos+1)
		if err != nil {
			return 0, newPos, err
		}
		return math.Pow(num, exponent), newPos, nil
	}

	return num, pos, nil
}

// isDigit checks if a character is a digit
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
