package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// PeriodicTableArgs represents the arguments for periodic table lookup
type PeriodicTableArgs struct {
	Element string `json:"element"` // Can be symbol or name
}

// PeriodicTableTool implements the Tool interface for looking up element information
type PeriodicTableTool struct{}

// NewPeriodicTableTool creates a new periodic table lookup tool
func NewPeriodicTableTool() *PeriodicTableTool {
	return &PeriodicTableTool{}
}

func (p *PeriodicTableTool) Name() string {
	return "lookup_element"
}

func (p *PeriodicTableTool) Description() string {
	return "Look up chemical element information from the periodic table including atomic number, mass, electron configuration, and properties."
}

func (p *PeriodicTableTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"element": map[string]any{
				"type":        "string",
				"description": "Element symbol (e.g., 'H', 'O', 'Fe') or name (e.g., 'Hydrogen', 'Oxygen')",
			},
		},
		"required": []string{"element"},
	}
}

func (p *PeriodicTableTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args PeriodicTableArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse periodic table args: %w", err)
	}

	// Mock element database
	elementInfo := getElementInfo(args.Element)
	if elementInfo == nil {
		return "", fmt.Errorf("element not found: %s", args.Element)
	}

	resultJSON, err := json.Marshal(elementInfo)
	if err != nil {
		return "", fmt.Errorf("failed to marshal element info: %w", err)
	}

	return string(resultJSON), nil
}

func getElementInfo(element string) map[string]any {
	// Mock database of common elements
	elements := map[string]map[string]any{
		"H": {
			"symbol":                 "H",
			"name":                   "Hydrogen",
			"atomic_number":          1,
			"atomic_mass":            1.008,
			"electron_configuration": "1s¹",
			"category":               "Nonmetal",
			"group":                  1,
			"period":                 1,
			"state":                  "Gas",
			"common_oxidation":       "+1",
			"electronegativity":      2.20,
		},
		"Hydrogen": {
			"symbol":                 "H",
			"name":                   "Hydrogen",
			"atomic_number":          1,
			"atomic_mass":            1.008,
			"electron_configuration": "1s¹",
			"category":               "Nonmetal",
			"group":                  1,
			"period":                 1,
			"state":                  "Gas",
			"common_oxidation":       "+1",
			"electronegativity":      2.20,
		},
		"O": {
			"symbol":                 "O",
			"name":                   "Oxygen",
			"atomic_number":          8,
			"atomic_mass":            15.999,
			"electron_configuration": "1s² 2s² 2p⁴",
			"category":               "Nonmetal",
			"group":                  16,
			"period":                 2,
			"state":                  "Gas",
			"common_oxidation":       "-2",
			"electronegativity":      3.44,
		},
		"C": {
			"symbol":                 "C",
			"name":                   "Carbon",
			"atomic_number":          6,
			"atomic_mass":            12.011,
			"electron_configuration": "1s² 2s² 2p²",
			"category":               "Nonmetal",
			"group":                  14,
			"period":                 2,
			"state":                  "Solid",
			"common_oxidation":       "+4, -4",
			"electronegativity":      2.55,
		},
		"Fe": {
			"symbol":                 "Fe",
			"name":                   "Iron",
			"atomic_number":          26,
			"atomic_mass":            55.845,
			"electron_configuration": "[Ar] 3d⁶ 4s²",
			"category":               "Transition Metal",
			"group":                  8,
			"period":                 4,
			"state":                  "Solid",
			"common_oxidation":       "+2, +3",
			"electronegativity":      1.83,
		},
		"Na": {
			"symbol":                 "Na",
			"name":                   "Sodium",
			"atomic_number":          11,
			"atomic_mass":            22.990,
			"electron_configuration": "[Ne] 3s¹",
			"category":               "Alkali Metal",
			"group":                  1,
			"period":                 3,
			"state":                  "Solid",
			"common_oxidation":       "+1",
			"electronegativity":      0.93,
		},
		"Cl": {
			"symbol":                 "Cl",
			"name":                   "Chlorine",
			"atomic_number":          17,
			"atomic_mass":            35.45,
			"electron_configuration": "[Ne] 3s² 3p⁵",
			"category":               "Halogen",
			"group":                  17,
			"period":                 3,
			"state":                  "Gas",
			"common_oxidation":       "-1",
			"electronegativity":      3.16,
		},
		"N": {
			"symbol":                 "N",
			"name":                   "Nitrogen",
			"atomic_number":          7,
			"atomic_mass":            14.007,
			"electron_configuration": "1s² 2s² 2p³",
			"category":               "Nonmetal",
			"group":                  15,
			"period":                 2,
			"state":                  "Gas",
			"common_oxidation":       "-3, +3, +5",
			"electronegativity":      3.04,
		},
	}

	if info, ok := elements[element]; ok {
		return info
	}

	return nil
}
