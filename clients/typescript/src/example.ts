// example.ts is a trivial checker that always reports success.
import { reportFailure, reportSuccess } from './client';

async function main(): Promise<void> {
  try {
    // TODO: Add your own health check logic here.
    await reportSuccess();
    console.log('Reported success to Kuberhealthy');
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    try {
      await reportFailure([message]);
    } catch (reportErr) {
      console.error('Failed to report failure:', reportErr);
    }
    process.exit(1);
  }
}

main();
