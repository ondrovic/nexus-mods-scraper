package extractors

import (
	"bytes"
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

func TestPromptForCookieSelectionWithIO_NoStores(t *testing.T) {
	var out bytes.Buffer
	idx, err := promptForCookieSelectionWithIO(strings.NewReader(""), &out, nil)
	assert.Error(t, err)
	assert.Equal(t, -1, idx)
	assert.Contains(t, err.Error(), "no cookie stores available")
}

func TestPromptForCookieSelectionWithIO_SingleStore(t *testing.T) {
	var out bytes.Buffer
	idx, err := promptForCookieSelectionWithIO(strings.NewReader(""), &out, []string{"Chrome"})
	require.NoError(t, err)
	assert.Equal(t, 0, idx)
}

func TestPromptForCookieSelectionWithIO_ValidChoice(t *testing.T) {
	in := strings.NewReader("2\n")
	var out bytes.Buffer
	idx, err := promptForCookieSelectionWithIO(in, &out, []string{"Chrome", "Firefox"})
	require.NoError(t, err)
	assert.Equal(t, 1, idx)
}

func TestPromptForCookieSelectionWithIO_AutoSelect(t *testing.T) {
	in := strings.NewReader("\n")
	var out bytes.Buffer
	idx, err := promptForCookieSelectionWithIO(in, &out, []string{"Chrome", "Firefox"})
	require.NoError(t, err)
	assert.Equal(t, -1, idx)
}

func TestPromptForCookieSelectionWithIO_InvalidSelection(t *testing.T) {
	in := strings.NewReader("x\n")
	var out bytes.Buffer
	idx, err := promptForCookieSelectionWithIO(in, &out, []string{"Chrome", "Firefox"})
	assert.Error(t, err)
	assert.Equal(t, -1, idx)
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
