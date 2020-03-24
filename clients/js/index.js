const kh = require("./client");
// const k8s = require('@kubernetes/client-node');

// console.log("Building k8s client from config.")
// const kc = new k8s.KubeConfig();
// kc.loadFromDefault();
// console.log("Built k8s client.");

// const k8sApi = kc.makeApiClient(k8s.CoreV1Api);

let fail = false;
fail = process.env["FAILURE"];
if (fail == 'true') {
    fail = true;
}

if (fail) {
    console.log("Reporting failure.");
    try {
        kh.ReportFailure(["example failure message"]);
    } catch (err) {
        console.error("Error when reporting failure: " + err.message);
        process.exit(1);
    }
    process.exit(0);
}

console.log("Reporting success.");
try {
    kh.ReportSuccess();
} catch (err) {
    console.error("Error when reporting success: " + err.message);
    process.exit(1);
}
process.exit(0);
