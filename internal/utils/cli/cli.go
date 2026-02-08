// Package cli provides Cobra flag registration helpers.
package cli

import (
	"reflect"

	"github.com/spf13/cobra"
)

// RegisterFlag registers a command-line flag for a Cobra command based on the provided
// name, shorthand, value, usage description, and target variable. It supports bool,
// string, float64, int, and string slice types, ensuring the target is a pointer.
// If the value type is unsupported, the function panics.
func RegisterFlag(cmd *cobra.Command, name, shorthand string, value interface{}, usage string, target interface{}) {
	targetValue := reflect.ValueOf(target)

	// Ensure target is a pointer
	if targetValue.Kind() != reflect.Ptr {
		panic("target must be a pointer")
	}

	// Dereference the pointer to get the actual value type
	elemType := targetValue.Elem().Kind()

	// Format the usage string, adding a newline and the default value if it's a bool flag
	switch v := value.(type) {
	case bool:
		if !v {
			usage += "\n (default false)"
		} else {
			usage += "\n"
		}
	case string:
		usage += "\n"
	case float64, int, []string:
		usage += "\n"
	default:
		panic("unsupported flag type")
	}

	// Register the flag based on the value type
	switch elemType {
	case reflect.Bool:
		cmd.Flags().BoolVarP(target.(*bool), name, shorthand, value.(bool), usage)
	case reflect.String:
		cmd.Flags().StringVarP(target.(*string), name, shorthand, value.(string), usage)
	case reflect.Float64:
		cmd.Flags().Float64VarP(target.(*float64), name, shorthand, value.(float64), usage)
	case reflect.Int:
		cmd.Flags().IntVarP(target.(*int), name, shorthand, value.(int), usage)
	case reflect.Slice:
		if targetValue.Elem().Type().Elem().Kind() == reflect.String {
			cmd.Flags().StringSliceVarP(target.(*[]string), name, shorthand, value.([]string), usage)
		} else {
			panic("unsupported slice type")
		}
	default:
		panic("unsupported flag type")
	}
}
