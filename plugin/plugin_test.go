package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

// LogEntry captures a single log entry.
type LogEntry struct {
	Level   logrus.Level
	Message string
	Fields  logrus.Fields
}

// MockLogHook is a hook to capture log entries.
type MockLogHook struct {
	Entries []LogEntry
}

// Fire is called for each log entry.
func (hook *MockLogHook) Fire(entry *logrus.Entry) error {
	hook.Entries = append(hook.Entries, LogEntry{
		Level:   entry.Level,
		Message: entry.Message,
		Fields:  entry.Data,
	})
	return nil
}

// Levels returns the log levels supported by the hook.
func (hook *MockLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// NewMockLogHook creates a new instance of MockLogHook.
func NewMockLogHook() *MockLogHook {
	return &MockLogHook{}
}

func TestLocateFiles(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected []string
		err      string
	}{
		{
			name:     "ValidPatternWithFiles",
			pattern:  "../testdata/*.xml",
			expected: []string{filepath.FromSlash("../testdata/invalid-suite.xml"), filepath.FromSlash("../testdata/invalid.xml"), filepath.FromSlash("../testdata/testng-report.xml"), filepath.FromSlash("../testdata/testng-report-valid.xml")},
			err:      "",
		},
		{
			name:     "NoFilesMatchPattern",
			pattern:  "../testdata/*.log",
			expected: nil,
			err:      "no files found matching the report filename pattern",
		},
		{
			name:     "InvalidPattern",
			pattern:  "[invalidpattern",
			expected: nil,
			err:      "failed to search for files",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := locateFiles(tc.pattern)

			// Sort results for consistency
			sort.Strings(result)
			sort.Strings(tc.expected)

			// Compare result with expected output
			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Errorf("locateFiles() mismatch (-want +got):\n%s", diff)
			}

			// Check error
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Errorf("locateFiles() expected error %v, got %v", tc.err, err)
				}
			} else if err != nil {
				t.Errorf("locateFiles() unexpected error: %v", err)
			}
		})
	}
}

// TestProcessFile tests the processFile function with various cases
func TestProcessFile(t *testing.T) {
	tests := []struct {
		name      string
		filePath  string
		expected  Results
		expectErr bool
		errMsg    string
	}{
		{
			name:     "ValidTestNGReport",
			filePath: "../testdata/testng-report.xml",
			expected: Results{
				Total:      3,
				Failures:   1,
				Skipped:    0,
				DurationMS: 15.0,
			},
			expectErr: false,
		},
		{
			name:      "NonExistentFile",
			filePath:  "../testdata/nonexistent.xml",
			expected:  Results{},
			expectErr: true,
			errMsg:    "file not found",
		},
		{
			name:      "InvalidXMLFile",
			filePath:  "../testdata/invalid.xml",
			expected:  Results{},
			expectErr: true,
			errMsg:    "failed to parse TestNG XML",
		},
		{
			name:      "IncorrectXMLStructure",
			filePath:  "../testdata/invalid-suite.xml",
			expected:  Results{},
			expectErr: true,
			errMsg:    "no test suites found in the XML structure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := processFile(tc.filePath)

			// Compare results
			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Errorf("processFile() mismatch (-want +got):\n%s", diff)
			}

			// Check error
			if tc.expectErr {
				if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("processFile() expected error %q but got %v", tc.errMsg, err)
				}
			} else if err != nil {
				t.Errorf("processFile() unexpected error: %v", err)
			}
		})
	}
}

// TestValidateInputs tests the ValidateInputs function with various cases
func TestValidateInputs(t *testing.T) {
	tests := []struct {
		name      string
		args      Args
		expectErr bool
		errMsg    string
	}{
		{
			name: "ValidInputs",
			args: Args{
				ReportFilenamePattern: "testdata/*.xml",
				FailedFails:           1,
				FailedSkips:           0,
				ThresholdMode:         "absolute",
			},
			expectErr: false,
		},
		{
			name: "MissingReportFilenamePattern",
			args: Args{
				FailedFails:   1,
				FailedSkips:   0,
				ThresholdMode: "absolute",
			},
			expectErr: true,
			errMsg:    "missing required parameter",
		},
		{
			name: "InvalidThresholdMode",
			args: Args{
				ReportFilenamePattern: "testdata/*.xml",
				ThresholdMode:         "invalid",
			},
			expectErr: true,
			errMsg:    "invalid ThresholdMode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateInputs(tc.args)

			// Check error
			if tc.expectErr {
				if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("ValidateInputs() expected error %q but got %v", tc.errMsg, err)
				}
			} else if err != nil {
				t.Errorf("ValidateInputs() unexpected error: %v", err)
			}
		})
	}
}

// TestValidateThresholds tests the validateThresholds function for various scenarios
func TestValidateThresholds(t *testing.T) {
	tests := []struct {
		name      string
		results   Results
		args      Args
		expectErr bool
		errMsg    string
	}{
		{
			name: "ValidAbsoluteThresholds",
			results: Results{
				Total:    10,
				Failures: 1,
				Skipped:  1,
			},
			args: Args{
				FailedFails:   2,
				FailedSkips:   2,
				ThresholdMode: "absolute",
			},
			expectErr: false,
		},
		{
			name: "ExceededAbsoluteFailureThreshold",
			results: Results{
				Total:    10,
				Failures: 3,
				Skipped:  1,
			},
			args: Args{
				FailedFails:   2,
				FailedSkips:   2,
				ThresholdMode: "absolute",
			},
			expectErr: true,
			errMsg:    "\nabsolute threshold validation failed: number of failed tests (3) exceeded the threshold (2)",
		},
		{
			name: "ExceededPercentageFailureThreshold",
			results: Results{
				Total:    100,
				Failures: 15,
				Skipped:  5,
			},
			args: Args{
				FailedFails:   10,
				FailedSkips:   10,
				ThresholdMode: "percentage",
			},
			expectErr: true,
			errMsg:    "\npercentage threshold validation failed: failure rate (15.00%) exceeded the threshold (10.00%)",
		},
		{
			name: "ValidPercentageThresholds",
			results: Results{
				Total:    100,
				Failures: 5,
				Skipped:  5,
			},
			args: Args{
				FailedFails:   10,
				FailedSkips:   10,
				ThresholdMode: "percentage",
			},
			expectErr: false,
		},
		{
			name: "EdgeCaseVeryHighValues",
			results: Results{
				Total:    1000000,
				Failures: 500000,
				Skipped:  400000,
			},
			args: Args{
				FailedFails:   600000,
				FailedSkips:   500000,
				ThresholdMode: "absolute",
			},
			expectErr: false,
		},
		{
			name: "EdgeCaseEmptyResults",
			results: Results{
				Total:    0,
				Failures: 0,
				Skipped:  0,
			},
			args: Args{
				FailedFails:   0,
				FailedSkips:   0,
				ThresholdMode: "absolute",
			},
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateThresholds(tc.results, tc.args)

			// Check error
			if tc.expectErr {
				if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("validateThresholds() expected error %q but got %v", tc.errMsg, err)
				}
			} else if err != nil {
				t.Errorf("validateThresholds() unexpected error: %v", err)
			}
		})
	}
}

func TestExecWithMixedFiles(t *testing.T) {
	args := Args{
		ReportFilenamePattern: "../testdata/*.xml",
		FailedFails:           4,
		FailedSkips:           1,
		ThresholdMode:         ThresholdModeAbsolute,
	}

	err := Exec(context.Background(), args)

	// Check if the plugin processes valid files and skips invalid ones
	if err == nil {
		t.Logf("Exec successfully processed mixed valid and invalid files.")
	} else {
		t.Errorf("Exec failed unexpectedly with error: %v", err)
	}
}

func TestProcessFileWithLargeFile(t *testing.T) {
	// Simulate a large XML file by creating a temporary file
	const numTestMethods = 10000
	largeXML := `
		<testng-results>
			<suite name="LargeSuite">
				<test name="LargeTest">
					<class name="com.example.Test">
	`

	// Append many test methods to simulate a large file
	for i := 0; i < numTestMethods; i++ {
		largeXML += fmt.Sprintf(`
					<test-method status="PASS" name="test-%d" duration-ms="10" />
		`, i)
	}

	largeXML += `
					</class>
				</test>
			</suite>
		</testng-results>
	`

	tmpFile, err := os.CreateTemp("", "large_testng_*.xml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(largeXML)
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Process the large file
	results, err := processFile(tmpFile.Name())
	if err != nil {
		t.Errorf("processFile() failed for large file: %v", err)
	} else {
		t.Logf("processFile() succeeded: %+v", results)
	}
}

func TestLogSuiteGroupsWithMockLogger(t *testing.T) {
	// Setup mock log hook
	hook := NewMockLogHook()
	logrus.AddHook(hook)

	// Input suite to log
	suite := Suite{
		Name: "TestSuite",
		Groups: []Group{
			{
				Name: "Group1",
				Methods: []Method{
					{Name: "Method1", ClassName: "Class1", Signature: "Signature1"},
					{Name: "Method2", ClassName: "Class2", Signature: "Signature2"},
				},
			},
		},
	}

	// Call the function that generates logs
	logSuiteGroups(suite)

	// Validate logs
	expectedEntries := []LogEntry{
		{Message: "\nGroups:"},
		{Message: "\n- Group: Group1"},
		{Message: "\n  - Method: Method1 | Class: Class1 | Signature: Signature1"},
		{Message: "\n  - Method: Method2 | Class: Class2 | Signature: Signature2"},
	}

	// Compare log messages
	for i, expected := range expectedEntries {
		if i >= len(hook.Entries) {
			t.Fatalf("Missing expected log entry: %+v", expected)
		}
		actual := hook.Entries[i]
		if actual.Message != expected.Message {
			t.Errorf("Log message mismatch at entry %d: expected %q, got %q", i, expected.Message, actual.Message)
		}
	}
}

func TestLogSuiteTestDetailsWithMockLogger(t *testing.T) {
	// Setup mock log hook
	hook := NewMockLogHook()
	logrus.AddHook(hook)

	// Input suite to log
	suite := Suite{
		Name: "TestSuite",
		Classes: []Class{
			{
				Name: "Class1",
				Tests: []Test{
					{Name: "Test1", Status: "PASS", DurationMS: "10"},
					{Name: "Test2", Status: "FAIL", DurationMS: "20", Exception: "SomeException"},
				},
			},
			{
				Name: "Class2",
				Tests: []Test{
					{Name: "Test3", Status: "SKIP", DurationMS: "15"},
					{Name: "Test4", Status: "PASS", DurationMS: "invalid"},
				},
			},
		},
	}

	// Call the function that generates logs
	logSuiteTestDetails(suite)

	// Validate logs
	expectedEntries := []LogEntry{
		{Message: "\nTest Details:"},
		{Message: "\n- Test: Test1 | Status: PASS | Duration: 10 ms"},
		{Message: "\n- Test: Test2 | Status: FAIL | Duration: 20 ms"},
		{Message: "\n    Exception: SomeException"},
		{Message: "\n- Test: Test3 | Status: SKIP | Duration: 15 ms"},
		{Message: "\n- Test: Test4 | Status: PASS | Duration: invalid ms"},
	}

	// Compare log messages
	for i, expected := range expectedEntries {
		if i >= len(hook.Entries) {
			t.Fatalf("Missing expected log entry: %+v", expected)
		}
		actual := hook.Entries[i]
		if actual.Message != expected.Message {
			t.Errorf("Log message mismatch at entry %d: expected %q, got %q", i, expected.Message, actual.Message)
		}
	}
}

func TestLogSuiteSummaryWithMockLogger(t *testing.T) {
	// Setup mock log hook
	hook := NewMockLogHook()
	logrus.AddHook(hook)

	// Input suite name and results to log
	suiteName := "SampleSuite"
	results := Results{
		Total:      10,
		Failures:   2,
		Skipped:    1,
		DurationMS: 100,
	}

	// Call the function that generates logs
	logSuiteSummary(suiteName, results)

	// Validate logs
	expectedEntries := []LogEntry{
		{Message: "\n==============================================="},
		{Message: "\nSuite: SampleSuite"},
		{Message: "\nTotal Tests: 10 | Failures: 2 | Skips: 1 | Duration: 100.00 ms"},
		{Message: "\n==============================================="},
	}

	// Compare log messages
	for i, expected := range expectedEntries {
		if i >= len(hook.Entries) {
			t.Fatalf("Missing expected log entry: %+v", expected)
		}
		actual := hook.Entries[i]
		if actual.Message != expected.Message {
			t.Errorf("Log message mismatch at entry %d: expected %q, got %q", i, expected.Message, actual.Message)
		}
	}
}

// TestAggregateClassResultsWithInvalidDuration tests handling of invalid DurationMS and failed/skipped tests.
func TestAggregateClassResultsWithInvalidDuration(t *testing.T) {
	// Test data: Class with valid and invalid DurationMS
	class := Class{
		Name: "TestClass",
		Tests: []Test{
			{Name: "Test1", Status: "PASS", DurationMS: "10"},
			{Name: "Test2", Status: "FAIL", DurationMS: "invalid"},
			{Name: "Test3", Status: "SKIP", DurationMS: "5"},
		},
	}

	// Setup a logrus test hook to capture logs
	logger, hook := test.NewNullLogger()
	logrus.SetOutput(logger.Writer())
	logrus.SetLevel(logrus.WarnLevel)

	// Call the function to aggregate class results
	results, failedTests, skippedTests := aggregateClassResults(class)

	// Define the expected aggregated results
	expectedResults := Results{
		Total:      3,  // Total tests: 3
		Failures:   1,  // 1 failed test
		Skipped:    1,  // 1 skipped test
		DurationMS: 15, // Duration only includes Test1 (valid DurationMS)
	}

	// Define expected failed and skipped test names
	expectedFailedTests := []string{"Test2"}
	expectedSkippedTests := []string{"Test3"}

	// Compare the actual and expected results
	if diff := cmp.Diff(expectedResults, results); diff != "" {
		t.Errorf("aggregateClassResultsWithNames() Results mismatch (-want +got):\n%s", diff)
	}

	// Compare the failed test names
	if diff := cmp.Diff(expectedFailedTests, failedTests); diff != "" {
		t.Errorf("Failed tests mismatch (-want +got):\n%s", diff)
	}

	// Compare the skipped test names
	if diff := cmp.Diff(expectedSkippedTests, skippedTests); diff != "" {
		t.Errorf("Skipped tests mismatch (-want +got):\n%s", diff)
	}

	// Validate the log for invalid DurationMS
	expectedLogMessage := "Invalid or missing DurationMS for test 'Test2'"
	found := false
	for _, entry := range hook.AllEntries() {
		if strings.Contains(entry.Message, expectedLogMessage) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected log message not found: %s", expectedLogMessage)
	}
}

func TestExecWithMixedValidAndInvalidFiles(t *testing.T) {
	args := Args{
		ReportFilenamePattern: "../testdata/*.xml", // Adjust this path as necessary
		FailedFails:           4,
		FailedSkips:           1,
		ThresholdMode:         ThresholdModeAbsolute,
	}

	// Mock a list of valid and invalid files for processing
	validFiles := []string{
		filepath.FromSlash("../testdata/testng-report.xml"),
		filepath.FromSlash("../testdata/testng-report-valid.xml"),
	}
	invalidFiles := []string{
		filepath.FromSlash("../testdata/invalid.xml"),
		filepath.FromSlash("../testdata/invalid-suite.xml"),
	}

	// Combine valid and invalid files into a test case
	files := append(validFiles, invalidFiles...)

	// Expected number of results and errors
	expectedValidResults := 6 // 3 tests in each valid file (2 files)
	expectedInvalidFiles := 2 // The two invalid files should be skipped
	expectedFailedTests := 3  // Both valid files contain 1 failed test each

	// Create channels for results and errors
	resultsChan := make(chan Results, len(files))
	errorsChan := make(chan error, len(files))

	// Start processing files in parallel
	for _, file := range files {
		go func(f string) {
			res, err := processFile(f)
			if err != nil {
				errorsChan <- fmt.Errorf("failed to process file %s: %w", f, err)
				return
			}
			resultsChan <- res
		}(file)
	}

	var aggregatedResults Results
	var skippedFiles []string

	// Process results and errors
	for i := 0; i < len(files); i++ {
		select {
		case res := <-resultsChan:
			// Only aggregate results from valid files
			if res.Total > 0 {
				aggregatedResults.Total += res.Total
				aggregatedResults.Failures += res.Failures
				aggregatedResults.Skipped += res.Skipped
				aggregatedResults.DurationMS += res.DurationMS
			}
		case err := <-errorsChan:
			logrus.Warn(err)
			skippedFiles = append(skippedFiles, err.Error())
		}
	}

	// Assert that the number of skipped files matches the expected invalid files
	if len(skippedFiles) != expectedInvalidFiles {
		t.Errorf("Expected %d skipped files, got %d", expectedInvalidFiles, len(skippedFiles))
	}

	// Assert that valid files were processed and aggregated results are correct
	if aggregatedResults.Total-aggregatedResults.Failures != expectedValidResults {
		t.Errorf("Expected %d total tests processed, got %d", expectedValidResults, aggregatedResults.Total)
	}

	// Assert that the number of failed tests matches the expected value
	if aggregatedResults.Failures != expectedFailedTests {
		t.Errorf("Expected %d failed tests, got %d", expectedFailedTests, aggregatedResults.Failures)
	}

	// If no error occurred during execution, validate thresholds
	if err := validateThresholds(aggregatedResults, args); err != nil {
		t.Errorf("Threshold validation failed: %v", err)
	} else {
		t.Log("Threshold validation passed successfully.")
	}
}
