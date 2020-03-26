### client-js

This is a `JavaScript` client for Kuberhealthy external checks. This client exports functions and methods for sending status
report POST requests to kuberhealthy. This is a `JavaScript` implementation of the go client found [here](../../pkg/checks/external/checkclient/main.go). More documentation on external checks can be found [here](../../docs/EXTERNAL_CHECKS.md).

##### Usage

Download the client into your JavaScript project by navigating to your project directory and downloading the client file:

```shell
cd my-kh-check
curl -O -L https://raw.githubusercontent.com/Comcast/kuberhealthy/master/clients/js/kh-client.js
```

In your project, require the file you just downloaded:

```js
const kh = require("./kh-client");
```

Then you can report check status to Kuberhealthy using `ReportSuccess()` or `ReportFailure()`:

```js
    // Report failure. 
    kh.ReportFailure(["example failure message"]);

    // Report success.
    kh.ReportSuccess();
```

##### Example Use

```js
try {
    kh.ReportSuccess();
} catch (err) {
    console.error("Error when reporting success: " + err.message);
    process.exit(1);
}
process.exit(0);
```

##### Example Check

There is an [example](./example/check.js) check in this directory.