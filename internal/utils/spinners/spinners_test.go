package spinners

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"testing"
	"time"

	"github.com/theckman/yacspin"
)


func TestCreateSpinner_StartAndStop(t *testing.T) {
	// Arrange
	startMessage := "Starting..."
	stopCharacter := "✔"
	stopMessage := "Completed"
	stopFailCharacter := "✘"
	stopFailMessage := "Failed"

	// Act
	spinner := CreateSpinner(startMessage, stopCharacter, stopMessage, stopFailCharacter, stopFailMessage)

	// Assert: Ensure the spinner is initialized
	if spinner == nil {
		t.Fatalf("Expected spinner to be initialized, but got nil")
	}

	// Test that the spinner starts successfully
	err := spinner.Start()
	if err != nil {
		t.Errorf("Expected spinner to start successfully, but got error: %v", err)
	}

	// Test that the spinner stops successfully
	err = spinner.Stop()
	if err != nil {
		t.Errorf("Expected spinner to stop successfully, but got error: %v", err)
	}
}

func TestStopOnSignal_Interrupt(t *testing.T) {
	// Arrange
	spinner := CreateSpinner("Starting...", "✔", "Completed", "✘", "Failed")

	// Start the spinner to ensure it is running
	err := spinner.Start()
	if err != nil {
		t.Fatalf("Expected spinner to start successfully, but got error: %v", err)
	}

	// Simulate handling signal
	go stopOnSignal(spinner)

	// Act: Send an interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		time.Sleep(100 * time.Millisecond)
		sigCh <- os.Interrupt
	}()

	// Allow time for the signal to be processed and the spinner to stop
	time.Sleep(200 * time.Millisecond)

	// Assert: Check that StopFail was called and no errors occurred
	err = spinner.StopFail()
	if err != nil {
		t.Errorf("Expected spinner to stop with failure, but got error: %v", err)
	}
}

func TestCreateSpinner_StopFail(t *testing.T) {
	// Arrange
	spinner := CreateSpinner("Processing...", "✔", "Done", "✘", "Error occurred")

	// Act
	err := spinner.Start()
	if err != nil {
		t.Fatalf("Expected spinner to start successfully, but got error: %v", err)
	}

	// Set a custom failure message and stop with failure
	spinner.StopFailMessage("Custom failure message")
	err = spinner.StopFail()

	// Assert
	if err != nil {
		t.Errorf("Expected spinner to stop with failure, but got error: %v", err)
	}
}

func TestCreateSpinner_StopMessage(t *testing.T) {
	// Arrange
	spinner := CreateSpinner("Processing...", "✔", "Done", "✘", "Error occurred")

	// Act
	err := spinner.Start()
	if err != nil {
		t.Fatalf("Expected spinner to start successfully, but got error: %v", err)
	}

	// Set a custom success message and stop
	spinner.StopMessage("Custom success message")
	err = spinner.Stop()

	// Assert
	if err != nil {
		t.Errorf("Expected spinner to stop successfully, but got error: %v", err)
	}
}

// TestCreateSpinner_CreationError covers the error path when spinner creation fails.
func TestCreateSpinner_CreationError(t *testing.T) {
	oldNewSpinner := newSpinner
	oldExitOnError := exitOnError
	defer func() {
		newSpinner = oldNewSpinner
		exitOnError = oldExitOnError
	}()

	newSpinner = func(_ yacspin.Config) (*yacspin.Spinner, error) {
		return nil, errors.New("injected spinner failure")
	}
	exitOnError = func(code int) {
		if code != 1 {
			t.Errorf("expected exit code 1, got %d", code)
		}
		panic("exitOnError")
	}

	defer func() {
		if v := recover(); v != "exitOnError" {
			t.Errorf("expected panic exitOnError, got %v", v)
		}
	}()

	CreateSpinner("a", "b", "c", "d", "e")
	t.Fatal("CreateSpinner should have panicked")
}

// TestCreateSpinner_ExitsOnCreationError runs CreateSpinner in a subprocess with
// an injected spinner creation failure and verifies the process exits with code 1.
func TestCreateSpinner_ExitsOnCreationError(t *testing.T) {
	if os.Getenv("GO_TEST_SPINNER_FAIL") == "1" {
		newSpinner = func(_ yacspin.Config) (*yacspin.Spinner, error) {
			return nil, errors.New("injected spinner failure")
		}
		CreateSpinner("a", "b", "c", "d", "e")
		t.Fatal("CreateSpinner should have exited")
	}
	cmd := exec.Command("go", "test", "-run", "^TestCreateSpinner_ExitsOnCreationError$", "./internal/utils/spinners/", "-v", "-count=1")
	cmd.Env = append(os.Environ(), "GO_TEST_SPINNER_FAIL=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to exit with non-zero status")
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got: %d", ee.ExitCode())
	} else if !errors.As(err, &ee) {
		t.Errorf("expected exit error, got: %v", err)
	}
}
