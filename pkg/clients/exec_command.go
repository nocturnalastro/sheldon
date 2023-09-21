// SPDX-License-Identifier: GPL-2.0-or-later

package clients

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

var NewSPDYExecutor = remotecommand.NewSPDYExecutor

// ContainerContext encapsulates the context in which a command is run; the namespace, pod, and container.
type ContainerContext struct {
	clientset     *Clientset
	namespace     string
	podName       string
	containerName string
	podNamePrefix string
}

func (clientsholder *Clientset) FindPodNameFromPrefix(namespace, prefix string) (string, error) {
	podList, err := clientsholder.K8sClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to getting pod list: %w", err)
	}
	podNames := make([]string, 0)

	for i := range podList.Items {
		hasPrefix := strings.HasPrefix(podList.Items[i].Name, prefix)
		isDebug := strings.HasSuffix(podList.Items[i].Name, "-debug")
		if hasPrefix && !isDebug {
			podNames = append(podNames, podList.Items[i].Name)
		}
	}

	switch len(podNames) {
	case 0:
		return "", fmt.Errorf("no pod with prefix %v found in namespace %v", prefix, namespace)
	case 1:
		return podNames[0], nil
	default:
		return "", fmt.Errorf("too many (%v) pods with prefix %v found in namespace %v", len(podNames), prefix, namespace)
	}
}

func (c *ContainerContext) Refresh() error {
	newPodname, err := c.clientset.FindPodNameFromPrefix(c.namespace, c.podNamePrefix)
	if err != nil {
		return err
	}
	c.podName = newPodname
	return nil
}

func NewContainerContext(
	clientset *Clientset,
	namespace, podNamePrefix, containerName string,
) (ContainerContext, error) {
	podName, err := clientset.FindPodNameFromPrefix(namespace, podNamePrefix)
	if err != nil {
		return ContainerContext{}, err
	}
	ctx := ContainerContext{
		namespace:     namespace,
		podName:       podName,
		containerName: containerName,
		podNamePrefix: podNamePrefix,
		clientset:     clientset,
	}
	return ctx, nil
}

func (c *ContainerContext) GetNamespace() string {
	return c.namespace
}

func (c *ContainerContext) GetPodName() string {
	return c.podName
}

func (c *ContainerContext) GetContainerName() string {
	return c.containerName
}

type SheldonHandle struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	quit   chan os.Signal
	result chan error
	wg     sync.WaitGroup
}

func (sh *SheldonHandle) Listen() {
	fmt.Println("Try typing in to the terminal\n$")
	sh.wg.Wait()
}

func NewSheldonHandle(stdin io.Reader, stdout, stderr io.Writer) *SheldonHandle {
	return &SheldonHandle{
		quit:   make(chan os.Signal),
		result: make(chan error),
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
}

const shellCommand = "/usr/bin/sh"

//nolint:lll // allow slightly long function definition
func (ctx ContainerContext) OpenShell(sheldon *SheldonHandle) {
	log.Debugf(
		"execute command on ns=%s, pod=%s container=%s, cmd: %s",
		ctx.GetNamespace(),
		ctx.GetPodName(),
		ctx.GetContainerName(),
		shellCommand,
	)
	req := ctx.clientset.K8sRestClient.Post().
		Namespace(ctx.GetNamespace()).
		Resource("pods").
		Name(ctx.GetPodName()).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: ctx.GetContainerName(),
			Command:   []string{shellCommand},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	// quit := make(chan os.Signal)
	exec, err := NewSPDYExecutor(ctx.clientset.RestConfig, "POST", req.URL())
	if err != nil {
		log.Debug(err)
		err = fmt.Errorf("error setting up remote command: %w", err)
		// return stdout, stderr, err, quit,

	}

	sheldon.wg.Add(1)
	go func(sheldon *SheldonHandle) {
		defer sheldon.wg.Done()

		err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
			Stdin:  sheldon.Stdin,
			Stdout: sheldon.Stdout,
			Stderr: sheldon.Stderr,
			Tty:    true,
		})
	}(sheldon)
}
