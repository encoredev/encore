---
seotitle: How to debug your application with Delve
seodesc: Learn how to debug your Go backend application using Delve and Encore.
title: Debug with Delve
lang: go
---

Encore makes it easy to debug your application using [Delve](https://github.com/go-delve/delve "Delve").

First, make sure you have `dlv` installed by running (Go 1.16 and later):

```shell
$ go install github.com/go-delve/delve/cmd/dlv@latest
```

You have two debugger options, you can either debug by attaching to a running process or by starting the process in debug mode.

## Debug by starting the process in debug mode
Run your Encore application with `encore run --debug=break`. This will launch your encore application with a headless Delve server, which will pause your application until a debugger is attached.

```shell
$ encore run --debug=break
API Base URL:      http://localhost:4000
Dev Dashboard URL: http://localhost:9400/hello-world-cgu2

API server listening at: 127.0.0.1:2345
```

Now it's time to attach the debugger. The instructions differ depending on how you would like to debug (in your terminal or in your editor). If instructions for your editor aren’t listed below, consult your editor for information on how to attach to a Delve server.

### Terminal debugging
To debug in your terminal, run `dlv attach :2345`. You should see:

```shell
$ dlv connect :2345
Type 'help' for list of commands.
(dlv)
```
How to use Delve’s terminal interface for debugging is out of scope for this guide, but there are great resources available. For a good introduction, see [](https://golang.cafe/blog/golang-debugging-with-delve.html "Debugging with Delve").

### Visual Studio Code
To debug with VS Code you must first add a debug configuration. Press `Run -> Add Configuration`, choose `Go -> Connect to server`. Input `127.0.0.1` as host and `2345` as port. The resulting configuration should look something like this:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Connect to server",
            "type": "go",
            "request": "attach",
            "mode": "remote",
            "remotePath": "${workspaceFolder}",
            "port": 2345,
            "host": "127.0.0.1"
        }
    ]
}
```

Next, open the **Run and Debug** menu in the toolbar on the left, select `Connect to server` (the configuration you just created), and then press the green arrow.

That’s it! You should be able to set breakpoints and have the Encore application pause when they’re hit like you would expect.

### Goland
To debug with Goland, you must create a new Go Remote configuration. Press `Run | Edit Configurations`, click the `+` button, and choose `Go Remote`. Give it a name and hit `OK`. 

Now select the configuration you just created and press the green bug.

That's it. You should be able to set breakpoints and have the Encore application pause when they’re hit like you would expect.

## Debug by attaching to a running process

Run your Encore application with `encore run --debug`. This will cause Encore to print the Process ID to the terminal, which you will use to attach your debugger:

```shell
$ encore run --debug
API Base URL:      http://localhost:4000
Dev Dashboard URL: http://localhost:9400/hello-world-cgu2
Process ID:        51894
1:48PM INF registered endpoint path=/hello/:name service=hello endpoint=Hello
```

(Your process id will differ).

When your Encore application is running, it’s time to attach the debugger. The instructions differ depending on how you would like to debug (in your terminal or in your editor). If instructions for your editor aren’t listed below, consult your editor for information on how to attach a debugger to a running process.

### Terminal debugging
To debug in your terminal, run `dlv attach $PID` (replace `$PID` with your Process ID from the previous step). You should see:

```shell
$ dlv attach 51894
Type 'help' for list of commands.
(dlv)
```

How to use Delve’s terminal interface for debugging is out of scope for this guide, but there are great resources available. For a good introduction, see [](https://golang.cafe/blog/golang-debugging-with-delve.html "Debugging with Delve").

### Visual Studio Code
To debug with VS Code you must first add a debug configuration. Press `Run -> Add Configuration`, choose `Go -> Attach to local process`. In the generated configuration, you should see `"processId": 0` as a field. Replace `0` with the process id from above.

Next, open the **Run and Debug** menu in the toolbar on the left, select Attach to Process (the configuration you just created), and then press the green arrow.

That’s it! You should be able to set breakpoints and have the Encore application pause when they’re hit like you would expect.

### Goland
To debug with Goland, you must first install the `gops` package. Open a terminal and run the following command

```shell
go get -t github.com/google/gops/
```

Then click `Run | Attach to Process`. If a notification window appears, click the `Invoke 'go get gops'` link. Once 
it has completed, click `Run | Attach to Process` again. In the dialog that appears, select the process with the
process ID from above.

That's it. You should be able to set breakpoints and have the Encore application pause when they’re hit like you would expect.

