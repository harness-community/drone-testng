package plugin

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// Args represents the plugin's configurable arguments.
type Args struct {
	ReportFilenamePattern     string `envconfig:"PLUGIN_REPORT_FILENAME_PATTERN"`
	FailedFails               int    `envconfig:"PLUGIN_FAILED_FAILS"`
	FailedSkips               int    `envconfig:"PLUGIN_FAILED_SKIPS"`
	FailureOnFailedTestConfig bool   `envconfig:"PLUGIN_FAILURE_ON_FAILED_TEST_CONFIG"`
	UnstableFails             int    `envconfig:"PLUGIN_UNSTABLE_FAILS"`
	UnstableSkips             int    `envconfig:"PLUGIN_UNSTABLE_SKIPS"`
	JobStatus                 string `envconfig:"PLUGIN_JOB_STATUS"`
	ThresholdMode             int    `envconfig:"PLUGIN_THRESHOLD_MODE"`
	PluginFailIfNoResults     bool   `envconfig:"PLUGIN_FAIL_IF_NO_RESULTS"`
	Level                     string `envconfig:"PLUGIN_LOG_LEVEL"`
}

// ValidateInputs ensures the user inputs meet the plugin requirements.
func ValidateInputs(args Args) error {
	if args.ReportFilenamePattern == "" {
		return errors.New("missing required parameter: ReportFilenamePattern. Please specify the pattern to locate the TestNG report files")
	}
	if args.FailedFails < 0 || args.FailedSkips < 0 || args.UnstableFails < 0 || args.UnstableSkips < 0 {
		return errors.New("threshold values must be non-negative. Check the configured values for failed and skipped tests")
	}
	if args.ThresholdMode != 1 && args.ThresholdMode != 2 {
		return errors.New("invalid ThresholdMode value. It must be 1 (absolute) or 2 (percentage). Check the configuration")
	}
	return nil
}

// Exec handles TestNG XML report processing and logs details.
func Exec(ctx context.Context, args Args) error {
	files, err := locateFiles(args.ReportFilenamePattern)
	if err != nil {
		logger := logrus.WithError(err)
		logger.Error("Error locating files")
		return errors.New("failed to locate files: " + err.Error())
	}

	if len(files) == 0 {
		if args.PluginFailIfNoResults {
			return errors.New("no TestNG XML report files found. Check the report file pattern")
		}
		logrus.Warn("No TestNG XML report files found, continuing execution as PluginFailIfNoResults is false")
		return nil
	}

	var aggregatedResults Results

	for _, file := range files {
		results, err := processFile(file)
		if err != nil {
			logger := logrus.WithField("File", file).WithError(err)
			logger.Error("Error processing file")
			return errors.New("failed to process file: " + err.Error())
		}
		aggregatedResults.Total += results.Total
		aggregatedResults.Failures += results.Failures
		aggregatedResults.Skipped += results.Skipped
		aggregatedResults.DurationMS += results.DurationMS
	}

	// Log aggregated results
	logrus.Infof("\n===============================================")
	logrus.Infof("\nTotal Tests Results: %d | Failures: %d | Skips: %d | Duration: %.2f ms", aggregatedResults.Total, aggregatedResults.Failures, aggregatedResults.Skipped, aggregatedResults.DurationMS)
	logrus.Infof("\n===============================================")

	// Validate thresholds at the aggregate level
	if err := validateThresholds(aggregatedResults, args); err != nil {
		logger := logrus.WithFields(logrus.Fields{
			"Total Tests": aggregatedResults.Total,
			"Failures":    aggregatedResults.Failures,
			"Skipped":     aggregatedResults.Skipped,
			"DurationMS":  aggregatedResults.DurationMS,
		})
		logger.Error(err.Error())
		return err
	}

	return nil
}

// locateFiles identifies files matching the given pattern.
func locateFiles(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		logger := logrus.WithError(err).WithField("Pattern", pattern)
		logger.Error("Error occurred while searching for files")
		return nil, errors.New("failed to search for files: " + err.Error())
	}
	if len(matches) == 0 {
		return nil, errors.New("no files found matching the report filename pattern")
	}
	return matches, nil
}

// processFile reads a TestNG XML report and logs details.
func processFile(filename string) (Results, error) {
	logrus.Infof("Processing file: %s", filename)

	data, err := os.ReadFile(filename)
	if err != nil {
		logger := logrus.WithError(err).WithField("File", filename)
		logger.Error("Failed to read file")
		return Results{}, errors.New("failed to read file: " + err.Error())
	}

	var report TestNGReport
	if err := xml.Unmarshal(data, &report); err != nil {
		logger := logrus.WithError(err).WithField("File", filename)
		logger.Error("Failed to parse TestNG XML")
		return Results{}, errors.New("failed to parse TestNG XML: " + err.Error())
	}

	return logTestNGReportDetails(report), nil
}

// logTestNGReportDetails logs the details of a TestNG report and returns the aggregated results.
func logTestNGReportDetails(report TestNGReport) Results {
	results := Results{}
	totalTests, totalFailures, totalSkipped, totalDuration := 0, 0, 0, 0.0

	// Aggregate data across all suites
	for _, suite := range report.Suites {
		suiteTests, suiteFailures, suiteSkipped, suiteDuration := 0, 0, 0, 0.0

		for _, class := range suite.Classes {
			for _, test := range class.Tests {
				suiteTests++
				if test.Status == "FAIL" {
					suiteFailures++
				} else if test.Status == "SKIP" {
					suiteSkipped++
				}
				duration, _ := strconv.ParseFloat(test.DurationMS, 64)
				suiteDuration += duration
			}
		}

		totalTests += suiteTests
		totalFailures += suiteFailures
		totalSkipped += suiteSkipped
		totalDuration += suiteDuration

		// Log suite summary
		logrus.Infof("\n===============================================")
		logrus.Infof("\nSuite: %s", suite.Name)
		logrus.Infof("\nTotal Tests: %d | Failures: %d | Skips: %d | Duration: %.2f ms", suiteTests, suiteFailures, suiteSkipped, suiteDuration)
		logrus.Infof("\n---------------------------------------------------------------------------")

		// Log groups
		logrus.Infof("\nGroups:")
		for _, group := range suite.Groups {
			logrus.Infof("\n- Group: %s", group.Name)
			for _, method := range group.Methods {
				logrus.Infof("\n  - Method: %s | Class: %s | Signature: %s", method.Name, method.ClassName, method.Signature)
			}
		}

		// Log test details
		logrus.Infof("\nTest Details:")
		for _, class := range suite.Classes {
			for _, test := range class.Tests {
				logrus.Infof("\n- Test: %s | Status: %s | Duration: %s ms", test.Name, test.Status, test.DurationMS)
				if test.Status == "FAIL" && test.Exception != "" {
					logrus.Infof("\n    Exception: %s", test.Exception)
				}
			}
		}
	}

	results.Total = totalTests
	results.Failures = totalFailures
	results.Skipped = totalSkipped
	results.DurationMS = totalDuration
	return results
}

// validateThresholds validates test report thresholds based on aggregate results.
func validateThresholds(results Results, args Args) error {

	if args.FailureOnFailedTestConfig && results.Failures > 0 {
		return errors.New("\nbuild marked as failed due to failed configuration methods as FailureOnFailedTestConfig is true")
	}

	switch args.ThresholdMode {
	case 1: // Absolute thresholds
		if err := validateAbsoluteThresholds(results, args); err != nil {
			return errors.New("\nabsolute threshold validation failed: " + err.Error())
		}

		if strings.ToUpper(args.JobStatus) == "FAILED" {
			if err := validateUnstableAbsoluteThresholds(results, args); err != nil {
				return errors.New("\nfail absolute threshold validation failed: " + err.Error())
			}
		}
	case 2: // Percentage thresholds
		if err := validatePercentageThresholds(results, args); err != nil {
			return errors.New("\npercentage threshold validation failed: " + err.Error())
		}

		if strings.ToUpper(args.JobStatus) == "FAILED" {
			if err := validateUnstablePercentageThresholds(results, args); err != nil {
				return errors.New("\nfail percentage threshold validation failed: " + err.Error())
			}
		}
	default:
		return fmt.Errorf("\ninvalid ThresholdMode: %d, expected 1 (absolute) or 2 (percentage)", args.ThresholdMode)
	}
	return nil
}

// validateAbsoluteThresholds checks absolute thresholds.
func validateAbsoluteThresholds(results Results, args Args) error {
	if args.FailedFails > 0 && results.Failures > args.FailedFails {
		return fmt.Errorf("\nnumber of failed tests (%d) exceeded the failure threshold (%d)", results.Failures, args.FailedFails)
	}
	if args.FailedSkips > 0 && results.Skipped > args.FailedSkips {
		return fmt.Errorf("\nnumber of skipped tests (%d) exceeded the skip threshold (%d)", results.Skipped, args.FailedSkips)
	}
	return nil
}

// validatePercentageThresholds checks percentage-based thresholds.
func validatePercentageThresholds(results Results, args Args) error {
	totalTests := results.Total
	if totalTests == 0 {
		return nil // No tests to validate
	}

	failureRate := float64(results.Failures) / float64(totalTests) * 100
	skipRate := float64(results.Skipped) / float64(totalTests) * 100

	if args.FailedFails > 0 && failureRate > float64(args.FailedFails) {
		return fmt.Errorf("\nfailure rate (%.2f%%) exceeded the threshold (%.2f%%)", failureRate, float64(args.FailedFails))
	}
	if args.FailedSkips > 0 && skipRate > float64(args.FailedSkips) {
		return fmt.Errorf("\nskip rate (%.2f%%) exceeded the threshold (%.2f%%)", skipRate, float64(args.FailedSkips))
	}
	return nil
}

// validateUnstableAbsoluteThresholds checks absolute thresholds for marking the build as unstable.
func validateUnstableAbsoluteThresholds(results Results, args Args) error {
	if args.UnstableFails > 0 && results.Failures > args.UnstableFails {
		return fmt.Errorf("\nBuild marked as fail: number of failed tests (%d) exceeded the unstable threshold (%d)", results.Failures, args.UnstableFails)
	}

	if args.UnstableSkips > 0 && results.Skipped > args.UnstableSkips {
		return fmt.Errorf("\nBuild marked as fail: number of skipped tests (%d) exceeded the unstable threshold (%d)", results.Skipped, args.UnstableSkips)
	}

	return nil
}

// validateUnstablePercentageThresholds checks percentage-based thresholds for marking the build as unstable.
func validateUnstablePercentageThresholds(results Results, args Args) error {
	totalTests := results.Total
	if totalTests == 0 {
		return nil // No tests to validate
	}

	failureRate := float64(results.Failures) / float64(totalTests) * 100
	skipRate := float64(results.Skipped) / float64(totalTests) * 100

	if args.UnstableFails > 0 && failureRate > float64(args.UnstableFails) {
		return fmt.Errorf("\nBuild marked as fail: failure rate (%.2f%%) exceeded the unstable threshold (%.2f%%)", failureRate, float64(args.UnstableFails))
	}

	if args.UnstableSkips > 0 && skipRate > float64(args.UnstableSkips) {
		return fmt.Errorf("\nBuild marked as fail: skip rate (%.2f%%) exceeded the unstable threshold (%.2f%%)", skipRate, float64(args.UnstableSkips))
	}

	return nil
}
