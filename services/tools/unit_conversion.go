package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// UnitConversionArgs represents the arguments for unit conversion tool
type UnitConversionArgs struct {
	Value    float64 `json:"value"`
	FromUnit string  `json:"from_unit"`
	ToUnit   string  `json:"to_unit"`
	Category string  `json:"category,omitempty"` // length, mass, temperature, volume, etc.
}

// UnitConversionTool implements the Tool interface for converting between units
type UnitConversionTool struct{}

// NewUnitConversionTool creates a new unit conversion tool
func NewUnitConversionTool() *UnitConversionTool {
	return &UnitConversionTool{}
}

func (u *UnitConversionTool) Name() string {
	return "convert_units"
}

func (u *UnitConversionTool) Description() string {
	return "Convert between different units of measurement (length, mass, temperature, volume, energy, etc.). Useful for physics and chemistry problems."
}

func (u *UnitConversionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{
				"type":        "number",
				"description": "The numerical value to convert",
			},
			"from_unit": map[string]any{
				"type":        "string",
				"description": "The unit to convert from (e.g., 'meters', 'kg', 'celsius', 'joules')",
			},
			"to_unit": map[string]any{
				"type":        "string",
				"description": "The unit to convert to (e.g., 'feet', 'lbs', 'fahrenheit', 'calories')",
			},
			"category": map[string]any{
				"type":        "string",
				"description": "Category of units (length, mass, temperature, volume, energy, pressure)",
			},
		},
		"required": []string{"value", "from_unit", "to_unit"},
	}
}

func (u *UnitConversionTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args UnitConversionArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse unit conversion args: %w", err)
	}

	// Perform mock conversion
	convertedValue, formula := convertUnits(args.Value, args.FromUnit, args.ToUnit)

	result := map[string]any{
		"original_value":  args.Value,
		"original_unit":   args.FromUnit,
		"converted_value": convertedValue,
		"converted_unit":  args.ToUnit,
		"formula":         formula,
		"category":        detectCategory(args.FromUnit, args.ToUnit),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal conversion result: %w", err)
	}

	return string(resultJSON), nil
}

func convertUnits(value float64, from, to string) (float64, string) {
	// Mock conversion with common conversions
	conversionKey := from + "_to_" + to

	conversions := map[string]struct {
		factor  float64
		formula string
	}{
		// Length
		"meters_to_feet":        {3.28084, "meters × 3.28084"},
		"feet_to_meters":        {0.3048, "feet × 0.3048"},
		"meters_to_kilometers":  {0.001, "meters ÷ 1000"},
		"kilometers_to_meters":  {1000, "kilometers × 1000"},
		"inches_to_centimeters": {2.54, "inches × 2.54"},
		"centimeters_to_inches": {0.393701, "centimeters × 0.393701"},

		// Mass
		"kg_to_lbs":   {2.20462, "kg × 2.20462"},
		"lbs_to_kg":   {0.453592, "lbs × 0.453592"},
		"grams_to_kg": {0.001, "grams ÷ 1000"},
		"kg_to_grams": {1000, "kg × 1000"},

		// Temperature
		"celsius_to_fahrenheit": {0, "(°C × 9/5) + 32"},
		"fahrenheit_to_celsius": {0, "(°F - 32) × 5/9"},
		"celsius_to_kelvin":     {0, "°C + 273.15"},
		"kelvin_to_celsius":     {0, "K - 273.15"},

		// Volume
		"liters_to_gallons":     {0.264172, "liters × 0.264172"},
		"gallons_to_liters":     {3.78541, "gallons × 3.78541"},
		"milliliters_to_liters": {0.001, "mL ÷ 1000"},

		// Energy
		"joules_to_calories": {0.239006, "joules × 0.239006"},
		"calories_to_joules": {4.184, "calories × 4.184"},

		// Pressure
		"pascal_to_atm": {0.00000986923, "Pa × 0.00000986923"},
		"atm_to_pascal": {101325, "atm × 101325"},
	}

	if conv, ok := conversions[conversionKey]; ok {
		// Special handling for temperature
		if from == "celsius" && to == "fahrenheit" {
			return (value * 9 / 5) + 32, conv.formula
		}
		if from == "fahrenheit" && to == "celsius" {
			return (value - 32) * 5 / 9, conv.formula
		}
		if from == "celsius" && to == "kelvin" {
			return value + 273.15, conv.formula
		}
		if from == "kelvin" && to == "celsius" {
			return value - 273.15, conv.formula
		}

		return value * conv.factor, conv.formula
	}

	// Default: return same value if conversion not found
	return value, fmt.Sprintf("Direct conversion from %s to %s", from, to)
}

func detectCategory(from, to string) string {
	lengthUnits := []string{"meters", "feet", "kilometers", "inches", "centimeters", "miles"}
	massUnits := []string{"kg", "lbs", "grams", "pounds", "ounces"}
	tempUnits := []string{"celsius", "fahrenheit", "kelvin"}
	volumeUnits := []string{"liters", "gallons", "milliliters", "cubic meters"}
	energyUnits := []string{"joules", "calories", "kilocalories", "BTU"}

	for _, unit := range lengthUnits {
		if from == unit || to == unit {
			return "length"
		}
	}
	for _, unit := range massUnits {
		if from == unit || to == unit {
			return "mass"
		}
	}
	for _, unit := range tempUnits {
		if from == unit || to == unit {
			return "temperature"
		}
	}
	for _, unit := range volumeUnits {
		if from == unit || to == unit {
			return "volume"
		}
	}
	for _, unit := range energyUnits {
		if from == unit || to == unit {
			return "energy"
		}
	}

	return "unknown"
}
