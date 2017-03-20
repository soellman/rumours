package main

import (
	"context"
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/client-go/rest"
)

type clientCtx struct {
	ctx    context.Context
	done   chan bool
	client *kubernetes.Clientset
}

func process(ctx context.Context, done chan bool, name, namespace string) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Print(err)
		done <- true
		return
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Unable to connect to kubernetes exiting: %v", err)
		done <- true
		return
	}

	c := &clientCtx{
		ctx:    ctx,
		done:   done,
		client: client,
	}

	// namespaceChan carries a kubernetes namespace
	// from the namespaceWatcher to the namespaceUpdater
	// owner: namespaceWatcher
	namespaceChan := make(chan *v1.Namespace)

	// secretChan carries a k8s secret
	// from the secretWatcher to the namespaceUpdater
	// owner: secretWatcher
	secretChan := make(chan *v1.Secret)

	// scanChan carries a bool signal
	// from the namespaceUpdater to the namespaceWatcher
	// owner: namespaceUpdater
	scanChan := make(chan bool)

	go namespaceUpdater(c, scanChan, secretChan, namespaceChan)
	go secretWatcher(c, secretChan, name, namespace)
	go namespaceWatcher(c, scanChan, namespaceChan)
}

func copySecret(secret, dst *v1.Secret, ns *v1.Namespace) {
	dst.SetNamespace(ns.GetName())
	dst.SetName(secret.GetName())
	dst.Data = secret.Data
	dst.Type = secret.Type
}

func syncSecret(c *clientCtx, secret *v1.Secret, ns *v1.Namespace) {
	if secret.GetNamespace() == ns.GetName() {
		debugf("ss: not copying secret to source namespace")
		return
	}
	curr, err := c.client.Secrets(ns.GetName()).Get(secret.GetName())
	// TODO - how can I see if this error just means not found?
	if err != nil {
		debugf("ss: creating secret: %+v", err)
		s := &v1.Secret{}
		copySecret(secret, s, ns)
		_, err = c.client.Secrets(ns.GetName()).Create(s)
	} else {
		// TODO: determine if update is needed
		debugf("ss: updating secret")
		copySecret(secret, curr, ns)
		_, err = c.client.Secrets(ns.GetName()).Update(curr)
	}
	if err != nil {
		debugf("ss: err doing shit to a secret: %+v", err)
	} else {
		debugf("ss: updated secret in %q", ns.GetName())
	}
}

func namespaceUpdater(c *clientCtx, scanOut chan<- bool, secretIn <-chan *v1.Secret, nsIn <-chan *v1.Namespace) {
	log.Print("started namespaceUpdater")
	defer func() {
		close(scanOut)
		log.Print("stopped namespaceUpdater")
	}()

	var secret *v1.Secret

	for {
		select {
		case <-c.ctx.Done():
			return
		case ns := <-nsIn:
			if ns != nil {
				debugf("nsu: update %q", ns.GetName())
				syncSecret(c, secret, ns)
			}
		case s := <-secretIn:
			if s != nil {
				debugf("nsu: using secret %q, rev %q", s.GetName(), s.GetResourceVersion())
				secret = s
				scanOut <- true
			}
		}
	}
}

func secretWatcher(c *clientCtx, secretOut chan<- *v1.Secret, name, namespace string) {
	log.Print("started secretWatcher")
	defer func() {
		close(secretOut)
		log.Print("stopped secretWatcher")
	}()

	// get the secret in question
	// TODO: differentiate between error and not found
	secret, err := c.client.Secrets(namespace).Get(name)
	if err != nil {
		log.Printf("secretWatcher: Unable to connect to kubernetes exiting: %v", err)
		c.done <- true
		return
	}
	if secret == nil {
		debugf("secretWatcher: can't find secret. exiting.")
		c.done <- true
		return
	}
	secretOut <- secret

	for {
		select {
		case <-c.ctx.Done():
			return
		}
	}
}

func namespaceWatcher(c *clientCtx, scanIn <-chan bool, nsOut chan<- *v1.Namespace) {
	log.Print("started namespaceWatcher")

	var watcher watch.Interface
	var watchChan <-chan watch.Event
	var err error

	defer func() {
		close(nsOut)
		if watcher != nil {
			watcher.Stop()
		}
		log.Print("stopped namespaceWatcher")
	}()

	restartWatch := func() {
		if watcher != nil {
			watcher.Stop()
		}
		debugf("nsw: watch started")
		watcher, err = c.client.Namespaces().Watch(v1.ListOptions{})
		if err != nil {
			log.Printf("namespaceWatcher: Unable to connect to kubernetes exiting: %v", err)
			c.done <- true
			return
		}
		watchChan = watcher.ResultChan()
	}

	for {
		select {
		case <-c.ctx.Done():
			return
		case _, ok := <-scanIn:
			if ok {
				debugf("nsw: scan requested")
				restartWatch()
			}
		case event, ok := <-watchChan:
			if !ok {
				debugf("nsw: watch closed")
				restartWatch()
			}
			if event.Type == watch.Added {
				nsOut <- event.Object.(*v1.Namespace)
			}
		}
	}
}
