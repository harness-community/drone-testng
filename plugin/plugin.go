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

// Constants for threshold modes
const (
	ThresholdModeAbsolute   = "absolute"
	ThresholdModePercentage = "percentage"
	DefaultThresholdMode    = ThresholdModeAbsolute // Default value
)

// Args represents the plugin's configurable arguments.
type Args struct {
	ReportFilenamePattern     string `envconfig:"PLUGIN_REPORT_FILENAME_PATTERN"`
	FailedFails               int    `envconfig:"PLUGIN_FAILED_FAILS"`
	FailedSkips               int    `envconfig:"PLUGIN_FAILED_SKIPS"`
	FailureOnFailedTestConfig bool   `envconfig:"PLUGIN_FAILURE_ON_FAILED_TEST_CONFIG"`
	ThresholdMode             string `envconfig:"PLUGIN_THRESHOLD_MODE"`
	Level                     string `envconfig:"PLUGIN_LOG_LEVEL"`
}

// ValidateInputs ensures the user inputs meet the plugin requirements.
func ValidateInputs(args Args) error {
	if args.ReportFilenamePattern == "" {
		return errors.New("missing required parameter: ReportFilenamePattern. Please specify the pattern to locate the TestNG report files")
	}

	if args.FailedFails < 0 || args.FailedSkips < 0 {
		return errors.New("threshold values must be non-negative. Check the configured values for failed and skipped tests")
	}

	if args.ThresholdMode == "" {
		args.ThresholdMode = DefaultThresholdMode
		logrus.Infof("PLUGIN_THRESHOLD_MODE not specified. Defaulting to '%s'", DefaultThresholdMode)
	} else if args.ThresholdMode != ThresholdModeAbsolute && args.ThresholdMode != ThresholdModePercentage {
		return errors.New("invalid ThresholdMode value. It must be 'absolute' or 'percentage'. Check the configuration")
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
		return errors.New("no TestNG XML report files found. Check the report file pattern")
	}

	var (
		resultsChan = make(chan Results, len(files))
		errorsChan  = make(chan error, len(files))
	)

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

	for i := 0; i < len(files); i++ {
		select {
		case res := <-resultsChan:
			aggregatedResults.Total += res.Total
			aggregatedResults.Failures += res.Failures
			aggregatedResults.Skipped += res.Skipped
			aggregatedResults.DurationMS += res.DurationMS
		case err := <-errorsChan:
			logrus.Warn(err)
			if e, ok := err.(*os.PathError); ok {
				skippedFiles = append(skippedFiles, e.Path)
			}
		}
	}

	// Log skipped files
	if len(skippedFiles) > 0 {
		logrus.Warnf("Skipped %d files due to errors: %v", len(skippedFiles), skippedFiles)
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

// locateFiles identifies files matching the given pattern and checks read permissions.
func locateFiles(pattern string) ([]string, error) {
	// Use filepath.Glob to find files matching the pattern
	matches, err := filepath.Glob(pattern)
	if err != nil {
		logger := logrus.WithError(err).WithField("Pattern", pattern)
		logger.Error("Error occurred while searching for files")
		return nil, errors.New("failed to search for files: " + err.Error())
	}

	// Log the number of files found
	logrus.Infof("Found %d files matching the pattern: %s", len(matches), pattern)

	if len(matches) == 0 {
		return nil, errors.New("no files found matching the report filename pattern")
	}

	// Check read permissions for each file
	validFiles := []string{}
	for _, file := range matches {
		if fileInfo, err := os.Stat(file); err == nil {
			if fileInfo.Mode().Perm()&(1<<(uint(7))) != 0 {
				validFiles = append(validFiles, file)
			} else {
				logrus.Warnf("File found but not readable: %s", file)
			}
		} else {
			logrus.Warnf("Error accessing file: %s. Error: %v", file, err)
		}
	}

	// Log the number of readable files
	logrus.Infof("Number of readable files: %d", len(validFiles))

	if len(validFiles) == 0 {
		return nil, errors.New("no readable files found matching the report filename pattern")
	}

	return validFiles, nil
}

// processFile reads a TestNG XML report using xml.Decoder for streaming, validates its structure, and logs details.
func processFile(filename string) (Results, error) {
	logrus.Infof("Processing file: %s", filename)

	// Open the file for streaming
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Errorf("File not found: %s", filename)
			return Results{}, fmt.Errorf("file not found: %s", filename)
		}
		if os.IsPermission(err) {
			logrus.Errorf("Permission denied for file: %s", filename)
			return Results{}, fmt.Errorf("permission denied for file: %s", filename)
		}
		logrus.Errorf("Error opening file: %s. Error: %v", filename, err)
		return Results{}, fmt.Errorf("error opening file: %s. Error: %v", filename, err)
	}
	defer file.Close()

	// Use xml.Decoder for streaming
	decoder := xml.NewDecoder(file)
	var report TestNGReport

	if err := decoder.Decode(&report); err != nil {
		logrus.WithError(err).WithField("File", filename).Error("Failed to parse TestNG XML")
		return Results{}, fmt.Errorf("failed to parse TestNG XML for file: %s. Error: %v", filename, err)
	}

	// Validate structure
	if len(report.Suites) == 0 {
		logrus.Infof("File %s contains no test suites in the XML structure", filename)
		return Results{}, fmt.Errorf("no test suites found in the XML structure of file: %s", filename)
	}

	for _, suite := range report.Suites {
		if len(suite.Classes) == 0 {
			logrus.Infof("Suite '%s' in file %s contains no test classes", suite.Name, filename)
		}
	}

	// Log details and return results
	return logTestNGReportDetails(report), nil
}

// logTestNGReportDetails logs the details of a TestNG report and returns the aggregated results.
func logTestNGReportDetails(report TestNGReport) Results {
	results := Results{}
	var failedTests []string
	var skippedTests []string

	// Aggregate data across all suites
	for _, suite := range report.Suites {
		suiteResults, failed, skipped := aggregateSuiteResults(suite)
		results.Total += suiteResults.Total
		results.Failures += suiteResults.Failures
		results.Skipped += suiteResults.Skipped
		results.DurationMS += suiteResults.DurationMS

		failedTests = append(failedTests, failed...)
		skippedTests = append(skippedTests, skipped...)

		// Log suite summary
		logSuiteSummary(suite.Name, suiteResults)
		// Log groups and test details
		logSuiteGroups(suite)
		logSuiteTestDetails(suite)
	}

	// Log aggregated results with failed and skipped test names
	logrus.Infof("\n===============================================")
	if len(failedTests) > 0 {
		logrus.Infof("\nTest case Failures: %s", formatTestNames(failedTests))
	}
	if len(skippedTests) > 0 {
		logrus.Infof("\nTest case Skips: %s", formatTestNames(skippedTests))
	}

	return results
}

// formatTestNames formats test names as a comma-separated string.
func formatTestNames(names []string) string {
	return strings.Join(names, ", ")
}

// aggregateSuiteResults aggregates test results for a suite.
func aggregateSuiteResults(suite Suite) (Results, []string, []string) {
	results := Results{}
	var failedTests []string
	var skippedTests []string

	for _, class := range suite.Classes {
		classResults, failed, skipped := aggregateClassResults(class)
		results.Total += classResults.Total
		results.Failures += classResults.Failures
		results.Skipped += classResults.Skipped
		results.DurationMS += classResults.DurationMS

		failedTests = append(failedTests, failed...)
		skippedTests = append(skippedTests, skipped...)
	}

	return results, failedTests, skippedTests
}

// aggregateClassResults aggregates test results for a class.
func aggregateClassResults(class Class) (Results, []string, []string) {
	results := Results{}
	var failedTests []string
	var skippedTests []string

	for _, test := range class.Tests {
		results.Total++
		if test.Status == "FAIL" {
			results.Failures++
			failedTests = append(failedTests, test.Name)
		} else if test.Status == "SKIP" {
			results.Skipped++
			skippedTests = append(skippedTests, test.Name)
		}

		// Handle invalid or missing DurationMS
		duration, err := strconv.ParseFloat(test.DurationMS, 64)
		if err != nil {
			logrus.Warnf("Invalid or missing DurationMS for test '%s': %v", test.Name, err)
			continue
		}
		results.DurationMS += duration
	}

	return results, failedTests, skippedTests
}

// logSuiteSummary logs a summary for a suite.
func logSuiteSummary(suiteName string, results Results) {
	logrus.Infof("\n===============================================")
	logrus.Infof("\nSuite: %s", suiteName)
	logrus.Infof("\nTotal Tests: %d | Failures: %d | Skips: %d | Duration: %.2f ms",
		results.Total, results.Failures, results.Skipped, results.DurationMS)
	logrus.Infof("\n===============================================")
}

// logSuiteGroups logs group details for a suite.
func logSuiteGroups(suite Suite) {
	logrus.Infof("\nGroups:")
	for _, group := range suite.Groups {
		logrus.Infof("\n- Group: %s", group.Name)
		for _, method := range group.Methods {
			logrus.Infof("\n  - Method: %s | Class: %s | Signature: %s", method.Name, method.ClassName, method.Signature)
		}
	}
}

// logSuiteTestDetails logs test details for a suite.
func logSuiteTestDetails(suite Suite) {
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

// validateThresholds validates test report thresholds based on aggregate results.
func validateThresholds(results Results, args Args) error {

	if args.FailureOnFailedTestConfig && results.Failures > 0 {
		return errors.New("\nbuild marked as failed due to failed configuration methods as FailureOnFailedTestConfig is true")
	}

	switch args.ThresholdMode {
	case ThresholdModeAbsolute: // Absolute thresholds
		if err := validateAbsoluteThresholds(results, args); err != nil {
			return errors.New("\nabsolute threshold validation failed: " + err.Error())
		}

	case ThresholdModePercentage: // Percentage thresholds
		if err := validatePercentageThresholds(results, args); err != nil {
			return errors.New("\npercentage threshold validation failed: " + err.Error())
		}

	default:
		return fmt.Errorf("\ninvalid ThresholdMode: %s, expected 1 (absolute) or 2 (percentage)", args.ThresholdMode)
	}
	return nil
}

// checkThreshold compares actual values against thresholds and returns an error if exceeded.
func checkThreshold(metricName string, actualValue float64, thresholdValue float64, isPercentage bool) error {
	if thresholdValue > 0 && actualValue > thresholdValue {
		if isPercentage {
			return fmt.Errorf("%s rate (%.2f%%) exceeded the threshold (%.2f%%)", metricName, actualValue, thresholdValue)
		}
		return fmt.Errorf("number of %s tests (%d) exceeded the threshold (%d)", metricName, int(actualValue), int(thresholdValue))
	}
	return nil
}

// validateAbsoluteThresholds checks absolute thresholds using the helper function.
func validateAbsoluteThresholds(results Results, args Args) error {
	if err := checkThreshold("failed", float64(results.Failures), float64(args.FailedFails), false); err != nil {
		return err
	}
	if err := checkThreshold("skipped", float64(results.Skipped), float64(args.FailedSkips), false); err != nil {
		return err
	}
	return nil
}

// validatePercentageThresholds checks percentage-based thresholds using the helper function.
func validatePercentageThresholds(results Results, args Args) error {
	totalTests := results.Total
	if totalTests == 0 {
		logrus.Warn("No tests executed; skipping percentage-based threshold validation.")
		return nil // No tests to validate
	}

	failureRate := float64(results.Failures) / float64(totalTests) * 100
	skipRate := float64(results.Skipped) / float64(totalTests) * 100

	if err := checkThreshold("failure", failureRate, float64(args.FailedFails), true); err != nil {
		return err
	}
	if err := checkThreshold("skip", skipRate, float64(args.FailedSkips), true); err != nil {
		return err
	}
	return nil
}
