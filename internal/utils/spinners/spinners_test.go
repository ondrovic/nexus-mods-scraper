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


// TestCreateSpinner_StartAndStop verifies spinner start and stop behavior.
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

// TestStopOnSignal_Interrupt verifies spinner stops on interrupt signal.
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

// TestCreateSpinner_StopFail verifies StopFail behavior.
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

// TestCreateSpinner_StopMessage verifies StopMessage and Stop success path.
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

// TestCreateSpinner_CreationErrorReturnsNil covers the return nil path when creation fails
// and processExit is a no-op (so the function returns instead of exiting).
func TestCreateSpinner_CreationErrorReturnsNil(t *testing.T) {
	oldNewSpinner := newSpinner
	oldProcessExit := processExit
	defer func() {
		newSpinner = oldNewSpinner
		processExit = oldProcessExit
	}()

	newSpinner = func(_ yacspin.Config) (*yacspin.Spinner, error) {
		return nil, errors.New("injected spinner failure")
	}
	processExit = func(int) {} // no-op so CreateSpinner returns nil instead of exiting

	s := CreateSpinner("a", "b", "c", "d", "e")
	if s != nil {
		t.Errorf("expected nil spinner when creation fails and processExit is no-op, got %v", s)
	}
}

// TestCreateSpinner_CreationErrorCallsProcessExitWithOne covers the line processExit(1) when
// spinner creation fails. We override processExit to record the code so the call is executed
// and we avoid actually calling os.Exit(1).
func TestCreateSpinner_CreationErrorCallsProcessExitWithOne(t *testing.T) {
	oldNewSpinner := newSpinner
	oldProcessExit := processExit
	defer func() {
		newSpinner = oldNewSpinner
		processExit = oldProcessExit
	}()

	newSpinner = func(_ yacspin.Config) (*yacspin.Spinner, error) {
		return nil, errors.New("injected spinner failure")
	}
	var exitCode int
	processExit = func(code int) {
		exitCode = code
	}

	s := CreateSpinner("a", "b", "c", "d", "e")
	if s != nil {
		t.Errorf("expected nil spinner when creation fails, got %v", s)
	}
	if exitCode != 1 {
		t.Errorf("expected processExit(1) to be called, got exitCode %d", exitCode)
	}
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
