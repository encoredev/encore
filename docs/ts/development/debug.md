---
seotitle: How to debug your TS backend application
seodesc: Learn how to debug your TS backend application using Encore.
title: Debug with your IDE
lang: ts
---

Encore makes it easy to debug your application using your favorite IDE. 

## Enable debugging mode
Next, run your Encore application with `encore run --debug=break`. This will cause Encore to run your app with the `--inspect-brk` flag, which will pause your application until a debugger is attached. Encore will print the URL to the terminal, which you will use to attach your debugger:

```shell
$ encore run --debug=break
  Your API is running at:     http://127.0.0.1:4000
  Development Dashboard URL:  http://localhost:9400/ai-chat-ts-qhwi
  Process ID:                 38965

Debugger listening on ws://127.0.0.1:9229/473dd95f-e71e-4bf2-9eda-6132dd0d6ae3
```

(Your process id and url will differ).

If you don't want the application to pause on startup, you can use `encore run --debug` instead. This will start the application and wait for a debugger to attach, but it won't pause the application until the debugger is attached.

## Attach your debugger
When your Encore application is running, it’s time to attach the debugger. The instructions differ depending on how you would like to debug. If instructions for your editor aren’t listed below, consult your editor for information on how to attach a debugger to a running process.

### Visual Studio Code
To debug with VS Code you must first add a debug configuration. Press `Run -> Add Configuration`, choose `Node.js -> Attach`. The generated config should look something like this:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Attach",
      "port": 9229,
      "request": "attach",
      "skipFiles": [
        "<node_internals>/**"
      ],
      "type": "node"
    }
  ]
}
```

Next, open the **Run and Debug** menu in the toolbar on the left, select Attach (the configuration you just created), and then press the green arrow.

That’s it! You should be able to set breakpoints and have the Encore application pause when they’re hit like you would expect.

## WebStorm
To debug with WebStorm (or any other JetBrains IDE), you must first configure a Node.js Attach configuration. Press `Run -> Edit Configurations`, click the `+` button, and choose `Attach to Node.js/Chrome`. Give it a name and hit `OK`. 
Now select the configuration you just created and press the green bug.

That's it. You should be able to set breakpoints and have the Encore application pause when they’re hit like you would expect.

