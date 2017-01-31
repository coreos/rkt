package main

// Program used to orchestrate w/ other apps in the pod.  Accept a single
// argument to reflect the service name of the first

import (
    "fmt"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    if len(os.Args) != 2 {
        fmt.Printf("ERROR: Expecting the service to start")
        os.Exit(254)
    }

    systemctlCmd := "/usr/bin/systemctl"
    systemctlArgs := []string{systemctlCmd, "start", os.Args[1]}

    pid, err := syscall.ForkExec(systemctlCmd, systemctlArgs, nil); if err != nil {
        fmt.Errorf("ERROR: Unable to start service: ", err)
        os.Exit(254)
    }

    fmt.Printf("Starting %s (%d)\n", os.Args[1], pid)

    c := make(chan os.Signal, 1)
    signal.Notify(c, syscall.SIGINT)
    _ = <-c
}