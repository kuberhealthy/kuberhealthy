const kh = require("./kh-client");

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
