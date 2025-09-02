# Status Page

You can directly access the current test statuses by accessing the `kuberhealthy.kuberhealthy` HTTP service on port 80. The status page displays server status in the format shown below. The boolean `OK` field can be used to indicate global up/down status, while the `Errors` array will contain a list of all check error descriptions. Granular, per-check information, including how long the check took to run (Run Duration), the last time a check was run, and the Kuberhealthy pod that ran that specific check is available under the `CheckDetails` object.

```json
{
    "OK": true,
    "Errors": [],
    "CheckDetails": {
        "kuberhealthy/daemonset": {
            "OK": true,
            "Errors": [],
            "RunDuration": "22.512278967s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:24:16.7718171Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "9abd3ec0-b82f-44f0-b8a7-fa6709f759cd"
        },
        "kuberhealthy/deployment": {
            "OK": true,
            "Errors": [],
            "RunDuration": "29.142295647s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:26:40.7444659Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "5f0d2765-60c9-47e8-b2c9-8bc6e61727b2"
        },
        "kuberhealthy/dns-status-internal": {
            "OK": true,
            "Errors": [],
            "RunDuration": "2.43940936s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:34:04.8927434Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "c85f95cb-87e2-4ff5-b513-e02b3d25973a"
        },
        "kuberhealthy/pod-restarts": {
            "OK": true,
            "Errors": [],
            "RunDuration": "2.979083775s",
            "Namespace": "kuberhealthy",
            "LastRun": "2019-11-14T23:34:06.1938491Z",
            "AuthoritativePod": "kuberhealthy-67bf8c4686-mbl2j",
            "uuid": "a718b969-421c-47a8-a379-106d234ad9d8"
        }
    },
    "CurrentMaster": "kuberhealthy-7cf79bdc86-m78qr"
}
```

