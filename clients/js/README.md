### client-js

This is a `JavaScript` client for Kuberhealthy external checks. This client exports functions for sending status
report POST requests to Kuberhealthy. This is a `JavaScript` implementation of the `Go` client found [here](../../pkg/checks/external/checkclient/main.go). More documentation on external checks can be found [here](../../docs/CHECKS.md).

#### NPM:

The kuberhealthy [NPM package](https://www.npmjs.com/package/kuberhealthy) is conformant with the reference sample syntax but also supports async/await as well as arbitrary host and port. 

- more info: [kuberhealthy-client](https://github.com/gWOLF3/kuberhealthy-client)
- get started: `npm i --save kuberhealthy`

##### Usage

Download the client into your JavaScript project by navigating to your project directory and downloading the client file:

```shell
cd my-kh-check
curl -O -L https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/master/clients/js/kh-client.js
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
