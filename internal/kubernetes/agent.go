package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

// Agent is a Kubernetes agent for performing operations that interact with the
// api server
type Agent struct {
	RESTClientGetter genericclioptions.RESTClientGetter
	Clientset        kubernetes.Interface
}

// ListNamespaces simply lists namespaces
func (a *Agent) ListNamespaces() (*v1.NamespaceList, error) {
	return a.Clientset.CoreV1().Namespaces().List(
		context.TODO(),
		metav1.ListOptions{},
	)
}

// GetPodLogs streams real-time logs from a given pod.
func (a *Agent) GetPodLogs(pod *v1.Pod) (string, error) {
	podLogOpts := v1.PodLogOptions{}
	req := a.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "Error: Cannot open log stream.", fmt.Errorf("Cannot open log stream for pod %s", pod.Name)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)

	if err != nil {
		return "Error: Cannot encode Pod logs.", fmt.Errorf("Cannot copy logs from pod %s to buf", pod.Name)
	}
	str := buf.String()

	return str, nil
}
