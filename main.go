package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func debugf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func main() {
	log.Print("Spreading rumours.\n")
	defer log.Print("rumours stopped.\n")

	// TODO: env parsing
	secretName := "test-secret"
	secretNamespace := "kube-system"

	done := make(chan bool, 1)

	// create context and defer cancallation
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		debugf("cancelling context..\n")
		cancel()
		// Give the context time to cancel
		time.Sleep(100 * time.Millisecond)
	}()

	// trap signals and run handler
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		debugf("received signal %s", sig)
		done <- true
	}()

	process(ctx, done, secretName, secretNamespace)

	<-done
	close(done)
}
