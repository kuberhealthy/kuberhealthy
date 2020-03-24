const fetch = require("node-fetch");
const KHReportingURL = "KH_REPORTING_URL"

exports.ReportSuccess = () => {
    console.log("DEBUG: Reporting SUCCESS");

    let report = newReport([]);

    try {
        sendReport(report);
    } catch (err) {
        let reportErr = new Error("failed to send report: " + err.message);
        console.error(reportErr.message);
        throw reportErr;
    }
}

exports.ReportFailure = (errorMessages) => {
    console.log("DEBUG: Reporting FAILURE");

    let report = newReport(errorMessages);

    try {
        sendReport(report);
    } catch (err) {
        let reportErr = new Error("failed to send report: " + err.message);
        console.error(reportErr.message);
        throw reportErr;
    }
}

function sendReport(statusReport) {
    // console.log("DEBUG: Sending report with error length of: " + statusReport.Errors.length);
    // console.log("DEBUG: Sending report with ok state of: " + statusReport.OK);

    let data;
    try {
        data = JSON.stringify(statusReport);
    } catch (err) {
        let jsonErr = new Error("failed to convert status report to json string: " + err.message);
        console.error(jsonErr.message);
        throw jsonErr;
    }
    console.log(data);

    let url;
    try {
        url = getKuberhealthyURL();
    } catch (err) {
        let urlErr = new Error("failed to fetch the kuberhealthy url: " + err.message);
        console.error(urlErr.message);
        throw urlErr;
    }
    console.log("INFO: Using kuberhealthy reporting URL: " + url);

    
    let resp = fetch(url, {
        "method": "POST",
        "headers": { "Content-Type": "application/json" },
        "body": data
    }).then(res => {
        console.log("Recieved a response from POST request to kuberhealthy: " + res)
        return res.json()
    }).catch(err => {
        let reportErr = new Error("got an error sending POST to kuberhealthy: " + err.message)
        console.error(reportErr.message)
        throw reportErr
    });
    
    let statusCode = resp.status;
    console.log(resp);

    if (statusCode != 200) {
        let reportErr = new Error("got a bad status code from kuberhealthy: " + statusCode);
        console.error(reportErr.message);
        throw reportErr;
    }

    console.log("INFO: Got a good http return status code from kuberhealthy URL: " + statusCode);
}

function getKuberhealthyURL() {
    reportingURL = process.env[KHReportingURL];

    if (reportingURL.length < 1) {
        let err = new Error("fetched " + KHReportingURL + " environment variable but it was blank");
        console.error(err.message);
        throw err;
    }

    return reportingURL;
}

// newReport creates a new error reprot to be sent to the server. If
// the number of errors supplied is 0, then we assume the status report is OK.
// if any errors are present, we assume the status is DOWN.
function newReport(errorMessages) {
    let ok = false;
    if (errorMessages.length == 0) {
        ok = true;
    }
    return {
        Errors: errorMessages,
        OK: ok,
    };
}