package plugin

import (
	"errors"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestLocateFiles tests the locateFiles function with various cases
func TestLocateFiles(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected []string
		err      error
	}{
		{
			name:     "ValidPatternWithFiles",
			pattern:  "../testdata/*.xml",
			expected: []string{filepath.FromSlash("../testdata/testng-report.xml")},
			err:      nil,
		},
		{
			name:     "NoFilesMatchPattern",
			pattern:  "testdata/*.log",
			expected: nil,
			err:      errors.New("no files found matching the report filename pattern"),
		},
		{
			name:     "InvalidPattern",
			pattern:  "[invalidpattern",
			expected: nil,
			err:      errors.New("failed to search for files: syntax error in pattern"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := locateFiles(tc.pattern)

			// Sort results to ensure order consistency
			sort.Strings(result)
			sort.Strings(tc.expected)

			// Compare result with expected output
			if diff := cmp.Diff(result, tc.expected); diff != "" {
				t.Errorf("locateFiles() mismatch (-want +got):\n%s", diff)
			}

			// Compare errors
			if tc.err != nil && err != nil {
				if err.Error() != tc.err.Error() {
					t.Errorf("locateFiles() error mismatch (-want +got): %v", err)
				}
			} else if err != tc.err {
				t.Errorf("locateFiles() expected error: %v, got: %v", tc.err, err)
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
		},
		{
			name:      "InvalidXMLFile",
			filePath:  "testdata/invalid.xml",
			expected:  Results{},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := processFile(tc.filePath)

			// Compare results
			if diff := cmp.Diff(result, tc.expected); diff != "" {
				t.Errorf("processFile() mismatch (-want +got):\n%s", diff)
			}

			// Check error presence
			if tc.expectErr && err == nil {
				t.Errorf("processFile() expected error but got nil")
			} else if !tc.expectErr && err != nil {
				t.Errorf("processFile() did not expect error but got: %v", err)
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
	}{
		{
			name: "ValidInputs",
			args: Args{
				ReportFilenamePattern: "testdata/*.xml",
				FailedFails:           1,
				FailedSkips:           0,
				ThresholdMode:         1,
			},
			expectErr: false,
		},
		{
			name: "MissingReportFilenamePattern",
			args: Args{
				FailedFails:   1,
				FailedSkips:   0,
				ThresholdMode: 1,
			},
			expectErr: true,
		},
		{
			name: "InvalidThresholdMode",
			args: Args{
				ReportFilenamePattern: "testdata/*.xml",
				ThresholdMode:         3,
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateInputs(tc.args)

			// Check error presence
			if tc.expectErr && err == nil {
				t.Errorf("ValidateInputs() expected error but got nil")
			} else if !tc.expectErr && err != nil {
				t.Errorf("ValidateInputs() did not expect error but got: %v", err)
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
				ThresholdMode: 1,
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
				ThresholdMode: 1,
			},
			expectErr: true,
			errMsg:    "\nabsolute threshold validation failed: \nnumber of failed tests (3) exceeded the failure threshold (2)",
		},
		{
			name: "ExceededAbsoluteSkipThreshold",
			results: Results{
				Total:    10,
				Failures: 1,
				Skipped:  3,
			},
			args: Args{
				FailedFails:   2,
				FailedSkips:   2,
				ThresholdMode: 1,
			},
			expectErr: true,
			errMsg:    "\nabsolute threshold validation failed: \nnumber of skipped tests (3) exceeded the skip threshold (2)",
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
				ThresholdMode: 2,
			},
			expectErr: false,
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
				ThresholdMode: 2,
			},
			expectErr: true,
			errMsg:    "\npercentage threshold validation failed: \nfailure rate (15.00%) exceeded the threshold (10.00%)",
		},
		{
			name: "ExceededPercentageSkipThreshold",
			results: Results{
				Total:    100,
				Failures: 5,
				Skipped:  15,
			},
			args: Args{
				FailedFails:   10,
				FailedSkips:   10,
				ThresholdMode: 2,
			},
			expectErr: true,
			errMsg:    "\npercentage threshold validation failed: \nskip rate (15.00%) exceeded the threshold (10.00%)",
		},
		{
			name: "InvalidThresholdMode",
			results: Results{
				Total:    100,
				Failures: 5,
				Skipped:  5,
			},
			args: Args{
				FailedFails:   10,
				FailedSkips:   10,
				ThresholdMode: 3, // Invalid
			},
			expectErr: true,
			errMsg:    "\ninvalid ThresholdMode: 3, expected 1 (absolute) or 2 (percentage)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateThresholds(tc.results, tc.args)

			// Check error presence
			if tc.expectErr && err == nil {
				t.Errorf("validateThresholds() expected error but got nil")
			} else if !tc.expectErr && err != nil {
				t.Errorf("validateThresholds() did not expect error but got: %v", err)
			}

			// Check error message
			if tc.expectErr && err != nil {
				if diff := cmp.Diff(err.Error(), tc.errMsg); diff != "" {
					t.Errorf("validateThresholds() error message mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

// TestFailureOnFailedTestConfig tests the FailureOnFailedTestConfig scenario
func TestFailureOnFailedTestConfig(t *testing.T) {
	tests := []struct {
		name      string
		results   Results
		args      Args
		expectErr bool
		errMsg    string
	}{
		{
			name: "FailureOnFailedTestConfigTrueWithFailures",
			results: Results{
				Total:    10,
				Failures: 2,
				Skipped:  1,
			},
			args: Args{
				FailureOnFailedTestConfig: true,
				ThresholdMode:             1, // Absolute thresholds
			},
			expectErr: true,
			errMsg:    "\nbuild marked as failed due to failed configuration methods as FailureOnFailedTestConfig is true",
		},
		{
			name: "FailureOnFailedTestConfigTrueWithoutFailures",
			results: Results{
				Total:    10,
				Failures: 0,
				Skipped:  1,
			},
			args: Args{
				FailureOnFailedTestConfig: true,
				ThresholdMode:             1, // Absolute thresholds
			},
			expectErr: false,
		},
		{
			name: "FailureOnFailedTestConfigFalseWithFailures",
			results: Results{
				Total:    10,
				Failures: 2,
				Skipped:  1,
			},
			args: Args{
				FailureOnFailedTestConfig: false,
				ThresholdMode:             1, // Absolute thresholds
			},
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateThresholds(tc.results, tc.args)

			// Check error presence
			if tc.expectErr && err == nil {
				t.Errorf("validateThresholds() expected error but got nil")
			} else if !tc.expectErr && err != nil {
				t.Errorf("validateThresholds() did not expect error but got: %v", err)
			}

			// Check error message
			if tc.expectErr && err != nil {
				if diff := cmp.Diff(err.Error(), tc.errMsg); diff != "" {
					t.Errorf("validateThresholds() error message mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestValidateUnstableThresholds(t *testing.T) {
	tests := []struct {
		name      string
		results   Results
		args      Args
		expectErr bool
		errMsg    string
	}{
		// Absolute threshold tests
		{
			name: "ValidUnstableAbsoluteThresholds",
			results: Results{
				Total:    10,
				Failures: 2,
				Skipped:  1,
			},
			args: Args{
				UnstableFails: 5,
				UnstableSkips: 3,
				ThresholdMode: 1, // Absolute mode
			},
			expectErr: false,
		},
		{
			name: "ExceededUnstableAbsoluteFailureThreshold",
			results: Results{
				Total:    10,
				Failures: 6,
				Skipped:  1,
			},
			args: Args{
				UnstableFails: 5,
				UnstableSkips: 3,
				ThresholdMode: 1, // Absolute mode
			},
			expectErr: true,
			errMsg:    "\nBuild marked as fail: number of failed tests (6) exceeded the unstable threshold (5)",
		},
		{
			name: "ExceededUnstableAbsoluteSkipThreshold",
			results: Results{
				Total:    10,
				Failures: 2,
				Skipped:  4,
			},
			args: Args{
				UnstableFails: 5,
				UnstableSkips: 3,
				ThresholdMode: 1, // Absolute mode
			},
			expectErr: true,
			errMsg:    "\nBuild marked as fail: number of skipped tests (4) exceeded the unstable threshold (3)",
		},

		// Percentage threshold tests
		{
			name: "ValidUnstablePercentageThresholds",
			results: Results{
				Total:    100,
				Failures: 10,
				Skipped:  10,
			},
			args: Args{
				UnstableFails: 20, // 20%
				UnstableSkips: 20, // 20%
				ThresholdMode: 2,  // Percentage mode
			},
			expectErr: false,
		},
		{
			name: "ExceededUnstablePercentageFailureThreshold",
			results: Results{
				Total:    100,
				Failures: 25, // 25%
				Skipped:  10, // 10%
			},
			args: Args{
				UnstableFails: 20, // 20%
				UnstableSkips: 20, // 20%
				ThresholdMode: 2,  // Percentage mode
			},
			expectErr: true,
			errMsg:    "\nBuild marked as fail: failure rate (25.00%) exceeded the unstable threshold (20.00%)",
		},
		{
			name: "ExceededUnstablePercentageSkipThreshold",
			results: Results{
				Total:    100,
				Failures: 10, // 10%
				Skipped:  25, // 25%
			},
			args: Args{
				UnstableFails: 20, // 20%
				UnstableSkips: 20, // 20%
				ThresholdMode: 2,  // Percentage mode
			},
			expectErr: true,
			errMsg:    "\nBuild marked as fail: skip rate (25.00%) exceeded the unstable threshold (20.00%)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			switch tc.args.ThresholdMode {
			case 1: // Absolute mode
				err = validateUnstableAbsoluteThresholds(tc.results, tc.args)
			case 2: // Percentage mode
				err = validateUnstablePercentageThresholds(tc.results, tc.args)
			default:
				t.Fatalf("%s: invalid ThresholdMode in test case: %d", tc.name, tc.args.ThresholdMode)
			}

			// Check if error is expected
			if tc.expectErr && err == nil {
				t.Errorf("%s: expected error but got nil", tc.name)
			} else if !tc.expectErr && err != nil {
				t.Errorf("%s: did not expect error but got: %v", tc.name, err)
			}

			// Check error message
			if tc.expectErr && err != nil {
				if diff := cmp.Diff(err.Error(), tc.errMsg); diff != "" {
					t.Errorf("%s: error message mismatch (-want +got):\n%s", tc.name, diff)
				}
			}
		})
	}
}
