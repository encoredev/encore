---
title: Debug with Delve
---

Encore makes it easy to debug your application using [Delve](https://github.com/go-delve/delve "Delve").

First, make sure you have `dlv` installed by running (Go 1.16 and later):

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

## Enable debugging mode
Next, run your Encore application with `encore run --debug`. This will cause Encore to print the Process ID to the terminal, which you will use to attach your debugger:

```bash
$ encore run --debug
API Base URL:      http://localhost:4060
Dev Dashboard URL: http://localhost:62709/hello-world-cgu2
Process ID:        51894
1:48PM INF registered endpoint path=/hello/:name service=hello endpoint=Hello
```

(Your process id will differ).

## Attach your debugger
When your Encore application is running, it’s time to attach the debugger. The instructions differ depending on how you would like to debug (in your terminal or in your editor). If instructions for your editor aren’t listed below, consult your editor for information on how to attach a debugger to a running process.

### Terminal debugging
To debug in your terminal, run `dlv attach $PID` (replace `$PID` with your Process ID from the previous step). You should see:

```bash
$ dlv attach 51894
Type 'help' for list of commands.
(dlv)
```

How to use Delve’s terminal interface for debugging is out of scope for this guide, but there are great resources available. For a good introduction, see [](https://golang.cafe/blog/golang-debugging-with-delve.html "Debugging with Delve").

### Visual Studio Code
To debug with VS Code you must first add a debug configuration. Press `Run -> Add Configuration`, choose `Go -> Attach to local process`. In the generated configuration, you should see `"processId": 0` as a field. Replace `0` with the process id from above.

Next, open the **Run and Debug** menu in the toolbar on the left, select Attach to Process (the configuration you just created), and then press the green arrow.

That’s it! You should be able to set breakpoints and have the Encore application pause when they’re hit like you would expect.