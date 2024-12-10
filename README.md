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
## TestNG: This plugin process the TestNG report files and log the Test Reports in the console.
```
docker run --rm \
  -e PLUGIN_REPORT_FILENAME_PATTERN="**/target/testng-results.xml" \
  -e PLUGIN_FAILED_FAILS=5 \
  -e PLUGIN_FAILED_SKIPS=3 \
  -e PLUGIN_THRESHOLD_MODE=1 \
  -e PLUGIN_FAILURE_ON_FAILED_TEST_CONFIG=true \
  -e PLUGIN_PLUGIN_FAIL_IF_NO_RESULTS=true \
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
        threshold_mode: 1
        failure_on_failed_test_config: true
        fail_if_no_results: true
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

- `PLUGIN_THRESHOLD_MODE`: (Optional) Specifies the mode for threshold validation:
  - `absolute`: Validates against absolute values of failures and skips.
  - `percentage`: Validates against percentage values of failures and skips.
  - Default: `absolute`.

- `PLUGIN_FAILURE_ON_FAILED_TEST_CONFIG`
Description: If true, the build will fail if any configuration method (e.g., @BeforeSuite, @AfterTest) fails.
Example: true

- `LOG_LEVEL` debug/info Level defines the plugin log level. Set this to debug to see the response from NUnit
	
