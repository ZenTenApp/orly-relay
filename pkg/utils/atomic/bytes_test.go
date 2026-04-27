// Copyright (c) 2020-2025 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package atomic

import (
	"encoding/json"
	"git.smesh.lol/orly/pkg/utils"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBytesNoInitialValue(t *testing.T) {
	atom := NewBytes([]byte{})
	require.Equal(t, []byte{}, atom.Load(), "Initial value should be empty")
}

func TestBytes(t *testing.T) {
	atom := NewBytes([]byte{})
	require.Equal(
		t, []byte{}, atom.Load(),
		"Expected Load to return initialized empty value",
	)

	emptyBytes := []byte{}
	atom = NewBytes(emptyBytes)
	require.Equal(
		t, emptyBytes, atom.Load(),
		"Expected Load to return initialized empty value",
	)

	testBytes := []byte("test data")
	atom = NewBytes(testBytes)
	loadedBytes := atom.Load()
	require.Equal(
		t, testBytes, loadedBytes, "Expected Load to return initialized value",
	)

	// Verify that the returned value is a copy
	loadedBytes[0] = 'X'
	require.NotEqual(
		t, loadedBytes, atom.Load(), "Load should return a copy of the data",
	)

	// Store and verify
	newBytes := []byte("new data")
	atom.Store(newBytes)
	require.Equal(t, newBytes, atom.Load(), "Unexpected value after Store")

	// Modify original data and verify it doesn't affect stored value
	newBytes[0] = 'X'
	require.NotEqual(t, newBytes, atom.Load(), "Store should copy the data")

	t.Run(
		"JSON/Marshal", func(t *testing.T) {
			jsonBytes := []byte("json data")
			atom.Store(jsonBytes)
			bytes, err := json.Marshal(atom)
			require.NoError(t, err, "json.Marshal errored unexpectedly.")
			require.Equal(
				t, []byte(`"anNvbiBkYXRh"`), bytes,
				"json.Marshal should encode as base64",
			)
		},
	)

	t.Run(
		"JSON/Unmarshal", func(t *testing.T) {
			err := json.Unmarshal(
				[]byte(`"dGVzdCBkYXRh"`), &atom,
			) // "test data" in base64
			require.NoError(t, err, "json.Unmarshal errored unexpectedly.")
			require.Equal(
				t, []byte("test data"), atom.Load(),
				"json.Unmarshal didn't set the correct value.",
			)
		},
	)

	t.Run(
		"JSON/Unmarshal/Error", func(t *testing.T) {
			err := json.Unmarshal([]byte("42"), &atom)
			require.Error(t, err, "json.Unmarshal didn't error as expected.")
		},
	)
}

func TestBytesConcurrentAccess(t *testing.T) {
	const (
		parallelism = 4
		iterations  = 1000
	)

	atom := NewBytes([]byte("initial"))

	var wg sync.WaitGroup
	wg.Add(parallelism)

	// Start multiple goroutines that read and write concurrently
	for i := 0; i < parallelism; i++ {
		go func(id int) {
			defer wg.Done()

			// Each goroutine writes a different value
			myData := []byte{byte(id)}

			for j := 0; j < iterations; j++ {
				// Store our data
				atom.Store(myData)

				// Load the data (which might be from another goroutine)
				loaded := atom.Load()

				// Verify the loaded data is valid (either our data or another goroutine's data)
				require.LessOrEqual(
					t, len(loaded), parallelism,
					"Loaded data length should not exceed parallelism",
				)

				// If it's our data, verify it's correct
				if len(loaded) == 1 && loaded[0] == byte(id) {
					require.Equal(t, myData, loaded, "Data corruption detected")
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestBytesDataIntegrity(t *testing.T) {
	// Test that large byte slices maintain integrity under concurrent access
	const (
		parallelism = 4
		dataSize    = 1024 // 1KB
		iterations  = 100
	)

	// Create test data sets, each with a unique pattern
	testData := make([][]byte, parallelism)
	for i := 0; i < parallelism; i++ {
		testData[i] = make([]byte, dataSize)
		for j := 0; j < dataSize; j++ {
			testData[i][j] = byte((i + j) % 256)
		}
	}

	atom := NewBytes(nil)
	var wg sync.WaitGroup
	wg.Add(parallelism)

	for i := 0; i < parallelism; i++ {
		go func(id int) {
			defer wg.Done()
			myData := testData[id]

			for j := 0; j < iterations; j++ {
				atom.Store(myData)
				loaded := atom.Load()

				// Verify the loaded data is one of our test data sets
				for k := 0; k < parallelism; k++ {
					if utils.FastEqual(loaded, testData[k]) {
						// Found a match, data is intact
						break
					}
					if k == parallelism-1 {
						// No match found, data corruption
						t.Errorf("Data corruption detected: loaded data doesn't match any test set")
					}
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestBytesStress(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))

	atom := NewBytes([]byte("initial"))
	var wg sync.WaitGroup

	// We'll run 8 goroutines concurrently
	workers := 8
	iterations := 1000
	wg.Add(workers)

	start := make(chan struct{})

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()

			// Wait for the start signal
			<-start

			// Each worker gets its own data
			myData := []byte{byte(id)}

			for j := 0; j < iterations; j++ {
				// Alternate between reads and writes
				if j%2 == 0 {
					atom.Store(myData)
				} else {
					_ = atom.Load()
				}
			}
		}(i)
	}

	// Start all goroutines simultaneously
	close(start)
	wg.Wait()
}

func BenchmarkBytesParallel(b *testing.B) {
	atom := NewBytes([]byte("benchmark"))

	b.RunParallel(
		func(pb *testing.PB) {
			// Each goroutine gets its own data to prevent false sharing
			myData := []byte("goroutine data")

			for pb.Next() {
				atom.Store(myData)
				_ = atom.Load()
			}
		},
	)
}
