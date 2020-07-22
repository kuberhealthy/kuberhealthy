const http = require("http");
const https = require("https");
const KHReportingURL = "KH_REPORTING_URL";

/**
 * ReportSuccess reports a success to kuberhealthy.
 * @throws Throws an error if there was an issue sending the report to kuberhealthy.
 */
exports.ReportSuccess = () => {

    let report = newReport([]);

    try {
        sendReport(report);
    } catch (err) {
        // Throw the error upstream.
        let reportErr = new Error("failed to send report: " + err.message);
        // console.error(reportErr.message);
        throw reportErr;
    }
}

/**
 * ReportFailure reports a failure to kuberhealthy.
 * @params {Array} errorMessages - An array of strings containing a list of check errors.
 * @throws Throws an error if there was an issue sending the report to kuberhealthy.
 */
exports.ReportFailure = (errorMessages) => {

    let report = newReport(errorMessages);

    try {
        sendReport(report);
    } catch (err) {
        // Throw the error upstream.
        let reportErr = new Error("failed to send report: " + err.message);
        // console.error(reportErr.message);
        throw reportErr;
    }
}

/**
 * sendReport sends a report to Kuberhealthy based on a check's status. This takes a status 
 * that contains a list of check errors and a boolean value representing the status of the check.
 * @param {Object} statusReport - An object status report for the check. Contains list of string `Errors` and boolean `OK`.
 * @throws Throws an error if there was an issue stringifying the report, retrieving reporting url, or making a POST request.
 */
function sendReport(statusReport) {

    // Convert the status report into a JSON string for the request body.
    let data;
    try {
        data = JSON.stringify(statusReport);
    } catch (err) {
        // Throw an error if there was a problem converting the report to JSON string.
        let jsonErr = new Error("failed to convert status report to json string: " + err.message);
        // console.error(jsonErr.message);
        throw jsonErr;
    }

    // Fetch the kuberhealthy reporting URL.
    let khURL;
    try {
        khURL = getKuberhealthyURL();
    } catch (err) {
        // Throw an error if there was a problem fetching the reporting URL.
        let urlErr = new Error("failed to fetch the kuberhealthy url: " + err.message);
        // console.error(urlErr.message);
        throw urlErr;
    }

    // Check the protocol used for the reporting URL.
    let httpsOn = false;
    if (khURL.protocol.localeCompare("https") == 0) {
        httpsOn = true;
    }

    // For https:
    if (httpsOn) {
        // Create an options object for a https request.
        let opts = {
            hostname: khURL.hostname,
            port: 443,
            path: khURL.pathname,
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Content-Length": data.length,
            },
        };

        // Send a POST via https.
        let req = https.request(opts, (res) => {
            // Throw an error if status code was not OK / 200.
            if (res.statusCode != 200) {
                let reportErr = new Error("got a bad status code from kuberhealthy: " + res.statusCode);
                // console.error(reportErr.message);
                throw reportErr;
            }
        });

        // Throw an error if there was a problem sending the request.
        req.on("error", (error) => {
            let requestErr = new Error("error occurred during POST request to kuberhealthy: " + error);
            // console.error(requestErr);
            throw requestErr;
        });

        // Write status report data to the request.
        req.write(data);
        req.end();
        return;
    }

    // For http:
    // Create an options object for a http request.
    let opts = {
        hostname: khURL.hostname,
        port: 80,
        path: khURL.pathname,
        method: "POST",
        headers: {
            "Content-Type": "application/json",
            "Content-Length": data.length,
        },
    };

    // Send a POST via http.
    let req = http.request(opts, (res) => {
        // Throw an error if status code was not OK / 200.
        if (res.statusCode != 200) {
            let reportErr = new Error("got a bad status code from kuberhealthy: " + res.statusCode);
            // console.error(reportErr.message);
            throw reportErr;
        }
    });

    // Throw an error if there was a problem sending the request.
    req.on("error", (error) => {
        let requestErr = new Error("error occurred during POST request to kuberhealthy: " + error);
        // console.error(requestErr);
        throw requestErr;
    });

    // Write status report data to the request.
    req.write(data);
    req.end();
}

/**
 * getKuberhealthyURL retrieves the kuberhealthy reporting URL from the environment and returns it.
 * @returns {URL} Returns a URL object describing the reporting URL.
 * @throws Throws an error if the reporting URL is blank.
 */
function getKuberhealthyURL() {
    // Get the reporting URL from the environment.
    reportingURLEnv = process.env[KHReportingURL];

    // Throw an error if the URL is empty.
    if (reportingURLEnv.length < 1) {
        let err = new Error("fetched " + KHReportingURL + " environment variable but it was blank");
        // console.error(err.message);
        throw err;
    }

    // Create a URL object from the URL pulled from the environment.
    let reportingURL = new URL(reportingURLEnv);

    return reportingURL;
}

/**
 * newReport creates a new error report to be sent to the kuberhealthy server. If the
 * number of errors supplied is 0, then we assume the status report is OK. If any errors
 * are present in the list, we assume the status is DOWN.
 * @param {Array} errorMessages - A list of strings representing error messages from the check.
 * @returns {StatusReport} Returns an object representing a status report for a check.
 */
function newReport(errorMessages) {
    // Assume that the check failed.
    let ok = false;
    if (errorMessages.length == 0) {
        ok = true;
    }
    return {
        Errors: errorMessages,
        OK: ok,
    };
}