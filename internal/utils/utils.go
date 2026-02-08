// Package utils provides concurrency and filesystem helpers.
package utils

import (
	"os"
	"sync"
)

// ConcurrentFetch runs multiple tasks concurrently, returning the first error
// encountered. If all tasks succeed, it returns nil.
func ConcurrentFetch(tasks ...func() error) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))

	for _, task := range tasks {
		wg.Add(1)
		go func(t func() error) {
			defer wg.Done()
			if err := t(); err != nil {
				errChan <- err
			}
		}(task)
	}

	wg.Wait()
	close(errChan)

	// Return the first error encountered, if any
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// EnsureDirExists checks if a directory exists at the given path and creates it
// if it does not. Returns an error if the directory cannot be created or accessed.
func EnsureDirExists(path string) error {
	// Check if the directory exists
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		// Create the directory if it doesn't exist
		err := os.MkdirAll(path, os.ModePerm) // os.ModePerm ensures the directory is created with the correct permissions
		if err != nil {
			return err
		}

	} else if err != nil {
		return err
	}
	return nil
}
