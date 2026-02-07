package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterFlag_BoolFlag(t *testing.T) {
	// Arrange
	var boolTarget bool
	cmd := &cobra.Command{}

	// Act
	RegisterFlag(cmd, "verbose", "v", false, "Enable verbose mode", &boolTarget)

	// Assert
	flag := cmd.Flags().Lookup("verbose")
	require.NotNil(t, flag)
	assert.Equal(t, "verbose", flag.Name)
	assert.Equal(t, "v", flag.Shorthand)
	assert.Equal(t, "Enable verbose mode\n (default false)", flag.Usage)
	assert.Equal(t, "false", flag.DefValue)
}

func TestRegisterFlag_StringFlag(t *testing.T) {
	// Arrange
	var stringTarget string
	cmd := &cobra.Command{}

	// Act
	RegisterFlag(cmd, "config", "c", "default.conf", "Path to config file", &stringTarget)

	// Assert
	flag := cmd.Flags().Lookup("config")
	require.NotNil(t, flag)
	assert.Equal(t, "config", flag.Name)
	assert.Equal(t, "c", flag.Shorthand)
	assert.Equal(t, "Path to config file\n", flag.Usage)
	assert.Equal(t, "default.conf", flag.DefValue)
}

func TestRegisterFlag_Float64Flag(t *testing.T) {
	// Arrange
	var floatTarget float64
	cmd := &cobra.Command{}

	// Act
	RegisterFlag(cmd, "ratio", "r", 1.5, "Compression ratio", &floatTarget)

	// Assert
	flag := cmd.Flags().Lookup("ratio")
	require.NotNil(t, flag)
	assert.Equal(t, "ratio", flag.Name)
	assert.Equal(t, "r", flag.Shorthand)
	assert.Equal(t, "Compression ratio\n", flag.Usage)
}

func TestRegisterFlag_IntFlag(t *testing.T) {
	// Arrange
	var intTarget int
	cmd := &cobra.Command{}

	// Act
	RegisterFlag(cmd, "threads", "t", 4, "Number of threads", &intTarget)

	// Assert
	flag := cmd.Flags().Lookup("threads")
	require.NotNil(t, flag)
	assert.Equal(t, "threads", flag.Name)
	assert.Equal(t, "t", flag.Shorthand)
	assert.Equal(t, "Number of threads\n", flag.Usage)
	assert.Equal(t, "4", flag.DefValue)
}

func TestRegisterFlag_StringSliceFlag(t *testing.T) {
	// Arrange
	var sliceTarget []string
	cmd := &cobra.Command{}

	// Act
	RegisterFlag(cmd, "include", "i", []string{"file1", "file2"}, "Files to include", &sliceTarget)

	// Assert
	flag := cmd.Flags().Lookup("include")
	require.NotNil(t, flag)
	assert.Equal(t, "include", flag.Name)
	assert.Equal(t, "i", flag.Shorthand)
	assert.Equal(t, "Files to include\n", flag.Usage)
	assert.Equal(t, "[file1,file2]", flag.DefValue)
}

func TestRegisterFlag_PanicOnNonPointerTarget(t *testing.T) {
	// Arrange
	var stringTarget string
	cmd := &cobra.Command{}

	// Act & Assert
	assert.PanicsWithValue(t, "target must be a pointer", func() {
		RegisterFlag(cmd, "config", "c", "default.conf", "Path to config file", stringTarget)
	})
}

func TestRegisterFlag_PanicOnUnsupportedType(t *testing.T) {
	// Arrange
	var unsupportedTarget map[string]string
	cmd := &cobra.Command{}

	// Act & Assert
	assert.PanicsWithValue(t, "unsupported flag type", func() {
		RegisterFlag(cmd, "config", "c", map[string]string{}, "Unsupported type", &unsupportedTarget)
	})
}

func TestRegisterFlag_BoolFlagDefaultTrue(t *testing.T) {
	// Arrange
	var boolTarget bool
	cmd := &cobra.Command{}

	// Act
	RegisterFlag(cmd, "enabled", "e", true, "Enable feature", &boolTarget)

	// Assert
	flag := cmd.Flags().Lookup("enabled")
	require.NotNil(t, flag)
	assert.Equal(t, "enabled", flag.Name)
	assert.Equal(t, "e", flag.Shorthand)
	// When default is true, it just adds newline without "(default false)"
	assert.Equal(t, "Enable feature\n", flag.Usage)
	assert.Equal(t, "true", flag.DefValue)
}

func TestRegisterFlag_PanicOnUnsupportedSliceType(t *testing.T) {
	// Arrange
	var unsupportedSlice []int
	cmd := &cobra.Command{}

	// Act & Assert
	assert.PanicsWithValue(t, "unsupported slice type", func() {
		RegisterFlag(cmd, "numbers", "n", []string{"1", "2"}, "Numbers", &unsupportedSlice)
	})
}

func TestRegisterFlag_PanicOnUnsupportedTargetType(t *testing.T) {
	// Arrange - target type doesn't match value type
	var floatTarget float32 // Using float32 instead of float64
	cmd := &cobra.Command{}

	// Act & Assert
	assert.PanicsWithValue(t, "unsupported flag type", func() {
		RegisterFlag(cmd, "ratio", "r", 1.5, "Ratio", &floatTarget)
	})
}
