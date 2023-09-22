package main

import (
	"flag"
	"fmt"
	"ksheldon/pkg/clients"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	expect "github.com/Netflix/go-expect"
)

var kubeconfigPath string

const (
	PTPNamespace     = "openshift-ptp"
	PTPPodNamePrefix = "linuxptp-daemon-"
	PTPContainer     = "linuxptp-daemon-container"
	GPSContainer     = "gpsd"
	EOFINT           = 65533
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

var timeout time.Duration = 2 * time.Minute

func main() {
	flag.StringVar(&kubeconfigPath, "k", "", "Path to kubeconfig. Required.")
	flag.Parse()

	if kubeconfigPath == "" {
		log.Println("Kubeconfig path (-k) is Required")
		flag.Usage()
		os.Exit(1)
	}
	log.SetLevel(log.InfoLevel)

	fmt.Println("kubeconfig: ", kubeconfigPath)
	clientset, err := clients.GetClientset(string(kubeconfigPath))
	ifErrPanic(err)
	ctx, err := GetPTPDaemonContext(clientset)
	ifErrPanic(err)

	expecter, err := expect.NewConsole(
		// expect.WithLogger(log.Default()),
		expect.WithDefaultTimeout(timeout),
	)
	ifErrPanic(err)

	sheldon := clients.NewSheldonHandle(expecter.Tty(), expecter.Tty(), expecter.Tty())

	ctx.OpenShell(sheldon)
	log.Info("Waiting for prompt")
	_, err = expecter.ExpectString("sh-4.4#")
	ifErrPanic(err)
	log.Info("ls")
	_, err = expecter.SendLine("ls -ltr")
	ifErrPanic(err)
	y, err := expecter.ExpectString("sh-4.4#")
	ifErrPanic(err)
	fmt.Println(y)
	log.Info("ubxtool")
	_, err = expecter.SendLine("ubxtool -t -p MON-VER -P 29.20")
	ifErrPanic(err)
	x, err := expecter.ExpectString("extension FWVER=TIM")
	ifErrPanic(err)
	fmt.Println(x)
	_, err = expecter.ExpectString("sh-4.4#")
	ifErrPanic(err)
	log.Info("Sending exit")
	_, err = expecter.SendLine("exit")
	ifErrPanic(err)
	expecter.Write([]byte{0})
	log.Info("Waiting for sheldon to close")
	sheldon.Wait()

	log.Info("Closing")
	err = expecter.Close()
	ifErrPanic(err)
}
