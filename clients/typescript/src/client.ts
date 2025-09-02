// client.ts provides helpers for reporting status back to Kuberhealthy.

interface Status {
  ok: boolean;
  errors: string[];
}

// getEnv retrieves a required environment variable or throws an error.
function getEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`Environment variable ${name} is required`);
  }
  return value;
}

// report sends the given status to the Kuberhealthy reporting URL.
async function report(status: Status): Promise<void> {
  const url = getEnv('KH_REPORTING_URL');
  const uuid = getEnv('KH_RUN_UUID');

  const res = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'kh-run-uuid': uuid,
    },
    body: JSON.stringify(status),
  });

  if (!res.ok) {
    throw new Error(`Failed to report status: ${res.status} ${res.statusText}`);
  }
}

// reportSuccess reports a successful check run.
export async function reportSuccess(): Promise<void> {
  await report({ ok: true, errors: [] });
}

// reportFailure reports a failed check run with error messages.
export async function reportFailure(errors: string[]): Promise<void> {
  await report({ ok: false, errors });
}
