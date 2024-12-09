# drone-testng

## Building

Build the plugin binary:

```text
scripts/build.sh
```

Build the plugin image:

```text
docker build -t plugins/testng -f docker/Dockerfile .
```

## Testing

Execute the plugin from your current working directory:
## TestNG: This plugin converts TestNG XML reports (generated using org.testng.reporters.XMLReporter) to JUnit-compatible XML reports, enforcing thresholds and configuration validations to manage build statuses.
```
docker run --rm \
  -e PLUGIN_REPORT_FILENAME_PATTERN="**/target/testng-results.xml" \
  -e PLUGIN_FAILED_FAILS=5 \
  -e PLUGIN_FAILED_SKIPS=3 \
  -e PLUGIN_UNSTABLE_FAILS=10 \
  -e PLUGIN_UNSTABLE_SKIPS=5 \
  -e PLUGIN_THRESHOLD_MODE=1 \
  -e PLUGIN_FAILURE_ON_FAILED_TEST_CONFIG=true \
  -e PLUGIN_PLUGIN_FAIL_IF_NO_RESULTS=true \
  -e PLUGIN_JOB_STATUS="failed" \
  -v $(pwd):$(pwd) \
  plugins/testng
```
## Example Harness Step:
```
- step:
    identifier: testngtojunitconversion
    name: TestNG
    spec:
      image: plugins/testng
      settings:
        report_filename_pattern: "**/target/testng-results.xml"
        failed_fails: 5
        failed_skips: 3
        unstable_fails: 10
        unstable_skips: 5
        threshold_mode: 1
        failure_on_failed_test_config: true
        fail_if_no_results: true
        job_status: "failed"
    timeout: ''
    type: Plugin
```

## Plugin Settings
- `PLUGIN_REPORT_FILENAME_PATTERN`
Description: The file name pattern to locate TestNG XML report files. Supports Ant-style patterns.
Example: **/target/testng-results.xml

- `PLUGIN_FAILED_FAILS`
Description: Maximum number of failed tests before the build is marked as FAILURE.
Example: 5

- `PLUGIN_FAILED_SKIPS`
Description: Maximum number of skipped tests before the build is marked as FAILURE.
Example: 3

- `PLUGIN_UNSTABLE_FAILS`
Description: Maximum number of failed tests before the build is marked as UNSTABLE.
Example: 10

- `PLUGIN_UNSTABLE_SKIPS`
Description: Maximum number of skipped tests before the build is marked as UNSTABLE.
Example: 5

- `PLUGIN_THRESHOLD_MODE`
Description: Determines whether thresholds are based on absolute numbers (1) or percentages (2).
Example: 1 (absolute numbers)

- `PLUGIN_FAILURE_ON_FAILED_TEST_CONFIG`
Description: If true, the build will fail if any configuration method (e.g., @BeforeSuite, @AfterTest) fails.
Example: true

- `PLUGIN_PLUGIN_FAIL_IF_NO_RESULTS`
Description: If true, the build will fail if no TestNG XML report files are found.
Example: true

- `LOG_LEVEL` debug/info Level defines the plugin log level. Set this to debug to see the response from NUnit

- `PLUGIN_JOB_STATUS` if the pipeline status is <+pipeline.status> is failed then it validates the PLUGIN_UNSTABLE_FAILS, PLUGIN_UNSTABLE_SKIPS thresholds.
	
