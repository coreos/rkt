package main

// Program used to orchestrate w/ other apps in the pod

import (
    "syscall"
    "os"
    "os/signal"
)

func main() {
    c := make(chan os.Signal, 1)
    signal.Notify(c, syscall.SIGINT)
    _ = <-c
}