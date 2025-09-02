const reportingURL = process.env.KH_REPORTING_URL;
const runUUID = process.env.KH_RUN_UUID;

if (!reportingURL || !runUUID) {
  console.error('KH_REPORTING_URL and KH_RUN_UUID must be set');
  process.exit(1);
}

// report sends a result to Kuberhealthy
async function report(ok, errors) {
  const res = await fetch(reportingURL, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'kh-run-uuid': runUUID,
    },
    body: JSON.stringify({ ok, errors }),
  });

  if (res.ok) {
    return;
  }

  const text = await res.text();
  throw new Error(`Kuberhealthy responded with ${res.status}: ${text}`);
}

async function main() {
  try {
    // Add your check logic here.
    await report(true, []);
    console.log('Reported success to Kuberhealthy');
  } catch (err) {
    console.error('Check logic failed:', err);
    try {
      await report(false, [err.message]);
    } catch (e) {
      console.error('Failed to report failure:', e);
    }
    process.exit(1);
  }
}

main();

module.exports = { report };
