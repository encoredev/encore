---
seotitle: Encore Telemetry
seodesc: Encore collects telemetry data about app usage
title: Telemetry
lang: ts
---
Telemetry helps us improve the Encore by collecting usage data. This data provides insights into how Encore is used, enabling us to make informed decisions to enhance performance, add new features, and fix bugs more efficiently.

Encore only collects telemetry data in the local development tools and the Encore Cloud dashboard. It does **not** collect any telemetry data from your running applications or cloud services, ensuring complete privacy and security for your operations.

## Why We Collect Data

We collect telemetry data for several important reasons:

1. **Improvement of Features**: Understanding which features are most used helps us prioritize improvements and new feature development.
2. **Performance Monitoring**: Tracking performance metrics enables us to identify and resolve issues, ensuring a smoother user experience.
3. **Bug Detection**: Telemetry data can help us detect and fix bugs faster by providing context on how and when issues occur.
4. **User Experience**: Insights from telemetry data guide us in making Encore more intuitive and user-friendly.

## How Data is Collected

Encore collects data in a way that prioritizes user privacy and security. Here's how we do it:

1. **User Identifiable Data**: The data collected includes identifiable information that helps us understand specific user interactions and contexts.
2. **Types of Data**: We collect data on usage patterns, performance metrics, and error reports.
3. **Secure Transmission**: All data is transmitted securely using industry-standard encryption protocols.
4. **Minimal Impact**: Data collection is designed to have minimal impact on Encore's performance.

### Example of Data Being Sent

Here is an example of the type of data that is sent:

```json
{
    "event": "app.create",
    "anonymousId": "a-uuid-unique-for-the-installation",
    "properties": {
        "error": false,
        "lang": "go",
        "template": "graphql"
    }
}
```

## Data We Don't Collect

At Encore, we prioritize your privacy and ensure that no sensitive data is collected through our telemetry. Specifically, we do not collect:

1. **Environment Variables**: We do not collect any environment variables set in your development or production environments.
2. **File Paths**: The specific paths of your files and directories are not collected.
3. **Contents of Files**: We do not access or collect the contents of your code files or any other files in your projects.
4. **Logs**: No log files from your application or development environment are collected.
5. **Serialized Errors**: We do not collect serialized errors that may contain sensitive information.

Our goal is to gather useful data that helps improve Encore while ensuring that your sensitive information remains private and secure.

## Disabling Telemetry

While telemetry helps us improve Encore, we understand that some users may prefer to opt out. Disabling telemetry is straightforward and can be done in two ways:

1. **Using the CLI Command**: You can disable telemetry by executing a simple command in your terminal.

   ```sh
   encore telemetry disable
   ```

2. **Setting an Environment Variable**: Alternatively, you can disable telemetry by setting the `DISABLE_ENCORE_TELEMETRY` environment variable.

   ```sh
   export DISABLE_ENCORE_TELEMETRY=1
   ```

3. **Confirmation**: After disabling telemetry, either by the CLI command or environment variable, you will receive a confirmation message indicating that telemetry has been successfully disabled.

4. **Re-enabling Telemetry**: If you decide to re-enable telemetry later, you can do so with the following CLI command:

   ```sh
   encore telemetry enable
   ```

## Debugging Telemetry

For users who want more visibility into what telemetry data is being sent, you can enable debug mode:

1. **Setting Debug Mode**: Enable debug mode by setting the `ENCORE_TELEMETRY_DEBUG` environment variable.

   ```sh
   export ENCORE_TELEMETRY_DEBUG=1
   ```

2. **Log Statements**: When debug mode is enabled, a log statement prepended by `[telemetry]` will be printed every time telemetry data is sent.

## Conclusion

Telemetry is a vital tool for improving Encore, but we respect your choice regarding data sharing. With easy-to-use commands and environment variables, you can manage your telemetry settings as you see fit. If you have any further questions or need assistance, please refer to our support documentation or contact our support team.

Thank you for helping us make Encore better!
