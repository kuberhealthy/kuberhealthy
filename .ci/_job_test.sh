#!/bin/bash

# ========== Utility functions ==========
function get_khjob_status_ok {
    kubectl get khjob -n $NS kh-test-job -ojsonpath='{.status.ok}'
}

function get_khstate_ok {
    kubectl get khstate -n $NS kh-test-job -ojsonpath='{.spec.OK}'
}

function job_phase {
    kubectl get khjob -n $NS kh-test-job -ojsonpath='{.spec.phase}'
}

function fail_test {
    # Print debug_information
    echo ---
    kubectl get khjob -n $NS kh-test-job -oyaml
    echo ---
    kubectl get khstate -n $NS kh-test-job -oyaml
    exit 1
}

echo ========== Job E2E test - Job successful case ==========
sed s/REPORT_FAILURE_VALUE/false/ .ci/khjob.yaml |kubectl apply -n $NS -f-

if [ "$(get_khjob_status_ok)" != "" ]; then
    echo "There should not be any OK field initially"; fail_test
fi

if [ "$(job_phase)" != "Running" ]; then
    echo "Job should be in running phase"; fail_test
fi

# Wait until the field is available
TIMEOUT=30
while [ "$(job_phase)" == "Running" ] && [ $TIMEOUT -gt 0 ]; do sleep 1; echo Job phase: $(job_phase), timeout: ${TIMEOUT}; let TIMEOUT-=1; done

# Check the result
if [ "$(get_khjob_status_ok)" != "true" ]; then
    echo "khjob status should have returned OK"; fail_test
fi

if [ "$(get_khstate_ok)" != "true" ]; then
    echo "khstate should have returned OK"; fail_test
fi

if [ "$(job_phase)" != "Completed" ]; then
    echo "Job phase should be Completed."; fail_test
fi

# Delete the job
kubectl delete khjob -n $NS kh-test-job



echo ========== Job E2E test - Job fail case ==========

sed s/REPORT_FAILURE_VALUE/true/ .ci/khjob.yaml |kubectl apply -n $NS -f-

if [ "$(get_khjob_status_ok)" != "" ]; then
    echo "There should not be any OK field initially"; fail_test
fi

if [ "$(job_phase)" != "Running" ]; then
    echo "Job should be in running phase"; fail_test
fi

# Wait until the field is available
TIMEOUT=30
while [ "$(job_phase)" == "Running" ] && [ $TIMEOUT -gt 0 ]; do sleep 1; echo Job phase: $(job_phase), timeout: ${TIMEOUT}; let TIMEOUT-=1; done

# Check the result
if [ "$(get_khjob_status_ok)" != "false" ]; then
    echo "khjob status should have NOT returned OK"; fail_test
fi

if [ "$(get_khstate_ok)" != "false" ]; then
    echo "khstate should have NOT returned OK"; fail_test
fi

if [ "$(job_phase)" != "Completed" ]; then
    echo "Job phase should be Completed."; fail_test
fi

# Delete the job
kubectl delete khjob -n $NS kh-test-job
