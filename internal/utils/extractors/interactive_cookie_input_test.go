package extractors

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractiveCookieInputWithIO_Success(t *testing.T) {
	in := strings.NewReader("value1\nvalue2\n")
	var out bytes.Buffer
	cookies, err := interactiveCookieInputWithIO(in, &out, []string{"cookie1", "cookie2"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"cookie1": "value1", "cookie2": "value2"}, cookies)
	assert.Contains(t, out.String(), "Manual Cookie Entry")
}

func TestInteractiveCookieInputWithIO_EmptyValueError(t *testing.T) {
	in := strings.NewReader("\n")
	var out bytes.Buffer
	_, err := interactiveCookieInputWithIO(in, &out, []string{"cookie1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestInteractiveCookieInputWithIO_ReadError(t *testing.T) {
	var out bytes.Buffer
	_, err := interactiveCookieInputWithIO(&errReader{}, &out, []string{"cookie1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read input")
}

func TestPromptForCookieSelectionWithIO_NoStores(t *testing.T) {
	var out bytes.Buffer
	idx, autoSelect, err := promptForCookieSelectionWithIO(strings.NewReader(""), &out, nil)
	assert.Error(t, err)
	assert.Equal(t, -1, idx)
	assert.False(t, autoSelect)
	assert.Contains(t, err.Error(), "no cookie stores available")
}

func TestPromptForCookieSelectionWithIO_SingleStore(t *testing.T) {
	var out bytes.Buffer
	idx, autoSelect, err := promptForCookieSelectionWithIO(strings.NewReader(""), &out, []string{"Chrome"})
	require.NoError(t, err)
	assert.Equal(t, 0, idx)
	assert.False(t, autoSelect)
}

func TestPromptForCookieSelectionWithIO_ValidChoice(t *testing.T) {
	in := strings.NewReader("2\n")
	var out bytes.Buffer
	idx, autoSelect, err := promptForCookieSelectionWithIO(in, &out, []string{"Chrome", "Firefox"})
	require.NoError(t, err)
	assert.Equal(t, 1, idx)
	assert.False(t, autoSelect)
}

func TestPromptForCookieSelectionWithIO_AutoSelect(t *testing.T) {
	in := strings.NewReader("\n")
	var out bytes.Buffer
	idx, autoSelect, err := promptForCookieSelectionWithIO(in, &out, []string{"Chrome", "Firefox"})
	require.NoError(t, err)
	assert.True(t, autoSelect)
	assert.Equal(t, 0, idx) // index is 0 when autoSelect is true
}

func TestPromptForCookieSelectionWithIO_InvalidSelection(t *testing.T) {
	in := strings.NewReader("x\n")
	var out bytes.Buffer
	idx, autoSelect, err := promptForCookieSelectionWithIO(in, &out, []string{"Chrome", "Firefox"})
	assert.Error(t, err)
	assert.Equal(t, -1, idx)
	assert.False(t, autoSelect)
	assert.Contains(t, err.Error(), "invalid selection")
}

func TestConfirmActionWithIO_Yes(t *testing.T) {
	assert.True(t, confirmActionWithIO(strings.NewReader("y\n"), &bytes.Buffer{}, "Continue?"))
	assert.True(t, confirmActionWithIO(strings.NewReader("yes\n"), &bytes.Buffer{}, "Continue?"))
}

func TestConfirmActionWithIO_No(t *testing.T) {
	assert.False(t, confirmActionWithIO(strings.NewReader("n\n"), &bytes.Buffer{}, "Continue?"))
	assert.False(t, confirmActionWithIO(strings.NewReader("\n"), &bytes.Buffer{}, "Continue?"))
	assert.False(t, confirmActionWithIO(strings.NewReader("other\n"), &bytes.Buffer{}, "Continue?"))
}

// errReader returns an error on Read to cover the confirmActionWithIO read-error path
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, assert.AnError
}

func TestConfirmActionWithIO_ReadError(t *testing.T) {
	assert.False(t, confirmActionWithIO(&errReader{}, &bytes.Buffer{}, "Continue?"))
}

func TestPromptForCookieSelectionWithIO_ReadError(t *testing.T) {
	var out bytes.Buffer
	idx, autoSelect, err := promptForCookieSelectionWithIO(&errReader{}, &out, []string{"Chrome", "Firefox"})
	assert.Error(t, err)
	assert.Equal(t, -1, idx)
	assert.False(t, autoSelect)
	assert.Contains(t, err.Error(), "failed to read input")
}

func TestSelectExtractionMethodWithIO_Auto(t *testing.T) {
	method, err := selectExtractionMethodWithIO(strings.NewReader("1\n"), &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, "auto", method)

	method, err = selectExtractionMethodWithIO(strings.NewReader("\n"), &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, "auto", method)
}

func TestSelectExtractionMethodWithIO_Manual(t *testing.T) {
	method, err := selectExtractionMethodWithIO(strings.NewReader("2\n"), &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, "manual", method)
}

func TestSelectExtractionMethodWithIO_Invalid(t *testing.T) {
	_, err := selectExtractionMethodWithIO(strings.NewReader("3\n"), &bytes.Buffer{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid selection")
}

func TestSelectExtractionMethodWithIO_ReadError(t *testing.T) {
	_, err := selectExtractionMethodWithIO(&errReader{}, &bytes.Buffer{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read input")
}

// Tests for public wrappers that use os.Stdin/Stdout (cover the 0% wrapper functions)
func TestInteractiveCookieInput_WithPipedStdin(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()
	go func() {
		_, _ = w.WriteString("v1\nv2\n")
		_ = w.Close()
	}()
	cookies, err := InteractiveCookieInput([]string{"c1", "c2"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"c1": "v1", "c2": "v2"}, cookies)
}

func TestPromptForCookieSelection_WithPipedStdin(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()
	go func() {
		_, _ = w.WriteString("2\n")
		_ = w.Close()
	}()
	idx, autoSelect, err := PromptForCookieSelection([]string{"Chrome", "Firefox"})
	require.NoError(t, err)
	assert.Equal(t, 1, idx)
	assert.False(t, autoSelect)
}

func TestConfirmAction_WithPipedStdin(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()
	go func() {
		_, _ = w.WriteString("y\n")
		_ = w.Close()
	}()
	assert.True(t, ConfirmAction("Continue?"))
}

func TestSelectExtractionMethod_WithPipedStdin(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()
	go func() {
		_, _ = w.WriteString("2\n")
		_ = w.Close()
	}()
	method, err := SelectExtractionMethod()
	require.NoError(t, err)
	assert.Equal(t, "manual", method)
}
