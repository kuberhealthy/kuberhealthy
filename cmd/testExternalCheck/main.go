package main

import (
    "log"
    "time"

    checkclient "github.com/Comcast/kuberhealthy/pkg/checks/external/checkClient"
)

func main(){
    time.Sleep(time.Second * 5)
    err := checkclient.ReportSuccess()
    if err != nil {
        log.Println("Error reporting success:", err)
    }
}
