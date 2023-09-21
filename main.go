package main

import (
	"flag"
	"fmt"
	"ksheldon/pkg/clients"
	"log"
	"os"
)

var kubeconfigPath string

const (
	PTPNamespace     = "openshift-ptp"
	PTPPodNamePrefix = "linuxptp-daemon-"
	PTPContainer     = "linuxptp-daemon-container"
	GPSContainer     = "gpsd"
)

func GetPTPDaemonContext(clientset *clients.Clientset) (clients.ContainerContext, error) {
	ctx, err := clients.NewContainerContext(clientset, PTPNamespace, PTPPodNamePrefix, PTPContainer)
	if err != nil {
		return ctx, fmt.Errorf("could not create container context %w", err)
	}
	return ctx, nil
}

func ifErrPanic(err error) {
	if err != nil {
		log.Panic(err.Error())
	}
}

func main() {
	flag.StringVar(&kubeconfigPath, "k", "", "Path to kubeconfig. Required.")
	flag.Parse()

	if kubeconfigPath == "" {
		log.Println("Kubeconfig path (-k) is Required")
		flag.Usage()
		os.Exit(1)
	}

	fmt.Println("kubeconfig: ", kubeconfigPath)
	sheldon := clients.NewSheldonHandle(os.Stdin, os.Stdout, os.Stderr)
	clientset, err := clients.GetClientset(string(kubeconfigPath))
	ifErrPanic(err)
	ctx, err := GetPTPDaemonContext(clientset)
	ifErrPanic(err)
	ctx.OpenShell(sheldon)
	sheldon.Listen()
}
