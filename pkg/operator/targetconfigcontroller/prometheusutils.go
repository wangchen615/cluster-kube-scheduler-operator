package targetconfigcontroller

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	openshiftrouteclientset "github.com/openshift/client-go/route/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	promNamespace          = "openshift-monitoring"
	promRouteName          = "prometheus-k8s"
	promTokenPrefix        = "prometheus-k8s-token"
	promAddressPlaceholder = "PROMETHEUS_ADDRESS"
	promTokenPlaceholder   = "PROMETHEUS_TOKEN"
	templateFileSuffix     = ".template"
)

func getPromInfo(kClient kubernetes.Interface, osrClient openshiftrouteclientset.Interface, promNS string, promRouteName string, promTokenPrefix string) (string, string, error) {
	rclient := osrClient.RouteV1().Routes(promNS)
	promRoute, err := rclient.Get(context.TODO(), promRouteName, metav1.GetOptions{})

	if err != nil {
		return "", "", errors.Wrap(err, "getting Route object failed")
	}

	u := &url.URL{
		Scheme: "http",
		Host:   promRoute.Spec.Host,
		Path:   promRoute.Spec.Path,
	}

	if promRoute.Spec.TLS != nil && promRoute.Spec.TLS.Termination != "" {
		u.Scheme = "https"
	}

	// Add Kubernetes client
	allTokens, err := kClient.CoreV1().Secrets(promNS).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return u.String(), "", errors.Wrap(err, "list secret tokens failed")
	}

	promToken := ""
	for _, token := range allTokens.Items {
		if strings.Contains(token.ObjectMeta.Name, promTokenPrefix) {
			promToken = string(token.Data["token"])
			break
		}
	}

	return u.String(), promToken, nil
}

func updateConfigYaml(yamlFileName string, kClient kubernetes.Interface, osrClient openshiftrouteclientset.Interface) {
	templateFileName := yamlFileName + templateFileSuffix

	promAddress, promToken, err := getPromInfo(kClient, osrClient, promNamespace, promRouteName, promTokenPrefix)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	input, err := ioutil.ReadFile(templateFileName)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	tmp := bytes.Replace(input, []byte(promAddressPlaceholder), []byte(promAddress), -1)
	output := bytes.Replace(tmp, []byte(promTokenPlaceholder), []byte(promToken), -1)

	if err = ioutil.WriteFile(yamlFileName, output, 0666); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
