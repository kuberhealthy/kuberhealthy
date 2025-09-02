use reqwest::blocking::Client;
use serde::Serialize;
use std::{env, error::Error};

#[derive(Serialize)]
struct Report {
    #[serde(rename = "Errors")]
    errors: Vec<String>,
    #[serde(rename = "OK")]
    ok: bool,
}

fn main() -> Result<(), Box<dyn Error>> {
    // Fetch environment variables injected by Kuberhealthy.
    let reporting_url = env::var("KH_REPORTING_URL").expect("KH_REPORTING_URL must be set");
    let run_uuid = env::var("KH_RUN_UUID").expect("KH_RUN_UUID must be set");

    // Placeholder for your own check logic.
    // Pass --fail to simulate a failure.
    let ok = env::args().find(|a| a == "--fail").is_none();

    let report = if ok {
        Report {
            errors: vec![],
            ok: true,
        }
    } else {
        Report {
            errors: vec!["example failure".into()],
            ok: false,
        }
    };

    Client::new()
        .post(&reporting_url)
        .header("kh-run-uuid", run_uuid)
        .json(&report)
        .send()?
        .error_for_status()?;

    Ok(())
}
