package main

import (
    "log"
    "time"

    checkclient "github.com/Comcast/kuberhealthy/pkg/checks/external/checkClient"
)

func main(){
    log.Println("Waiting 30 seconds before reporting success...")
    time.Sleep(time.Second * 30)
    err := checkclient.ReportSuccess()
    if err != nil {
        log.Println("Error reporting success to Kuberhealthy servers:", err)
        return
    }
    log.Println("Successfully reported success to Kuberhealthy servers")
}
